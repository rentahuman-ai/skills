# Effect Batching Reference

## Quick Reference

| Concept                | Purpose              | Key Functions                            |
| ---------------------- | -------------------- | ---------------------------------------- |
| `Request`              | Typed data operation | `Request.tagged`, `Schema.TaggedRequest` |
| `RequestResolver`      | Batch handler        | `makeBatched`, `fromEffect`, `batchN`    |
| `Effect.request`       | Execute request      | `Effect.request(req, resolver)`          |
| `Request.succeed/fail` | Complete request     | Used in resolver to return results       |
| `SqlResolver`          | Database batching    | `findById`, `grouped`, `ordered`         |

| Configuration                       | Purpose                      |
| ----------------------------------- | ---------------------------- |
| `RequestResolver.batchN(n)`         | Limit batch size             |
| `Effect.withRequestCaching(true)`   | Enable request caching       |
| `Effect.withRequestBatching(false)` | Disable batching             |
| `Layer.setRequestCache(cache)`      | Configure cache TTL/capacity |

## Defining Requests

```typescript
import { Request, Schema } from 'effect';

// Simple tagged request
interface GetUserById extends Request.Request<User, UserNotFound> {
  readonly _tag: 'GetUserById';
  readonly id: number;
}
const GetUserById = Request.tagged<GetUserById>('GetUserById');

// Schema-based request (for RPC)
class GetTodoById extends Schema.TaggedRequest<GetTodoById>()('GetTodoById', {
  failure: GetTodoError,
  success: Todo,
  payload: { id: Schema.Number },
}) {}
```

## RequestResolver.makeBatched

Primary pattern for batching multiple requests into a single operation:

```typescript
import { Effect, Request, RequestResolver } from 'effect';

const getUserByIdResolver = RequestResolver.makeBatched(
  (requests: ReadonlyArray<GetUserById>) =>
    Effect.gen(function* () {
      const ids = requests.map((r) => r.id);

      // Single batch operation
      const users = yield* Effect.tryPromise({
        try: () =>
          fetch('/api/users/batch', {
            method: 'POST',
            body: JSON.stringify({ ids }),
          }).then((r) => r.json()),
        catch: () => new UserNotFound(),
      });

      // Create lookup map for O(1) access
      const userMap = new Map(users.map((u: User) => [u.id, u]));

      // Complete each request with its result
      yield* Effect.forEach(requests, (request) => {
        const user = userMap.get(request.id);
        return user
          ? Request.succeed(request, user)
          : Request.fail(request, new UserNotFound({ id: request.id }));
      });
    })
);

// With context dependencies
const resolver = RequestResolver.makeBatched(
  (requests: ReadonlyArray<GetUser>) =>
    Effect.gen(function* () {
      const db = yield* Database;
      const users = yield* db.queryUsers(requests.map((r) => r.id));
      const map = new Map(users.map((u) => [u.id, u]));
      yield* Effect.forEach(requests, (r) =>
        Request.succeed(r, map.get(r.id) ?? null)
      );
    })
).pipe(RequestResolver.contextFromServices(Database));
```

## Effect.request (DataLoader Pattern)

```typescript
// Create query function binding request to resolver
const getUserById = (id: number): Effect.Effect<User, UserNotFound> =>
  Effect.request(GetUserById({ id }), getUserByIdResolver);

// Automatic batching - concurrent requests become ONE batch call
const program = Effect.gen(function* () {
  const todos = yield* getTodos;

  // All these requests are batched together
  const owners = yield* Effect.forEach(
    todos,
    (todo) => getUserById(todo.ownerId),
    { concurrency: 'unbounded' } // Required for batching
  );

  return { todos, owners };
});

// Effect.all also batches
const [user1, user2, user3] =
  yield * Effect.all([getUserById(1), getUserById(2), getUserById(3)]);
// Only ONE API call executed
```

## Caching

```typescript
import { Effect, Layer, Request, Duration } from 'effect';

// Enable caching per request
const getUserById = (id: number) =>
  Effect.request(GetUserById({ id }), resolver).pipe(
    Effect.withRequestCaching(true)
  );

// Configure cache globally
const cacheLayer = Layer.setRequestCache(
  Request.makeCache({
    capacity: 1000,
    timeToLive: Duration.minutes(30),
  })
);

const program = myProgram.pipe(Effect.provide(cacheLayer));
```

## SqlResolver (Database)

| Resolver   | Use Case                 | Returns                   |
| ---------- | ------------------------ | ------------------------- |
| `findById` | Lookup by ID             | `Option<A>`               |
| `grouped`  | Multiple results per key | `Array<A>`                |
| `ordered`  | Batch inserts            | Single result per request |

```typescript
import { Effect, Schema } from 'effect';
import { SqlResolver, SqlClient } from '@effect/sql';

class User extends Schema.Class<User>('User')({
  id: Schema.Number,
  name: Schema.String,
}) {}

const makeUserService = Effect.gen(function* () {
  const sql = yield* SqlClient.SqlClient;

  // findById - lookup by ID
  const GetById = yield* SqlResolver.findById('GetUserById', {
    Id: Schema.Number,
    Result: User,
    ResultId: (user) => user.id,
    execute: (ids) => sql`SELECT * FROM users WHERE ${sql.in('id', ids)}`,
  });

  // grouped - multiple results per key
  const GetPostsByAuthor = yield* SqlResolver.grouped('GetPostsByAuthor', {
    Request: Schema.Number,
    RequestGroupKey: (authorId) => authorId,
    Result: Post,
    ResultGroupKey: (post) => post.authorId,
    execute: (authorIds) =>
      sql`SELECT * FROM posts WHERE ${sql.in('author_id', authorIds)}`,
  });

  // ordered - batch inserts
  const InsertPerson = yield* SqlResolver.ordered('InsertPerson', {
    Request: Schema.Struct({ name: Schema.String, email: Schema.String }),
    Result: Person,
    execute: (requests) => sql`
      INSERT INTO people ${sql.insert(requests)}
      RETURNING *
    `,
  });

  return {
    getById: GetById.execute,
    getPostsByAuthor: GetPostsByAuthor.execute,
  };
});
```

## Common Patterns

### Nested Resolution (GraphQL-style)

```typescript
const getAuthorsWithBooks = (authorIds: string[]) =>
  Effect.gen(function* () {
    // First batch: fetch all authors
    const authors = yield* Effect.all(authorIds.map(getAuthor), {
      concurrency: 'unbounded',
    });
    // Second batch: fetch all books
    return yield* Effect.all(
      authors
        .filter(Boolean)
        .map((author) =>
          Effect.map(getBooks(author!.id), (books) => ({ ...author, books }))
        ),
      { concurrency: 'unbounded' }
    );
  });
// Only 2 queries regardless of author count
```

### Limit Batch Size

```typescript
const limitedResolver = resolver.pipe(RequestResolver.batchN(50));
```

### Non-Batched Resolver

```typescript
const singleResolver = RequestResolver.fromEffect((_: GetTodos) =>
  Effect.tryPromise({
    try: () => fetch('/api/todos').then((r) => r.json()),
    catch: () => new GetTodosError(),
  })
);
```
