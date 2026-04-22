# Effect Data Types Reference

## Quick Reference

| Type           | Purpose            | Create                                                      | Extract                                |
| -------------- | ------------------ | ----------------------------------------------------------- | -------------------------------------- |
| `Option<A>`    | Optional values    | `Option.some(v)`, `Option.none()`, `Option.fromNullable(v)` | `Option.getOrElse()`, `Option.match()` |
| `Either<R, L>` | Success/failure    | `Either.right(v)`, `Either.left(e)`                         | `Either.match()`, `Either.isRight()`   |
| `Cause<E>`     | Rich failure info  | `Cause.fail(e)`, `Cause.die(defect)`                        | `Cause.match()`, `Cause.pretty()`      |
| `Chunk<A>`     | Immutable sequence | `Chunk.make()`, `Chunk.fromIterable()`                      | `Chunk.toReadonlyArray()`              |
| `Duration`     | Time spans         | `Duration.seconds(n)`, `Duration.minutes(n)`                | `Duration.toMillis()`                  |
| `Data.struct`  | Value equality     | `Data.struct({ ... })`                                      | `Equal.equals()`                       |

---

## Option

Represents values that may or may not exist. Use instead of `null`/`undefined`.

```typescript
import { Option } from 'effect';

// Creating
Option.some(1); // Some(1)
Option.none(); // None
Option.fromNullable(null); // None
Option.fromNullable(1); // Some(1)

// Transforming
Option.map(Option.some(1), (n) => n + 1); // Some(2)
Option.flatMap(Option.some(1), (n) => Option.some(n * 2)); // Some(2)
Option.filter(Option.some(5), (n) => n > 3); // Some(5)

// Extracting
Option.getOrElse(Option.none(), () => 0); // 0
Option.getOrThrow(Option.some(10)); // 10
Option.getOrNull(Option.none()); // null

// Pattern matching
Option.match(option, { onNone: () => 'empty', onSome: (v) => `has: ${v}` });

// Combining
Option.zipWith(Option.some('John'), Option.some(25), (name, age) => ({
  name,
  age,
}));
Option.all([Option.some(1), Option.some(2)]); // Some([1, 2])

// Generator syntax (short-circuits on None)
Option.gen(function* () {
  const name = yield* Option.some('John');
  return { name, age: yield* Option.some(25) };
});
```

---

## Either

Discriminated union for success (`Right`) or failure (`Left`).

```typescript
import { Either } from 'effect';

Either.right(42); // Right(42)
Either.left('error'); // Left("error")

// Type guards
if (Either.isRight(value)) console.log(value.right);

// Mapping
Either.map(Either.right(1), (n) => n + 1); // Right(2)
Either.mapLeft(Either.left('err'), (s) => s + '!'); // Left("err!")

// Pattern matching
Either.match(either, {
  onLeft: (e) => `Error: ${e}`,
  onRight: (v) => `Success: ${v}`,
});

// Combining
Either.all([Either.right('John'), Either.right(25)]); // Right(["John", 25])

// Generator syntax (short-circuits on Left)
Either.gen(function* () {
  return { name: yield* Either.right('John'), age: yield* Either.right(25) };
});
```

---

## Cause

Captures comprehensive failure information: expected errors, defects, interruptions.

| Type        | Constructor                | Description         |
| ----------- | -------------------------- | ------------------- |
| `Fail`      | `Cause.fail(error)`        | Expected errors     |
| `Die`       | `Cause.die(defect)`        | Unexpected defects  |
| `Interrupt` | `Cause.interrupt(fiberId)` | Fiber interruptions |

```typescript
import { Cause, Effect } from 'effect';

Cause.fail('Connection error');
Cause.die(new TypeError('Cannot read property'));

// Pattern matching
Cause.match(cause, {
  onEmpty: () => 'No error',
  onFail: (error) => `Expected: ${error}`,
  onDie: (defect) => `Unexpected: ${defect}`,
  onInterrupt: (fiberId) => `Interrupted`,
  onSequential: (l, r) => `Sequential`,
  onParallel: (l, r) => `Parallel`,
});

// Analysis
Cause.failures(cause); // Chunk<E> - all expected errors
Cause.defects(cause); // Chunk<unknown> - all defects
Cause.pretty(cause); // Human-readable string

// With Effect
const cause = yield * Effect.cause(failedEffect);
Effect.failCause(Cause.fail('error'));
```

---

## Chunk

Immutable sequence optimized for repeated concatenation. Use for streams.

```typescript
import { Chunk, Equal } from 'effect';

Chunk.empty<number>();
Chunk.make(1, 2, 3);
Chunk.fromIterable([1, 2, 3]);

Chunk.append(numbers, 4); // [1, 2, 3, 4]
Chunk.appendAll(Chunk.make(1, 2), Chunk.make(3, 4)); // [1, 2, 3, 4]
Chunk.drop(Chunk.make(1, 2, 3, 4), 2); // [3, 4]
Chunk.take(Chunk.make(1, 2, 3, 4), 2); // [1, 2]
Chunk.toReadonlyArray(chunk); // Convert to array

Equal.equals(Chunk.make(1, 2), Chunk.make(1, 2)); // true (structural)
```

---

## Duration

Non-negative time spans for timeouts, delays, scheduling.

```typescript
import { Duration, Effect } from 'effect';

Duration.millis(100);
Duration.seconds(2);
Duration.minutes(5);
Duration.hours(7);
Duration.infinity;

// Decoding (flexible input)
Duration.decode(1000); // 1 second (from ms)
Duration.decode('5 minutes'); // 5 minutes

// Operations
Duration.toMillis(Duration.seconds(5)); // 5000
Duration.lessThan(Duration.seconds(1), Duration.seconds(2)); // true
Duration.sum(Duration.seconds(30), Duration.seconds(30)); // 60 seconds

// With Effect
Effect.timeout(fetchData, Duration.seconds(30));
```

---

## Data Module

Create data structures with built-in structural equality.

### Value Constructors

```typescript
import { Data, Equal } from 'effect';

// Struct - one-off data with equality
const alice = Data.struct({ name: 'Alice', age: 30 });
Equal.equals(alice, Data.struct({ name: 'Alice', age: 30 })); // true

Data.tuple('a', 1); // Tuple with equality
Data.array([1, 2, 3]); // Array with equality
```

### Case Classes

```typescript
// Data.case - reusable factory
interface Person {
  readonly name: string;
  readonly age: number;
}
const Person = Data.case<Person>();
Person({ name: 'Alice', age: 30 });

// Data.tagged - adds _tag automatically
const User = Data.tagged<{ _tag: 'User'; name: string }>('User');
User({ name: 'Alice' }); // { _tag: "User", name: "Alice" }

// Data.Class - with custom methods
class Person extends Data.Class<{ name: string; age: number }> {}

// Data.TaggedClass - class with _tag
class User extends Data.TaggedClass('User')<{ name: string }> {}
```

### Tagged Enums

```typescript
type RemoteData = Data.TaggedEnum<{
  Loading: {};
  Success: { readonly data: string };
  Failure: { readonly reason: string };
}>;
const { Loading, Success, Failure } = Data.taggedEnum<RemoteData>();
const RemoteData = Data.taggedEnum<RemoteData>();

RemoteData.$is('Success')(value); // Type guard
RemoteData.$match({
  Loading: () => '...',
  Success: ({ data }) => data,
  Failure: ({ reason }) => reason,
});
```

### Tagged Errors

```typescript
class NotFound extends Schema.TaggedError<NotFound>()('NotFound', {
  message: Schema.String,
  file: Schema.String,
}) {}

// Yieldable in Effect.gen, works with catchTag
Effect.gen(function* () {
  yield* new NotFound({ message: 'Missing', file: 'x.txt' });
}).pipe(Effect.catchTag('NotFound', (err) => Effect.succeed(null)));
```

---

## Type Selection

| Need                   | Use                          |
| ---------------------- | ---------------------------- |
| Optional value         | `Option`                     |
| Simple success/failure | `Either`                     |
| Rich error analysis    | `Cause` / `Exit`             |
| Repeated concatenation | `Chunk`                      |
| Time spans/timeouts    | `Duration`                   |
| Value equality         | `Data.struct` / `Data.Class` |
| Discriminated unions   | `Data.TaggedEnum`            |
| Custom errors          | `Schema.TaggedError`         |
