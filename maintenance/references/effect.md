# Effect Housekeeping

Enforce canonical Effect-TS patterns across all backend modules.

**Also enforce all rules in [code-quality-checklist.md](code-quality-checklist.md)** — universal code quality rules that apply to every domain.

## Scope

- `apps/api/src/Modules/**`
- `apps/worker/src/Modules/**`
- `apps/rpc/src/**`
- `apps/actors/src/**`
- `apps/electric-proxy/src/**`
- `packages/platform/*` (auth, config, database, effect, server)
- `packages/clients/*`
- `packages/comcom/*` (api-core, agent-harness, agents, email, rate-limit, billing)

## Source of Truth

Load the `domain-effect` skill before auditing. Focus on Naming & Signature Conventions, Anti-Patterns table, and Module File Structure sections.

## Audit Checklist

Rule IDs use prefix `E` (effect): `E-P1-1`, `E-P2-3`, etc. Work through **P1 first**, then P2, then P3.

### P1 — Must Fix

| #   | Rule                                   | Violation                                                                                          | Correct                                                                                                          |
| --- | -------------------------------------- | -------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| 1   | Entity-first type naming               | `CreateBookmarkInput`, `DeleteBookmarkParams`                                                      | `BookmarkCreateInput`, `BookmarkDeleteParams`                                                                    |
| 2   | Input vs Params distinction            | `BookmarkCreateParams` (Schema.Struct) in service/API                                              | `BookmarkCreateInput` (Schema.Class) in services/API; `BookmarkCreateParams` (Schema.Struct) only in agent tools |
| 3   | Schema.Class for exported domain types | `export const BookmarkInput = Schema.Struct({...})`                                                | `export class BookmarkCreateInput extends Schema.Class<...>()({...})` with `identifier`, `title`, `description`  |
| 4   | Error suffix on error types            | `BookmarkNotFound`, `NotFound`                                                                     | `BookmarkNotFoundError`                                                                                          |
| 5   | Entity-scoped error naming             | `NotFoundError`, `ConflictError` (generic)                                                         | `BookmarkNotFoundError`, `BookmarkUrlConflictError` (entity-prefixed)                                            |
| 6   | NotFound error fields                  | `new BookmarkNotFoundError({ description: "..." })`                                                | `new BookmarkNotFoundError({ id })` — only the ID                                                                |
| 7   | Conflict error fields                  | `new BookmarkConflictError({ id })`                                                                | `new BookmarkUrlConflictError({ url })` — the conflicting field                                                  |
| 8   | Hidden cause on operational errors     | `cause: Schema.optional(Schema.Unknown)` in error schema                                           | Non-schema `cause` via constructor override — never leak to HTTP response                                        |
| 9   | Service method: create                 | `add(data)`, `createBookmark(input)`, `insert(values)`                                             | `create(input: BookmarkCreateInput)` → returns `Entity`                                                          |
| 10  | Service method: get                    | `getById(id)`, `findById(id)`, `find(id)`                                                          | `get(id: BookmarkId, options?: BookmarkGetOptions)` → returns `Entity`, fails with `NotFoundError`               |
| 11  | Service method: list                   | `getAll()`, `findAll()`, `search(filters)`                                                         | `list(options?: BookmarkListOptions)` → returns `PaginatedResult<Entity>`                                        |
| 12  | Service method: update                 | `modify(id, data)`, `patch(id, fields)`                                                            | `update(id: BookmarkId, input: BookmarkUpdateInput)` → returns `Entity`                                          |
| 13  | Service method: delete                 | `remove(id)`, `destroy(id)`                                                                        | `delete(id: BookmarkId)` → returns `Entity`. Expose as `delete: del` in return object                            |
| 14  | Service method: batch                  | `batchCreate(items)`, `createAll(items)`                                                           | `createMany(input: BookmarkCreateInput[])` → `Entity[]`                                                          |
| 15  | Repo method: find                      | `getById(id)`, `findById(id)`                                                                      | `find(id: BookmarkId)` → `Option<Row>` (never null)                                                              |
| 16  | Repo method: findMany                  | `findAll()`, `getAll()`, `list()`                                                                  | `findMany(query?: { pagination? })` → `{ rows: Row[], hasMore: boolean }`                                        |
| 17  | Repo method: insert                    | `create(values)`, `add(values)`                                                                    | `insert(values: BookmarkInsert)` → `Row`                                                                         |
| 18  | Repo method: update                    | `modify(id, values)`                                                                               | `update(id: BookmarkId, values: Partial<...>)` → `Option<Row>`                                                   |
| 19  | Repo method: delete                    | `remove(id)`, `destroy(id)`                                                                        | `delete(id: BookmarkId)` → `Option<Row>`. Expose as `delete: del`                                                |
| 20  | Repo returns Option not null           | `return row ?? null`, `return row \|\| undefined`                                                  | `return Option.fromNullable(row)`                                                                                |
| 21  | Effect.Service not Context.Tag         | `class MyService extends Context.Tag(...)` (for app services)                                      | `class MyService extends Effect.Service<MyService>()(...)`                                                       |
| 22  | Dependencies declared                  | `effect:` body uses `yield* Database` with no `dependencies` array                                 | Add `dependencies: [Database.Default]`                                                                           |
| 23  | Service identifier format              | `"FeedbackService"`, `"Feedback"`, `"feedback"`                                                    | `"@apps/api/Feedback"` — package prefix from package.json `name`, no `Service` suffix                            |
| 24  | Effect.fn for all service/repo methods | `const get = (id) => Effect.gen(function*() {...})`                                                | `const get = Effect.fn("Bookmarks.get")(function*(id: BookmarkId) {...})`                                        |
| 25  | Effect.fn span naming                  | `Effect.fn("get")`, `Effect.fn("BookmarkService.get")`                                             | `Effect.fn("Bookmarks.get")` — ServiceName.method, no "Service" suffix                                           |
| 26  | No throw in Effect code                | `throw new Error("not found")`                                                                     | `yield* new BookmarkNotFoundError({ id })`                                                                       |
| 27  | No try/catch around Effects            | `try { yield* ... } catch (e) {...}`                                                               | `Effect.catchTag("BookmarkNotFoundError", handler)`                                                              |
| 28  | No Effect.catchAll                     | `Effect.catchAll(effect, handler)`                                                                 | `Effect.catchTag` / `Effect.catchTags` (preserves error types)                                                   |
| 29  | No console.log                         | `console.log(...)`, `console.error(...)`                                                           | `Effect.log(...)`, `Effect.logError(...)`                                                                        |
| 30  | No Date.now()                          | `const now = new Date()`, `Date.now()`                                                             | `const now = yield* DateTime.now`                                                                                |
| 31  | scoped: for lifecycle resources        | `effect:` body with `Effect.addFinalizer`, `Effect.fork`, `Queue.bounded`, `Effect.acquireRelease` | Use `scoped:` not `effect:` in `Effect.Service`                                                                  |
| 32  | No Effect.runPromise in handlers       | `Effect.runPromise(myEffect)` per request                                                          | `ManagedRuntime` (create once) or `Effect.runtime()` capture                                                     |
| 33  | TaggedError is an Effect               | `Effect.fail(new BookmarkNotFoundError({id}))`                                                     | `yield* new BookmarkNotFoundError({id})` directly (Schema.TaggedError is itself an Effect)                       |
| 34  | No extracted make variable             | `const make = Effect.gen(...); class Svc extends Effect.Service()("...", { effect: make })`        | Inline the `effect:` body directly in `Effect.Service`                                                           |

### P2 — Should Fix

| #   | Rule                                         | Violation                                                          | Correct                                                                                                                                             |
| --- | -------------------------------------------- | ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Effect.Service JSDoc with @errors            | No JSDoc or missing `@errors` on service class                     | `/** Manages bookmark CRUD. @errors BookmarkNotFoundError, BookmarkUrlConflictError */`                                                             |
| 2   | Effect.fn JSDoc with @param/@returns/@errors | Service/repo method without full JSDoc                             | `/** Creates a bookmark. @param input - URL and title. @returns The created entity. @errors BookmarkUrlConflictError */`                            |
| 3   | delete: del in return objects                | `return { find, create, remove }` or bare `return { ..., delete }` | `const del = Effect.fn(...)(...); return { find, create, delete: del } as const`                                                                    |
| 4   | getOrNotFound local helper                   | Inline `Option.match` repeated in multiple get methods             | `const getOrNotFound = <A>(id, result: Option<A>) => Option.match(result, { onNone: () => new EntityNotFoundError({id}), onSome: Effect.succeed })` |
| 5   | stripUndefined for partial updates           | `db.update(t).set(values)` with possible undefined fields          | `db.update(t).set({ ...stripUndefined(values), updatedAt: DateTime.toDateUtc(now) })` — import from `@platform/effect/Helpers`                      |
| 6   | Effect.fn.Return<T> on complex methods       | Complex public method with no explicit return type                 | `function*(input: BookmarkCreateInput): Effect.fn.Return<Bookmark> {...}`                                                                           |
| 7   | Schema annotations on API types              | `Schema.Class` without identifier/title/description                | Add `identifier: "BookmarkCreateInput"`, `title: "Create Bookmark"`, `description: "..."`                                                           |
| 8   | Repo types in Repo.ts not Domain.ts          | `type BookmarkRow`, `type BookmarkInsert` in Domain.ts             | Drizzle-derived types (`$inferSelect`, `$inferInsert`) belong in Repo.ts                                                                            |
| 9   | Service returns Entity, Repo returns Row     | Service `get` returns raw DB row                                   | Service decodes with `Schema.decode` and returns domain Entity; Repo returns Row                                                                    |
| 10  | External calls retry + timeout               | `yield* httpClient.get(url)` bare                                  | `yield* httpClient.get(url).pipe(Effect.retry(schedule), Effect.timeout(duration))`                                                                 |
| 11  | No bare yield                                | `yield someEffect`                                                 | `yield* someEffect` (always `yield*`)                                                                                                               |
| 12  | Sub-resource method naming                   | `createBookmarkTag(...)`, `getBookmarkTags(...)`                   | `createTag(...)`, `listTags(...)` — sub-resource operations drop the parent entity name                                                             |
| 13  | Status transition verbs                      | `setStatus(id, "active")`, `changeState(id, "archived")`           | `start(id)`, `stop(id)`, `archive(id)`, `terminate(id)` — use descriptive verb names                                                                |

### P3 — Recommended

| #   | Rule                              | Violation                                                        | Correct                                                                             |
| --- | --------------------------------- | ---------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| 1   | Module file structure             | Service + Repo + Domain all in one file                          | Split to Domain.ts, Repo.ts, Service.ts                                             |
| 2   | Service directory promotion       | Single Service.ts with 2+ Effect.Service classes                 | Promote to `Service/` directory, one class per file                                 |
| 3   | Domain directory promotion        | Domain.ts exceeds ~200 lines                                     | Promote to `Domain/` directory with focused files                                   |
| 4   | Import style                      | `import { X } from "../Bookmarks/Domain.ts"`                     | `import { X } from "@/Modules/Bookmarks/Domain"` (path alias, no extension)         |
| 5   | Span annotations in services only | `yield* Effect.annotateCurrentSpan("id", id)` inside repo method | Remove from repos; add in services. Repos get tracing via `Effect.fn` only          |
| 6   | Policy method naming              | `authorize(action, entity)`, `checkPermission(user, action)`     | `canCreate(...)`, `canGet(...)`, `canUpdate(...)`, `canDelete(...)`                 |
| 7   | No tacit/point-free               | `Effect.map(fn)`, `Array.map(process)`                           | `Effect.map((x) => fn(x))`, `Array.map((item) => process(item))` — explicit lambdas |
| 8   | Testing patterns                  | `vi.mock()` for Effect services                                  | `@effect/vitest`, `makeTestLayer` / `Layer.mock`                                    |
| 9   | No section separator comments     | `// ─── Methods ───`, `// --- Queries ---`                       | Remove; use blank lines between groups                                              |

## Discovery

```bash
find apps/api/src/Modules -maxdepth 1 -type d | tail -n +2
find apps/worker/src/Modules -maxdepth 1 -type d 2>/dev/null | tail -n +2
find apps/rpc/src -maxdepth 1 -type d 2>/dev/null | tail -n +2
find apps/actors/src -maxdepth 1 -type d 2>/dev/null | tail -n +2
find apps/electric-proxy/src -maxdepth 1 -type d | tail -n +2
find packages/platform -maxdepth 1 -type d | tail -n +2
find packages/clients -maxdepth 1 -type d | tail -n +2
find packages/comcom -maxdepth 1 -type d | tail -n +2
```

## Audit Approach

Spawn one **Opus sub-agent per module directory or package**. There is no limit on the number of agents — every module and package gets its own agent for thorough coverage.

Each agent:

1. Loads the `domain-effect` skill and `practice-code-quality` skill
2. Reads all files in its assigned module
3. Works through this checklist **rule by rule**, P1 first, then P2, then P3
4. Also enforces all rules in [code-quality-checklist.md](code-quality-checklist.md)
5. Returns violations referencing rule IDs:

```
Module: {name} | Violations: {n}
- [E-P1-3] Schema.Class for exported types — Domain.ts:42
  Current: `export const BookmarkInput = Schema.Struct({...})`
  Should be: `export class BookmarkCreateInput extends Schema.Class<...>()({...})`
- [Q-P2-1] Boolean naming — Service.ts:88
  Current: `const loading = ...`
  Should be: `const isLoading = ...`
```

Or `Module: {name} — clean` if none found.

## Cross-Cutting References

- [knip.md](knip.md) — unused deps in platform/clients/comcom packages
- [npm-packages.md](npm-packages.md) — catalogue compliance
