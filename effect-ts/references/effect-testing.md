# Effect Testing Infrastructure

Testing helpers from `@tooling/testing` for Effect-TS backend tests. Complements [testing.md](testing.md) which covers `@effect/vitest` APIs and test templates.

## makeTestLayer — Mock Service Layers

Creates Effect layers from partial mock implementations using Proxy:

```typescript
import { makeTestLayer } from '@tooling/testing/test-layer';

const TestSandboxes = makeTestLayer(Sandboxes)({
  create: Effect.succeed(new Sandbox({ ... })),
  // Unimplemented methods throw at runtime with service + method name
});
```

### How It Works

```typescript
export const makeTestLayer =
  <I, S extends object>(tag: {
    readonly key: string;
    readonly Service: S;
    readonly Identifier: I;
    of(self: S): S;
    context(self: S): object;
  }) =>
  (service: MockService<S>): Layer.Layer<I> => {
    const proxy = makeUnimplementedProxy<S>(tag.key, service);
    const ctx = tag.context(tag.of(proxy));
    return Layer.succeedContext(ctx as never) as Layer.Layer<I>;
  };
```

Uses structural typing for tags to avoid `unique symbol` mismatches across monorepo package boundaries.

## Test Containers — PostgreSQL

Spins up a real Postgres container for integration tests:

```typescript
import {
  makeTestContainerPgLayer,
  TestContainerUrl,
} from '@tooling/testing/pg-test-container';

const TestDbLayer = makeTestContainerPgLayer({
  image: 'postgres:17-alpine',
  maxConnections: 5,
  setup: (sql, url) =>
    Effect.gen(function* () {
      yield* sql.executeUnprepared('CREATE EXTENSION ...', [], undefined);
    }),
});
```

### Lifecycle

- Container starts via `Effect.acquireRelease` (auto-cleanup on scope exit)
- Waits for "database system is ready" log message
- Runs optional `setup` callback for migrations/seeding
- Provides `PgClient`, `SqlClient`, and `TestContainerUrl` in context

## Transaction Isolation — TestTransactionLive

Wraps each test in a transaction that rolls back:

```typescript
import { TestTransactionLive } from '@tooling/testing/test-transaction';

const TestLayer = Layer.mergeAll(TestDbLayer, TestTransactionLive);
```

### How It Works

```typescript
export const TestTransactionLive = Layer.scopedContext(
  Effect.gen(function* () {
    const sql = yield* SqlClient.SqlClient;
    const conn = yield* sql.reserve;
    yield* conn.executeUnprepared('BEGIN', [], undefined);
    yield* Effect.addFinalizer(() =>
      conn.executeUnprepared('ROLLBACK', [], undefined).pipe(Effect.ignore)
    );
    return Context.make(SqlClient.TransactionConnection, [conn, 1] as const);
  })
);
```

Tests see a real database but changes never persist — each test starts clean.

## Test Composition — Harness Patterns

For packages with multiple services (e.g. agent-harness):

```typescript
const makeHarnessLayers = (setup?: (db: Db) => Effect.Effect<void>) =>
  Layer.mergeAll(
    ThreadsService.Default,
    BranchesService.Default,
    MessagesService.Default
  ).pipe(Layer.provide(makeTestDb(setup)));

const runHarnessTest = <A>(
  effect: Effect.Effect<
    A,
    any,
    ThreadsService | BranchesService | MessagesService
  >,
  setup?: (db: Db) => Effect.Effect<void>
) => Effect.runPromise(effect.pipe(Effect.provide(makeHarnessLayers(setup))));
```

## Running Effects in Tests

Three patterns depending on context:

```typescript
// Simple: Effect.runPromise for async effects
it('creates a sandbox', async () => {
  const result = await Effect.runPromise(
    sandboxes.create(input).pipe(Effect.provide(TestLayer))
  );
  expect(result.id).toBeDefined();
});

// Sync: Effect.runSync for pure effects
it('rejects missing header', () => {
  expect(() => Effect.runSync(verifyWebhook({ headers: {} }))).toThrow();
});

// Service injection via higher-order function
const withClient = <A>(fn: (client: Client) => Effect.Effect<A, Error>) =>
  Effect.gen(function* () {
    const service = yield* MyClient;
    const client = yield* service.forCredentials({ token: 'test' });
    return yield* fn(client);
  }).pipe(Effect.provide(MyClient.Default));

it.effect('sends auth header', () =>
  withClient((client) =>
    Effect.gen(function* () {
      yield* client.getMe();
      expect(lastFetch().headers.authorization).toBe('Bearer test');
    })
  )
);
```

## Config Layer for Tests

```typescript
import { makeConfigLayer } from '@tooling/testing/config-layer';

const TestConfig = makeConfigLayer({
  DATABASE_URL: 'postgresql://test:test@localhost/test',
  REDIS_URL: 'redis://localhost:6379',
});
```

## When to Use What

| Scenario                      | Tool                                                    |
| ----------------------------- | ------------------------------------------------------- |
| Unit test (pure logic)        | `makeTestLayer` with mocked deps                        |
| Integration test (DB queries) | `makeTestContainerPgLayer` + `TestTransactionLive`      |
| E2E test (full API)           | Test container + real service layers + HTTP test client |
| Client package test           | `Effect.runPromise` + mock fetch                        |
