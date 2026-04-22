# Effect Error Handling Reference

## Quick Reference

| Pattern                                    | Use Case                    | Type Change                 |
| ------------------------------------------ | --------------------------- | --------------------------- |
| `yield* new MyError()`                     | Yieldable error in gen      | Adds to `E` channel         |
| `Effect.catchTag("Tag", handler)`          | Handle one error type       | Removes `Tag` from `E`      |
| `Effect.catchTags({ Tag1: h1, Tag2: h2 })` | Handle multiple types       | Removes handled tags        |
| `Effect.catchAll(handler)`                 | Handle all errors           | `E` becomes handler's error |
| `Effect.catchSome(handler)`                | Selective recovery          | Error type unchanged        |
| `Effect.catchIf(pred, handler)`            | Predicate recovery          | Narrows error type          |
| `Effect.catchAllCause(handler)`            | Handle all causes           | `E` becomes handler's error |
| `Effect.retry({ times: 3 })`               | Simple retry                | No change                   |
| `Effect.retry(schedule)`                   | Retry with backoff          | No change                   |
| `Effect.orElse(() => fallback)`            | Fallback effect             | Unions both error types     |
| `Effect.orElseSucceed(() => value)`        | Default on failure          | `E` becomes `never`         |
| `Effect.firstSuccessOf([...])`             | Sequential fallback chain   | Last effect's error         |
| `Effect.either(effect)`                    | Convert to Either           | `E` becomes `never`         |
| `Effect.option(effect)`                    | Errors to None              | `E` becomes `never`         |
| `Effect.sandbox(effect)`                   | Expose full Cause           | `E` becomes `Cause<E>`      |
| `Effect.match(e, {...})`                   | Pattern match outcomes      | Returns pure value          |
| `Effect.validate(e1)(e2)`                  | Accumulate errors           | Collects all errors         |
| `Effect.validateAll(items, fn)`            | All-or-nothing validation   | Array of errors             |
| `Effect.partition(items, fn)`              | Separate successes/failures | `E` becomes `never`         |

---

## Yieldable Errors

`Schema.TaggedError` creates yieldable errors that can be yielded directly in `Effect.gen` without wrapping in `Effect.fail`:

```typescript
import { Schema, Effect } from 'effect';

// Define errors with _tag discriminant
class NetworkError extends Schema.TaggedError<NetworkError>()('NetworkError', {
  url: Schema.String,
  statusCode: Schema.Number,
}) {}

class ValidationError extends Schema.TaggedError<ValidationError>()(
  'ValidationError',
  {
    field: Schema.String,
    message: Schema.String,
  }
) {}

class NotFoundError extends Schema.TaggedError<NotFoundError>()(
  'NotFoundError',
  {
    resource: Schema.String,
    id: Schema.String,
  }
) {}

// Yieldable - can yield errors directly without Effect.fail
const fetchUser = (
  id: string
): Effect.Effect<User, NetworkError | NotFoundError> =>
  Effect.gen(function* () {
    const response = yield* httpGet(`/users/${id}`);
    if (response.status === 404) {
      yield* new NotFoundError({ resource: 'User', id }); // Direct yield!
    }
    if (response.status >= 500) {
      yield* new NetworkError({
        url: `/users/${id}`,
        statusCode: response.status,
      });
    }
    return response.data;
  });

// Non-tagged errors with Data.Error
class SimpleError extends Data.Error<{ message: string }> {}

const program = Effect.gen(function* () {
  yield* new SimpleError({ message: 'Something went wrong' });
});
```

---

## Effect.fn — Traced Functions

`Effect.fn` is the preferred pattern for all service and repository methods. It automatically creates a span, attaches stack traces, and works as a generator:

```typescript
import { Effect } from 'effect';

// Service method with automatic tracing span
const create = Effect.fn('Bookmarks.create')(function* (
  userId: string,
  organizationId: string,
  input: CreateBookmark
) {
  yield* Effect.annotateCurrentSpan('organizationId', organizationId);
  yield* Effect.log('Creating bookmark');
  // ... business logic with yield* for each step
  return result;
});

// Repo method with automatic tracing span
const findById = Effect.fn('BookmarksRepo.findById')(function* (
  id: BookmarkId
) {
  yield* Effect.annotateCurrentSpan('bookmarkId', id);
  const rows = yield* db
    .select()
    .from(bookmark)
    .where(eq(bookmark.id, id))
    .limit(1);
  return rows.length > 0 ? Option.some(rows[0]) : Option.none();
});
```

**Prefer `Effect.fn` over manual `Effect.withSpan`** — it combines tracing, stack traces, and generator syntax into one pattern.

---

## Catching Errors

### catchTag - Handle Single Error Type

```typescript
import { Effect, Data } from "effect"

const program: Effect.Effect<string, NetworkError | ValidationError> = // ...

const handled = program.pipe(
  Effect.catchTag("NetworkError", (error) =>
    Effect.succeed(`Recovered from ${error.statusCode}`)
  )
)
// Type: Effect<string, ValidationError>
```

### catchTags - Handle Multiple Types

```typescript
const fullyHandled = program.pipe(
  Effect.catchTags({
    NetworkError: (e) => Effect.succeed(`Network: ${e.statusCode}`),
    ValidationError: (e) => Effect.succeed(`Validation: ${e.message}`),
  })
);
// Type: Effect<string, never>
```

### catchAll - Handle All Errors

```typescript
const recovered = program.pipe(
  Effect.catchAll((error) => Effect.succeed(`Recovered from ${error._tag}`))
);
// Type: Effect<string, never>
```

### catchSome - Selective Recovery

```typescript
import { Option } from 'effect';

const selective = program.pipe(
  Effect.catchSome(
    (error) =>
      error._tag === 'NetworkError'
        ? Option.some(Effect.succeed('recovered'))
        : Option.none() // Let other errors propagate
  )
);
// Type: Effect<string, NetworkError | ValidationError> (unchanged)
```

### catchIf - Predicate Recovery

```typescript
const conditional = program.pipe(
  Effect.catchIf(
    (error): error is NetworkError => error._tag === 'NetworkError',
    (error) => Effect.succeed(`Recovered from ${error.statusCode}`)
  )
);
// Type: Effect<string, ValidationError> (NetworkError removed with type guard)
```

### catchAllCause - Handle All Causes

```typescript
import { Cause } from 'effect';

// Handles both expected errors AND defects
const allCauses = program.pipe(
  Effect.catchAllCause((cause) =>
    Cause.isFailType(cause)
      ? Effect.succeed('Recovered from error')
      : Effect.succeed('Recovered from defect')
  )
);
```

---

## Error Matching

Pattern match on success and failure outcomes:

```typescript
import { Effect, Console } from 'effect';

// match - handle success/failure, returns pure value
const result = Effect.match(program, {
  onFailure: (error) => `Failed: ${error._tag}`,
  onSuccess: (value) => `Success: ${value}`,
});
// Type: Effect<string, never>

// matchEffect - handle with side effects
const withSideEffects = Effect.matchEffect(program, {
  onFailure: (error) =>
    Effect.succeed(`Failed: ${error._tag}`).pipe(
      Effect.tap(() => Console.log('Logging failure'))
    ),
  onSuccess: (value) => Effect.succeed(value),
});

// matchCause - access full Cause structure
const withCause = Effect.matchCause(program, {
  onFailure: (cause) => {
    if (cause._tag === 'Die') return `Defect: ${cause.defect}`;
    if (cause._tag === 'Fail') return `Error: ${cause.error._tag}`;
    if (cause._tag === 'Interrupt') return 'Interrupted';
    return 'Unknown cause';
  },
  onSuccess: (value) => `Success: ${value}`,
});

// matchCauseEffect - cause access with side effects
const fullMatch = Effect.matchCauseEffect(program, {
  onFailure: (cause) => Console.log(`Cause: ${cause._tag}`),
  onSuccess: (value) => Console.log(`Value: ${value}`),
});

// ignore - discard both success and failure
const ignored = Effect.ignore(program);
// Type: Effect<void, never>
```

---

## Recovery Patterns

### orElse - Fallback Effect Chain

```typescript
import { Effect } from "effect"

const fetchFromPrimary = // Effect that may fail
const fetchFromCache = Effect.succeed({ data: "cached", fromCache: true })

const resilient = fetchFromPrimary.pipe(
  Effect.orElse(() => fetchFromSecondary),
  Effect.orElse(() => fetchFromCache)
)
```

### orElseSucceed - Default Value

```typescript
const withDefault = fetchData.pipe(
  Effect.orElseSucceed(() => ({ items: [], fallback: true }))
);
// Type: Effect<Data, never> - always succeeds
```

### either - Convert to Either

```typescript
import { Either, Effect } from 'effect';

const asEither = Effect.either(program);
// Type: Effect<Either<A, E>, never>

const result = Effect.gen(function* () {
  const either = yield* asEither;
  return Either.match(either, {
    onLeft: (error) => `Error: ${error._tag}`,
    onRight: (value) => `Success: ${value}`,
  });
});
```

### option - Convert Errors to None

```typescript
import { Effect, Option } from 'effect';

const asOption = Effect.option(program);
// Type: Effect<Option<A>, never>

const result = Effect.gen(function* () {
  const opt = yield* asOption;
  return Option.getOrElse(opt, () => 'default');
});
```

### firstSuccessOf - Sequential Fallback Chain

```typescript
// Try effects in order, return first success
const config = Effect.firstSuccessOf([
  loadFromRemote('primary'),
  loadFromRemote('secondary'),
  loadFromCache(),
  loadDefaults(),
]);
// Returns first success, or final error if all fail
// Execution is sequential, not concurrent
```

---

## Retry Patterns

### Basic Retry

```typescript
import { Effect, Schedule } from 'effect';

// Simple retry count
const withRetry = Effect.retry(unstableEffect, { times: 3 });

// With schedule
const withSchedule = Effect.retry(unstableEffect, Schedule.recurs(5));
```

### Exponential Backoff

```typescript
// 100ms, 200ms, 400ms, 800ms...
const exponential = Schedule.exponential('100 millis');

// With jitter to prevent thundering herd
const jittered = Schedule.jittered(Schedule.exponential('100 millis'));

// Bounded exponential (max 5 retries)
const bounded = Schedule.exponential('100 millis').pipe(
  Schedule.jittered,
  Schedule.intersect(Schedule.recurs(5))
);
```

### Schedule Combinators

| Combinator                    | Behavior                          |
| ----------------------------- | --------------------------------- |
| `Schedule.recurs(n)`          | Retry exactly n times             |
| `Schedule.exponential(base)`  | Exponential backoff (base \* 2^n) |
| `Schedule.spaced(duration)`   | Fixed delay between attempts      |
| `Schedule.jittered(schedule)` | Add randomness (80-120%)          |
| `Schedule.intersect(a, b)`    | Both must allow, use longer delay |
| `Schedule.union(a, b)`        | Either allows, use shorter delay  |

### Conditional Retry

```typescript
const conditionalRetry = Effect.retry(effect, {
  schedule: Schedule.exponential('100 millis').pipe(Schedule.recurs(5)),
  while: (error) => error._tag === 'NetworkError' && error.statusCode >= 500,
});
```

### Production Retry Pattern

```typescript
import { Effect, Schedule } from 'effect';

const productionRetry = <A, E extends { _tag: string }>(
  effect: Effect.Effect<A, E>,
  isRetryable: (error: E) => boolean
) =>
  effect.pipe(
    Effect.retry({
      schedule: Schedule.exponential('100 millis').pipe(
        Schedule.jittered,
        Schedule.intersect(Schedule.recurs(5))
      ),
      while: isRetryable,
    }),
    Effect.timeout('30 seconds')
  );

// Usage
const resilientFetch = productionRetry(
  httpCall,
  (e) => e._tag === 'NetworkError'
);
```

---

## Expected Errors vs Defects

| Type           | Description            | In Type Signature | Recovery                |
| -------------- | ---------------------- | ----------------- | ----------------------- |
| Expected Error | Anticipated failures   | Yes (`E` channel) | `catchTag`, `catchAll`  |
| Defect         | Bugs, unexpected state | No                | `catchAllDefect` (rare) |

### Creating Defects

```typescript
// Use for truly unexpected states (bugs)
const crash = Effect.die(new Error('Invariant violated'));
const crashMsg = Effect.dieMessage('This should never happen');

// Convert errors to defects (use sparingly)
const mustSucceed = riskyOp.pipe(Effect.orDie);
```

### Catching Defects (Boundary Only)

```typescript
// Only at system boundaries for logging
const atBoundary = program.pipe(
  Effect.catchAllDefect((defect) => {
    console.error('Unexpected defect:', defect);
    return Effect.fail(new InternalError({ cause: defect }));
  })
);

// catchSomeDefect - handle specific defects
const specific = program.pipe(
  Effect.catchSomeDefect((defect) => {
    if (Cause.isIllegalArgumentException(defect)) {
      return Option.some(Effect.succeed('recovered'));
    }
    return Option.none(); // Let other defects propagate
  })
);
```

---

## Sandboxing

Sandboxing exposes the full `Cause<E>` structure, enabling granular error handling:

```typescript
import { Effect, Cause } from 'effect';

// sandbox transforms Effect<A, E, R> into Effect<A, Cause<E>, R>
const sandboxed = Effect.sandbox(riskyOperation);

// Can now catch specific cause types
const handled = sandboxed.pipe(
  Effect.catchTags({
    Die: (cause) => Effect.succeed(`Defect: ${cause.defect}`),
    Interrupt: (cause) => Effect.succeed(`Interrupted by: ${cause.fiberId}`),
    Fail: (cause) => Effect.succeed(`Error: ${cause.error._tag}`),
  })
);

// Restore normal error semantics
const restored = Effect.unsandbox(handled);

// sandboxWith - transform in one step
const withSandbox = Effect.sandboxWith(riskyOperation, (effect) =>
  effect.pipe(
    Effect.catchTag('Die', () => Effect.succeed('recovered from defect'))
  )
);
```

### Cause Types

| Cause Type   | Description                  | Access                      |
| ------------ | ---------------------------- | --------------------------- |
| `Fail`       | Expected error               | `cause.error`               |
| `Die`        | Defect (unexpected)          | `cause.defect`              |
| `Interrupt`  | Fiber interruption           | `cause.fiberId`             |
| `Sequential` | Combined causes (sequential) | `cause.left`, `cause.right` |
| `Parallel`   | Combined causes (parallel)   | `cause.left`, `cause.right` |

---

## Timeouts

```typescript
import { Effect, Schema } from 'effect';

// Basic timeout (raises TimeoutException)
const withTimeout = longOp.pipe(Effect.timeout('5 seconds'));

// Custom timeout error
class OperationTimeout extends Schema.TaggedError<OperationTimeout>()(
  'OperationTimeout',
  {
    operation: Schema.String,
  }
) {}

const customTimeout = longOp.pipe(
  Effect.timeoutFail({
    duration: '5 seconds',
    onTimeout: () => new OperationTimeout({ operation: 'fetch' }),
  })
);
```

---

## Error Aggregation

By default, Effect uses fail-fast: execution halts on first error. These functions collect errors instead:

```typescript
import { Effect } from 'effect';

// Collect all errors instead of fail-fast
const validated = Effect.all(validations, { mode: 'validate' });

// Partition successes and failures
const [failures, successes] = yield * Effect.partition(items, process);
// Type: [E[], A[]]
```

### validate - Chain with Error Accumulation

```typescript
// Accumulate errors from multiple effects
const validated = task1.pipe(
  Effect.validate(task2),
  Effect.validate(task3),
  Effect.validate(task4)
);
// All tasks run, errors are collected
```

### validateAll - All-or-Nothing

```typescript
// Process all items, lose successes if any fail
const allOrNothing = Effect.validateAll([1, 2, 3, 4, 5], (n) =>
  n < 4 ? Effect.succeed(n) : Effect.fail(`invalid: ${n}`)
);
// If any fail: all errors collected, successes discarded
// If all succeed: array of successes
```

### validateFirst - First Success Wins

```typescript
// Return first success, or all errors if none succeed
const firstValid = Effect.validateFirst(configs, loadConfig);
// Tries each config, returns first success
// If all fail: array of all errors
```

### partition - Separate Successes and Failures

```typescript
// Always succeeds, separates results
const [failures, successes] =
  yield *
  Effect.partition(
    items,
    processItem,
    { concurrency: 'unbounded' } // Optional parallel execution
  );
// Type: [E[], A[]] - no error channel
```

### Comparison

| Function                           | Fail-Fast | Behavior                      | Error Type              |
| ---------------------------------- | --------- | ----------------------------- | ----------------------- |
| `Effect.all`                       | Yes       | Stops on first error          | Single error            |
| `Effect.all({ mode: "validate" })` | No        | Collects all errors           | Array of errors         |
| `Effect.validate`                  | No        | Chain effects, collect errors | Combined errors         |
| `Effect.validateAll`               | No        | All-or-nothing                | Array of errors         |
| `Effect.validateFirst`             | No        | First success wins            | Array of errors         |
| `Effect.partition`                 | No        | Separate results              | Never (always succeeds) |

---

## Anti-Patterns

| Anti-Pattern                                  | Problem                                         | Better Approach                         |
| --------------------------------------------- | ----------------------------------------------- | --------------------------------------- |
| `Effect.catchAll(() => Effect.succeed(null))` | Swallows all errors                             | Use `catchTag` for specific errors      |
| `Effect.fail("error string")`                 | No type safety                                  | Use `Schema.TaggedError`                |
| `Effect.retry(e, Schedule.forever)`           | Infinite retries                                | Bound with `Schedule.recurs`            |
| `fetchUser(id).pipe(Effect.orDie)`            | Hides expected errors as defects                | Handle with `catchTag` or let propagate |
| `repo.query().pipe(Effect.orDie)`             | Converts SqlError to defect, losing type safety | Map to domain error at service level    |
| `try/catch` in `Effect.gen`                   | Won't catch Effect errors                       | Use Effect error combinators            |

### NEVER use `Effect.orDie`

`Effect.orDie` converts typed errors into defects (uncaught exceptions), destroying the type safety that Effect provides. This is always an anti-pattern in this codebase.

**Especially for SQL errors:** When a repo call returns `Effect<A, SqlError>`, handle the `SqlError` in the service layer — map it to a domain error (e.g., `NotFoundError`, `ConflictError`) or let it propagate as a typed error so callers know what can fail. Never use `orDie` to make it disappear.

```typescript
// BAD — hides SqlError as a defect
const getUser = (id: string) => repo.findById(id).pipe(Effect.orDie);

// GOOD — maps to domain error
const getUser = (id: string) =>
  repo.findById(id).pipe(
    Effect.flatMap(
      Option.match({
        onNone: () => new UserNotFoundError({ id }),
        onSome: Effect.succeed,
      })
    )
  );

// ALSO GOOD — let SqlError propagate in the type signature
const getUser = (id: string): Effect<User, SqlError | UserNotFoundError> =>
  repo.findById(id).pipe(/* ... */);
```
