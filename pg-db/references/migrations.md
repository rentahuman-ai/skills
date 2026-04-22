# Migrations

Drizzle Kit manages all database migrations in `packages/platform/database`.

## Commands

```bash
# Available from repo root:
bun run db:migrate     # Apply pending migrations to database
bun run db:reset       # Reset database

# Run from packages/platform/database/ (or use --filter):
bun run --filter=@platform/database db:generate   # Generate SQL migration from schema diff
bun run --filter=@platform/database db:pull       # Introspect remote DB and update schema
bun run --filter=@platform/database db:seed       # Run seed script for development data
bun run --filter=@platform/database db:studio     # Open Drizzle Studio (visual DB browser)
bun run --filter=@platform/database db:create     # Create the database
bun run --filter=@platform/database db:drop       # Drop the database
```

## Workflow: Adding a Table

1. Define the table in a schema file under `src/schema/` (e.g. `grants.ts`, `integrations.ts`)
2. Run `bun run --filter=@platform/database db:generate` — creates a timestamped SQL file in `drizzle/`
3. Review the generated SQL
4. Run `bun run db:migrate` — applies to your local DB
5. Commit the schema change AND the migration file together

## Workflow: Modifying a Table

1. Edit the table definition in the schema file
2. Run `bun run --filter=@platform/database db:generate` — Drizzle Kit diffs against the last migration
3. Review carefully — check for data loss (dropping columns, changing types)
4. Run `bun run db:migrate`
5. Commit both files

## Bootstrap Migration

`0000_app_setup.sql` creates foundational infrastructure:

- `app` schema with context functions (`current_organization_id`, `current_user_id`, etc.)
- `set_rls_context()` stored procedure
- `app_user` role for RLS policies
- Statement breakpoints between operations

## Migration File Conventions

- Files are in `drizzle/` directory
- Named with timestamp prefix: `0001_cool_migration.sql`
- Contain `-- statement-breakpoint` between SQL statements
- Never hand-edit after they've been applied to any environment
- Forward-only — no rollback files (fix forward with a new migration)

## Drizzle Config

```typescript
// drizzle.config.ts
export default defineConfig({
  schema: ['./src/schema/index.ts'],
  out: './drizzle',
  dialect: 'postgresql',
  dbCredentials: {
    url: process.env.DATABASE_URL!,
  },
});
```

## Safe Schema Changes

| Change                            | Safe?   | Notes                                                  |
| --------------------------------- | ------- | ------------------------------------------------------ |
| Add column (nullable)             | Yes     | No data migration needed                               |
| Add column (NOT NULL + default)   | Yes     | Postgres backfills default                             |
| Add column (NOT NULL, no default) | No      | Fails if rows exist                                    |
| Drop column                       | Careful | Data loss — ensure no code references                  |
| Rename column                     | No      | Breaks queries — add new, migrate data, drop old       |
| Change column type                | Careful | May need explicit USING cast                           |
| Add index                         | Yes     | `CREATE INDEX CONCURRENTLY` preferred for large tables |
| Add RLS policy                    | Yes     | Won't affect existing queries                          |
| Drop table                        | No      | Irreversible data loss                                 |
