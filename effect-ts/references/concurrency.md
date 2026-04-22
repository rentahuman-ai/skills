# Effect Concurrency Reference

## Quick Reference

| Primitive    | Purpose              | Key Functions                       |
| ------------ | -------------------- | ----------------------------------- |
| `Fiber`      | Background execution | `fork`, `join`, `interrupt`         |
| `Ref`        | Shared mutable state | `make`, `get`, `update`, `modify`   |
| `Queue`      | Producer-consumer    | `bounded`, `offer`, `take`          |
| `Deferred`   | One-time signal      | `make`, `succeed`, `await`          |
| `Semaphore`  | Rate limiting        | `makeSemaphore`, `withPermits`      |
| `Effect.all` | Parallel execution   | `{ concurrency: N \| "unbounded" }` |

## Fiber

```typescript
import { Effect, Fiber } from 'effect';

const program = Effect.gen(function* () {
  const fiber = yield* Effect.fork(someEffect); // Start background
  const result = yield* Fiber.join(fiber); // Wait for result
  yield* Fiber.interrupt(fiber); // Cancel
  const exit = yield* Fiber.await(fiber); // Get Exit value
  const combined = Fiber.zip(fiber1, fiber2); // Combine fibers
});
```

| Fork Variant        | Behavior                                 |
| ------------------- | ---------------------------------------- |
| `Effect.fork`       | Child terminates with parent             |
| `Effect.forkDaemon` | Survives parent, runs until app shutdown |
| `Effect.forkScoped` | Tied to specific scope                   |

## Ref

```typescript
import { Effect, Ref } from 'effect';

const program = Effect.gen(function* () {
  const counter = yield* Ref.make(0);
  const value = yield* Ref.get(counter);
  yield* Ref.set(counter, 5);
  yield* Ref.update(counter, (n) => n + 1);
  // Atomic read-modify-write: returns [returnValue, newState]
  const old = yield* Ref.modify(counter, (n) => [n, n + 1]);
});

// Ref as Service
class AppState extends Context.Tag('AppState')<
  AppState,
  Ref.Ref<{ count: number }>
>() {}
const increment = Effect.flatMap(AppState, (s) =>
  Ref.update(s, (x) => ({ ...x, count: x.count + 1 }))
);
const runnable = program.pipe(
  Effect.provideServiceEffect(AppState, Ref.make({ count: 0 }))
);
```

## Queue

| Type                | Behavior at Capacity         |
| ------------------- | ---------------------------- |
| `Queue.bounded(n)`  | `offer` suspends until space |
| `Queue.unbounded()` | Unlimited growth             |
| `Queue.dropping(n)` | New values discarded         |
| `Queue.sliding(n)`  | Old values removed           |

```typescript
import { Effect, Queue, Fiber } from 'effect';

const program = Effect.gen(function* () {
  const queue = yield* Queue.bounded<number>(10);
  yield* Queue.offer(queue, 1); // Suspends if full
  yield* Queue.offerAll(queue, [2, 3]);
  const item = yield* Queue.take(queue); // Suspends if empty
  const maybe = yield* Queue.poll(queue); // Option<A>, non-blocking
  const batch = yield* Queue.takeUpTo(queue, 5);

  // Producer-consumer pattern
  const producer = yield* Effect.fork(
    Effect.forEach([1, 2, 3], (n) => Queue.offer(queue, n))
  );
  const consumer = yield* Effect.fork(
    Effect.gen(function* () {
      let sum = 0;
      for (let i = 0; i < 3; i++) sum += yield* Queue.take(queue);
      return sum;
    })
  );
  const result = yield* Fiber.join(consumer);
});
```

## Deferred

One-time signal for fiber coordination.

```typescript
import { Effect, Deferred, Fiber } from 'effect';

const program = Effect.gen(function* () {
  const signal = yield* Deferred.make<string, never>();
  const waiter = yield* Effect.fork(Deferred.await(signal));
  yield* Effect.sleep('100 millis');
  yield* Deferred.succeed(signal, 'ready'); // First completion wins
  const result = yield* Fiber.join(waiter);
});
```

| Method                       | Purpose               |
| ---------------------------- | --------------------- |
| `Deferred.succeed(d, value)` | Complete with success |
| `Deferred.fail(d, error)`    | Complete with error   |
| `Deferred.interrupt(d)`      | Interrupt all waiters |

## Semaphore

```typescript
import { Effect } from 'effect';

const program = Effect.gen(function* () {
  const semaphore = yield* Effect.makeSemaphore(3);
  const task = (id: number) =>
    semaphore.withPermits(1)(
      Effect.log(`Task ${id} running`).pipe(
        Effect.andThen(Effect.sleep('1 second'))
      )
    );
  // Run 10 tasks, max 3 concurrent
  yield* Effect.all(
    Array.from({ length: 10 }, (_, i) => task(i)),
    { concurrency: 'unbounded' }
  );
});

// Mutex pattern: single permit
const mutex = yield * Effect.makeSemaphore(1);
const critical = mutex.withPermits(1)(Effect.log('exclusive access'));
```

## Parallel Operations

### Effect.all

| Option                         | Behavior         |
| ------------------------------ | ---------------- |
| (none)                         | Sequential       |
| `{ concurrency: N }`           | Max N concurrent |
| `{ concurrency: "unbounded" }` | All parallel     |

```typescript
import { Effect } from 'effect';

yield * Effect.all([t1, t2, t3]); // Sequential
yield * Effect.all([t1, t2, t3], { concurrency: 2 }); // Max 2
yield * Effect.all([t1, t2, t3], { concurrency: 'unbounded' }); // All parallel
yield * Effect.all(tasks, { concurrency: 'unbounded', discard: true }); // void

// Preserves structure
const [a, b] = yield * Effect.all([Effect.succeed(1), Effect.succeed('hi')]);
const { x, y } =
  yield * Effect.all({ x: Effect.succeed(1), y: Effect.succeed(2) });

// forEach with concurrency
const users =
  yield * Effect.forEach(userIds, (id) => fetchUser(id), { concurrency: 3 });
```

### Effect.race

```typescript
// First to complete wins, others interrupted
const result =
  yield *
  Effect.race(
    Effect.delay(Effect.succeed('slow'), '1 second'),
    Effect.delay(Effect.succeed('fast'), '100 millis')
  );
const fastest = yield * Effect.raceAll([server1, server2, server3]);

// Timeout pattern
const withTimeout = Effect.race(
  slowOp,
  Effect.delay(Effect.fail('timeout'), '5 seconds')
);
```

## Resource Management

```typescript
import { Effect, Exit } from 'effect';

// acquireRelease: cleanup always runs
const managed = Effect.acquireRelease(Effect.succeed({ id: 1 }), (conn) =>
  Effect.log(`Closing ${conn.id}`)
);

// Exit-aware cleanup
const transaction = Effect.acquireRelease(createTx(), (tx, exit) =>
  Exit.isFailure(exit) ? rollback(tx) : commit(tx)
);

const program = Effect.scoped(
  Effect.gen(function* () {
    const conn = yield* managed; // Auto-closed when scope ends
  })
);

// Uninterruptible region
const critical = Effect.uninterruptible(sensitiveOp);

// Interrupt handler
const withCleanup = someEffect.pipe(
  Effect.onInterrupt(() => Effect.log('Cleaning up...'))
);
```

## Common Patterns

### Worker Pool

```typescript
const workerPool = Effect.gen(function* () {
  const queue = yield* Queue.bounded<Task>(100);
  const semaphore = yield* Effect.makeSemaphore(4);
  yield* Effect.all(
    Array.from({ length: 4 }, () =>
      Effect.fork(
        Effect.forever(
          semaphore.withPermits(1)(
            Effect.flatMap(Queue.take(queue), processTask)
          )
        )
      )
    ),
    { discard: true }
  );
  return queue;
});
```

### Coordinated Shutdown

```typescript
const program = Effect.gen(function* () {
  const shutdown = yield* Deferred.make<void, never>();
  const service = yield* Effect.fork(
    Effect.gen(function* () {
      yield* Effect.addFinalizer(() => Effect.log('Service stopped'));
      yield* Deferred.await(shutdown);
    })
  );
  yield* Effect.sleep('2 seconds');
  yield* Deferred.succeed(shutdown, undefined);
  yield* Fiber.join(service);
});
```
