# Tenant Patterns

Multi-tenancy via PostgreSQL Row-Level Security (RLS).

## Architecture

```
Request → Auth middleware → requireRls() → set_rls_context() → SQL executes with RLS
```

1. Auth middleware extracts user/org/team from session
2. `requireRls()` wraps the Effect in an RLS-scoped transaction
3. `set_rls_context(userId, orgId, teamId, role)` is called as a SQL procedure
4. All subsequent queries in that transaction are filtered by RLS policies

## SQL Context Functions

Defined in `app` schema (created by bootstrap migration):

| Function                        | Returns                                     |
| ------------------------------- | ------------------------------------------- |
| `app.current_organization_id()` | UUID of the active organization             |
| `app.current_user_id()`         | UUID of the authenticated user              |
| `app.current_team_id()`         | UUID of the active team (nullable)          |
| `app.current_role()`            | User's role in the org (member/admin/owner) |

### Setting Context

```sql
CALL app.set_rls_context(
  p_user_id := '...',
  p_org_id := '...',
  p_team_id := '...',
  p_role := 'member'
);
```

In Effect code, this is handled by `withRlsContext()` in `apps/api/src/Infrastructure/Rls.ts`.

## tenantColumns() Defaults

Column defaults reference the context functions:

```typescript
organizationId: uuid('organization_id').default(
  sql`app.current_organization_id()`
);
```

This means inserts within an RLS transaction auto-fill tenant columns — no need to pass them explicitly from application code.

## RLS Policy Tiers

### Tier 1: Simple Org Isolation

For tables where every row belongs to one org and one owner:

```typescript
pgPolicy('table_org_isolation', {
  as: 'permissive',
  for: 'all',
  to: appUser,
  using: sql`organization_id = app.current_organization_id()
    AND (app.current_role() IN ('admin', 'owner')
         OR owner_id = app.current_user_id())`,
  withCheck: sql`organization_id = app.current_organization_id()
    AND (app.current_role() IN ('admin', 'owner')
         OR owner_id = app.current_user_id())`,
});
```

Admins/owners see everything in their org. Members see only their own rows.

### Tier 2: Visibility-Based

For tables with a `visibility` column (`'private' | 'organization'`):

```typescript
const visibilityRlsPolicies = (tableName: string) => [
  pgPolicy(`${tableName}_select`, {
    for: 'select',
    to: appUser,
    using: sql`organization_id = app.current_organization_id()
      AND (app.current_role() IN ('admin', 'owner')
           OR owner_id = app.current_user_id()
           OR visibility = 'organization')`,
  }),
  pgPolicy(`${tableName}_insert`, {
    for: 'insert',
    to: appUser,
    withCheck: sql`organization_id = app.current_organization_id()
      AND (app.current_role() IN ('admin', 'owner')
           OR (owner_id = app.current_user_id() AND visibility = 'private'))`,
  }),
  // update and delete policies follow same pattern
];
```

Members can read org-visible rows but only create private rows.

### Tier 3: Grant-Based

For fine-grained sharing via `resource_grant` table:

```typescript
using: sql`...
  OR EXISTS (
    SELECT 1 FROM resource_grant rg
    WHERE rg.resource_type = 'my_resource'
      AND rg.resource_id = my_table.id
      AND rg.organization_id = app.current_organization_id()
      AND (
        (rg.grantee_type = 'user' AND rg.grantee_id = app.current_user_id())
        OR (rg.grantee_type = 'team' AND rg.grantee_id IN (
          SELECT tm.team_id FROM team_member tm
          WHERE tm.user_id = app.current_user_id()
        ))
        OR (rg.grantee_type = 'organization'
            AND rg.grantee_id = app.current_organization_id())
      )
  )`;
```

Grantee types: `user`, `team`, `organization`.

## Enabling RLS

Every tenant-scoped table must call `.enableRLS()`:

```typescript
export const myTable = pgTable('my_table', {
  ...idColumn(),
  ...tenantColumns(),
  ...timestamps(),
}, (table) => [
  pgPolicy('my_table_org_isolation', { ... }),
]).enableRLS();
```

## appUser Role

Policies target `appUser` — a PostgreSQL role created in the bootstrap migration:

```typescript
const appUser = pgRole('app_user').existing();
```

The application connects as `app_user` after setting the RLS context.

## Effect Integration

```typescript
// In Http.ts handlers — always pipe through requireRls()
.handle('create', ({ payload }) =>
  service.create(payload).pipe(
    requireRls(),
    requirePolicy(policy.canCreate()),
  )
)
```

`requireRls()` reads `AuthContext`, calls `set_rls_context`, then runs the effect within the scoped transaction.

## Anti-Patterns

- Don't forget `.enableRLS()` — without it, policies are defined but not enforced
- Don't use `for: 'all'` with different `using`/`withCheck` — split into per-operation policies
- Don't bypass RLS in application code — use `withSystemActor` only for system-level operations
- Don't hardcode org/user IDs in policies — always use `app.current_*()` functions
