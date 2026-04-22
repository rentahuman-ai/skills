# Effect-TS Core Fundamentals

## Quick Reference

### Creating Effects

| Function                     | Type                                 | Use Case                      |
| ---------------------------- | ------------------------------------ | ----------------------------- |
| `Effect.succeed(value)`      | `Effect<A, never, never>`            | Wrap pure value               |
| `Effect.fail(error)`         | `Effect<never, E, never>`            | Create failure                |
| `Effect.sync(() => a)`       | `Effect<A, never, never>`            | Sync side effect (no throw)   |
| `Effect.try(() => a)`        | `Effect<A, UnknownException, never>` | Sync that may throw           |
| `Effect.promise(() => p)`    | `Effect<A, never, never>`            | Promise (always resolves)     |
| `Effect.tryPromise(() => p)` | `Effect<A, UnknownException, never>` | Promise (may reject)          |
| `Effect.suspend(() => e)`    | `Effect<A, E, R>`                    | Deferred/lazy effect creation |

### Transforming Effects

| Function                  | Use Case                                               |
| ------------------------- | ------------------------------------------------------ |
| `Effect.map(f)`           | Transform success value: `A => B`                      |
| `Effect.flatMap(f)`       | Chain effects: `A => Effect<B>`                        |
| `Effect.andThen(f)`       | Flexible chaining: value, function, Promise, or Effect |
| `Effect.tap(f)`           | Side effect, keep original value                       |
| `Effect.as(value)`        | Replace success value with constant                    |
| `Effect.all([...])`       | Combine effects into tuple/struct                      |
| `Effect.zip(a, b)`        | Combine two effects into tuple                         |
| `Effect.zipWith(a, b, f)` | Combine two effects with function                      |

### Running Effects

| Function                   | Returns               | Use Case                        |
| -------------------------- | --------------------- | ------------------------------- |
| `Effect.runPromise(e)`     | `Promise<A>`          | Standard async execution        |
| `Effect.runPromiseExit(e)` | `Promise<Exit<A, E>>` | Structured error handling       |
| `Effect.runSync(e)`        | `A`                   | Sync-only effects               |
| `Effect.runSyncExit(e)`    | `Exit<A, E>`          | Sync with detailed outcome      |
| `Effect.runFork(e)`        | `RuntimeFiber<A, E>`  | Background/concurrent execution |

### Control Flow

| Function                                 | Use Case                              |
| ---------------------------------------- | ------------------------------------- |
| `Effect.if(pred, {onTrue, onFalse})`     | Effectful conditional                 |
| `Effect.when(() => bool)`                | Execute if true, returns `Option<A>`  |
| `Effect.unless(() => bool)`              | Execute if false, returns `Option<A>` |
| `Effect.loop(init, {while, step, body})` | State-based iteration                 |
| `Effect.forEach(items, fn)`              | Effectful iteration                   |

### Error Handling

| Function                    | Use Case                     |
| --------------------------- | ---------------------------- |
| `Effect.catchAll(handler)`  | Handle all errors            |
| `Effect.catchTag("Tag", h)` | Handle specific tagged error |
| `Effect.catchTags({...})`   | Handle multiple error types  |

---

## The Effect Type

```typescript
Effect<Success, Error, Requirements>;
```

| Parameter    | Description         | `never` means   |
| ------------ | ------------------- | --------------- |
| Success      | Value on success    | Never succeeds  |
| Error        | Expected errors     | Cannot fail     |
| Requirements | Dependencies needed | No dependencies |

Effects are **lazy** - they describe computations but don't execute until run.

---

## Effect.gen - Generator Pattern

The primary pattern for writing Effect code. Provides async/await-like syntax.

```typescript
import { Effect } from 'effect';

const program = Effect.gen(function* () {
  const user = yield* fetchUser(userId);
  const orders = yield* fetchOrders(user.id);
  return orders.total;
});
```

### Key Rules

1. **Always use `yield*`** (not `yield`) - the asterisk is required
2. **Return value** determines the Success type
3. **Errors short-circuit** - if one step fails, subsequent steps don't run

### Control Flow

Standard JavaScript works naturally inside generators:

```typescript
import { Effect } from 'effect';

const program = Effect.gen(function* () {
  const user = yield* fetchUser(id);

  if (user.isAdmin) {
    yield* grantAdminAccess();
  }

  for (const item of user.items) {
    yield* processItem(item);
  }

  if (!user.verified) {
    return yield* Effect.fail(new UnverifiedError());
  }

  return user;
});
```

### Type Narrowing Fix

Always include explicit `return` when failing for proper type narrowing:

```typescript
import { Effect } from 'effect';

const program = Effect.gen(function* () {
  const user = yield* findUser(id);

  // CORRECT: explicit return helps TypeScript narrow the type
  if (user === undefined) {
    return yield* Effect.fail(new NotFoundError());
  }

  return user.name; // user is narrowed to User (not User | undefined)
});
```

### Accessing Services

```typescript
import { Effect, Context } from 'effect';

class Logger extends Context.Tag('Logger')<
  Logger,
  {
    readonly log: (msg: string) => Effect.Effect<void>;
  }
>() {}

const program = Effect.gen(function* () {
  const logger = yield* Logger;
  yield* logger.log('Hello!');
});
```

---

## Creating Effects

### Pure Values

```typescript
import { Effect } from 'effect';

const success = Effect.succeed(42); // Effect<number, never, never>
const failure = Effect.fail(new Error()); // Effect<never, Error, never>
```

### Synchronous Operations

```typescript
import { Effect } from 'effect';

// Safe sync (cannot throw)
const log = (msg: string) => Effect.sync(() => console.log(msg));

// Sync that may throw
const parse = (input: string) => Effect.try(() => JSON.parse(input));

// With custom error
const parseWithError = (input: string) =>
  Effect.try({
    try: () => JSON.parse(input),
    catch: (e) => new ParseError(`Failed: ${e}`),
  });
```

### Asynchronous Operations

```typescript
import { Effect } from 'effect';

// Promise that always resolves
const delay = (ms: number) =>
  Effect.promise(() => new Promise<void>((r) => setTimeout(r, ms)));

// Promise that may reject
const fetchData = (url: string) =>
  Effect.tryPromise(() => fetch(url).then((r) => r.json()));

// With custom error
const fetchDataSafe = (url: string) =>
  Effect.tryPromise({
    try: () => fetch(url).then((r) => r.json()),
    catch: (e) => new FetchError(`Request failed: ${e}`),
  });
```

### Callback APIs

```typescript
import { Effect } from 'effect';
import * as fs from 'node:fs';

const readFile = (path: string) =>
  Effect.async<Buffer, NodeJS.ErrnoException>((resume) => {
    fs.readFile(path, (error, data) => {
      if (error) resume(Effect.fail(error));
      else resume(Effect.succeed(data));
    });
  });
```

### Deferred Evaluation

```typescript
import { Effect } from 'effect';

// Effect.suspend - delay effect creation until needed
// Use for: lazy evaluation, circular dependencies, unifying return types
const lazyEffect = Effect.suspend(() => {
  console.log('Creating effect now');
  return Effect.succeed(computeExpensiveValue());
});

// Useful for recursive effects
const countdown = (n: number): Effect.Effect<void> =>
  Effect.suspend(() =>
    n <= 0
      ? Effect.void
      : Console.log(`${n}`).pipe(Effect.andThen(countdown(n - 1)))
  );
```

---

## Running Effects

Run effects at application boundaries only.

```typescript
import { Effect } from 'effect';

const main = Effect.gen(function* () {
  const config = yield* loadConfig;
  yield* startServer(config);
});

// Entry point
Effect.runPromise(main).catch((error) => {
  console.error('Failed:', error);
  process.exit(1);
});
```

### Background Execution with Fibers

```typescript
import { Effect, Fiber } from 'effect';

// runFork - execute in background, returns fiber for control
const fiber = Effect.runFork(longRunningTask);

// Interrupt the fiber (cancellation)
Effect.runFork(Fiber.interrupt(fiber));

// Await fiber result
const result = await Effect.runPromise(Fiber.join(fiber));
```

---

## Services and Layers

### Define Service

```typescript
import { Effect } from 'effect';

class UserService extends Effect.Service<UserService>()('UserService', {
  effect: Effect.gen(function* () {
    const db = yield* Database;
    return {
      findById: (id: string) =>
        db.query(`SELECT * FROM users WHERE id = ?`, [id]),
      save: (user: User) => db.insert('users', user),
    };
  }),
  dependencies: [DatabaseLive],
}) {}
```

### Use Service

```typescript
const program = Effect.gen(function* () {
  const users = yield* UserService;
  return yield* users.findById('123');
});

// Provide and run
Effect.runPromise(program.pipe(Effect.provide(UserService.Default)));
```

---

## Error Handling

### Tagged Errors

```typescript
import { Effect, Data } from 'effect';

class NotFoundError extends Schema.TaggedError<NotFoundError>()(
  'NotFoundError',
  {
    id: Schema.String,
  }
) {}

class ValidationError extends Schema.TaggedError<ValidationError>()(
  'ValidationError',
  {
    field: Schema.String,
  }
) {}

const program = Effect.gen(function* () {
  const user = yield* fetchUser(id);
  if (!user) return yield* Effect.fail(new NotFoundError({ id }));
  return user;
});
```

### Catching Errors

```typescript
import { Effect } from 'effect';

const handled = program.pipe(
  Effect.catchTags({
    NotFoundError: (e) => Effect.succeed(null),
    ValidationError: (e) => Effect.fail(new BadRequestError()),
  })
);
```

---

## Control Flow Operators

Effect provides declarative control flow operators beyond standard JavaScript constructs.

### Conditional Execution

```typescript
import { Effect } from 'effect';

// Effect.if - effectful predicate with branches
const checkAccess = Effect.if(hasPermission(userId), {
  onTrue: () => Effect.succeed('Access granted'),
  onFalse: () => Effect.fail(new PermissionDenied()),
});

// Effect.when - execute conditionally, returns Option<A>
const maybeLog = Effect.succeed('logged').pipe(
  Effect.when(() => config.debugMode)
); // Effect<Option<string>, never, never>

// Effect.unless - inverse of when (execute if false)
const skipInProduction = Effect.succeed('debug info').pipe(
  Effect.unless(() => isProduction)
);

// Effect.whenEffect - condition from another effect
const conditionalOp = myEffect.pipe(
  Effect.whenEffect(checkFeatureFlag('new-feature'))
);
```

### Iteration

```typescript
import { Effect, Console } from 'effect';

// Effect.loop - state-based iteration with collection
const countdown = Effect.loop(5, {
  while: (n) => n > 0,
  step: (n) => n - 1,
  body: (n) => Console.log(`${n}...`),
  discard: false, // set true to ignore collected results
});

// Effect.iterate - repeated state updates
const doubleUntilOver100 = Effect.iterate(1, {
  while: (n) => n <= 100,
  body: (n) => Effect.succeed(n * 2),
}); // Effect<number, never, never> => 128

// Effect.forEach - effectful iteration over collections
const processItems = Effect.forEach(
  items,
  (item, index) => processItem(item),
  { concurrency: 5 } // parallel execution
);
```

---

## Building Pipelines

Pipelines compose operations in a readable, sequential manner.

### Core Pipeline Operators

```typescript
import { Effect } from 'effect';

// Effect.andThen - flexible chaining (value, function, Promise, or Effect)
const program = Effect.succeed(100).pipe(
  Effect.andThen((n) => n * 2), // value function
  Effect.andThen(200), // constant value
  Effect.andThen(Effect.succeed('done')) // another effect
);

// Effect.as - replace value with constant
const completed = fetchUser(id).pipe(Effect.as('fetch complete'));

// Effect.zip - combine two effects into tuple
const userAndOrders = Effect.zip(fetchUser(id), fetchOrders(id)); // Effect<[User, Order[]], Error, never>

// Effect.zipWith - combine with function
const total = Effect.zipWith(
  getPrice,
  getQuantity,
  (price, qty) => price * qty
);

// Concurrent zip
const parallel = Effect.zip(effectA, effectB, { concurrent: true });
```

### Effect.all with Options

```typescript
import { Effect } from 'effect';

// Basic - sequential, short-circuits on first error
const results = Effect.all([task1, task2, task3]);

// With concurrency
const parallel = Effect.all([task1, task2, task3], {
  concurrency: 'unbounded', // or a number like 5
});

// Mode: "either" - returns Either for each result
const withEithers = Effect.all([task1, task2], {
  mode: 'either',
}); // Effect<[Either<E, A>, Either<E, B>], never, R>

// Mode: "validate" - collects all errors
const validated = Effect.all([task1, task2], {
  mode: 'validate',
});

// Struct form
const config = Effect.all({
  db: loadDbConfig,
  cache: loadCacheConfig,
  api: loadApiConfig,
});
```

---

## Common Pitfalls

### 1. Using `yield` instead of `yield*`

```typescript
// WRONG
const value = yield someEffect; // Returns Effect, not value

// CORRECT
const value = yield * someEffect;
```

### 2. Running effects inside effects

```typescript
// WRONG
const bad = Effect.gen(function* () {
  const result = Effect.runSync(someEffect); // Breaks composition
});

// CORRECT
const good = Effect.gen(function* () {
  const result = yield* someEffect;
});
```

### 3. Using Promise.all instead of Effect.all

```typescript
// WRONG
const results = await Promise.all([
  Effect.runPromise(task1),
  Effect.runPromise(task2),
]);

// CORRECT
const results = yield * Effect.all([task1, task2]);
```

### 4. Forgetting to provide dependencies

```typescript
// WRONG - Type error or runtime error
Effect.runPromise(program);

// CORRECT
Effect.runPromise(program.pipe(Effect.provide(AppLayer)));
```
