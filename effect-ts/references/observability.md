# Effect Observability Reference

## Quick Reference

| Function                             | Purpose                              |
| ------------------------------------ | ------------------------------------ |
| `Effect.log(msg)`                    | Log at INFO level with fiber context |
| `Effect.logDebug/Warning/Error(msg)` | Log at specific levels               |
| `Effect.annotateLogs(k, v)`          | Add context to all logs in scope     |
| `Effect.withSpan(name)`              | Create trace span                    |
| `Effect.annotateCurrentSpan(k, v)`   | Add span attribute                   |
| `Metric.counter(name)`               | Cumulative count                     |
| `Metric.gauge(name)`                 | Point-in-time value                  |
| `Metric.histogram(name, bounds)`     | Distribution                         |
| `Effect.tap/tapError(fn)`            | Debug without modifying result       |

---

## Preferred Tracing: Effect.fn

`Effect.fn` automatically creates spans — prefer it over manual `Effect.withSpan` for service and repo methods:

```typescript
// PREFERRED: Effect.fn auto-creates "Bookmarks.create" span
const create = Effect.fn('Bookmarks.create')(function* (
  userId: string,
  input: CreateBookmark
) {
  yield* Effect.annotateCurrentSpan('userId', userId);
  yield* Effect.log('Creating bookmark');
  return yield* repo.insert(input);
});

// AVOID: Manual Effect.withSpan
const create = (userId: string, input: CreateBookmark) =>
  pipe(
    repo.insert(input),
    Effect.withSpan('Bookmarks.create', { attributes: { userId } })
  );
```

Use `Effect.annotateCurrentSpan` inside `Effect.fn` bodies to add span attributes. Use `Effect.log` for structured logging of significant operations.

---

## Structured Logging

```typescript
import { Effect, Logger, LogLevel } from 'effect';

// Basic logging
yield * Effect.log('Application started');
yield * Effect.logDebug('Debug info'); // Disabled by default
yield * Effect.logWarning('Warning msg');
yield * Effect.logError('Error occurred');

// Enable debug logs
const debugProgram = myEffect.pipe(Logger.withMinimumLogLevel(LogLevel.Debug));

// Log annotations - add context to all logs
const annotated = myEffect.pipe(
  Effect.annotateLogs({
    requestId: 'abc-123',
    userId: 'user-456',
  })
);
```

### Built-in Loggers

| Logger                | Use Case             |
| --------------------- | -------------------- |
| `Logger.stringLogger` | Default, key-value   |
| `Logger.prettyLogger` | Development (colors) |
| `Logger.jsonLogger`   | Production           |

```typescript
// Use pretty logger for development
const devProgram = myEffect.pipe(Effect.provide(Logger.pretty));

// Custom JSON logger
const jsonLogger = Logger.make(
  ({ logLevel, message, date, annotations, cause }) => {
    console.log(
      JSON.stringify({
        timestamp: date.toISOString(),
        level: logLevel.label,
        message: String(message),
        ...Object.fromEntries(annotations),
        ...(cause ? { error: Cause.pretty(cause) } : {}),
      })
    );
  }
);
const JsonLoggerLive = Logger.replace(Logger.defaultLogger, jsonLogger);
```

---

## Tracing with Spans

```typescript
import { Effect } from 'effect';

// Basic span
const traced = myEffect.pipe(Effect.withSpan('processUserData'));

// Span with attributes
const tracedDb = myEffect.pipe(
  Effect.withSpan('database-query', {
    attributes: { 'db.system': 'postgresql', 'db.operation': 'SELECT' },
  })
);

// Nested spans (automatic parent-child linking)
const nestedSpans = Effect.gen(function* () {
  const users = yield* fetchUsers().pipe(Effect.withSpan('fetchUsers'));
  const result = yield* processUsers(users).pipe(
    Effect.withSpan('processUsers')
  );
  return result;
}).pipe(Effect.withSpan('parentOperation'));

// Add attributes to current span
const annotatedSpan = Effect.gen(function* () {
  yield* Effect.annotateCurrentSpan('user.id', userId);
  yield* Effect.annotateCurrentSpan('result.count', result.length);
});

// Light timing (log only, no full span)
const timed = myEffect.pipe(Effect.withLogSpan('database-query'));
```

---

## OpenTelemetry Integration

```bash
npm install @effect/opentelemetry @opentelemetry/sdk-trace-base @opentelemetry/exporter-trace-otlp-http
```

```typescript
import { NodeSdk } from '@effect/opentelemetry';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { Effect } from 'effect';

const NodeSdkLive = NodeSdk.layer(() => ({
  resource: { serviceName: 'my-service', serviceVersion: '1.0.0' },
  spanProcessor: new BatchSpanProcessor(
    new OTLPTraceExporter({ url: 'http://localhost:4318/v1/traces' })
  ),
}));

const program = myEffect.pipe(Effect.provide(NodeSdkLive));
```

---

## Metrics

| Type      | Use Case             | Create                           |
| --------- | -------------------- | -------------------------------- |
| Counter   | Request count        | `Metric.counter(name)`           |
| Gauge     | Active connections   | `Metric.gauge(name)`             |
| Histogram | Latency distribution | `Metric.histogram(name, bounds)` |

```typescript
import { Effect, Metric, MetricBoundaries } from 'effect';

// Counter
const requestCounter = Metric.counter('http_requests_total');
yield * Metric.increment(requestCounter);

// Counter with tags
const taggedCounter = requestCounter.pipe(
  Metric.tagged('method', 'GET'),
  Metric.tagged('path', '/api/users')
);

// Gauge
const activeConnections = Metric.gauge('active_connections');
yield * Metric.set(activeConnections, 42);

// Histogram
const requestLatency = Metric.histogram(
  'http_request_duration_seconds',
  MetricBoundaries.linear({ start: 0, width: 0.1, count: 10 })
);
yield * Metric.record(requestLatency, durationInSeconds);

// Auto-track duration
const timedEffect = myEffect.pipe(Metric.trackDuration(requestLatency));
```

---

## Debugging

| Function                                   | Inspects            |
| ------------------------------------------ | ------------------- |
| `Effect.tap(fn)`                           | Success value       |
| `Effect.tapError(fn)`                      | Error               |
| `Effect.tapErrorTag("Tag", fn)`            | Specific error type |
| `Effect.tapErrorCause(fn)`                 | Full cause          |
| `Effect.tapBoth({ onSuccess, onFailure })` | Both outcomes       |

```typescript
import { Effect, Console, Cause } from 'effect';

const debugProgram = fetchData().pipe(
  Effect.tap((data) => Console.log('Raw:', data)),
  Effect.map(processData),
  Effect.tapError((error) => Effect.log(`Error: ${error.message}`)),
  Effect.tapErrorCause((cause) => Console.log('Cause:', Cause.pretty(cause)))
);

// Inspect specific error type
const handleErrors = myEffect.pipe(
  Effect.tapErrorTag('NetworkError', (e) =>
    Console.log(`Status: ${e.statusCode}`)
  )
);
```

---

## Production Patterns

### Request Tracing Helper

```typescript
import { Effect, FiberRef } from 'effect';

const withRequestId = <A, E, R>(
  requestId: string,
  effect: Effect.Effect<A, E, R>
) =>
  effect.pipe(
    Effect.annotateLogs('requestId', requestId),
    Effect.withSpan('request', { attributes: { requestId } })
  );

const handleRequest = (req: Request) =>
  withRequestId(
    req.headers.get('x-request-id') ?? crypto.randomUUID(),
    processRequest(req)
  );
```

### Complete Observability Layer

```typescript
import { Effect, Layer, Logger, LogLevel } from 'effect';
import { NodeSdk } from '@effect/opentelemetry';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';

const ObservabilityLive = Layer.mergeAll(
  NodeSdk.layer(() => ({
    resource: { serviceName: process.env.SERVICE_NAME! },
    spanProcessor: new BatchSpanProcessor(
      new OTLPTraceExporter({ url: process.env.OTLP_ENDPOINT! })
    ),
  })),
  Logger.replace(Logger.defaultLogger, jsonLogger),
  Logger.minimumLogLevel(LogLevel.fromLiteral(process.env.LOG_LEVEL || 'Info'))
);

const main = myEffect.pipe(Effect.provide(ObservabilityLive));
```

---

## Anti-Patterns

| Anti-Pattern                 | Problem                | Better Approach             |
| ---------------------------- | ---------------------- | --------------------------- |
| `console.log` in Effect code | Loses fiber context    | Use `Effect.log`            |
| No request IDs               | Hard to trace requests | Use `Effect.annotateLogs`   |
| Missing spans on async ops   | Incomplete traces      | Wrap with `Effect.withSpan` |
| Dynamic metric names         | Memory issues          | Use tags instead            |
