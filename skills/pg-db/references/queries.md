# Query Patterns

Drizzle queries within Effect-TS services. For raw `@effect/sql` patterns, see the [effect skill's sql.md](../../domain-effect/references/sql.md).

## Repo Structure

Each domain module has a `Repo.ts` that extends `Effect.Service`:

```typescript
export class SandboxesRepo extends Effect.Service<SandboxesRepo>()(
  '@api/SandboxesRepo',
  {
    effect: Effect.gen(function* () {
      const db = yield* Drizzle;

      const find = Effect.fn('SandboxesRepo.find')(function* (id: string) {
        const rows = yield* db
          .select()
          .from(sandbox)
          .where(eq(sandbox.id, id))
          .limit(1);
        return rows.length > 0 ? Option.some(rows[0]) : Option.none();
      });

      const insert = Effect.fn('SandboxesRepo.insert')(function* (data) {
        const [row] = yield* db.insert(sandbox).values(data).returning();
        return row;
      });

      return { find, insert, list, update, remove } as const;
    }),
  }
) {}
```

## Common Query Patterns

### Find by ID (returns Option)

```typescript
const find = Effect.fn('Repo.find')(function* (id: string) {
  const rows = yield* db
    .select()
    .from(myTable)
    .where(eq(myTable.id, id))
    .limit(1);
  return rows.length > 0 ? Option.some(rows[0]) : Option.none();
});
```

### Insert with Returning

```typescript
const insert = Effect.fn('Repo.insert')(function* (data: NewRow) {
  const [row] = yield* db.insert(myTable).values(data).returning();
  return row;
});
```

### Upsert (Insert or Update)

```typescript
const upsert = Effect.fn('Repo.upsert')(function* (data: NewRow) {
  const [row] = yield* db
    .insert(myTable)
    .values(data)
    .onConflictDoUpdate({
      target: myTable.id,
      set: { name: data.name, updatedAt: new Date() },
    })
    .returning();
  return row;
});
```

### Cursor-Based Pagination

```typescript
const list = Effect.fn('Repo.list')(function* (params: PaginationInput) {
  const { conditions, limit } = paginatedQuery(myTable.id, params);
  const rows = yield* db
    .select()
    .from(myTable)
    .where(and(...conditions))
    .orderBy(desc(myTable.id))
    .limit(limit + 1); // fetch one extra to detect hasMore
  return new PaginatedResult(rows.slice(0, limit), rows.length > limit);
});
```

### Update by ID

```typescript
const update = Effect.fn('Repo.update')(function* (
  id: string,
  data: Partial<Row>
) {
  const [row] = yield* db
    .update(myTable)
    .set(data)
    .where(eq(myTable.id, id))
    .returning();
  return row;
});
```

### Delete

```typescript
const remove = Effect.fn('Repo.remove')(function* (id: string) {
  yield* db.delete(myTable).where(eq(myTable.id, id));
});
```

### Filtered Queries

```typescript
const findByOrg = Effect.fn('Repo.findByOrg')(function* (orgId: string) {
  return yield* db
    .select()
    .from(myTable)
    .where(eq(myTable.organizationId, orgId));
});
```

## SQL Error Handling

Use `catchDbViolation` from `@comcom/api-core/Lib/Errors`:

```typescript
import { catchDbViolation } from '@comcom/api-core/Lib/Errors';

repo.insert(data).pipe(
  catchDbViolation({
    onUnique: () => new DuplicateError({ field: 'email' }),
    onForeignKey: () => new NotFoundError({ entity: 'organization' }),
  })
);
```

## Drizzle Operators

```typescript
import {
  eq,
  and,
  or,
  lt,
  gt,
  gte,
  lte,
  ne,
  like,
  ilike,
  isNull,
  isNotNull,
  inArray,
  desc,
  asc,
  sql,
} from 'drizzle-orm';
```

## Anti-Patterns

- Don't use raw SQL strings — use Drizzle query builder
- Don't fetch all columns when you only need a few — use `.select({ id: myTable.id })`
- Don't skip pagination on list endpoints — always use cursor-based pagination
- Don't return raw rows from services — wrap in domain Schema classes
