---
name: effect-ts
description: 'Effect-TS patterns for building type-safe applications. Use when working with Effect services, error handling, streams, concurrency, caching, batching, configuration, scheduling, or resource management.'
---

# Effect-TS Rules & Conventions

**Goal: Build production-quality, type-safe Effect modules with explicit dependencies, tracked errors, and managed resources.**

## Core Principles

1. **Errors are values** — Track all expected errors in the type system with `Schema.TaggedError`
2. **Dependencies are explicit** — Use Services and Layers, never global singletons
3. **Resources are managed** — Use `acquireRelease` for anything that needs cleanup
4. **Effects are descriptions** — They don't run until explicitly executed at the boundary

## Fundamental Patterns

### Use `Effect.gen` for Multi-Step Logic

```typescript
/** Loads config and finds user by ID. */
const program = Effect.gen(function* () {
  const config = yield* ConfigService;
  const user = yield* UserRepo.find(id);
  yield* Effect.log(`Found: ${user.name}`);
  return user;
});

// OK: Pipe for simple single transforms
const doubled = Effect.map(number, (n) => n * 2);
```

### Use `Effect.fn` for Traced Service Methods

`Effect.fn` is the default pattern for service and repository methods. It automatically creates traced spans and provides stack traces:

```typescript
/**
 * Creates a bookmark after verifying URL uniqueness.
 *
 * @param params - URL and optional title.
 * @returns The created bookmark.
 * @errors BookmarkUrlConflictError if URL already exists.
 */
const create = Effect.fn('Bookmarks.create')(function* (
  input: BookmarkCreateInput
) {
  yield* Effect.annotateCurrentSpan('url', input.url);
  const existing = yield* repo.findByUrl(input.url);
  if (Option.isSome(existing)) {
    return yield* new BookmarkUrlConflictError({ url: input.url });
  }
  return yield* repo.create({ url: input.url, title: input.title ?? null });
});

// OK: Simple pipe for single-step read-through methods
const get = (id: BookmarkId) => repo.find(id);
```

**Explicit return types** — use `Effect.fn.Return<T>` for complex methods or public API surfaces:

```typescript
/** @returns The created bookmark entity. */
const create = Effect.fn('Bookmarks.create')(function* (
  input: BookmarkCreateInput
): Effect.fn.Return<Bookmark> {
  // ...
  return new Bookmark(row);
});
```

**Pipeline access** — second argument receives the effect and original arguments:

```typescript
/** Creates a bookmark with URL-annotated logs. */
const create = Effect.fn('Bookmarks.create')(
  function* (input: BookmarkCreateInput) {
    return yield* repo.create({ url: input.url, title: input.title ?? null });
  },
  (effect, input) => Effect.annotateLogs(effect, { url: input.url })
);
```

### Use `Effect.Service` for Application Services

```typescript
/**
 * Manages user persistence.
 * @errors DatabaseError
 */
class UserRepository extends Effect.Service<UserRepository>()(
  'UserRepository',
  {
    dependencies: [Database.Default],
    effect: Effect.gen(function* () {
      const db = yield* Database;
      return {
        find: Effect.fn('UserRepository.find')(function* (id: string) {
          yield* Effect.annotateCurrentSpan('userId', id);
          return yield* db.query('...', [id]);
        }),
      };
    }),
  }
) {}

// Use: yield* UserRepository (service) or UserRepository.Default (layer)
```

**Use `scoped:` for services with lifecycle resources** — when a service owns connections, background fibers, queues, or anything requiring cleanup:

```typescript
class Worker extends Effect.Service<Worker>()('Worker', {
  dependencies: [JobQueue.Default],
  scoped: Effect.gen(function* () {
    const queue = yield* JobQueue;
    const fiber = yield* Effect.fork(pollLoop);
    yield* Effect.addFinalizer(() =>
      Effect.gen(function* () {
        yield* Fiber.interrupt(fiber);
        yield* Effect.log('Worker shut down');
      })
    );
    return {
      /* methods */
    };
  }),
}) {}
```

**Rule of thumb:** If your service constructor uses `Effect.addFinalizer`, `Effect.acquireRelease`, `Effect.fork`, or `Queue.bounded` — you need `scoped:`, not `effect:`.

**When `Context.Tag` is acceptable:** Infrastructure layers with runtime injection (e.g., Cloudflare KV namespace, worker bindings) or factory patterns where the resource is provided externally at runtime.

### Declare `dependencies` in `Effect.Service`

Always declare service dependencies in the `dependencies` array:

```typescript
// BAD: Dependencies leaked — every consumer must manually wire them
class MyService extends Effect.Service<MyService>()('MyService', {
  effect: Effect.gen(function* () {
    const db = yield* Database; // dependency not declared
    return {
      /* ... */
    };
  }),
}) {}

// GOOD: Dependencies declared — MyService.Default includes everything
class MyService extends Effect.Service<MyService>()('MyService', {
  dependencies: [Database.Default],
  effect: Effect.gen(function* () {
    const db = yield* Database;
    return {
      /* ... */
    };
  }),
}) {}
```

**Detecting leaks:** If `MyService.Default` has unsatisfied requirements in its `R` type parameter, you have a leaked dependency.

### Use `Layer.unwrapEffect` for Config-Dependent Layers

```typescript
const MyClientLive = Layer.unwrapEffect(
  Effect.gen(function* () {
    const env = yield* Config.string('APP_ENV').pipe(
      Config.withDefault('production')
    );
    if (env === 'development') {
      return MyClientMock.Default;
    }
    return MyClient.Default;
  })
);
```

### Bridge Effect to Non-Effect Boundaries

When Effect code runs inside non-Effect callbacks (Next.js route handlers, serverless functions, webhooks), capture the runtime:

```typescript
// Option A: ManagedRuntime — create once at startup, reuse across requests
const runtime = ManagedRuntime.make(MyService.Default);
const result = await runtime.runPromise(myEffect);
await runtime.dispose();

// Option B: Effect.runtime() — capture inside a scoped service
const program = Effect.gen(function* () {
  const rt = yield* Effect.runtime<MyService>();
  const handler = (req: Request) => Runtime.runPromise(rt)(handleRequest(req));
});
```

**Never use `Effect.runPromise` directly** for request handlers — it creates a fresh runtime per call with no shared service lifecycle, no disposal, and no layer memoization.

### Use `DateTime.now` for Timestamps

Always use Effect's `DateTime.now` instead of `Date.now()` in effectful code:

```typescript
import { DateTime } from 'effect';

/** Creates a note with effectful timestamps. */
const create = Effect.fn('Notes.create')(function* (input: NoteCreateInput) {
  const now = yield* DateTime.now;
  const row = yield* repo.create({ ...input, createdAt: now, updatedAt: now });
  return new Note(row);
});
```

This ensures timestamps are testable via `TestClock` and consistent within a single effect execution.

## Naming & Signature Conventions

### Service Identifiers

Use descriptive PascalCase names: `'UserRepository'`, `'BookmarkService'`, `'StripeClient'`.

### Method Naming

The singular/plural paradigm: `<verb>` for single, `<verb>Many` for batch.

**Service methods:**

| Operation              | Method       | Signature                                      | Returns                                       |
| ---------------------- | ------------ | ---------------------------------------------- | --------------------------------------------- |
| Create                 | `create`     | `(input: <Entity>CreateInput)`                 | `Entity`                                      |
| Create batch           | `createMany` | `(input: <Entity>CreateInput[])`               | `Entity[]`                                    |
| Get (fails if missing) | `get`        | `(id: EntityId, options?: <Entity>GetOptions)` | `Entity` (fails with `<Entity>NotFoundError`) |
| Get batch              | `getMany`    | `(ids: EntityId[])`                            | `Entity[]` (fails if any missing)             |
| List (paginated)       | `list`       | `(options?: <Entity>ListOptions)`              | `PaginatedResult<Entity>`                     |
| Update                 | `update`     | `(id: EntityId, input: <Entity>UpdateInput)`   | `Entity`                                      |
| Delete                 | `delete`     | `(id: EntityId)`                               | `Entity`                                      |

Notes:

- Expose `delete` as `delete: del` in the return object (`delete` is a reserved word as a variable name):
  ```typescript
  const del = Effect.fn('NotesRepo.delete')(function* (id: NoteId) { ... })
  return { find, create, update, delete: del } as const
  ```
- Only add `*Many` variants when there's an actual use case.
- Sub-resource operations: `create<Sub>`, `list<Subs>`, `delete<Sub>`.
- Status transitions use verb names: `start`, `stop`, `archive`, `terminate`, `complete`.

**Repo methods:**

| Operation             | Method     | Signature                              | Returns                             |
| --------------------- | ---------- | -------------------------------------- | ----------------------------------- |
| Find single           | `find`     | `(id: EntityId)`                       | `Option<Row>`                       |
| Find many (paginated) | `findMany` | `(query?: { pagination? })`            | `{ rows: Row[], hasMore: boolean }` |
| Insert single         | `insert`   | `(values: EntityInsert)`               | `Row`                               |
| Update single         | `update`   | `(id: EntityId, values: Partial<...>)` | `Option<Row>`                       |
| Delete single         | `delete`   | `(id: EntityId)`                       | `Option<Row>`                       |
| Upsert                | `upsert`   | `(values: EntityInsert)`               | `Row`                               |

`find*` always returns `Option` (single) or `Row[]` (multi). Never `null`. Never throws.

**DB -> domain type narrowing**: Use `Schema.decode` at the Service boundary. Keep the Repo returning raw DB types:

```typescript
const parseScopes = Schema.decode(Schema.Array(Scope));

const get = Effect.fn('MyService.get')(function* (id: MyId) {
  const row = yield* repo
    .find(id)
    .pipe(Effect.flatMap((r) => getOrNotFound(id, r)));
  const scopes = yield* parseScopes(row.scopes);
  return new MyEntity({ ...row, scopes });
});
```

**Policy methods** mirror service verbs with a `can` prefix: `canCreate`, `canGet`, `canList`, `canUpdate`, `canDelete`.

### Return Type Conventions

| Layer                                  | Pattern                             | Returns                              |
| -------------------------------------- | ----------------------------------- | ------------------------------------ |
| Repo `find`                            | `Option<Row>`                       | Never null, never throws             |
| Repo `findMany`                        | `{ rows: Row[], hasMore: boolean }` | Paginated                            |
| Repo `create` / `update` / `delete`    | `Row` or `Option<Row>`              | Single row                           |
| Service `get`                          | `Entity`                            | Fails with `<Entity>NotFoundError`   |
| Service `list`                         | `PaginatedResult<Entity>`           | `{ data, firstId, lastId, hasMore }` |
| Service `create` / `update` / `delete` | `Entity`                            | The affected entity                  |

### Parameter Conventions

- **Max 2 positional params:** `(id, input)` or `(id, options?)`. Three or more -> pack into object.
- **Required data** uses `input` with `Input` suffix: `input: BookmarkCreateInput`.
- **Optional extras** use `options` with `Options` suffix: `options?: BookmarkListOptions`.

### Domain Type Naming

Entity-first ordering — all types for one entity sort together.

| Type                 | Pattern                        | Example                          |
| -------------------- | ------------------------------ | -------------------------------- |
| Branded ID           | `<Entity>Id`                   | `NoteId`                         |
| Entity schema        | `<Entity>`                     | `Note`                           |
| Service create input | `<Entity>CreateInput`          | `NoteCreateInput` (Schema.Class) |
| Service update input | `<Entity>UpdateInput`          | `NoteUpdateInput` (Schema.Class) |
| Get options          | `<Entity>GetOptions`           | `NoteGetOptions`                 |
| List options         | `<Entity>ListOptions`          | `NoteListOptions`                |
| Not found error      | `<Entity>NotFoundError`        | `NoteNotFoundError`              |
| Conflict error       | `<Entity><Field>ConflictError` | `NoteTitleConflictError`         |
| Domain error         | `<Entity>Error`                | `NoteError`                      |
| Row type             | `<Entity>Row`                  | `NoteRow` (in Repo.ts)           |
| Insert type          | `<Entity>Insert`               | `NoteInsert` (in Repo.ts)        |
| Update type          | `<Entity>Update`               | `NoteUpdate` (in Repo.ts)        |

No `<Entity>IdFromString` — use `<Entity>Id.make(str)` directly.

### Schema Type Usage

| Pattern              | When to use                                                              |
| -------------------- | ------------------------------------------------------------------------ |
| `Schema.Class`       | Named, exported domain models (entities, inputs, responses)              |
| `Schema.Struct`      | Inline shapes only: path params, nested objects, function-scoped parsing |
| `Schema.TaggedError` | All error types (in `Errors.ts`)                                         |

If a struct is exported or used in more than one place, promote to `Schema.Class` with `identifier`, `title`, `description`.

### Error Field Conventions

Error types live in `Errors.ts` at the module level, not in `Domain.ts`.

| Error Type                           | Schema Fields            | Non-schema Properties |
| ------------------------------------ | ------------------------ | --------------------- |
| `<Entity>NotFoundError` (404)        | `{ id: EntityId }`       | —                     |
| `<Entity><Field>ConflictError` (409) | `{ <field>: FieldType }` | —                     |
| `<Entity>Error` (500)                | `{}`                     | `cause: unknown`      |
| `<Entity>UnavailableError` (503)     | `{}`                     | `cause: unknown`      |

**Hidden `cause` pattern:** Operational errors (500, 503) carry the original error via a non-schema class property:

```typescript
class BookmarkError extends Schema.TaggedError<BookmarkError>()(
  'BookmarkError',
  {},
  HttpApiSchema.annotations({ status: 500 })
) {
  declare readonly cause: unknown;

  constructor(props: { readonly cause?: unknown }) {
    super(props);
    this.cause = props.cause;
  }

  get message() {
    return 'Bookmark error';
  }
}
```

### `Effect.fn` Span Naming

Format: `'ServiceName.methodName'`

| Layer   | Pattern                    | Example                                    |
| ------- | -------------------------- | ------------------------------------------ |
| Service | `'Bookmarks.create'`       | `Effect.fn('Bookmarks.create')(...)`       |
| Repo    | `'BookmarksRepo.find'`     | `Effect.fn('BookmarksRepo.find')(...)`     |
| Policy  | `'BookmarksPolicy.canGet'` | `Effect.fn('BookmarksPolicy.canGet')(...)` |

### Observability Annotations

`Effect.fn` already creates a named span for every method — you never need to add `Effect.withSpan` manually. Annotations add contextual key-value pairs to those spans or logs.

**Two annotation mechanisms:**

| Mechanism                                     | When to use                                                                                               | Overhead             |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------------- | -------------------- |
| `Effect.annotateCurrentSpan(key, value)`      | When you need attributes visible in tracing dashboards (Grafana, Datadog) for successful AND failed calls | Every call           |
| `Effect.annotateLogs(effect, { key: value })` | When context only matters on failures (error logs, debugging)                                             | Only on log emission |

**Where to annotate by layer:**

| Layer        | Strategy                                                 | What to annotate                           |
| ------------ | -------------------------------------------------------- | ------------------------------------------ |
| **Services** | `Effect.annotateCurrentSpan` inside the `Effect.fn` body | Entity IDs, key business identifiers       |
| **Repos**    | No annotations                                           | Let `Effect.fn` handle spans automatically |
| **Policies** | No annotations                                           | Let `Effect.fn` handle spans automatically |

**Service pattern — annotate inside the body:**

```typescript
const get = Effect.fn('Bookmarks.get')(function* (id: BookmarkId) {
  yield* Effect.annotateCurrentSpan('bookmarkId', id);
  const row = yield* repo
    .find(id)
    .pipe(Effect.flatMap((r) => getOrNotFound(id, r)));
  return new Bookmark(row);
});
```

**What to annotate:** Entity IDs and key identifiers that help locate a specific operation in traces/logs. Don't annotate derived values, large payloads, or sensitive data.

**Every method gets annotations:** If a method accepts identifying parameters (channel, user, entity ID), it should have annotations. Methods with no identifying parameters need no annotations beyond the automatic `Effect.fn` span.

### `getOrNotFound` Pattern

Define a local helper in each service:

```typescript
const getOrNotFound = <A>(id: BookmarkId, result: Option.Option<A>) =>
  Option.match(result, {
    onNone: () => new BookmarkNotFoundError({ id }),
    onSome: (row) => Effect.succeed(row),
  });

const get = Effect.fn('Bookmarks.get')(function* (id: BookmarkId) {
  const row = yield* repo
    .find(id)
    .pipe(Effect.flatMap((r) => getOrNotFound(id, r)));
  return new Bookmark(row);
});
```

## Comment Conventions

### Effect.Service Classes

JSDoc block (2-4 sentences) with `@errors` tag:

```typescript
/**
 * Manages bookmark CRUD with URL deduplication.
 * Delegates persistence to BookmarksRepo and enforces uniqueness constraints.
 *
 * @errors BookmarkNotFoundError, BookmarkUrlConflictError
 */
export class Bookmarks extends Effect.Service<Bookmarks>()('Bookmarks', {
```

### Effect.fn Methods

Full JSDoc with `@param`, `@returns`, `@errors`:

```typescript
/**
 * Creates a new bookmark after verifying URL uniqueness.
 *
 * @param input - URL and optional title for the bookmark.
 * @returns The created bookmark entity.
 * @errors BookmarkUrlConflictError if a bookmark with the same URL exists.
 */
const create = Effect.fn('Bookmarks.create')(function* (input: BookmarkCreateInput) {
```

### Domain Types

Schema classes with `identifier`/`title`/`description` annotations don't need JSDoc. Add JSDoc only for branded types with non-obvious validation.

### Inline Comments

- `// why` for non-obvious guards, workarounds, business rules
- Never comment standard Effect patterns (`yield*`, `pipe`, `Effect.gen`)
- TODO format: `// TODO: description`

## Anti-Patterns

| Anti-Pattern                                | Problem                                              | Do Instead                                                   |
| ------------------------------------------- | ---------------------------------------------------- | ------------------------------------------------------------ |
| `throw new Error()`                         | Untracked, crashes fiber                             | `Effect.fail(new MyError())`                                 |
| Global singletons                           | Untestable, hidden deps                              | `Effect.Service` + Layers                                    |
| Missing `dependencies` in `Effect.Service`  | Leaks requirements to every consumer                 | Declare all deps in `dependencies: [...]`                    |
| `try/catch` around Effects                  | Wrong abstraction level                              | `Effect.catchTag` / `catchTags`                              |
| `Effect.catchAll`                           | Erases error type information                        | `Effect.catchTag` / `catchTags`                              |
| Extracted `make` variable                   | Unnecessary indirection                              | Inline `effect:` body in `Effect.Service`                    |
| Span annotations in repos                   | Repos shouldn't annotate spans                       | Let `Effect.fn` handle spans; annotate in services           |
| `annotateCurrentSpan` in client packages    | Overhead on every successful call                    | `Effect.annotateLogs` via pipeline — only surfaces on errors |
| Methods with params but no annotations      | Key identifiers lost in traces/logs                  | Annotate entity IDs and resource identifiers                 |
| `Effect.withSpan` alongside `Effect.fn`     | Double-span, redundant                               | `Effect.fn` already creates the span                         |
| `Promise.all` in Effects                    | Breaks Effect semantics                              | `Effect.all` with concurrency option                         |
| Unbounded retries                           | Can loop forever                                     | `Schedule.recurs(n)` with max                                |
| Manual resource cleanup                     | Can leak on errors                                   | `Effect.acquireRelease`                                      |
| `yield` instead of `yield*`                 | Returns Effect, doesn't unwrap                       | Always use `yield*`                                          |
| `console.log` for logging                   | Not structured, not traced                           | `Effect.log` / `Effect.logInfo`                              |
| Service methods without `Effect.fn`         | No automatic tracing or stack traces                 | Wrap with `Effect.fn("Service.method")`                      |
| `Effect.runPromise` for entry points        | No signal handling, no graceful shutdown             | `NodeRuntime.runMain`                                        |
| `Effect.runPromise` in request handlers     | New runtime per call, no shared lifecycle            | `ManagedRuntime` or `Effect.runtime()` capture               |
| `effect:` with lifecycle resources          | Finalizers won't run, resources leak                 | Use `scoped:` in `Effect.Service`                            |
| `Effect.uninterruptible` blanket            | Breaks `timeout` and `race`                          | `Effect.uninterruptibleMask` with `restore`                  |
| Tacit/point-free `Effect.map(fn)`           | Type erasure from overloads                          | Explicit lambdas: `Effect.map((x) => fn(x))`                 |
| `remove` as delete method                   | Wrong verb                                           | Define `delete` inline in return object                      |
| `CreateEntityPayload` / bare `CreateEntity` | Inconsistent naming                                  | `EntityCreateInput` / `EntityUpdateInput`                    |
| `findById` / `deleteById` in repo           | Redundant `ById` suffix                              | `find` / `delete`                                            |
| `findAll` in repo                           | Wrong name                                           | `findMany`                                                   |
| `getById` in service                        | Redundant `ById`                                     | `get`                                                        |
| 3+ positional params                        | Hard to read, easy to misorder                       | Pack into object param                                       |
| `null` from repo lookup                     | Inconsistent with Effect                             | `Option` (repo) or `NotFoundError` (service)                 |
| `Schema.Struct` for exported domain types   | No identity, no `$ref` in OpenAPI                    | `Schema.Class` with `identifier`                             |
| `<Entity>NotFound` without Error suffix     | Inconsistent naming                                  | `<Entity>NotFoundError`                                      |
| Verb-first type names (`CreateEntityInput`) | Doesn't group by entity                              | Entity-first: `EntityCreateInput`                            |
| `cause: Schema.optional(Schema.Unknown)`    | Leaks raw errors to HTTP responses                   | Non-schema `cause` via constructor override                  |
| `Date.now()` in Effect code                 | Not testable, inconsistent                           | `yield* DateTime.now`                                        |
| `Effect.fail(new TaggedError())`            | Redundant — `Schema.TaggedError` is itself an Effect | `new TaggedError()` or `yield* new TaggedError()` directly   |
| TaggedError in `Domain.ts`                  | Mixes error definitions with domain types            | Separate `Errors.ts` file at module level                    |

## References

| Topic              | Reference                                                                                                                                                                                              |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Core**           | [core-fundamentals](references/core-fundamentals.md), [services-layers](references/services-layers.md), [error-handling](references/error-handling.md)                                                 |
| **Data**           | [schema](references/schema.md), [streams](references/streams.md), [data-types](references/data-types.md)                                                                                               |
| **Infrastructure** | [config](references/config.md), [scheduling](references/scheduling.md), [observability](references/observability.md), [testing](references/testing.md), [effect-testing](references/effect-testing.md) |
| **Performance**    | [caching](references/caching.md), [batching](references/batching.md), [concurrency](references/concurrency.md)                                                                                         |
| **Other**          | [http-client](references/http-client.md), [resource-management](references/resource-management.md), [code-style](references/code-style.md), [sink](references/sink.md)                                 |
| **External**       | [EffectPatterns](https://github.com/PaulJPhilp/EffectPatterns) — community reference patterns and examples                                                                                             |
