# Effect Sink Reference

Sinks consume Stream elements and produce aggregated results. Connect via `Stream.run(stream, sink)`.

## Quick Reference

| Sink                     | Purpose                  | Result Type |
| ------------------------ | ------------------------ | ----------- |
| `Sink.collectAll()`      | Collect all elements     | `Chunk<A>`  |
| `Sink.head()`            | First element            | `Option<A>` |
| `Sink.last()`            | Last element             | `Option<A>` |
| `Sink.take(n)`           | First n elements         | `Chunk<A>`  |
| `Sink.count`             | Count elements           | `number`    |
| `Sink.sum`               | Sum numbers              | `number`    |
| `Sink.foldLeft(init, f)` | Reduce to single value   | `S`         |
| `Sink.forEach(f)`        | Side effects per element | `void`      |
| `Sink.drain`             | Discard all elements     | `void`      |

## Type Signature

```typescript
Sink<A, In, L, E, R>;
//    A   - Result type after processing
//    In  - Input element type from Stream
//    L   - Leftover elements (unconsumed)
//    E   - Error type
//    R   - Required dependencies
```

---

## Collection Sinks

```typescript
import { Stream, Sink, Effect } from 'effect';

// Collect all elements
Stream.make(1, 2, 3).pipe(Stream.run(Sink.collectAll()));
// Chunk([1, 2, 3])

// Collect first n elements
Stream.make(1, 2, 3, 4, 5).pipe(Stream.run(Sink.take<number>(3)));
// Chunk([1, 2, 3])

// Collect while predicate holds
Stream.make(1, 2, 3, 4, 5).pipe(
  Stream.run(Sink.collectAllWhile<number>((n) => n < 4))
);
// Chunk([1, 2, 3])

// Collect into Set (deduplicated)
Stream.make(1, 2, 2, 3, 3).pipe(Stream.run(Sink.collectAllToSet()));
// HashSet([1, 2, 3])

// Collect into Map with key function and merge
Stream.make({ id: 'a', v: 1 }, { id: 'b', v: 2 }, { id: 'a', v: 3 }).pipe(
  Stream.run(
    Sink.collectAllToMap(
      (item) => item.id, // Key function
      (v1, v2) => v1 + v2 // Merge duplicates
    )
  )
);
// HashMap([["a", 4], ["b", 2]])
```

---

## Folding Sinks

### foldLeft - Simple Reduction

```typescript
import { Stream, Sink, Effect } from 'effect';

// Sum all numbers
Stream.make(1, 2, 3, 4, 5).pipe(
  Stream.run(Sink.foldLeft(0, (acc, n) => acc + n))
);
// 15

// Build array
Stream.make('a', 'b', 'c').pipe(
  Stream.run(Sink.foldLeft<string, string[]>([], (acc, s) => [...acc, s]))
);
// ["a", "b", "c"]
```

### fold - With Early Termination

```typescript
import { Stream, Sink, Effect } from 'effect';

// Sum until reaching 10
Stream.make(1, 2, 3, 4, 5, 6, 7, 8).pipe(
  Stream.run(
    Sink.fold(
      0, // Initial state
      (sum) => sum < 10, // Continue while true
      (acc, n) => acc + n // Accumulator
    )
  )
);
// 10 (1+2+3+4=10, stops)
```

### foldUntil - Fixed Element Count

```typescript
import { Stream, Sink, Effect } from 'effect';

// Fold exactly 3 elements
Stream.make(1, 2, 3, 4, 5).pipe(
  Stream.run(Sink.foldUntil(0, 3, (acc, n) => acc + n))
);
// 6 (1+2+3)
```

### foldWeighted - Cost-Based Accumulation

```typescript
import { Stream, Sink, Effect } from 'effect';

// Accumulate strings until total length exceeds 10
Stream.make('hello', 'world', 'foo', 'bar').pipe(
  Stream.run(
    Sink.foldWeighted({
      initial: [],
      maxCost: 10,
      cost: (_, str) => str.length,
      body: (acc, str) => [...acc, str],
    })
  )
);
// ["hello", "world"] (5+5=10)
```

---

## Side Effect Sinks

### forEach - Process Each Element

```typescript
import { Stream, Sink, Effect, Console } from 'effect';

Stream.make(1, 2, 3).pipe(
  Stream.run(Sink.forEach((n) => Console.log(`Processing: ${n}`)))
);
// Logs: Processing: 1, Processing: 2, Processing: 3

// With effectful operations
Stream.make('user1', 'user2').pipe(
  Stream.run(
    Sink.forEach((userId) =>
      Effect.gen(function* () {
        yield* saveToDatabase(userId);
        yield* Console.log(`Saved ${userId}`);
      })
    )
  )
);
```

### forEachChunk - Batch Processing

```typescript
import { Stream, Sink, Effect, Console } from 'effect';

Stream.make(1, 2, 3, 4, 5).pipe(
  Stream.run(
    Sink.forEachChunk((chunk) =>
      Console.log(`Batch: ${JSON.stringify([...chunk])}`)
    )
  )
);
```

### drain - Consume Without Result

```typescript
import { Stream, Sink, Effect } from 'effect';

Stream.make(1, 2, 3).pipe(
  Stream.tap((n) => Effect.log(`Side effect: ${n}`)),
  Stream.run(Sink.drain)
);
// Runs side effects, returns void
```

---

## Concurrent Sinks

### zip - Combine Results

```typescript
import { Stream, Sink, Effect } from 'effect';

// Run sum and count concurrently
const combinedSink = Sink.zip(Sink.sum, Sink.count, { concurrent: true });

Stream.make(1, 2, 3, 4, 5).pipe(Stream.run(combinedSink));
// [15, 5]
```

### zipWith - Combine and Transform

```typescript
import { Stream, Sink, Effect } from 'effect';

// Calculate average
const averageSink = Sink.zipWith(
  Sink.sum,
  Sink.count,
  (sum, count) => sum / count,
  { concurrent: true }
);

Stream.make(1, 2, 3, 4, 5).pipe(Stream.run(averageSink));
// 3
```

### Multiple Aggregates Pattern

```typescript
import { Stream, Sink, Effect } from 'effect';

const statsSink = Sink.zip(
  Sink.zip(Sink.count, Sink.sum, { concurrent: true }),
  Sink.zip(
    Sink.foldLeft(Infinity, Math.min),
    Sink.foldLeft(-Infinity, Math.max),
    { concurrent: true }
  ),
  { concurrent: true }
).pipe(
  Sink.map(([[count, sum], [min, max]]) => ({
    count,
    sum,
    min: count > 0 ? min : 0,
    max: count > 0 ? max : 0,
    average: count > 0 ? sum / count : 0,
  }))
);

Stream.make(1, 5, 3, 8, 2).pipe(Stream.run(statsSink));
// { count: 5, sum: 19, min: 1, max: 8, average: 3.8 }
```

### race - First to Complete Wins

```typescript
import { Stream, Sink, Effect } from 'effect';

const racingSink = Sink.race(
  Sink.take<number>(100), // Takes longer
  Sink.head<number>() // Completes immediately
);

Stream.make(1, 2, 3, 4, 5).pipe(Stream.run(racingSink));
// Some(1) - head finishes first
```

---

## Transformation Operations

| Operation                       | Purpose                           |
| ------------------------------- | --------------------------------- |
| `Sink.map(f)`                   | Transform result                  |
| `Sink.mapEffect(f)`             | Transform result with Effect      |
| `Sink.mapInput(f)`              | Transform input before processing |
| `Sink.filterInput(pred)`        | Filter inputs                     |
| `Sink.flatMap(f)`               | Chain sinks sequentially          |
| `Sink.dimap({onInput, onDone})` | Transform both input and output   |

```typescript
import { Stream, Sink, Effect } from 'effect';

// Transform input: strings to numbers
const stringSum = Sink.sum.pipe(Sink.mapInput((s: string) => parseInt(s, 10)));
Stream.make('1', '2', '3').pipe(Stream.run(stringSum));
// 6

// Transform output
const doubledSum = Sink.sum.pipe(Sink.map((sum) => sum * 2));
Stream.make(1, 2, 3).pipe(Stream.run(doubledSum));
// 12

// Filter inputs
const positiveSum = Sink.sum.pipe(Sink.filterInput((n: number) => n > 0));
Stream.make(-1, 2, -3, 4).pipe(Stream.run(positiveSum));
// 6
```

---

## Leftovers Handling

Leftovers are unconsumed elements when a sink completes early.

```typescript
import { Stream, Sink, Effect } from 'effect';

// Collect result AND leftovers
Stream.make(1, 2, 3, 4, 5).pipe(
  Stream.run(Sink.take<number>(3).pipe(Sink.collectLeftover))
);
// [Chunk([1, 2, 3]), Chunk([4, 5])]

// Ignore leftovers explicitly
Stream.make(1, 2, 3, 4, 5).pipe(
  Stream.run(Sink.take<number>(3).pipe(Sink.ignoreLeftover))
);
// Chunk([1, 2, 3])
```

---

## Common Patterns

### Write to External Service

```typescript
import { Stream, Sink, Effect } from 'effect';

const writeToService = (
  sendBatch: (items: string[]) => Effect.Effect<void, Error>
) =>
  Sink.foldWeighted<string, string[]>({
    initial: [],
    maxCost: 100, // Batch size
    cost: () => 1,
    body: (acc, item) => [...acc, item],
  }).pipe(
    Sink.mapEffect((batch) =>
      batch.length > 0 ? sendBatch(batch) : Effect.void
    )
  );
```

### Find First Matching Element

```typescript
import { Stream, Sink, Effect, Option } from 'effect';

const findFirst = <A>(predicate: (a: A) => boolean) =>
  Sink.fold<Option.Option<A>, A>(Option.none(), Option.isNone, (_, a) =>
    predicate(a) ? Option.some(a) : Option.none()
  );

Stream.make(1, 3, 5, 6, 7).pipe(
  Stream.run(findFirst((n: number) => n % 2 === 0))
);
// Some(6)
```

### Validate and Collect

```typescript
import { Stream, Sink, Effect, Either } from 'effect';

class ValidationError {
  readonly _tag = 'ValidationError';
  constructor(readonly message: string) {}
}

const collectValid = <A>(validate: (a: A) => boolean) =>
  Sink.foldLeft<A, A[]>([], (acc, item) =>
    validate(item) ? [...acc, item] : acc
  );

Stream.make(1, -2, 3, -4, 5).pipe(
  Stream.run(collectValid((n: number) => n > 0))
);
// [1, 3, 5]
```
