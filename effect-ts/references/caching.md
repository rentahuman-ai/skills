# Effect Caching Reference

## Quick Reference

| Function                         | Duration     | Invalidation | Use Case                    |
| -------------------------------- | ------------ | ------------ | --------------------------- |
| `Effect.cachedWithTTL`           | Time-limited | No           | Periodically changing data  |
| `Effect.cachedInvalidateWithTTL` | Time-limited | Yes          | Cache with manual refresh   |
| `Effect.cachedFunction`          | Permanent    | No           | Memoize functions by input  |
| `Effect.once`                    | Permanent    | No           | One-time initialization     |
| `Cache.make`                     | Time-limited | Yes          | Full-featured keyed cache   |
| `ScopedCache.make`               | Time-limited | Yes          | Cache with resource cleanup |

## Effect-Level Caching

### Effect.cachedWithTTL

```typescript
import { Effect, Duration } from 'effect';

const fetchUserData = Effect.gen(function* () {
  yield* Effect.log('Fetching from API...');
  return { id: 1, name: 'Alice', timestamp: Date.now() };
});

const cachedUserData = Effect.cachedWithTTL(fetchUserData, Duration.minutes(5));

const program = Effect.gen(function* () {
  const getUserData = yield* cachedUserData;
  const user1 = yield* getUserData; // Fetches from API
  const user2 = yield* getUserData; // Returns cached value
});
```

### Effect.cachedInvalidateWithTTL

```typescript
import { Effect, Duration } from 'effect';

// Returns tuple: [cachedEffect, invalidateEffect]
const cachedConfig = Effect.cachedInvalidateWithTTL(
  Effect.succeed({ apiUrl: 'https://api.example.com' }),
  Duration.minutes(10)
);

const program = Effect.gen(function* () {
  const [getConfig, invalidateConfig] = yield* cachedConfig;
  const config1 = yield* getConfig;
  yield* invalidateConfig; // Force refresh
  const config2 = yield* getConfig; // Recomputes
});
```

### Effect.cachedFunction

```typescript
import { Effect } from 'effect';

const expensiveCalc = (input: number) => Effect.succeed(input * input);

const program = Effect.gen(function* () {
  const memoized = yield* Effect.cachedFunction(expensiveCalc);
  const r1 = yield* memoized(5); // Computes: 25
  const r2 = yield* memoized(5); // Cached: 25
  const r3 = yield* memoized(10); // Computes: 100
});
```

### Effect.once

```typescript
import { Effect } from 'effect';

const initOnce = Effect.once(Effect.log('Initializing...'));

const program = Effect.gen(function* () {
  const init = yield* initOnce;
  yield* Effect.all([init, init, init], { concurrency: 'unbounded' }); // Runs once
});
```

## Cache Module

### Cache.make

```typescript
import { Effect, Cache, Duration } from 'effect';

const program = Effect.gen(function* () {
  const userCache = yield* Cache.make({
    capacity: 100,
    timeToLive: Duration.minutes(5),
    lookup: (userId: string) =>
      Effect.succeed({ id: userId, name: `User ${userId}` }),
  });

  const user1 = yield* userCache.get('alice'); // Triggers lookup
  const user2 = yield* userCache.get('alice'); // Returns cached
});
```

### Cache Methods

| Method            | Behavior                                           |
| ----------------- | -------------------------------------------------- |
| `get(key)`        | Return cached or compute via lookup                |
| `getOption(key)`  | Return `Option`, no lookup triggered               |
| `set(key, value)` | Manually set value, bypass lookup                  |
| `refresh(key)`    | Recompute in background, serve old value meanwhile |
| `invalidate(key)` | Remove specific entry                              |
| `invalidateAll`   | Clear entire cache                                 |
| `cacheStats`      | Get hits, misses, size metrics                     |

```typescript
import { Effect, Cache, Duration, Option } from 'effect';

const program = Effect.gen(function* () {
  const cache = yield* Cache.make({
    capacity: 100,
    timeToLive: Duration.minutes(5),
    lookup: (key: string) => Effect.succeed(`value-${key}`),
  });

  yield* cache.set('config', 'value'); // Pre-populate
  const maybe = yield* cache.getOption('key'); // Check without lookup
  yield* cache.refresh('config'); // Background refresh
  yield* cache.invalidate('config'); // Remove entry
  yield* cache.invalidateAll; // Clear all

  const stats = yield* cache.cacheStats;
  yield* Effect.log(`Hits: ${stats.hits}, Misses: ${stats.misses}`);
});
```

### Thundering Herd Protection

Multiple concurrent requests for same key trigger only one lookup:

```typescript
const program = Effect.gen(function* () {
  let lookupCount = 0;
  const cache = yield* Cache.make({
    capacity: 100,
    timeToLive: Duration.minutes(5),
    lookup: (key: string) =>
      Effect.sync(() => {
        lookupCount++;
        return `value-${key}`;
      }),
  });

  yield* Effect.all(
    Array.from({ length: 100 }, () => cache.get('shared')),
    { concurrency: 'unbounded' }
  );
  yield* Effect.log(`Total lookups: ${lookupCount}`); // 1
});
```

### Exit-Dependent TTL

```typescript
import { Cache, Duration, Effect, Exit } from 'effect';

const cache = Cache.makeWith({
  capacity: 100,
  lookup: (key: string) =>
    key === 'error'
      ? Effect.fail(new Error('Failed'))
      : Effect.succeed(`value-${key}`),
  timeToLive: (exit) =>
    Exit.isSuccess(exit) ? Duration.minutes(5) : Duration.seconds(10),
});
```

## ScopedCache

For caching resources requiring cleanup (connections, file handles):

```typescript
import { Effect, ScopedCache, Duration, Scope } from 'effect';

const acquireConnection = (
  host: string
): Effect.Effect<Connection, Error, Scope.Scope> =>
  Effect.acquireRelease(
    Effect.succeed({
      id: `conn-${Date.now()}`,
      close: () => Effect.log('Closing'),
    }),
    (conn) => conn.close()
  );

const program = Effect.scoped(
  Effect.gen(function* () {
    const connCache = yield* ScopedCache.make({
      capacity: 10,
      timeToLive: Duration.minutes(5),
      lookup: acquireConnection,
    });
    const conn1 = yield* connCache.get('postgres://db1');
    const conn2 = yield* connCache.get('postgres://db1'); // Same instance
    // All connections auto-released when scope closes
  })
);
```

## Invalidation Patterns

### Event-Driven Invalidation

```typescript
import { Effect, Cache, Duration, Queue } from 'effect';

const program = Effect.gen(function* () {
  const cache = yield* Cache.make({
    capacity: 100,
    timeToLive: Duration.minutes(10),
    lookup: fetchUser,
  });
  const eventQueue = yield* Queue.unbounded<{ userId: string }>();

  yield* Effect.forkDaemon(
    Effect.forever(
      Effect.flatMap(Queue.take(eventQueue), (event) =>
        cache.invalidate(event.userId)
      )
    )
  );
  return { cache, eventQueue };
});
```

### Periodic Auto-Refresh

```typescript
import { Effect, Cache, Duration } from 'effect';

const program = Effect.gen(function* () {
  const cache = yield* Cache.make({
    capacity: 10,
    timeToLive: Duration.minutes(5),
    lookup: fetchConfig,
  });

  yield* Effect.forkDaemon(
    Effect.forever(
      Effect.flatMap(Effect.sleep(Duration.minutes(4)), () =>
        Effect.flatMap(cache.keys, (keys) =>
          Effect.forEach(keys, (k) => cache.refresh(k), { concurrency: 5 })
        )
      )
    )
  );
  return cache;
});
```

## Layer-Based Caching

Wrap any service with caching transparently:

```typescript
import { Effect, Layer, Cache, Duration, Context } from 'effect';

interface UserRepository {
  readonly findById: (id: string) => Effect.Effect<User | null, Error>;
}
class UserRepository extends Context.Tag('UserRepository')<
  UserRepository,
  UserRepository
>() {}

const UserRepositoryLive = Layer.succeed(UserRepository, {
  findById: (id) => Effect.succeed({ id, name: `User ${id}` }),
});

const UserRepositoryCached = Layer.effect(
  UserRepository,
  Effect.gen(function* () {
    const repo = yield* UserRepository;
    const cache = yield* Cache.make({
      capacity: 100,
      timeToLive: Duration.minutes(5),
      lookup: repo.findById,
    });
    return UserRepository.of({ findById: (id) => cache.get(id) });
  })
).pipe(Layer.provide(UserRepositoryLive));
```
