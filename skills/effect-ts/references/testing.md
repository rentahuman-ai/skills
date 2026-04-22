# Effect Testing Reference

## Quick Reference

| Pattern                   | Use Case                | Key Import       |
| ------------------------- | ----------------------- | ---------------- |
| `it.effect()`             | Standard Effect tests   | `@effect/vitest` |
| `it.scoped()`             | Tests with resources    | `@effect/vitest` |
| `it.live()`               | Real time/services      | `@effect/vitest` |
| `Layer.succeed()`         | Simple mock layers      | `effect`         |
| `Layer.effect()`          | Stateful mocks          | `effect`         |
| `Effect.exit()`           | Capture success/failure | `effect`         |
| `Effect.flip()`           | Test error values       | `effect`         |
| `TestClock.adjust()`      | Advance time            | `effect`         |
| `TestClock.setTime()`     | Set absolute time       | `effect`         |
| `TestRandom.setSeed()`    | Deterministic random    | `effect`         |
| `TestConsole.output`      | Capture console         | `effect`         |
| `TestContext.TestContext` | All test services       | `effect`         |

---

## Test Layers

### Creating Mock Layers

```typescript
import { Context, Layer, Effect } from 'effect';

class UserRepo extends Context.Tag('UserRepo')<
  UserRepo,
  {
    findById: (id: string) => Effect.Effect<User | null>;
    save: (user: User) => Effect.Effect<void>;
  }
>() {}

// Simple mock - no dependencies
const MockUserRepo = Layer.succeed(UserRepo, {
  findById: (id) => Effect.succeed({ id, name: 'Test User' }),
  save: () => Effect.succeed(undefined),
});
```

### Combining Layers

```typescript
const TestConfig = Layer.succeed(ConfigService, { appName: 'Test' });
const TestLogger = Layer.succeed(LoggerService, { log: () => Effect.void });

// Merge multiple layers
const TestEnv = Layer.merge(TestConfig, TestLogger);

// Provide to effect
const tested = program.pipe(Effect.provide(TestEnv));
```

### Stateful Mocks with Ref

```typescript
const StatefulMock = Layer.effect(
  CounterService,
  Effect.gen(function* () {
    const state = yield* Ref.make(0);
    return {
      increment: () => Ref.updateAndGet(state, (n) => n + 1),
      get: () => Ref.get(state),
    };
  })
);
```

---

## @effect/vitest Integration

### Installation

```bash
pnpm add -D vitest@^1.6.0 @effect/vitest
```

### Test Functions

```typescript
import { describe, expect, it } from '@effect/vitest';
import { Effect } from 'effect';

describe('MyService', () => {
  // Standard Effect test - auto-injects TestContext
  it.effect('basic test', () =>
    Effect.gen(function* () {
      const result = yield* myEffect;
      expect(result).toBe(expected);
    })
  );

  // For tests with resources (acquireRelease)
  it.scoped('resource test', () =>
    Effect.gen(function* () {
      const conn = yield* acquireConnection;
      expect(conn.active).toBe(true);
    })
  );

  // Real time/services instead of test mocks
  it.live('live test', () =>
    Effect.gen(function* () {
      yield* Effect.sleep('10 millis');
      // Actually waits 10ms
    })
  );
});
```

### Shared Layers with it.layer()

```typescript
const TestLayer = Layer.succeed(MyService, {
  fetch: () => Effect.succeed('data'),
});

describe('With shared layer', () => {
  it.layer(TestLayer)('shared context', (it) => {
    it.effect('test 1', () =>
      Effect.gen(function* () {
        const svc = yield* MyService;
        expect(yield* svc.fetch()).toBe('data');
      })
    );

    it.effect('test 2', () =>
      Effect.gen(function* () {
        // Same layer instance reused
      })
    );
  });
});
```

### Test Modifiers

```typescript
it.effect.skip('skipped', () => Effect.void);
it.effect.only('focused', () => Effect.void);
it.effect.fails('expected failure', () => Effect.fail(new Error()));
```

---

## Testing Error Paths

### Using Effect.exit

```typescript
it.effect('captures failure', () =>
  Effect.gen(function* () {
    const exit = yield* Effect.exit(validateEmail('invalid'));

    expect(Exit.isFailure(exit)).toBe(true);
    if (Exit.isFailure(exit)) {
      const error = Cause.failureOption(exit.cause);
      expect(error._tag).toBe('Some');
    }
  })
);
```

### Using Effect.flip

```typescript
it.effect('test error value directly', () =>
  Effect.gen(function* () {
    // Flip swaps success/error channels
    const error = yield* Effect.flip(findUser('missing'));

    expect(error._tag).toBe('NotFoundError');
    expect(error.id).toBe('missing');
  })
);
```

### Asserting Error Tags

```typescript
it.effect('handles specific error', () =>
  Effect.gen(function* () {
    const result = yield* fetchData().pipe(
      Effect.catchTag('NetworkError', () => Effect.succeed('recovered')),
      Effect.catchTag('TimeoutError', () => Effect.succeed('timed-out'))
    );
    expect(result).toBe('recovered');
  })
);
```

### Testing Retry Behavior

```typescript
it.effect('retries then succeeds', () =>
  Effect.gen(function* () {
    let attempts = 0;
    const flaky = Effect.gen(function* () {
      attempts++;
      if (attempts < 3) return yield* Effect.fail(new Error());
      return 'success';
    });

    const result = yield* flaky.pipe(Effect.retry(Schedule.recurs(3)));
    expect(result).toBe('success');
    expect(attempts).toBe(3);
  })
);
```

---

## Testing Concurrent Code

### TestClock for Time Control

TestClock is a testing utility that lets you control time. The clock time does NOT progress on its own - you must explicitly advance it.

**Critical Pattern: Fork BEFORE Adjust**

```typescript
import { Effect, Fiber, TestClock } from 'effect';

it.effect('advances time instantly', () =>
  Effect.gen(function* () {
    // 1. FORK the time-dependent effect FIRST
    const fiber = yield* Effect.fork(
      Effect.sleep('1 hour').pipe(Effect.as('done'))
    );

    // 2. THEN adjust the clock
    yield* TestClock.adjust('1 hour');

    // 3. Join to get results
    const result = yield* Fiber.join(fiber);
    expect(result).toBe('done');
  })
);
```

### TestClock Methods

```typescript
// Advance time by duration
yield * TestClock.adjust('1 hour');
yield * TestClock.adjust('500 millis');

// Set absolute time (milliseconds since epoch)
yield * TestClock.setTime(new Date('2024-01-01').getTime());

// Get current test time
const currentTime = yield * Clock.currentTimeMillis;
```

### Testing Timeouts

```typescript
it.effect('tests timeout', () =>
  Effect.gen(function* () {
    const withTimeout = Effect.sleep('10 minutes').pipe(
      Effect.as('completed'),
      Effect.timeout('5 minutes')
    );

    const fiber = yield* Effect.fork(withTimeout);
    yield* TestClock.adjust('6 minutes');

    const result = yield* Fiber.join(fiber);
    expect(result._tag).toBe('None'); // Timed out
  })
);
```

### Testing Recurring Effects

```typescript
it.effect('tests scheduled/recurring effects', () =>
  Effect.gen(function* () {
    const queue = yield* Queue.unbounded<number>();

    // Fork the recurring effect
    yield* Effect.gen(function* () {
      yield* Queue.offer(queue, 1);
    }).pipe(Effect.delay('60 minutes'), Effect.forever, Effect.fork);

    // Before time advances - nothing in queue
    const before = yield* Queue.poll(queue);
    expect(before._tag).toBe('None');

    // Advance past first execution
    yield* TestClock.adjust('60 minutes');

    // Now we have a value
    const after = yield* Queue.take(queue);
    expect(after).toBe(1);
  })
);
```

### Testing Deferred with Time

```typescript
it.effect('tests deferred completion with time', () =>
  Effect.gen(function* () {
    const deferred = yield* Deferred.make<number>();

    // Fork: sleep then complete deferred
    yield* Effect.all(
      [Effect.sleep('10 seconds'), Deferred.succeed(deferred, 42)],
      { concurrency: 'unbounded' }
    ).pipe(Effect.fork);

    // Advance time
    yield* TestClock.adjust('10 seconds');

    // Deferred should now be complete
    const value = yield* Deferred.await(deferred);
    expect(value).toBe(42);
  })
);
```

### Fiber Testing

```typescript
it.effect('fiber interruption', () =>
  Effect.gen(function* () {
    const fiber = yield* Effect.fork(Effect.never);
    yield* Fiber.interrupt(fiber);

    const exit = yield* fiber.await;
    expect(exit._tag).toBe('Failure');
  })
);
```

---

## Test Services

Effect provides test implementations of core services for deterministic testing.

### TestContext

`TestContext.TestContext` is a Layer that provides all test services (TestClock, TestRandom, TestConsole). `it.effect()` automatically provides this.

```typescript
import { TestContext } from 'effect';

// Manual usage (outside @effect/vitest)
const tested = myEffect.pipe(Effect.provide(TestContext.TestContext));
```

### TestRandom

Control random number generation for reproducible tests.

```typescript
import { TestRandom, Random } from 'effect';

it.effect('deterministic random', () =>
  Effect.gen(function* () {
    // Set seed for reproducibility
    yield* TestRandom.setSeed(12345);

    const r1 = yield* Random.next;
    const r2 = yield* Random.next;

    // Same seed = same sequence every time
    expect(r1).toBeDefined();
  })
);

it.effect('feed specific values', () =>
  Effect.gen(function* () {
    // Feed specific integers
    yield* TestRandom.feedInts(1, 2, 3);

    const a = yield* Random.nextInt;
    const b = yield* Random.nextInt;
    const c = yield* Random.nextInt;

    expect(a).toBe(1);
    expect(b).toBe(2);
    expect(c).toBe(3);
  })
);
```

### TestConsole

Capture and verify console output.

```typescript
import { TestConsole, Console } from 'effect';

it.effect('captures console output', () =>
  Effect.gen(function* () {
    yield* Console.log('Hello');
    yield* Console.log('World');

    const output = yield* TestConsole.output;
    expect(output).toContain('Hello');
    expect(output).toContain('World');
  })
);
```

---

## Testing Streams with Time

### Testing Debounced Streams

```typescript
it.effect('tests debounced stream', () =>
  Effect.gen(function* () {
    const queue = yield* Queue.unbounded<string>();
    const results: string[] = [];

    // Create stream from queue
    const stream = Stream.fromQueue(queue).pipe(
      Stream.debounce('300 millis'),
      Stream.runForEach((term) => Effect.sync(() => results.push(term)))
    );

    // Fork stream processing
    const fiber = yield* Effect.fork(stream);

    // Emit rapidly
    yield* Queue.offer(queue, 'a');
    yield* TestClock.adjust('100 millis');
    yield* Queue.offer(queue, 'ab');
    yield* TestClock.adjust('100 millis');
    yield* Queue.offer(queue, 'abc');

    // Wait for debounce
    yield* TestClock.adjust('300 millis');

    // Only final value should be emitted
    expect(results).toContain('abc');
    expect(results).not.toContain('a');

    yield* Fiber.interrupt(fiber);
  })
);
```

### Testing Throttled Streams

```typescript
it.effect('tests throttled stream', () =>
  Effect.gen(function* () {
    const source = Stream.range(1, 100);
    const results: number[] = [];

    const throttled = source.pipe(
      Stream.throttle({
        units: 10,
        duration: '1 second',
        strategy: 'enforce', // Drop excess
      }),
      Stream.runForEach((n) => Effect.sync(() => results.push(n)))
    );

    const fiber = yield* Effect.fork(throttled);

    // Advance time to allow processing
    yield* TestClock.adjust('10 seconds');

    yield* Fiber.join(fiber);

    // With enforce strategy, excess elements are dropped
    expect(results.length).toBeLessThan(100);
  })
);
```

---

## Service Mocking Patterns

### Configurable Mock

```typescript
const createMockPayment = (config: { shouldFail?: boolean }) =>
  Layer.succeed(PaymentService, {
    charge: (amount) =>
      config.shouldFail
        ? Effect.fail(new PaymentError('Declined'))
        : Effect.succeed(`txn-${amount}`),
  });

it.effect('payment succeeds', () =>
  Effect.gen(function* () {
    const svc = yield* PaymentService;
    const txn = yield* svc.charge(100);
    expect(txn).toBe('txn-100');
  }).pipe(Effect.provide(createMockPayment({ shouldFail: false })))
);
```

### Spy/Call Tracking

```typescript
const createSpyService = () =>
  Effect.gen(function* () {
    const calls = yield* Ref.make<string[]>([]);

    return {
      doWork: (arg: string) =>
        Ref.update(calls, (c) => [...c, arg]).pipe(Effect.as(`done: ${arg}`)),
      getCalls: () => Ref.get(calls),
    };
  });

it.effect('tracks calls', () =>
  Effect.gen(function* () {
    const spy = yield* createSpyService();
    yield* spy.doWork('first');
    yield* spy.doWork('second');

    const calls = yield* spy.getCalls();
    expect(calls).toEqual(['first', 'second']);
  })
);
```

---

## Test Utilities

### Assertion Helpers

```typescript
// test/utils.ts
export const expectFailure = <A, E>(effect: Effect.Effect<A, E>) =>
  Effect.gen(function* () {
    const exit = yield* Effect.exit(effect);
    expect(Exit.isFailure(exit)).toBe(true);
    if (Exit.isFailure(exit)) {
      return exit;
    }
    return yield* Effect.dieMessage('Expected effect to fail');
  });

export const expectErrorTag = <A, E extends { _tag: string }>(
  effect: Effect.Effect<A, E>,
  tag: E['_tag']
) =>
  Effect.gen(function* () {
    const error = yield* Effect.flip(effect);
    expect(error._tag).toBe(tag);
  });
```

### Test Data Factory

```typescript
const createUserFactory = () =>
  Effect.gen(function* () {
    const counter = yield* Ref.make(0);

    return {
      create: (overrides?: Partial<User>) =>
        Effect.gen(function* () {
          const id = yield* Ref.updateAndGet(counter, (n) => n + 1);
          return { id: `user-${id}`, name: `User ${id}`, ...overrides };
        }),
    };
  });
```

---

## Property-Based Testing (Advanced)

### Custom Arbitrary Annotations

Override default generation with custom arbitraries.

```typescript
import { Arbitrary, Schema } from 'effect';
import * as fc from 'fast-check';

// Custom arbitrary via annotation
const Username = Schema.String.annotations({
  arbitrary: () => (fc) =>
    fc.stringOf(fc.constantFrom(...'abcdefghijklmnopqrstuvwxyz0123456789'), {
      minLength: 3,
      maxLength: 20,
    }),
});

// Use in tests
const usernameArb = Arbitrary.make(Username);
```

### Pattern Optimization

Use `Schema.pattern()` for efficient string generation.

```typescript
// Efficient - uses FastCheck.stringMatching()
const Email = Schema.String.pipe(Schema.pattern(/^[a-z]+@[a-z]+\.[a-z]+$/));

// Less efficient - custom filter
const EmailManual = Schema.String.pipe(
  Schema.filter((s) => /^[a-z]+@[a-z]+\.[a-z]+$/.test(s))
);
```

### Filter Ordering Gotchas

Filters before transformations may be ignored during generation. Structure schemas carefully:

```typescript
// Apply filters to initial type, then transform, then filter result
const ProcessedString = Schema.String.pipe(
  Schema.filter((s) => s.length > 0), // Pre-transform filter
  Schema.transform(Schema.String, {
    decode: (s) => s.toUpperCase(),
    encode: (s) => s.toLowerCase(),
  }),
  Schema.filter((s) => s.startsWith('A')) // Post-transform filter
);
```

### Using it.prop with Effect

```typescript
import { describe, it } from '@effect/vitest';
import { Arbitrary, Schema } from 'effect';

const PositiveInt = Schema.Number.pipe(Schema.int(), Schema.positive());

describe('Property tests', () => {
  it.prop([Arbitrary.make(PositiveInt)])('doubles positive integers', ([n]) =>
    Effect.gen(function* () {
      const result = n * 2;
      expect(result).toBeGreaterThan(n);
    })
  );

  // Multiple arbitraries
  it.prop([
    Arbitrary.make(Schema.String.pipe(Schema.minLength(1))),
    Arbitrary.make(PositiveInt),
  ])('string repeat', ([str, count]) =>
    Effect.gen(function* () {
      const result = str.repeat(count);
      expect(result.length).toBe(str.length * count);
    })
  );
});
```

---

## Vitest Configuration

```typescript
// vitest.config.ts
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    testTimeout: 30_000,
    hookTimeout: 30_000,
  },
});
```

---

## Project-Specific Patterns

### `makeTestLayer` — Proxy-Based Mocking

`@tooling/testing/test-layer` provides `makeTestLayer`, a Proxy-based mock factory. Unimplemented methods `Effect.die` with descriptive errors instead of silently returning undefined.

```typescript
import { makeTestLayer } from '@tooling/testing/test-layer';
import { Effect, Layer, Option } from 'effect';

// Only implement the methods your test needs — others die on access
const RepoTest = makeTestLayer(MyRepo)({
  findById: (id) => Effect.succeed(Option.some({ id, name: 'Test' })),
});
```

### `DefaultWithoutDependencies` for Service Testing

Use `DefaultWithoutDependencies` to get a service layer that requires manual dep wiring. This lets you inject mock layers for each dependency.

```typescript
const testLayer = MyService.DefaultWithoutDependencies.pipe(
  Layer.provide(
    makeTestLayer(MyRepo)({
      /* mocks */
    })
  ),
  Layer.provide(ExternalClient.Test) // use .Test layers when available
);

it.scoped('creates a resource', () =>
  Effect.gen(function* () {
    const svc = yield* Effect.provide(MyService, testLayer);
    const result = yield* svc.create({ name: 'test' });
    expect(result.name).toBe('test');
  })
);
```

### Deterministic `.Test` Layers

Services that wrap non-deterministic operations (crypto, random, external APIs) should provide a static `.Test` layer with predictable behavior:

```typescript
class Crypto extends Effect.Service<Crypto>()('Crypto', {
  effect: Effect.gen(function* () {
    // ... real implementation
  }),
}) {
  // Deterministic test double — always returns the same key
  static Test = Layer.succeed(
    Crypto,
    new Crypto({
      generateApiKey: Effect.succeed('tcc_test_key_000000'),
      hashApiKey: (key) =>
        Effect.sync(() => createHash('sha256').update(key).digest('hex')),
      redactedKey: (key) => `tcc_${key.slice(4, 7)}...${key.slice(-3)}`,
    })
  );
}
```

### HTTP Endpoint Testing with `toWebHandler`

Build a mini-API containing only the group under test, mock all dependencies, and send real `Request` objects:

```typescript
import { HttpApi, HttpApiBuilder, HttpServer } from '@effect/platform';
import { SqlClient } from '@effect/sql';

// 1. Mini-API with only the group under test
class TestApi extends HttpApi.make('test').add(MyApi).prefix('/v1') {}

// 2. Mock authentication middleware
const AuthTest = Layer.succeed(
  Authentication,
  Authentication.of({
    cookie: () => Effect.succeed(testAuth),
    bearer: () => Effect.succeed(testAuth),
  })
);

// 3. Mock SqlClient for RLS passthrough
const SqlTest = makeTestLayer(SqlClient.SqlClient)({
  unsafe: () => Effect.succeed([{}]),
  withTransaction: (effect) => effect,
});

// 4. Mock service
const ServiceTest = makeTestLayer(MyService)({
  list: () => Effect.succeed([testItem]),
});

// 5. Wire and build handler
const HttpTestLive = HttpApiBuilder.group(TestApi, 'myGroup', (handlers) =>
  Effect.gen(function* () {
    const svc = yield* MyService;
    return handlers.handle('list', () => svc.list());
  })
);

const ApiLive = HttpApiBuilder.api(TestApi).pipe(
  Layer.provide(HttpTestLive),
  Layer.provide(AuthTest),
  Layer.provide(ServiceTest),
  Layer.provide(SqlTest)
);

const { handler } = HttpApiBuilder.toWebHandler(
  ApiLive.pipe(Layer.provideMerge(HttpServer.layerContext))
);

// 6. Send requests and assert
const response = await handler(
  new Request('http://localhost/v1/my-endpoint', {
    method: 'GET',
    headers: { authorization: 'Bearer test-token' },
  })
);
expect(response.status).toBe(200);
```

### Shared Auth Fixtures via `Setup.ts`

`apps/api/test/_shared/Setup.ts` exports shared auth constants — no factory functions, just reusable objects:

```typescript
import { testAuth, authLayer, AuthenticationTest } from '@test/_shared/Setup';

// authLayer — Layer<AuthContext> for policy tests
const policyLayer = Layer.merge(ApiKeysPolicy.Default, authLayer);

// AuthenticationTest — Layer<Authentication> middleware mock for HTTP tests
const ApiLive = HttpApiBuilder.api(TestApi).pipe(
  Layer.provide(AuthenticationTest)
  // ...
);
```

### Shared Layer with `it.layer()`

Use `it.layer()` to share a layer across all tests in a describe block. The layer is constructed once and provided to every test automatically. Use `Layer.merge` (not `Layer.provide`) when the layer and the auth context are peers:

```typescript
const policyLayer = Layer.merge(ApiKeysPolicy.Default, authLayer);

describe('ApiKeys.Policy', () => {
  it.layer(policyLayer)('authenticated user', (it) => {
    it.effect('canCreate allows authenticated user', () =>
      Effect.gen(function* () {
        const policy = yield* ApiKeysPolicy;
        const actor = yield* policy.canCreate();
        expect(actor).toBeDefined();
      })
    );
  });
});
```

### Testing Scoped Services

Services with `scoped:` (queues, fibers, `acquireRelease`) need scope management. Use `it.scoped()` for most cases. For complex scenarios with external layer dependencies, use `Effect.scoped` manually:

```typescript
// Simple: it.scoped manages the scope
it.scoped('works', () =>
  Effect.gen(function* () {
    const svc = yield* Effect.provide(MyScopedService, testLayer);
    // ...
  })
);

// Complex: manual Effect.scoped when mixing layer providers
it('works', async () => {
  const result = await Effect.runPromise(
    Effect.scoped(
      Effect.gen(function* () {
        const svc = yield* MyScopedService;
        return yield* svc.method();
      }).pipe(Effect.provide(testLayer))
    )
  );
  expect(result).toBeDefined();
});
```

---

## API Test Pyramid (Project-Specific)

### Test Tiers

| Tier        | Directory           | Database      | Auth                        | Use When                                              |
| ----------- | ------------------- | ------------- | --------------------------- | ----------------------------------------------------- |
| Unit        | `test/unit/`        | None (mocked) | Static (`_shared/Setup.ts`) | Business logic, schema validation, HTTP decode/encode |
| Integration | `test/integration/` | Real Postgres | Dynamic (`_shared/Auth.ts`) | Data persistence, RLS isolation, repo roundtrips      |
| E2E         | `test/e2e/`         | Real Postgres | Dynamic (`_shared/Auth.ts`) | Full HTTP lifecycle, status codes, serialization      |

### Shared Utilities

| Utility                                                    | Import                       | Purpose                                                           |
| ---------------------------------------------------------- | ---------------------------- | ----------------------------------------------------------------- |
| `testAuth`, `authLayer`, `AuthenticationTest`              | `@test/_shared/Setup`        | Static auth for unit tests (hardcoded `user-test`/`org-test` IDs) |
| `makeTestAuth(overrides?)`                                 | `@test/_shared/Auth`         | Dynamic auth with random UUIDs + DB `seed`/`cleanup` Effects      |
| `TestDatabaseLive`, `runEffect`                            | `@test/_shared/TestDatabase` | Real Postgres + Drizzle layers, convenience runner                |
| `makeIntegrationSuite(options?)`                           | `@test/integration/Setup`    | Integration harness: `{ auth, setup, teardown, runEffect }`       |
| `setupE2ESuite(options)`                                   | `@test/e2e/Helpers`          | E2E harness: handler + auth + HTTP helpers                        |
| `makeRequest`, `expectStatus`, `expectJson`, `expectError` | `@test/e2e/Helpers`          | HTTP request builder and response assertions                      |

### Unit Test Templates

#### Service Test

```typescript
import { describe, expect, it } from '@effect/vitest';
import { Effect, Layer, Option } from 'effect';
import { makeTestLayer } from '@tooling/testing/test-layer';
import { MyService } from '@/Modules/MyModule/Service/MyService';
import { MyRepo } from '@/Modules/MyModule/Repo';

const makeServiceLayer = (overrides = {}) =>
  MyService.DefaultWithoutDependencies.pipe(
    Layer.provide(
      makeTestLayer(MyRepo)({
        findById: () => Effect.succeed(Option.some({ id: 'test-id' })),
        ...overrides,
      })
    )
  );

describe('MyService', () => {
  it.scoped('creates a resource', () =>
    Effect.gen(function* () {
      const svc = yield* MyService;
      const result = yield* svc.create({ name: 'test' });
      expect(result.name).toBe('test');
    }).pipe(Effect.provide(makeServiceLayer()))
  );

  it.effect('returns error for missing resource', () =>
    Effect.gen(function* () {
      const svc = yield* MyService;
      const error = yield* Effect.flip(svc.getById('missing'));
      expect(error._tag).toBe('NotFound');
    }).pipe(
      Effect.provide(
        makeServiceLayer({
          findById: () => Effect.succeed(Option.none()),
        })
      )
    )
  );
});
```

#### Domain/Schema Test

```typescript
import { describe, expect, it } from 'vitest';
import { Schema } from 'effect';
import { MySchema } from '@/Modules/MyModule/Domain';

describe('MySchema', () => {
  it('decodes valid data', () => {
    const result = Schema.decodeUnknownSync(MySchema)({
      id: '1',
      name: 'test',
    });
    expect(result.id).toBe('1');
  });

  it('rejects invalid data', () => {
    expect(() => Schema.decodeUnknownSync(MySchema)({})).toThrow();
  });
});
```

#### Policy Test

```typescript
import { describe, expect, it } from '@effect/vitest';
import { Effect, Layer } from 'effect';
import { authLayer } from '@test/_shared/Setup';
import { MyPolicy } from '@/Modules/MyModule/Policy';

const policyLayer = Layer.merge(MyPolicy.Default, authLayer);

describe('MyPolicy', () => {
  it.layer(policyLayer)('authenticated user', (it) => {
    it.effect('canCreate allows access', () =>
      Effect.gen(function* () {
        const policy = yield* MyPolicy;
        const actor = yield* policy.canCreate();
        expect(actor).toBeDefined();
      })
    );
  });
});
```

#### HTTP Endpoint Test

```typescript
import { HttpApi, HttpApiBuilder, HttpServer } from '@effect/platform';
import { Effect, Layer } from 'effect';
import { makeTestLayer } from '@tooling/testing/test-layer';
import { AuthenticationTest } from '@test/_shared/Setup';
import { MyApi } from '@/Modules/MyModule/Api';
import { MyService } from '@/Modules/MyModule/Service/MyService';

class TestApi extends HttpApi.make('test').add(MyApi).prefix('/v1') {}

const makeHandler = (serviceOverrides = {}) => {
  const ServiceTest = makeTestLayer(MyService)(serviceOverrides);

  const HttpTestLive = HttpApiBuilder.group(
    TestApi,
    'myEndpoints',
    (handlers) =>
      Effect.gen(function* () {
        const svc = yield* MyService;
        return handlers.handle('list', () => svc.list());
      })
  );

  const ApiLive = HttpApiBuilder.api(TestApi).pipe(
    Layer.provide(HttpTestLive),
    Layer.provide(AuthenticationTest),
    Layer.provide(ServiceTest)
  );

  return HttpApiBuilder.toWebHandler(
    ApiLive.pipe(Layer.provideMerge(HttpServer.layerContext))
  );
};

describe('MyModule HTTP', () => {
  it('returns 200 for list', async () => {
    const { handler } = makeHandler({
      list: () => Effect.succeed([]),
    });
    const response = await handler(
      new Request('http://localhost/v1/my-endpoint', {
        method: 'GET',
        headers: { authorization: 'Bearer test-token' },
      })
    );
    expect(response.status).toBe(200);
  });
});
```

### Integration Test Template

```typescript
import { describe, expect, it } from '@effect/vitest';
import { afterAll, beforeAll } from 'vitest';
import { Effect, Layer } from 'effect';
import { withRlsContext } from '@/Infrastructure/Database/Rls';
import { MyService } from '@/Modules/MyModule/Service/MyService';
import { MyRepo } from '@/Modules/MyModule/Repo';
import { makeIntegrationSuite } from '@test/integration/Setup';
import { TestDatabaseLive } from '@test/_shared/TestDatabase';

const TestLayer = MyService.DefaultWithoutDependencies.pipe(
  Layer.provide(MyRepo.DefaultWithoutDependencies),
  Layer.provideMerge(TestDatabaseLive)
);

describe('MyService integration', () => {
  const suite = makeIntegrationSuite();

  beforeAll(() => suite.setup());
  afterAll(() => suite.teardown());

  const rls = <A, E, R>(effect: Effect.Effect<A, E, R>) =>
    withRlsContext(
      { userId: suite.auth.userId, orgId: suite.auth.orgId },
      effect
    );

  it.scoped('create + getById roundtrip', () =>
    Effect.gen(function* () {
      const svc = yield* MyService;
      const created = yield* rls(svc.create({ name: 'Test' }));
      const fetched = yield* rls(svc.getById(created.id));
      expect(fetched.id).toBe(created.id);
      yield* rls(svc.remove(created.id));
    }).pipe(Effect.provide(TestLayer))
  );

  it.scoped('RLS isolates data between organizations', () =>
    Effect.gen(function* () {
      const svc = yield* MyService;
      const created = yield* rls(svc.create({ name: 'Org A' }));

      const other = makeIntegrationSuite();
      yield* other.auth.seed;
      const otherRls = <A, E, R>(effect: Effect.Effect<A, E, R>) =>
        withRlsContext(
          { userId: other.auth.userId, orgId: other.auth.orgId },
          effect
        );

      const otherKeys = yield* otherRls(svc.list());
      expect(otherKeys).toHaveLength(0);

      yield* rls(svc.remove(created.id));
      yield* other.auth.cleanup;
    }).pipe(Effect.provide(TestLayer))
  );
});
```

### E2E Test Template

Uses `makeTestClient` for fully typed requests/responses via `HttpApiClient`. No manual URL construction, no manual JSON parsing — schemas are validated automatically.

```typescript
import { describe, expect, it } from '@effect/vitest';
import { afterAll, beforeAll } from 'vitest';
import { Effect } from 'effect';
import { MyResourceId } from '@/Modules/MyModule/Domain';
import { HttpMyModuleLive } from '@/Modules/MyModule/Http';
import { makeTestClient } from '@test/_shared/TestClient';

describe('MyModule e2e', () => {
  const suite = makeTestClient({ httpLayers: [HttpMyModuleLive] });

  beforeAll(() => suite.setup());
  afterAll(() => suite.teardown());

  it.effect('full CRUD lifecycle', () =>
    Effect.gen(function* () {
      const { myModule } = suite.client;

      const created = yield* myModule.create({ payload: { name: 'E2E Test' } });
      expect(created.name).toBe('E2E Test');

      const items = yield* myModule.list({});
      expect(items).toHaveLength(1);

      const fetched = yield* myModule.getById({
        path: { id: MyResourceId.make(created.id) },
      });
      expect(fetched.id).toBe(created.id);

      yield* myModule.remove({
        path: { id: MyResourceId.make(created.id) },
      });

      const itemsAfter = yield* myModule.list({});
      expect(itemsAfter).toHaveLength(0);
    })
  );

  it.effect('returns typed error for non-existent resource', () =>
    Effect.gen(function* () {
      const error = yield* Effect.flip(
        suite.client.myModule.getById({
          path: {
            id: MyResourceId.make('00000000-0000-0000-0000-000000000000'),
          },
        })
      );
      expect(error._tag).toBe('MyResourceNotFound');
    })
  );

  it.effect('RLS isolates data between organizations', () =>
    Effect.gen(function* () {
      yield* suite.client.myModule.create({ payload: { name: 'Org A' } });

      const otherSuite = makeTestClient({ httpLayers: [HttpMyModuleLive] });
      yield* Effect.promise(() => otherSuite.setup());

      try {
        const keys = yield* otherSuite.client.myModule.list({});
        expect(keys).toHaveLength(0);
      } finally {
        yield* Effect.promise(() => otherSuite.teardown());
      }
    })
  );
});
```

Key patterns:

- **`makeTestClient`** bridges `HttpApiBuilder.toWebHandler` → `HttpClient` → `HttpApiClient.makeWith` for in-process typed testing
- **`it.effect`** from `@effect/vitest` — runs `Effect.gen` as the test body
- **`Effect.flip`** — captures the typed error channel for error assertions
- **RLS isolation** — create a second `makeTestClient` with fresh auth to test multi-tenant isolation
- **Raw HTTP escape hatch** — for testing custom headers, malformed requests, or raw HTTP semantics, use `suite.handler` directly or helpers from `@test/e2e/Helpers`

### Fixture Patterns

Fixtures are factory functions colocated with their module's tests at `test/unit/Modules/<Module>/Fixtures.ts`:

```typescript
export const TEST_ID = 'test-id-123';
export const TEST_ORG_ID = 'org-test'; // matches testAuth

export const makeMyRow = (overrides: Partial<MyRow> = {}): MyRow => ({
  id: TEST_ID,
  organizationId: TEST_ORG_ID,
  name: 'Test Resource',
  createdAt: new Date('2024-01-01'),
  ...overrides,
});
```

Use static IDs matching `testAuth` constants (`user-test`, `org-test`) for unit tests. Use dynamic IDs from `makeTestAuth()` for integration/e2e tests.
