# Schema Design

## Column Helpers

Located in `packages/platform/database/src/schema/helpers.ts`. Spread into every table:

```typescript
export const myTable = pgTable('my_table', {
  name: text('name').notNull(),
  ...idColumn(),
  ...tenantColumns(),
  ...timestamps(),
});
```

### idColumn()

UUID v7 primary key (time-sortable):

```typescript
export const idColumn = () => ({
  id: uuid('id')
    .primaryKey()
    .default(sql`uuidv7()`),
});
```

### timestamps()

Auto-managed created/updated timestamps:

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

`$onUpdate` runs in the ORM layer, not SQL — ensures TS-side consistency.

### tenantColumns()

Multi-tenant ownership with RLS-aware defaults:

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

SQL defaults are set by RLS context at request time — inserts don't need to pass these values explicitly.

## Column Types

| Use case    | Drizzle type                                            | Example                                                |
| ----------- | ------------------------------------------------------- | ------------------------------------------------------ |
| Primary key | `uuid('id')`                                            | `.primaryKey().default(sql\`uuidv7()\`)`               |
| Foreign key | `uuid('ref_id')`                                        | `.references(() => other.id, { onDelete: 'cascade' })` |
| Text        | `text('name')`                                          | `.notNull().default('...')`                            |
| Enum        | `sandboxStatus('status')`                               | Use pgEnum, not string                                 |
| Timestamp   | `timestamp('at', { mode: 'date', withTimezone: true })` | Always use `withTimezone`                              |
| JSON        | `jsonb('data')`                                         | `.$type<MyType>()` for TS typing                       |
| Boolean     | `boolean('active')`                                     | `.notNull().default(true)`                             |

## Enums

```typescript
export const sandboxStatus = pgEnum('sandbox_status', [
  'pending', 'running', 'stopped', 'error'
]);

// Derive union type
export type SandboxStatus = (typeof sandboxStatus.$enumValues)[number];

// Use in table
status: sandboxStatus('status').notNull().default('pending'),
```

## Indexes

```typescript
(table) => [
  uniqueIndex('table_unique_field_idx').on(table.field),
  index('table_orgId_idx').on(table.organizationId),
  index('table_ownerId_idx').on(table.ownerId),
];
```

Always index: foreign keys, frequently filtered columns, unique constraints.

## Foreign Keys

```typescript
.references(() => otherTable.id, { onDelete: 'cascade' })
```

Use `cascade` for child records that shouldn't survive parent deletion. Use `set null` or `restrict` when orphans are acceptable or deletion should be blocked.

## Type Derivation

```typescript
// Select type (what you read from DB)
type ApiKey = typeof apiKey.$inferSelect;

// Insert type (what you write to DB)
type NewApiKey = typeof apiKey.$inferInsert;
```

Never manually define types that match table shapes — always derive.

## JSONB Typed Columns

```typescript
metadata: jsonb('metadata').$type<{
  source: string;
  tags: string[];
}>(),
```

The `$type<T>()` gives TypeScript typing without runtime validation.
