# Effect Code Style Reference

## Quick Reference

| Area              | Guideline                                                                        |
| ----------------- | -------------------------------------------------------------------------------- |
| Entry point       | `NodeRuntime.runMain(program)`                                                   |
| Complex logic     | `Effect.gen(function* () { ... })`                                               |
| Simple transforms | `pipe(effect, Effect.map((x) => ...))`                                           |
| Single operations | `Effect.map(effect, (x) => ...)`                                                 |
| Functions         | Always use explicit lambdas, not tacit style                                     |
| Layers            | `*Live` (production), `*Test` (testing)                                          |
| Errors            | `Schema.TaggedError` with `_tag`                                                 |
| Services          | Keep interface Requirements as `never`                                           |
| Domain types      | Branded types for type safety                                                    |
| Comments          | JSDoc on Effect.Service classes and Effect.fn methods; @param, @returns, @errors |

## Naming Conventions

| Element       | Convention                                                 | Example                          |
| ------------- | ---------------------------------------------------------- | -------------------------------- |
| Services      | PascalCase                                                 | `DatabaseService`, `UserService` |
| Tags          | PascalCase                                                 | `Database`, `Config`             |
| Layers (prod) | `*Live` suffix                                             | `DatabaseLive`, `ConfigLive`     |
| Layers (test) | `*Test` suffix                                             | `DatabaseTest`                   |
| Effects       | camelCase                                                  | `getUser`, `processPayment`      |
| Errors        | `*Error` suffix                                            | `HttpError`, `ValidationError`   |
| Branded types | PascalCase                                                 | `UserId`, `Email`, `PositiveInt` |
| Module files  | Domain.ts, Repo.ts, Service.ts, Policy.ts, Api.ts, Http.ts |

## Dual APIs

Effect supports **data-first** and **data-last** patterns.

### Data-Last (for pipelines)

```typescript
import { Effect, pipe } from 'effect';

const program = pipe(
  fetchUser(userId),
  Effect.map((user) => user.email),
  Effect.flatMap((email) => sendEmail(email)),
  Effect.tap(() => Effect.log('Email sent'))
);
```

### Data-First (for single operations)

```typescript
import { Effect } from 'effect';

const doubled = Effect.map(getValue(), (x) => x * 2);
const name = Effect.map(user, (u) => u.name);
```

### When to Use Which

| Scenario                     | Pattern    | Example                                         |
| ---------------------------- | ---------- | ----------------------------------------------- |
| Multiple chained operations  | Data-last  | `pipe(a, Effect.map(...), Effect.flatMap(...))` |
| Single transformation        | Data-first | `Effect.map(effect, fn)`                        |
| Building reusable transforms | Data-last  | `const double = Effect.map((n) => n * 2)`       |

## Do Notation (Effect.gen)

**Always prefer `Effect.gen` for complex logic.** It eliminates nesting and reads like imperative code.

```typescript
import { Effect } from 'effect';

// PREFER: Generator style
const processOrder = Effect.gen(function* () {
  const user = yield* getUser(userId);
  const inventory = yield* checkInventory(productId);

  if (inventory.available < quantity) {
    return yield* Effect.fail(new InsufficientInventoryError());
  }

  const order = yield* createOrder(user, productId, quantity);
  yield* sendConfirmationEmail(user.email, order);

  return order;
});
```

### Effect.Do Alternative

When generators are unavailable, use the Do simulation:

```typescript
import { Effect, pipe } from 'effect';

const elapsed = <R, E, A>(self: Effect.Effect<A, E, R>) =>
  Effect.Do.pipe(
    Effect.bind('startMillis', () => now),
    Effect.bind('result', () => self),
    Effect.bind('endMillis', () => now),
    Effect.let(
      'elapsed',
      ({ startMillis, endMillis }) => endMillis - startMillis
    ),
    Effect.tap(({ elapsed }) => Console.log(`Elapsed: ${elapsed}`)),
    Effect.map(({ result }) => result)
  );
```

| Approach     | Use Case                              |
| ------------ | ------------------------------------- |
| `Effect.gen` | **Default choice** - best readability |
| `Effect.Do`  | When generators unavailable           |
| Plain `pipe` | Simple single transforms only         |

## Pattern Matching

Three steps: create matcher, define patterns, finalize.

### Basic Pattern Matching

```typescript
import { Match, pipe } from 'effect';

type Result =
  | { _tag: 'Success'; data: string }
  | { _tag: 'Error'; message: string }
  | { _tag: 'Loading' };

const handleResult = pipe(
  Match.type<Result>(),
  Match.tag('Success', (r) => `Data: ${r.data}`),
  Match.tag('Error', (r) => `Error: ${r.message}`),
  Match.tag('Loading', () => 'Loading...'),
  Match.exhaustive
);
```

### Common Patterns

```typescript
import { Match, pipe } from 'effect';

// Match on value directly
const result = pipe(
  Match.value(someValue),
  Match.when({ type: 'success' }, (v) => v.data),
  Match.orElse(() => null)
);

// Match with predicates
const handleNumber = pipe(
  Match.type<number>(),
  Match.when(
    (n) => n > 0,
    () => 'positive'
  ),
  Match.when(
    (n) => n < 0,
    () => 'negative'
  ),
  Match.orElse(() => 'zero')
);

// Type-safe return type
const toNumber = pipe(
  Match.type<string | number>(),
  Match.withReturnType<number>(),
  Match.when(Match.string, (s) => parseInt(s, 10)),
  Match.when(Match.number, (n) => n),
  Match.exhaustive
);
```

### Built-in Predicates

| Predicate                 | Matches           |
| ------------------------- | ----------------- |
| `Match.string`            | string values     |
| `Match.number`            | number values     |
| `Match.boolean`           | boolean values    |
| `Match.null`              | null              |
| `Match.undefined`         | undefined         |
| `Match.defined`           | any defined value |
| `Match.date`              | Date objects      |
| `Match.instanceOf(Class)` | class instances   |
| `Match.is(value)`         | specific value    |

### Finalizers

| Finalizer          | Description                          |
| ------------------ | ------------------------------------ |
| `Match.exhaustive` | TypeScript error if cases missing    |
| `Match.orElse`     | Fallback for unmatched cases         |
| `Match.option`     | Returns `Option<T>`                  |
| `Match.either`     | Returns `Either<Unmatched, Matched>` |

## Branded Types

Add compile-time type safety for structurally identical types.

### Brand.nominal (no validation)

```typescript
import { Brand } from 'effect';

type UserId = string & Brand.Brand<'UserId'>;
const UserId = Brand.nominal<UserId>();

type OrderId = string & Brand.Brand<'OrderId'>;
const OrderId = Brand.nominal<OrderId>();

const userId = UserId('123');
const orderId = OrderId('456');
// getOrder(userId) // Type error!
```

### Brand.refined (with validation)

```typescript
import { Brand } from 'effect';

type Email = string & Brand.Brand<'Email'>;
const Email = Brand.refined<Email>(
  (s) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s),
  (s) => Brand.error(`Invalid email: ${s}`)
);

type PositiveInt = number & Brand.Brand<'PositiveInt'>;
const PositiveInt = Brand.refined<PositiveInt>(
  (n) => Number.isInteger(n) && n > 0,
  (n) => Brand.error(`Expected positive integer: ${n}`)
);

const email = Email('user@example.com'); // OK
const invalid = Email('not-an-email'); // Throws
```

### Brand.all (combine validators)

```typescript
import { Brand } from 'effect';

type Int = number & Brand.Brand<'Int'>;
const Int = Brand.refined<Int>(
  (n) => Number.isInteger(n),
  (n) => Brand.error(`Expected integer: ${n}`)
);

type Positive = number & Brand.Brand<'Positive'>;
const Positive = Brand.refined<Positive>(
  (n) => n > 0,
  (n) => Brand.error(`Expected positive: ${n}`)
);

type PositiveInt = number & Brand.Brand<'Int'> & Brand.Brand<'Positive'>;
const PositiveInt = Brand.all(Int, Positive);
```

| Method          | Validation | Use Case                                 |
| --------------- | ---------- | ---------------------------------------- |
| `Brand.nominal` | None       | IDs, tokens, semantic distinctions       |
| `Brand.refined` | Runtime    | Emails, positive numbers, validated data |
| `Brand.all`     | Combined   | Multiple constraints                     |

## Function Style

**Always use explicit lambdas, never tacit (point-free) style.**

```typescript
// AVOID: Tacit style - can cause type erasure bugs
Effect.map(fn);
Array.map(process);

// PREFER: Explicit lambdas - preserves types
Effect.map((x) => fn(x));
Array.map((item) => process(item));
```

## Entry Points

```typescript
import { NodeRuntime } from '@effect/platform-node';
import { Effect } from 'effect';

const program = Effect.gen(function* () {
  yield* Effect.log('Application started');
  // Main logic
});

NodeRuntime.runMain(program);
```

| Platform | Import                     | Runtime                  |
| -------- | -------------------------- | ------------------------ |
| Node.js  | `@effect/platform-node`    | `NodeRuntime.runMain`    |
| Bun      | `@effect/platform-bun`     | `BunRuntime.runMain`     |
| Browser  | `@effect/platform-browser` | `BrowserRuntime.runMain` |

## Module File Structure

Each feature module follows this canonical structure:

```
src/Modules/
  Bookmarks/
    Domain.ts     — Branded types, schemas, error classes
    Repo.ts       — Data access layer (Effect.fn for all methods)
    Service.ts    — Business logic (Effect.fn for all methods)
    Policy.ts     — Authorization rules
    Api.ts        — HttpApiGroup + endpoint definitions
    Http.ts       — Handler implementations (use generators)
```

- `Domain.ts` is always the source of truth for types and errors
- `Repo.ts` and `Service.ts` methods use `Effect.fn` for automatic span tracing
- `Http.ts` handlers use `Effect.gen(function* () { ... })` generators

### Flat file vs directory

Use a flat `Service.ts` or `Domain.ts` by default. Promote to a directory when:

- **`Service/`** -- the module has 2+ `Effect.Service` classes (e.g., Integrations has `IntegrationConnections`, `ProviderRegistry`, `ActionExecutor`). Each service gets its own file with an `index.ts` barrel.
- **`Domain/`** -- domain types exceed ~200 lines or span multiple distinct concerns (IDs, schemas, errors, provider types). Split into focused files (`Ids.ts`, `Schemas.ts`, `Errors.ts`, etc.) with an `index.ts` that uses named re-exports.

A module that does not need all canonical files is also valid. For example, Electric has no `Service.ts` because its logic lives in utility functions.
