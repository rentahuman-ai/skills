# Effect Resource Management

## Quick Reference

| Pattern                                           | Use Case                           | Returns                           |
| ------------------------------------------------- | ---------------------------------- | --------------------------------- |
| `Effect.acquireRelease(acquire, release)`         | Resources shared across operations | `Effect<A, E, R \| Scope>`        |
| `Effect.acquireUseRelease(acquire, use, release)` | Self-contained single operations   | `Effect<A2, E \| E2, R>`          |
| `Effect.scoped(effect)`                           | Run scoped effects, auto-cleanup   | `Effect<A, E, Exclude<R, Scope>>` |
| `Effect.addFinalizer(fn)`                         | Register cleanup in current scope  | `Effect<void, never, Scope>`      |
| `Effect.Service` with `scoped`                    | Resource-backed services with DI   | Layer with lifecycle              |

## acquireRelease - Composable Resources

```typescript
import { Effect, Console, Exit } from 'effect';

const connection = Effect.acquireRelease(
  Effect.succeed({ id: 1, query: (sql: string) => sql }),
  (conn) => Console.log(`Connection ${conn.id} released`)
);

// Exit-aware release for conditional cleanup
const workspace = Effect.acquireRelease(
  Effect.succeed({ id: 'ws-123' }),
  (ws, exit: Exit.Exit<unknown, unknown>) =>
    Exit.isFailure(exit)
      ? Console.log(`Rolling back ${ws.id}`)
      : Console.log(`Keeping ${ws.id}`)
);

const program = Effect.scoped(
  Effect.gen(function* () {
    const conn = yield* connection;
    return conn.query('SELECT * FROM users');
  })
);
```

**Guarantees:** Acquisition uninterruptible, release only after success, release guaranteed on success/failure/interruption.

## acquireUseRelease - Single Operations

```typescript
import { Effect, Console, Exit } from 'effect';

const processFile = (path: string) =>
  Effect.acquireUseRelease(
    Effect.succeed({ path, fd: 1 }), // Acquire
    (handle) => Effect.succeed(`Content of ${handle.path}`), // Use
    (handle, exit) => Console.log(`Closed ${handle.path}`) // Release
  );
```

## Scope Management

```typescript
import { Effect, Scope, Exit, Console } from 'effect';

// Automatic scope (preferred)
const auto = Effect.scoped(
  Effect.gen(function* () {
    const r = yield* Effect.acquireRelease(
      Console.log('Acquired').pipe(Effect.as({ data: 'value' })),
      () => Console.log('Released')
    );
    return r.data;
  })
);

// Manual scope (fine-grained control)
const manual = Effect.gen(function* () {
  const scope = yield* Scope.make();
  yield* Scope.addFinalizer(scope, Console.log('Cleanup'));
  yield* Console.log('Working...');
  yield* Scope.close(scope, Exit.void);
});

// Finalizers run LIFO (last added, first to run)
const lifo = Effect.scoped(
  Effect.gen(function* () {
    yield* Effect.addFinalizer(() => Console.log('3rd - runs first'));
    yield* Effect.addFinalizer(() => Console.log('2nd'));
    yield* Effect.addFinalizer(() => Console.log('1st - runs last'));
  })
);
```

## Layer.scoped - Resource-Backed Services

```typescript
import { Effect, Console, Layer } from 'effect';

interface DatabaseOps {
  readonly query: (sql: string) => Effect.Effect<string[]>;
}

class Database extends Effect.Service<DatabaseOps>()('Database', {
  scoped: Effect.gen(function* () {
    yield* Console.log('Pool acquired');
    yield* Effect.addFinalizer(() => Console.log('Pool released'));
    return { query: (sql: string) => Effect.succeed([`Result: ${sql}`]) };
  }),
}) {}

// With dependencies
class Config extends Effect.Service<Config>()('Config', {
  succeed: { dbUrl: 'postgres://localhost:5432' },
}) {}

class DbWithDeps extends Effect.Service<DbWithDeps>()('DbWithDeps', {
  scoped: Effect.gen(function* () {
    const config = yield* Config;
    yield* Console.log(`Connecting to: ${config.dbUrl}`);
    yield* Effect.addFinalizer(() => Console.log('Disconnected'));
    return { query: (sql: string) => Effect.succeed(`Result: ${sql}`) };
  }),
  dependencies: [Config.Default],
}) {}

// Compose multiple services
const AppLayer = Layer.merge(Database.Default, Cache.Default);

const program = Effect.gen(function* () {
  const db = yield* Database;
  return yield* db.query('SELECT * FROM users');
});

Effect.runPromise(
  Effect.scoped(program).pipe(Effect.provide(Database.Default))
);
```

## Practical Examples

### File Operations

```typescript
import { Effect, Console } from 'effect';

const openFile = (path: string) =>
  Effect.acquireRelease(
    Effect.succeed({
      path,
      read: () => Effect.succeed(`Contents of ${path}`),
      write: (data: string) => Console.log(`Writing: ${data}`),
    }),
    (h) => Console.log(`Closing ${h.path}`)
  );

const copyFile = (src: string, dest: string) =>
  Effect.scoped(
    Effect.gen(function* () {
      const source = yield* openFile(src);
      const target = yield* openFile(dest);
      yield* target.write(yield* source.read());
    })
  );
```

### Transactional All-or-Nothing

```typescript
import { Effect, Exit, Console } from 'effect';

const createBucket = Effect.acquireRelease(
  Console.log('Bucket created').pipe(Effect.as({ bucket: 'my-bucket' })),
  (_, exit) =>
    Exit.isFailure(exit) ? Console.log('Rollback bucket') : Effect.void
);

const createDatabase = Effect.acquireRelease(
  Console.log('DB created').pipe(Effect.as({ db: 'my-db' })),
  (_, exit) => (Exit.isFailure(exit) ? Console.log('Rollback DB') : Effect.void)
);

// If any step fails, all previous resources roll back
const createWorkspace = Effect.scoped(
  Effect.gen(function* () {
    const bucket = yield* createBucket;
    const db = yield* createDatabase;
    yield* Effect.fail(new Error('Failed')); // Triggers rollback
    return { bucket, db };
  })
);
```

### Temporary Resources

```typescript
import { Effect, Console } from 'effect';

const makeTempFile = (prefix: string) =>
  Effect.acquireRelease(
    Effect.gen(function* () {
      const path = `/tmp/${prefix}-${Date.now()}.tmp`;
      yield* Console.log(`Created: ${path}`);
      let contents = '';
      return {
        path,
        write: (data: string) =>
          Effect.sync(() => {
            contents = data;
          }),
        read: () => Effect.succeed(contents),
      };
    }),
    (file) => Console.log(`Deleted: ${file.path}`)
  );
```

## Anti-Patterns

| Anti-Pattern            | Problem               | Solution         |
| ----------------------- | --------------------- | ---------------- |
| `try/finally`           | Not interruption-safe | `acquireRelease` |
| Global singletons       | No lifecycle          | `Layer.scoped`   |
| Manual cleanup order    | Error-prone           | `Layer.merge`    |
| Missing `Effect.scoped` | Resource leak         | Always wrap      |

```typescript
// WRONG: Leaks on interruption
async function unsafe() {
  const conn = await getConnection();
  try {
    return await use(conn);
  } finally {
    await close(conn);
  } // May not run!
}

// RIGHT: Guaranteed cleanup
const safe = Effect.scoped(
  Effect.gen(function* () {
    const conn = yield* Effect.acquireRelease(getConnection, close);
    return yield* use(conn);
  })
);
```

## Pattern Selection

| Scenario                          | Pattern                        |
| --------------------------------- | ------------------------------ |
| Resource shared across operations | `acquireRelease` + `scoped`    |
| Single self-contained operation   | `acquireUseRelease`            |
| Service with lifecycle in DI      | `Effect.Service` with `scoped` |
| Conditional cleanup               | Exit-aware release             |
| Multiple independent resources    | `Layer.merge`                  |
| Resource hierarchies              | Nested `acquireRelease`        |
