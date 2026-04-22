# Effect Streams Reference

## Quick Reference

| Operation            | Function                                  | Use Case                   |
| -------------------- | ----------------------------------------- | -------------------------- |
| Create from values   | `Stream.make(1, 2, 3)`                    | Fixed sequences            |
| Create from array    | `Stream.fromIterable([...])`              | Convert collections        |
| Create from effect   | `Stream.fromEffect(eff)`                  | Single async value         |
| Create from callback | `Stream.async(emit => ...)`               | Event sources, WebSockets  |
| Paginated API        | `Stream.paginateEffect(init, fn)`         | Cursor-based fetching      |
| Transform            | `Stream.map(fn)`                          | Sync transformation        |
| Transform async      | `Stream.mapEffect(fn, {concurrency})`     | Parallel processing        |
| Filter               | `Stream.filter(pred)`                     | Keep matching elements     |
| Flatten              | `Stream.flatMap(fn)`                      | Nested streams             |
| Cancel previous      | `Stream.flatMap(fn, {switch: true})`      | Search/typeahead           |
| Side effects         | `Stream.tap(fn)`                          | Logging/debugging          |
| Batch by size        | `Stream.grouped(n)`                       | Fixed-size batches         |
| Batch by time        | `Stream.groupedWithin(n, duration)`       | Time-windowed batches      |
| Collect all          | `Stream.runCollect`                       | Get all as Chunk           |
| Process each         | `Stream.runForEach(fn)`                   | Side effects               |
| Accumulate           | `Stream.runFold(init, fn)`                | Reduce to single value     |
| Discard values       | `Stream.drain`                            | Execute effects only       |
| Handle errors        | `Stream.catchAll(fn)`                     | Error recovery             |
| Retry                | `Stream.retry(schedule)`                  | Automatic retry            |
| Resource cleanup     | `Stream.acquireRelease(acquire, release)` | Safe resource handling     |
| Rate limiting        | `Stream.throttle({...})`                  | Token bucket rate control  |
| Debounce             | `Stream.debounce(duration)`               | Wait for pause in input    |
| Buffer               | `Stream.buffer({capacity})`               | Decouple producer/consumer |
| Broadcast            | `Stream.broadcast(n, {maximumLag})`       | Fan-out to consumers       |
| Partition            | `Stream.partition(pred)`                  | Split by condition         |

## Type Signature

```typescript
Stream<A, E, R>;
// A: emitted value type
// E: error type
// R: required context/environment
```

---

## Creating Streams

### From Values

```typescript
import { Stream, Effect, Option, Chunk } from 'effect';

// Fixed values
const s1 = Stream.make(1, 2, 3);

// Range (inclusive)
const s2 = Stream.range(1, 5); // 1, 2, 3, 4, 5

// From array/set
const s3 = Stream.fromIterable([1, 2, 3]);

// From single effect
const s4 = Stream.fromEffect(Effect.succeed(42));
```

### From Async Sources

```typescript
// Callback-based (events, WebSockets)
const eventStream = Stream.async<string, Error>((emit) => {
  socket.on('message', (msg) => {
    emit(Effect.succeed(Chunk.of(msg)));
  });
  socket.on('close', () => {
    emit(Effect.fail(Option.none())); // Signal completion
  });
  socket.on('error', (err) => {
    emit(Effect.fail(Option.some(err))); // Signal error
  });
});

// From async iterable
const s5 = Stream.fromAsyncIterable(
  asyncGenerator(),
  (e) => new Error(`Failed: ${e}`)
);
```

### Generation Patterns

```typescript
// Infinite sequence with function
const powers = Stream.iterate(1, (n) => n * 2); // 1, 2, 4, 8, ...

// State-based generation (terminates with Option.none)
const countdown = Stream.unfold(5, (n) => {
  if (n <= 0) return Option.none();
  const next: [number, number] = [n, n - 1];
  return Option.some(next);
}); // 5, 4, 3, 2, 1

// Paginated API
const pages = Stream.paginateEffect<string | null>(null, (cursor) =>
  fetchPage(cursor).pipe(
    Effect.map((page) => [
      page.items,
      page.nextCursor ? Option.some(page.nextCursor) : Option.none(),
    ])
  )
);

// Repeat effect until failure
const limited = Stream.repeatEffectOption(
  Effect.suspend(() =>
    hasMore() ? Effect.succeed(nextItem()) : Effect.fail(Option.none())
  )
);
```

### Scheduling

```typescript
// Emit at intervals
const ticks = Stream.tick('1 second');

// From schedule
const scheduled = Stream.fromSchedule(Schedule.spaced('500 millis'));
```

---

## Transformations

### Mapping

```typescript
// Sync transform
const doubled = stream.pipe(Stream.map((n) => n * 2));

// Async transform with concurrency
const fetched = ids.pipe(
  Stream.mapEffect((id) => fetchItem(id), { concurrency: 5 })
);

// Map with running state
const runningSum = stream.pipe(
  Stream.mapAccum(0, (acc, n) => [acc + n, acc + n])
);

// Map and flatten
const expanded = stream.pipe(
  Stream.mapConcat((n) => [n, n * 10]) // 1 -> [1, 10]
);
```

### Filtering

```typescript
// Keep matching
const evens = stream.pipe(Stream.filter((n) => n % 2 === 0));

// Emit only on change
const distinct = stream.pipe(Stream.changes);

// Take/drop
const first10 = stream.pipe(Stream.take(10));
const skip5 = stream.pipe(Stream.drop(5));
const whileSmall = stream.pipe(Stream.takeWhile((n) => n < 100));
```

### Nested Streams

```typescript
// Flatten nested streams
const flat = stream.pipe(Stream.flatMap((id) => fetchRelatedItems(id)));

// Cancel previous on new value (search/typeahead)
const searchResults = searchTerms.pipe(
  Stream.flatMap((term) => searchApi(term), { switch: true })
);
```

### Grouping and Batching

```typescript
// Fixed-size batches
const batches = stream.pipe(Stream.grouped(100));

// Time-windowed batches (emit when size OR time reached)
const windowed = stream.pipe(Stream.groupedWithin(100, '5 seconds'));

// Group by key
const byCategory = items.pipe(Stream.groupByKey((item) => item.category));

// Running accumulation (emits intermediate values)
const scan = stream.pipe(
  Stream.scan(0, (acc, n) => acc + n) // 0, 1, 3, 6, 10...
);
```

---

## Combining Streams

```typescript
// Merge (interleaved as produced)
const merged = Stream.merge(streamA, streamB);

// Concat (sequential)
const sequential = Stream.concat(first, second);

// Zip (pair elements)
const zipped = Stream.zip(names, ages); // [name, age] pairs

// Zip with function
const combined = Stream.zipWith(a, b, (x, y) => x + y);

// Add index
const indexed = stream.pipe(Stream.zipWithIndex); // [value, 0], [value, 1]...

// Interleave (alternating)
const alternating = Stream.interleave(odds, evens);
```

---

## Consumption

### Basic Runners

```typescript
// Collect all to Chunk
const all = await Effect.runPromise(stream.pipe(Stream.runCollect));

// Process each element
await Effect.runPromise(stream.pipe(Stream.runForEach((n) => Console.log(n))));

// Fold to single value
const sum = await Effect.runPromise(
  stream.pipe(Stream.runFold(0, (acc, n) => acc + n))
);
```

### Using Sinks

```typescript
import { Sink } from 'effect';

// Common sinks
stream.pipe(Stream.run(Sink.head)); // Option<first>
stream.pipe(Stream.run(Sink.last)); // Option<last>
stream.pipe(Stream.run(Sink.count)); // total count
stream.pipe(Stream.run(Sink.sum)); // sum of numbers
stream.pipe(Stream.run(Sink.take(5))); // first 5 as Chunk
stream.pipe(Stream.run(Sink.drain)); // discard all
stream.pipe(Stream.run(Sink.forEach(fn))); // apply effect to each
```

---

## Error Handling

```typescript
// Fallback stream on error
const withFallback = stream.pipe(Stream.orElse(() => fallbackStream));

// Handle specific errors
const handled = stream.pipe(
  Stream.catchAll((error) => {
    if (error._tag === 'NetworkError') return cachedStream;
    return Stream.fail(error);
  })
);

// Retry with schedule
const retried = stream.pipe(Stream.retry(Schedule.exponential('1 second')));

// Timeout
stream.pipe(Stream.timeout('5 seconds'));
stream.pipe(Stream.timeoutFail(() => new TimeoutError(), '5 seconds'));

// Cleanup on error
stream.pipe(Stream.onError((cause) => Effect.log(`Failed: ${cause}`)));
```

---

## Resource Management

```typescript
// Acquire-release pattern (guaranteed cleanup)
const fileStream = Stream.acquireRelease(
  Effect.sync(() => fs.openSync(path, 'r')),
  (fd) => Effect.sync(() => fs.closeSync(fd))
).pipe(Stream.flatMap((fd) => readFromFd(fd)));

// Scoped resource as single-element stream
const connStream = Stream.scoped(
  Effect.acquireRelease(openConnection(), (conn) => conn.close())
);

// Finalizer (runs before stream ends)
const withCleanup = Stream.concat(
  mainStream,
  Stream.finalizer(Effect.log('Done'))
);
```

---

## Practical Patterns

### Paginated API

```typescript
interface Page<T> {
  items: T[];
  nextCursor: string | null;
}

const fetchAllPages = <T>(
  fetchPage: (cursor: string | null) => Effect.Effect<Page<T>, Error>
): Stream.Stream<T, Error> =>
  Stream.paginateEffect<string | null>(null, (cursor) =>
    fetchPage(cursor).pipe(
      Effect.map((page) => [
        page.items,
        page.nextCursor ? Option.some(page.nextCursor) : Option.none(),
      ])
    )
  ).pipe(Stream.flatMap((items) => Stream.fromIterable(items)));
```

### Rate-Limited Processing

```typescript
const rateLimited = <T, R>(
  items: Stream.Stream<string, Error>,
  fetch: (id: string) => Effect.Effect<T, Error>
) =>
  items.pipe(
    Stream.mapEffect(fetch, { concurrency: 5 }),
    Stream.grouped(10),
    Stream.mapEffect((batch) =>
      Effect.succeed(batch).pipe(Effect.tap(() => Effect.sleep('1 second')))
    ),
    Stream.flatMap(Stream.fromIterable)
  );
```

### Search with Debounce

```typescript
const searchWithDebounce = (terms: Stream.Stream<string, never>) =>
  terms.pipe(
    Stream.debounce('300 millis'),
    Stream.flatMap((term) => searchApi(term), { switch: true }),
    Stream.catchAll(() => Stream.empty)
  );
```

---

## Rate Control

### Throttling

Throttle controls emission rate using a token bucket algorithm.

```typescript
// Shape strategy - delays elements to match rate
const shaped = stream.pipe(
  Stream.throttle({
    cost: (chunk) => chunk.length, // Cost per chunk
    duration: '1 second', // Time window
    units: 10, // Tokens per window
    burst: 5, // Extra burst capacity
    strategy: 'shape', // Delay excess elements
  })
);

// Enforce strategy - drops elements exceeding rate
const enforced = stream.pipe(
  Stream.throttle({
    units: 100,
    duration: '1 second',
    strategy: 'enforce', // Drop excess elements
  })
);
```

### Debouncing

Debounce emits only after a pause in input.

```typescript
// Wait for 300ms pause before emitting
const debounced = userInput.pipe(Stream.debounce('300 millis'));
```

### Schedule-Based Timing

Control emission timing with schedules.

```typescript
// Add delay between emissions
const spaced = stream.pipe(Stream.schedule(Schedule.spaced('100 millis')));
```

---

## Buffering and Broadcasting

### Buffer

Decouple producer from consumer with buffering.

```typescript
// Bounded buffer with backpressure
const buffered = stream.pipe(Stream.buffer({ capacity: 100 }));
```

### Broadcast

Fan out a stream to multiple consumers.

```typescript
const program = Effect.gen(function* () {
  // Create 2 downstream streams with max lag of 16
  const [stream1, stream2] = yield* Stream.broadcast(
    sourceStream,
    2,
    { maximumLag: 16 } // Backpressure when consumer lags
  );

  // Process in parallel
  yield* Effect.all(
    [
      stream1.pipe(Stream.runForEach(processA)),
      stream2.pipe(Stream.runForEach(processB)),
    ],
    { concurrency: 'unbounded' }
  );
});
```

---

## Partitioning

### By Predicate

Split stream into two based on condition.

```typescript
const program = Effect.gen(function* () {
  const [evens, odds] = yield* Stream.partition(
    Stream.range(1, 10),
    (n) => n % 2 === 0
  );

  const evenResults = yield* Stream.runCollect(evens);
  const oddResults = yield* Stream.runCollect(odds);
});
```

### With Either

Route elements to left or right streams.

```typescript
const [errors, successes] =
  yield *
  Stream.partitionEither(results, (result) =>
    result.ok
      ? Effect.succeed(Either.right(result.value))
      : Effect.succeed(Either.left(result.error))
  );
```

---

## Additional Operations

### Tapping (Side Effects)

Execute effects without altering stream values.

```typescript
const logged = stream.pipe(
  Stream.tap((elem) => Console.log(`Processing: ${elem}`))
);
```

### Draining

Execute effects and discard values.

```typescript
// Process side effects, return void
const drained = stream.pipe(
  Stream.tap((item) => saveToDatabase(item)),
  Stream.drain
);
```

### Interspersing

Insert elements between stream values.

```typescript
// Add delimiter
const csv = values.pipe(Stream.intersperse(','));

// With prefix and suffix
const json = values.pipe(
  Stream.intersperseAffixes({
    start: '[',
    middle: ',',
    end: ']',
  })
);
```

### Cross Product

Cartesian product of two streams.

```typescript
const pairs = Stream.cross(Stream.make(1, 2), Stream.make('a', 'b'));
// Emits: [1,"a"], [1,"b"], [2,"a"], [2,"b"]

// Warning: right stream re-executes for each left element
```

---

### CSV Processing

```typescript
const processCSV = (lines: Stream.Stream<string, Error>) =>
  lines.pipe(
    Stream.drop(1), // Skip header
    Stream.map((line) => line.split(',')),
    Stream.filter((cols) => cols.length >= 3),
    Stream.map(([name, age, email]) => ({
      name: name.trim(),
      age: parseInt(age.trim()),
      email: email.trim(),
    })),
    Stream.grouped(100),
    Stream.mapEffect((batch) => insertBatch(batch))
  );
```

---

## Key Concepts

- **Pull-based**: Consumers request data, providing automatic backpressure
- **Lazy**: Streams are descriptions; nothing executes until run
- **Chunked**: Emits arrays internally for efficiency (transparent to user)
- **Resource-safe**: Cleanup guaranteed on success, error, or interruption
