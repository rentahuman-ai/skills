# Effect Services & Layers Reference

## Quick Reference

| Pattern                | Use Case                        | Signature                                                      |
| ---------------------- | ------------------------------- | -------------------------------------------------------------- |
| `Context.Tag`          | Define service interface        | `class Svc extends Context.Tag("id")<Svc, Interface>() {}`     |
| `Effect.Service`       | Modern service with auto-layers | `class Svc extends Effect.Service<Svc>()("id", { effect }) {}` |
| `Layer.succeed`        | Static implementation           | `Layer.succeed(Tag, impl)`                                     |
| `Layer.effect`         | Effectful construction          | `Layer.effect(Tag, Effect.gen(...))`                           |
| `Layer.scoped`         | Resource with lifecycle         | `Layer.scoped(Tag, Effect.gen(...))`                           |
| `Layer.merge`          | Combine independent layers      | `Layer.merge(A, B)`                                            |
| `Layer.provide`        | Wire dependencies               | `Layer.provide(Consumer, Provider)`                            |
| `Layer.provideMerge`   | Provide and expose dependency   | `Layer.provideMerge(Consumer, Provider)`                       |
| `Layer.fresh`          | Prevent layer sharing           | `Layer.fresh(layer)`                                           |
| `Layer.memoize`        | Manual memoization control      | `Layer.memoize(layer)` (returns scoped Effect)                 |
| `Layer.catchAll`       | Error recovery                  | `Layer.catchAll(layer, handler)`                               |
| `Layer.launch`         | Long-running service            | `Layer.launch(layer)`                                          |
| `Effect.provide`       | Satisfy requirements            | `Effect.provide(effect, layer)`                                |
| `Effect.serviceOption` | Optional dependency             | `Effect.serviceOption(Tag)`                                    |
| Default Services       | Auto-provided                   | `Clock`, `Console`, `Random`, `ConfigProvider`, `Tracer`       |

---

## 1. Context.Tag (Basic Service Definition)

```typescript
import { Context, Effect } from 'effect';

class Random extends Context.Tag('app/Random')<
  Random, // Self type
  { readonly next: Effect.Effect<number> } // Service interface
>() {}

const program = Effect.gen(function* () {
  const random = yield* Random;
  return yield* random.next;
});
// Type: Effect<number, never, Random>
```

---

## 2. Effect.Service (Preferred Pattern)

```typescript
import { Effect } from 'effect';

class ToolkitConfig extends Effect.Service<ToolkitConfig>()('ToolkitConfig', {
  effect: Effect.gen(function* () {
    return {
      maxResults: parseInt(process.env.MAX_RESULTS || '100'),
      logLevel: process.env.LOG_LEVEL || 'info',
    };
  }),
}) {}
// Auto-generated: ToolkitConfig.Default, ToolkitConfig.DefaultWithoutDependencies
```

### With Dependencies

```typescript
class ToolkitLogger extends Effect.Service<ToolkitLogger>()('ToolkitLogger', {
  dependencies: [ToolkitConfig.Default],
  effect: Effect.gen(function* () {
    const config = yield* ToolkitConfig;
    return {
      info: Effect.fn('ToolkitLogger.info')(function* (msg: string) {
        yield* Effect.log(`[${config.logLevel}] ${msg}`);
      }),
    };
  }),
}) {}
```

### With Scoped Resources

```typescript
class DatabaseService extends Effect.Service<DatabaseService>()(
  'DatabaseService',
  {
    dependencies: [ToolkitLogger.Default],
    scoped: Effect.gen(function* () {
      const conn = yield* Effect.tryPromise(() => createConnection());
      yield* Effect.addFinalizer(() => Effect.promise(() => conn.close()));
      return {
        query: (sql: string) => Effect.tryPromise(() => conn.query(sql)),
      };
    }),
  }
) {}
```

| Method                                   | Use Case           |
| ---------------------------------------- | ------------------ |
| `succeed: { value: 42 }`                 | Static value       |
| `sync: () => ({...})`                    | Synchronous        |
| `effect: Effect.gen(...)`                | Async/effectful    |
| `scoped: Effect.gen(...) + addFinalizer` | Resource lifecycle |

---

## 3. Layer Creation

### Layer.succeed

```typescript
import { Layer, Context } from 'effect';

class Config extends Context.Tag('Config')<Config, { port: number }>() {}
const ConfigLive = Layer.succeed(Config, { port: 3000 });
```

### Layer.effect

```typescript
class Logger extends Context.Tag('Logger')<
  Logger,
  { log: (msg: string) => Effect.Effect<void> }
>() {}

const LoggerLive = Layer.effect(
  Logger,
  Effect.gen(function* () {
    const config = yield* Config;
    return {
      log: (msg) => Effect.sync(() => console.log(`[${config.port}] ${msg}`)),
    };
  })
);
// Type: Layer<Logger, never, Config>
```

### Layer.scoped

```typescript
const DatabaseLive = Layer.scoped(
  Database,
  Effect.gen(function* () {
    const conn = yield* Effect.tryPromise(() => createDatabaseConnection());
    yield* Effect.addFinalizer(() => Effect.promise(() => conn.close()));
    return { query: (sql: string) => Effect.tryPromise(() => conn.query(sql)) };
  })
);
```

---

## 4. Layer Composition

```typescript
// Merge independent layers
const AppConfig = Layer.merge(ConfigLive, RandomLive);
// Type: Layer<Config | Random, never, never>

// Merge many
const Full = Layer.mergeAll(ConfigLive, LoggerLive, CacheLive);

// Wire dependencies
const LoggerWithConfig = Layer.provide(LoggerLive, ConfigLive);
// Or: LoggerLive.pipe(Layer.provide(ConfigLive))

// Full app layer
const AppLayer = UserRepoLive.pipe(
  Layer.provide(DatabaseLive),
  Layer.provide(LoggerLive),
  Layer.provide(ConfigLive)
);
```

---

## 5. Providing Dependencies

```typescript
// Effect.provide with layer
const program = Effect.gen(function* () {
  const db = yield* Database;
  return yield* db.query('SELECT * FROM users');
});
const runnable = Effect.provide(program, DatabaseLive);
Effect.runPromise(runnable);

// Effect.provideService for single service
const runnable2 = Effect.provideService(program, Random, {
  next: Effect.sync(() => Math.random()),
});
```

### ManagedRuntime

```typescript
import { ManagedRuntime, Layer, Effect } from 'effect';

const AppLayer = Layer.mergeAll(ConfigLive, LoggerLive, DatabaseLive);
const runtime = ManagedRuntime.make(AppLayer);

export const runEffect = <A, E>(
  effect: Effect.Effect<A, E, Layer.Layer.Success<typeof AppLayer>>
): Promise<A> => runtime.runPromise(effect);

process.on('SIGTERM', () => runtime.dispose());
```

---

## 6. Testing

```typescript
// Mock layer
const UserRepoMock = Layer.succeed(UserRepo, {
  findById: (id: string) =>
    Effect.succeed(id === '1' ? { id: '1', name: 'Test' } : null),
});

// Use in tests
const result = await Effect.runPromise(
  Effect.gen(function* () {
    return yield* (yield* UserRepo).findById('1');
  }).pipe(Effect.provide(UserRepoMock))
);
```

---

## Common Mistakes

**1. Leaking dependencies in interface**

```typescript
// BAD
interface Bad {
  findById: (id: string) => Effect.Effect<User, Error, Logger | Database>;
}
// GOOD - clean interface, dependencies at layer construction
interface Good {
  findById: (id: string) => Effect.Effect<User, Error>;
}
```

**2. Missing dependencies array**

```typescript
// BAD - Config used but not declared
class Bad extends Effect.Service<Bad>()('Bad', {
  effect: Effect.gen(function* () {
    const config = yield* Config;
    return {};
  }),
}) {}

// GOOD
class Good extends Effect.Service<Good>()('Good', {
  dependencies: [Config.Default],
  effect: Effect.gen(function* () {
    const config = yield* Config;
    return {};
  }),
}) {}
```

**3. Forgetting Effect.scoped**

```typescript
// BAD - resource leak on error
const bad = Effect.gen(function* () {
  const conn = yield* db.connect();
  return yield* conn.query('...');
});

// GOOD - cleanup on success/failure/cancellation
const good = Effect.scoped(
  Effect.gen(function* () {
    const conn = yield* db.connect();
    return yield* conn.query('...');
  })
);
```

---

## 7. Default Services

Effect automatically provides 5 services without explicit provision:

```typescript
// type DefaultServices = Clock | ConfigProvider | Console | Random | Tracer

import { Effect, Clock, Random, Console } from 'effect';

const program = Effect.gen(function* () {
  const timestamp = yield* Clock.currentTimeMillis;
  const n = yield* Random.next;
  yield* Console.log(`Time: ${timestamp}, Random: ${n}`);
});
// Type: Effect<void, never, never> - no requirements!
```

### Overriding Default Services

```typescript
import { Effect, Random, TestServices } from 'effect';

// Override for a single effect
const withSeed = program.pipe(Effect.withRandom(Random.make('seed')));

// Scoped override (restores original when scope exits)
const scoped = Effect.scoped(
  Effect.gen(function* () {
    yield* Effect.withRandomScoped(Random.make('test-seed'));
    // Uses seeded random within this scope
    yield* program;
  })
);
```

| Override Function           | Scoped Variant                    | Purpose               |
| --------------------------- | --------------------------------- | --------------------- |
| `Effect.withClock`          | `Effect.withClockScoped`          | Custom Clock          |
| `Effect.withConsole`        | `Effect.withConsoleScoped`        | Custom Console        |
| `Effect.withRandom`         | `Effect.withRandomScoped`         | Custom Random         |
| `Effect.withConfigProvider` | `Effect.withConfigProviderScoped` | Custom ConfigProvider |
| `Effect.withTracer`         | `Effect.withTracerScoped`         | Custom Tracer         |

---

## 8. Layer Memoization

Layers are automatically memoized when provided globally. The same layer instance is shared across all dependents.

### Automatic Memoization (Global Provision)

```typescript
// When ALive appears multiple times, it's constructed ONCE
const MainLayer = Layer.mergeAll(
  Layer.provide(BLive, ALive),
  Layer.provide(CLive, ALive)
);
// ALive initialized only once, shared by B and C
```

### Layer.fresh (Opt Out of Sharing)

```typescript
import { Layer } from 'effect';

// Create separate instances instead of sharing
const FreshMain = Layer.mergeAll(
  Layer.provide(BLive, Layer.fresh(ALive)),
  Layer.provide(CLive, Layer.fresh(ALive))
);
// ALive initialized TWICE - one for B, one for C
```

### Layer.memoize (Manual Control)

For local provisions where automatic memoization doesn't apply:

```typescript
import { Effect, Layer } from 'effect';

const program = Effect.scoped(
  Layer.memoize(ExpensiveLayer).pipe(
    Effect.andThen((memoized) =>
      Effect.gen(function* () {
        yield* Effect.provide(effect1, memoized);
        yield* Effect.provide(effect2, memoized);
        // Both share the same ExpensiveLayer instance
      })
    )
  )
);
```

**Key principle:** Layers use reference equality for memoization. If you call a factory function multiple times, you get different instances that won't be memoized together.

---

## 9. Optional Services

Handle services that may or may not be provided:

```typescript
import { Effect, Option } from 'effect';

class Analytics extends Context.Tag('Analytics')<
  Analytics,
  { readonly track: (event: string) => Effect.Effect<void> }
>() {}

const program = Effect.gen(function* () {
  const maybeAnalytics = yield* Effect.serviceOption(Analytics);

  if (Option.isSome(maybeAnalytics)) {
    yield* maybeAnalytics.value.track('page_view');
  }
  // Analytics not available - skip tracking
});
// Type: Effect<void, never, never> - NO Analytics requirement!
```

---

## 10. Layer Error Handling

### Layer.catchAll (Recovery)

```typescript
import { Layer } from 'effect';

const database = postgresDatabaseLayer.pipe(
  Layer.catchAll((error) => inMemoryDatabaseLayer)
);
```

### Layer.tapError (Observe Failures)

```typescript
import { Layer, Console } from 'effect';

const logged = databaseLayer.pipe(
  Layer.tapError((err) => Console.error(`Layer construction failed: ${err}`))
);
```

---

## 11. Layer.launch (Long-Running Services)

Convert a layer to a long-running effect (useful for servers):

```typescript
import { Effect, Layer } from 'effect';

const HttpServerLayer = Layer.scoped(
  HttpServer,
  Effect.gen(function* () {
    yield* Effect.log('Starting HTTP server on port 3000');
    const server = yield* createServer();
    yield* Effect.addFinalizer(() => Effect.log('Shutting down'));
    return server;
  })
);

// Launch and run until interrupted
Effect.runFork(Layer.launch(HttpServerLayer));
```

---

## 12. Layer.provideMerge

Provide a dependency while keeping it in the output:

```typescript
import { Layer } from "effect"

// DatabaseLive requires Config
const DatabaseLive: Layer.Layer<Database, never, Config> = ...

// Provide Config AND keep it available in output
const Combined = Layer.provideMerge(DatabaseLive, ConfigLive)
// Type: Layer<Database | Config, never, never>
// Both Database AND Config are now available to consumers
```
