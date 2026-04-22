# Effect Configuration

Type-safe configuration loading with validation, secrets handling, and testability.

## Quick Reference

| Primitive                       | Type               | Example               |
| ------------------------------- | ------------------ | --------------------- |
| `Config.string(name)`           | `string`           | `HOST=localhost`      |
| `Config.integer(name)`          | `number`           | `PORT=8080`           |
| `Config.number(name)`           | `number`           | `RATIO=3.14`          |
| `Config.boolean(name)`          | `boolean`          | `DEBUG=true`          |
| `Config.date(name)`             | `Date`             | `EXPIRY=2025-12-31`   |
| `Config.url(name)`              | `URL`              | `URL=https://api.com` |
| `Config.duration(name)`         | `Duration`         | `TIMEOUT=5 seconds`   |
| `Config.redacted(name)`         | `Redacted<string>` | `SECRET=abc123`       |
| `Config.literal(a, b, c)(name)` | Union              | `ENV=production`      |
| `Config.nonEmptyString(name)`   | `string`           | `KEY=value`           |

| Operator                                 | Purpose                           |
| ---------------------------------------- | --------------------------------- |
| `Config.withDefault(value)`              | Fallback value                    |
| `Config.option(config)`                  | Returns `Option<A>`               |
| `Config.map(fn)`                         | Transform value                   |
| `Config.mapAttempt(fn)`                  | Transform with exception handling |
| `Config.validate({message, validation})` | Add validation                    |
| `Config.nested(config, prefix)`          | Add key prefix                    |
| `Config.orElse(fallback)`                | Alternative config                |
| `Config.all({...})`                      | Combine into object               |
| `Config.array(config, name)`             | Comma-separated array             |

---

## Basic Usage

```typescript
import { Config, Effect } from 'effect';

const program = Effect.gen(function* () {
  const host = yield* Config.string('HOST');
  const port = yield* Config.integer('PORT');
  const debug = yield* Config.boolean('DEBUG').pipe(Config.withDefault(false));

  return { host, port, debug };
});
```

## Combining Configurations

```typescript
import { Config, Effect } from 'effect';

// Object syntax - most common
const serverConfig = Config.all({
  host: Config.string('HOST'),
  port: Config.integer('PORT').pipe(Config.withDefault(8080)),
  debug: Config.boolean('DEBUG').pipe(Config.withDefault(false)),
});

// Nested configuration (reads DB_HOST, DB_PORT)
const dbConfig = Config.nested(
  Config.all({
    host: Config.string('HOST'),
    port: Config.integer('PORT'),
  }),
  'DB'
);

const program = Effect.gen(function* () {
  const server = yield* serverConfig;
  const db = yield* dbConfig;
  return { server, db };
});
```

## Validation

```typescript
import { Config, Effect } from 'effect';

const port = Config.integer('PORT').pipe(
  Config.validate({
    message: 'Port must be between 1 and 65535',
    validation: (n) => n >= 1 && n <= 65535,
  })
);

const email = Config.string('EMAIL').pipe(
  Config.validate({
    message: 'Email must contain @',
    validation: (s) => s.includes('@'),
  })
);

// Literal for union types
const env = Config.literal('development', 'staging', 'production')('ENV');
// Type: "development" | "staging" | "production"
```

## Config.redacted - Sensitive Data

Use `Config.redacted` for secrets to prevent accidental logging:

```typescript
import { Config, Effect, Redacted } from 'effect';

const program = Effect.gen(function* () {
  const apiKey = yield* Config.redacted('API_KEY');

  // Safe to log - shows <redacted>
  yield* Effect.log(`API Key: ${apiKey}`);

  // Access actual value when needed
  const headers = {
    Authorization: `Bearer ${Redacted.value(apiKey)}`,
  };

  return headers;
});
```

### Redacted Operations

```typescript
import { Equivalence, Redacted } from 'effect';

// Create redacted value
const secret = Redacted.make('my-secret');

// Logging shows <redacted>
console.log(String(secret)); // <redacted>

// Access value explicitly
const value = Redacted.value(secret);

// Compare without exposing
const eq = Redacted.getEquivalence(Equivalence.string);
eq(secret1, secret2); // true/false

// Wipe from memory when done
Redacted.unsafeWipe(secret);
```

## ConfigProvider

### Default (Environment Variables)

```typescript
import { ConfigProvider, Effect, Layer } from 'effect';

// Default reads from process.env
const defaultProvider = ConfigProvider.fromEnv();

// Custom delimiters
const customProvider = ConfigProvider.fromEnv({
  pathDelim: '__', // Nested: DATABASE__HOST
  seqDelim: '|', // Arrays: HOSTS=a|b|c
});
```

### fromMap (Testing)

```typescript
import { Config, ConfigProvider, Effect } from 'effect';

const testProvider = ConfigProvider.fromMap(
  new Map([
    ['HOST', 'localhost'],
    ['PORT', '8080'],
    ['DEBUG', 'true'],
  ])
);

// Use with Effect.withConfigProvider
const result = await Effect.runPromise(
  Effect.withConfigProvider(testProvider)(program)
);
```

### fromJson

```typescript
import { Config, ConfigProvider, Effect } from 'effect';

const jsonProvider = ConfigProvider.fromJson({
  HOST: 'localhost',
  PORT: 8080,
  DATABASE: {
    HOST: 'db.example.com',
    PORT: 5432,
  },
});

const program = Effect.gen(function* () {
  const host = yield* Config.string('HOST');
  const dbConfig = yield* Config.nested(
    Config.all({
      host: Config.string('HOST'),
      port: Config.integer('PORT'),
    }),
    'DATABASE'
  );
  return { host, dbConfig };
});
```

### Provider Modifiers

```typescript
import { ConfigProvider } from 'effect';

// Add prefix to all keys
const prefixed = ConfigProvider.fromEnv().pipe(ConfigProvider.nested('APP'));
// Config.string("HOST") reads APP_HOST

const constantCase = ConfigProvider.fromEnv().pipe(ConfigProvider.constantCase);

// Combine providers (first wins, fallback to second)
const appConfig = ConfigProvider.fromEnv().pipe(
  ConfigProvider.constantCase,
  ConfigProvider.nested('APP')
);
```

## Schema.Config

Recommended for validated configuration with rich error messages:

```typescript
import { Config, Effect, Schema } from 'effect';

const Port = Schema.NumberFromString.pipe(
  Schema.int(),
  Schema.between(1, 65535)
);

const Environment = Schema.Literal('development', 'staging', 'production');

const program = Effect.gen(function* () {
  const port = yield* Schema.Config('PORT', Port);
  const env = yield* Schema.Config('ENV', Environment);
  return { port, env };
});
```

### Branded Types

```typescript
import { Effect, Schema } from 'effect';

const Port = Schema.NumberFromString.pipe(
  Schema.int(),
  Schema.between(1, 65535),
  Schema.brand('Port')
);
type Port = typeof Port.Type;

const program = Effect.gen(function* () {
  const port: Port = yield* Schema.Config('PORT', Port);
  // Type-safe: can't assign Port to regular number
  return port;
});
```

## Configuration as Layers

### Service Pattern

```typescript
import { Config, Context, Effect, Layer, Redacted } from 'effect';

class DatabaseConfig extends Context.Tag('@app/DatabaseConfig')<
  DatabaseConfig,
  {
    readonly host: string;
    readonly port: number;
    readonly password: Redacted.Redacted;
  }
>() {
  static readonly layer = Layer.effect(
    DatabaseConfig,
    Effect.gen(function* () {
      const host = yield* Config.string('DB_HOST');
      const port = yield* Config.integer('DB_PORT').pipe(
        Config.withDefault(5432)
      );
      const password = yield* Config.redacted('DB_PASSWORD');
      return { host, port, password };
    })
  );
}

// Usage
const program = Effect.gen(function* () {
  const config = yield* DatabaseConfig;
  yield* Effect.log(`Connecting to ${config.host}:${config.port}`);
});

Effect.runPromise(program.pipe(Effect.provide(DatabaseConfig.layer)));
```

## Testing

```typescript
import { Effect, Layer, Redacted } from 'effect';

// Direct layer provision
const testProgram = program.pipe(
  Effect.provide(
    Layer.succeed(DatabaseConfig, {
      host: 'localhost',
      port: 5432,
      password: Redacted.make('test-password'),
    })
  )
);

// With ConfigProvider
import { ConfigProvider } from 'effect';

const TestConfigLayer = Layer.setConfigProvider(
  ConfigProvider.fromMap(
    new Map([
      ['DB_HOST', 'localhost'],
      ['DB_PORT', '5432'],
      ['DB_PASSWORD', 'test-password'],
    ])
  )
);

const TestAppLayer = Layer.provideMerge(DatabaseConfig.layer, TestConfigLayer);
Effect.runPromise(program.pipe(Effect.provide(TestAppLayer)));
```

## Common Pitfalls

| Issue               | Problem                                            | Solution                                    |
| ------------------- | -------------------------------------------------- | ------------------------------------------- |
| Config not yielded  | `const host = Config.string("HOST")` prints object | `const host = yield* Config.string("HOST")` |
| Secret exposed      | `Config.string("API_KEY")` can be logged           | Use `Config.redacted("API_KEY")`            |
| Wrong delimiter     | `DATABASE.HOST` not found                          | Use `DATABASE_HOST` (underscore default)    |
| Array delimiter     | `HOSTS=a;b;c` returns single element               | Use commas: `HOSTS=a,b,c`                   |
| Missing test config | Tests read from `process.env`                      | Use `Effect.withConfigProvider()`           |
