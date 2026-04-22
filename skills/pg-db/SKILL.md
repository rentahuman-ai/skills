---
name: pg-db
description: 'Drizzle ORM patterns for schema design, migrations, queries, tenant/RLS patterns, and Effect SQL integration. Use when working with @platform/database, writing schemas, migrations, or database queries.'
---

# Database Patterns

Drizzle ORM with PostgreSQL, integrated with Effect-TS via `@effect/sql-drizzle`.

## Core Principles

1. **Column helpers over repetition** — Use `idColumn()`, `timestamps()`, `tenantColumns()` for every table
2. **RLS by default** — Every tenant-scoped table gets `.enableRLS()` with org isolation policies
3. **Types from source** — Derive types with `typeof table.$inferSelect`, never redefine
4. **snake_case SQL, camelCase TS** — Drizzle maps automatically via column name strings
5. **Migrations are forward-only** — Generate migrations, never hand-edit after applying

## Schema Package Layout

```
packages/platform/database/
  src/
    schema/
      helpers.ts      Column helper factories (idColumn, timestamps, tenantColumns)
      auth.ts          Better Auth tables (user, session, account, organization, etc.)
    index.ts           Schema + migrate exports
  drizzle/             Generated migration files
  drizzle.config.ts    Drizzle Kit configuration
```

## Column Helpers

Every table spreads these helpers:

```typescript
import {
  idColumn,
  timestamps,
  tenantColumns,
} from '@platform/database/schema/helpers';

export const myTable = pgTable('my_table', {
  name: text('name').notNull(),
  ...idColumn(), // id: uuid PK, default uuidv7()
  ...tenantColumns(), // organizationId, teamId, ownerId with RLS defaults
  ...timestamps(), // createdAt, updatedAt with auto-update
});
```

### idColumn

```typescript
export const idColumn = () => ({
  id: uuid('id')
    .primaryKey()
    .default(sql`uuidv7()`),
});
```

### timestamps

```typescript
export const timestamps = () => ({
  createdAt: timestamp('created_at', { mode: 'date', withTimezone: true })
    .default(sql`now()`)
    .notNull(),
  updatedAt: timestamp('updated_at', { mode: 'date', withTimezone: true })
    .default(sql`now()`)
    .$onUpdate(() => new Date())
    .notNull(),
});
```

### tenantColumns

```typescript
export const tenantColumns = () => ({
  organizationId: uuid('organization_id')
    .notNull()
    .default(sql`app.current_organization_id()`)
    .references(() => organization.id, { onDelete: 'cascade' }),
  teamId: uuid('team_id')
    .default(sql`app.current_team_id()`)
    .references(() => team.id, { onDelete: 'cascade' }),
  ownerId: uuid('owner_id')
    .notNull()
    .default(sql`app.current_user_id()`)
    .references(() => user.id, { onDelete: 'cascade' }),
});
```

Defaults use SQL context functions set by RLS middleware at request time.

## Enum Patterns

Use `pgEnum`, not TypeScript enums:

```typescript
export const sandboxStatus = pgEnum('sandbox_status', [
  'pending',
  'running',
  'stopped',
  'error',
]);

// Derive TS type from the enum
export type SandboxStatus = (typeof sandboxStatus.$enumValues)[number];
```

## Table Definition

Complete example:

```typescript
export const apiKey = pgTable(
  'api_key',
  {
    name: text('name').notNull().default('My API Key'),
    keyHash: text('key_hash').notNull(),
    redactedKey: text('redacted_key').notNull(),
    expiresAt: timestamp('expires_at', { mode: 'date', withTimezone: true }),
    lastUsedAt: timestamp('last_used_at', { mode: 'date', withTimezone: true }),
    ...idColumn(),
    ...tenantColumns(),
    ...timestamps(),
  },
  (table) => [
    uniqueIndex('api_key_hash_idx').on(table.keyHash),
    index('api_key_organizationId_idx').on(table.organizationId),
    index('api_key_ownerId_idx').on(table.ownerId),
    pgPolicy('api_key_org_isolation', {
      as: 'permissive',
      for: 'all',
      to: appUser,
      using: sql`organization_id = app.current_organization_id()
        AND (app.current_role() IN ('admin', 'owner')
             OR owner_id = app.current_user_id())`,
      withCheck: sql`organization_id = app.current_organization_id()
        AND (app.current_role() IN ('admin', 'owner')
             OR owner_id = app.current_user_id())`,
    }),
  ]
).enableRLS();
```

## RLS Policies

Two tiers:

### Simple Org Isolation

Single policy for all operations:

```typescript
pgPolicy('table_org_isolation', {
  as: 'permissive',
  for: 'all',
  to: appUser,
  using: sql`organization_id = app.current_organization_id()
    AND (app.current_role() IN ('admin', 'owner')
         OR owner_id = app.current_user_id())`,
  withCheck: sql`...same...`,
});
```

### Visibility-Based Access

Factory for tables with `visibility` column:

```typescript
const visibilityRlsPolicies = (tableName: string) => [
  pgPolicy(`${tableName}_select`, {
    for: 'select', to: appUser,
    using: sql`organization_id = app.current_organization_id()
      AND (app.current_role() IN ('admin', 'owner')
           OR owner_id = app.current_user_id()
           OR visibility = 'organization')`,
  }),
  pgPolicy(`${tableName}_insert`, { ... }),
  pgPolicy(`${tableName}_update`, { ... }),
  pgPolicy(`${tableName}_delete`, { ... }),
];
```

### Grant-Based Access

For fine-grained sharing via `resource_grant` table:

```typescript
using: sql`... OR EXISTS (
  SELECT 1 FROM resource_grant rg
  WHERE rg.resource_type = 'my_table'
    AND rg.resource_id = my_table.id
    AND rg.organization_id = app.current_organization_id()
    AND (
      (rg.grantee_type = 'user' AND rg.grantee_id = app.current_user_id())
      OR (rg.grantee_type = 'team' AND rg.grantee_id IN (
        SELECT tm.team_id FROM team_member tm
        WHERE tm.user_id = app.current_user_id()
      ))
    )
)`;
```

## SQL Context Functions

These are set per-request by the RLS middleware:

| Function                                           | Purpose                              |
| -------------------------------------------------- | ------------------------------------ |
| `app.current_organization_id()`                    | Active org for the request           |
| `app.current_user_id()`                            | Authenticated user ID                |
| `app.current_team_id()`                            | Active team (nullable)               |
| `app.current_role()`                               | User's org role (member/admin/owner) |
| `app.set_rls_context(userId, orgId, teamId, role)` | Called by middleware to set context  |

## Migration Workflow

```bash
bun run db:migrate     # Apply pending migrations (available from repo root)
bun run db:reset       # Reset database (available from repo root)

# Run from packages/platform/database/ (or use --filter):
bun run --filter=@platform/database db:generate   # Generate migration from schema changes
bun run --filter=@platform/database db:pull       # Pull remote schema into local
bun run --filter=@platform/database db:seed       # Seed development data
bun run --filter=@platform/database db:studio     # Open Drizzle Studio GUI
bun run --filter=@platform/database db:create     # Create database
bun run --filter=@platform/database db:drop       # Drop database
```

Bootstrap migration (`0000_app_setup.sql`) creates the `app` schema, context functions, and `app_user` role.

## Query Patterns

See the [domain-effect skill's sql.md](../domain-effect/references/sql.md) for raw `@effect/sql` patterns. Repos use Drizzle with Effect:

```typescript
// Repo pattern — returns Option for nullable results
const find = Effect.fn('MyRepo.find')(function* (id: string) {
  const rows = yield* db
    .select()
    .from(myTable)
    .where(eq(myTable.id, id))
    .limit(1);
  return rows.length > 0 ? Option.some(rows[0]) : Option.none();
});

// Cursor-based pagination
const list = Effect.fn('MyRepo.list')(function* (params: PaginationInput) {
  const { conditions, limit } = paginatedQuery(myTable.id, params);
  const rows = yield* db
    .select()
    .from(myTable)
    .where(and(...conditions))
    .orderBy(desc(myTable.id))
    .limit(limit + 1);
  return new PaginatedResult(rows.slice(0, limit), rows.length > limit);
});
```

## Anti-Patterns

- Don't use TypeScript `enum` — use `pgEnum` + derived union type
- Don't hand-write migration SQL after it's been applied
- Don't skip `.enableRLS()` on tenant-scoped tables
- Don't use `text()` for IDs — use `uuid()` with `uuidv7()`
- Don't add `onDelete: 'cascade'` without understanding the cascade chain
- Don't query without RLS context in API handlers — always pipe through `requireRls()`

## References

| Reference                                           | Use when                         |
| --------------------------------------------------- | -------------------------------- |
| [schema-design.md](references/schema-design.md)     | Creating or modifying tables     |
| [migrations.md](references/migrations.md)           | Running or creating migrations   |
| [queries.md](references/queries.md)                 | Writing Drizzle + Effect queries |
| [tenant-patterns.md](references/tenant-patterns.md) | Adding RLS or multi-tenancy      |
