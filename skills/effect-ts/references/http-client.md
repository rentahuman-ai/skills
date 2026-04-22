# Effect HttpClient Reference

## Quick Reference

| Operation       | Code                                                                                                                |
| --------------- | ------------------------------------------------------------------------------------------------------------------- |
| GET request     | `client.get(url).pipe(Effect.scoped)`                                                                               |
| POST JSON       | `HttpClientRequest.post(url).pipe(HttpClientRequest.bodyJson(data), Effect.flatMap(client.execute), Effect.scoped)` |
| Filter 2xx      | `client.pipe(HttpClient.filterStatusOk)`                                                                            |
| Parse JSON      | `yield* response.json`                                                                                              |
| Schema validate | `HttpClientResponse.schemaBodyJson(Schema)(response)`                                                               |
| Retry 3x        | `Effect.retry({ times: 3 })`                                                                                        |
| Timeout         | `Effect.timeout(Duration.seconds(5))`                                                                               |
| Base URL        | `HttpClient.mapRequest(HttpClientRequest.prependUrl(baseUrl))`                                                      |
| Stream response | `response.stream`                                                                                                   |
| WebSocket       | `Socket.makeWebSocket(url)`                                                                                         |
| API client      | `HttpApiClient.make(MyApi, { baseUrl })`                                                                            |

## Installation

```bash
pnpm add effect @effect/platform @effect/platform-node
```

## Basic Usage

```typescript
import { HttpClient, FetchHttpClient } from '@effect/platform';
import { Effect } from 'effect';

const program = Effect.gen(function* () {
  const client = yield* HttpClient.HttpClient;
  const response = yield* client.get('https://api.example.com/users');
  return yield* response.json;
}).pipe(Effect.scoped, Effect.provide(FetchHttpClient.layer));

Effect.runPromise(program);
```

## Request Construction

### Headers and Authentication

```typescript
import { HttpClientRequest } from '@effect/platform';

const request = HttpClientRequest.get('https://api.example.com/data').pipe(
  HttpClientRequest.setHeader('Authorization', 'Bearer token'),
  HttpClientRequest.setHeaders({ 'Content-Type': 'application/json' }),
  HttpClientRequest.acceptJson
);

// Auth shortcuts
HttpClientRequest.basicAuth('user', 'pass');
HttpClientRequest.bearerToken('jwt-token');
```

### Query Parameters

```typescript
const request = HttpClientRequest.get('https://api.example.com/search').pipe(
  HttpClientRequest.setUrlParams({ page: '1', limit: '10' }),
  HttpClientRequest.appendUrlParam('tag', 'typescript')
);
```

### Request Bodies

```typescript
// JSON body (returns Effect - validates serialization)
HttpClientRequest.post(url).pipe(
  HttpClientRequest.bodyJson({ name: 'John' }),
  Effect.flatMap(client.execute)
);

// URL-encoded form
HttpClientRequest.post(url).pipe(
  HttpClientRequest.bodyUrlParams({ username: 'john', password: 'secret' })
);

// Schema-validated JSON
HttpClientRequest.post(url).pipe(
  HttpClientRequest.schemaBodyJson(UserSchema)({
    name: 'John',
    email: 'j@e.com',
  })
);
```

## Response Handling

### Reading Response Data

```typescript
const response = yield * client.get(url);
const json = yield * response.json; // unknown
const text = yield * response.text; // string
const bytes = yield * response.uint8Array; // Uint8Array
console.log(response.status, response.headers);
```

### Schema Validation

```typescript
import { HttpClientResponse } from '@effect/platform';
import { Schema } from 'effect';

const User = Schema.Struct({
  id: Schema.Number,
  name: Schema.String,
  email: Schema.String,
});

const user = yield * HttpClientResponse.schemaBodyJson(User)(response);
```

### Status Filtering

```typescript
// Filter for 2xx only (errors on non-2xx)
const safeClient = client.pipe(HttpClient.filterStatusOk);

// Custom status filter
const customClient = client.pipe(
  HttpClient.filterStatus((status) => status >= 200 && status < 400)
);

// Filter with custom error
const strictClient = client.pipe(
  HttpClient.filterOrFail(
    (r) => r.status === 200,
    (r) => new Error(`Unexpected: ${r.status}`)
  )
);
```

### Error Handling

```typescript
yield *
  client.get(url).pipe(
    Effect.catchTag('RequestError', (e) => {
      // Network errors, DNS failures
      console.error('Request failed:', e.reason);
      return Effect.fail(e);
    }),
    Effect.catchTag('ResponseError', (e) => {
      // Server error response
      console.error('Response error:', e.response.status);
      return Effect.fail(e);
    }),
    Effect.scoped
  );
```

## Retry Patterns

### Basic Retry

```typescript
import { Schedule, Duration } from 'effect';

// Simple retry count
yield * client.get(url).pipe(Effect.retry({ times: 3 }), Effect.scoped);

// Exponential backoff with jitter (recommended)
const backoff = Schedule.jittered(Schedule.exponential(Duration.seconds(1)));
yield * client.get(url).pipe(Effect.retry(backoff), Effect.scoped);

// Bounded exponential (max 5 retries)
const bounded = Schedule.exponential(Duration.seconds(1)).pipe(
  Schedule.compose(Schedule.recurs(5))
);
```

### Conditional Retry

```typescript
yield *
  client.get(url).pipe(
    Effect.retry({
      times: 3,
      while: (error) => {
        if (error._tag === 'ResponseError') {
          return error.response.status >= 500; // Retry 5xx only
        }
        return error._tag === 'RequestError'; // Retry network errors
      },
    }),
    Effect.scoped
  );
```

### Retry Transient Errors

Built-in handling for rate limits (429), timeouts, and network issues:

```typescript
const client = (yield * HttpClient.HttpClient).pipe(
  HttpClient.retryTransient({
    schedule: Schedule.jittered(Schedule.exponential(Duration.seconds(1))),
    times: 3,
  })
);
```

## Timeout Handling

```typescript
import { Duration, Option } from 'effect';

// Simple timeout
yield *
  client.get(url).pipe(Effect.timeout(Duration.seconds(5)), Effect.scoped);

// Timeout with fallback
const result =
  yield *
  client
    .get(url)
    .pipe(Effect.timeout(Duration.seconds(5)), Effect.option, Effect.scoped);
if (Option.isNone(result)) return { cached: true, data: [] };

// Combined: timeout each attempt, retry, overall timeout
yield *
  client
    .get(url)
    .pipe(
      Effect.timeout(Duration.seconds(5)),
      Effect.retry({ times: 3 }),
      Effect.timeout(Duration.seconds(30)),
      Effect.scoped
    );
```

## Caching

### Effect Cache

```typescript
import { Cache, Duration } from 'effect';

const userCache =
  yield *
  Cache.make({
    capacity: 100,
    timeToLive: Duration.minutes(5),
    lookup: (userId: number) =>
      client
        .get(`/users/${userId}`)
        .pipe(
          Effect.flatMap(HttpClientResponse.schemaBodyJson(User)),
          Effect.scoped
        ),
  });

const user = yield * userCache.get(1); // Fetches
const cached = yield * userCache.get(1); // Returns cached
yield * userCache.invalidate(1); // Clear entry
yield * userCache.refresh(1); // Refresh in background
```

### Simple Memoization

```typescript
const getConfig = client
  .get('/config')
  .pipe(
    Effect.flatMap(HttpClientResponse.schemaBodyJson(ConfigSchema)),
    Effect.scoped,
    Effect.cachedWithTTL(Duration.minutes(10))
  );

const config = yield * yield * getConfig; // Note: double yield* for cached effect
```

## Reusable API Client Pattern

```typescript
import {
  HttpClient,
  HttpClientRequest,
  FetchHttpClient,
} from '@effect/platform';
import { Effect, Layer, pipe } from 'effect';

class ApiClient extends Effect.Service<ApiClient>()('ApiClient', {
  effect: Effect.gen(function* () {
    const baseClient = yield* HttpClient.HttpClient;

    const client = baseClient.pipe(
      HttpClient.mapRequest(
        pipe(
          HttpClientRequest.prependUrl('https://api.example.com'),
          HttpClientRequest.setHeader('Accept', 'application/json')
        )
      ),
      HttpClient.filterStatusOk
    );

    return {
      getUsers: client.get('/users').pipe(Effect.scoped),
      getUser: (id: string) => client.get(`/users/${id}`).pipe(Effect.scoped),
      createUser: (data: unknown) =>
        client
          .post('/users')
          .pipe(HttpClientRequest.bodyJson(data), Effect.scoped),
    };
  }),
}) {}

const ApiClientLive = ApiClient.Default.pipe(
  Layer.provide(FetchHttpClient.layer)
);
```

## Testing

### Mock Fetch Layer

```typescript
import { FetchHttpClient, HttpClient } from '@effect/platform';
import { Effect, Layer } from 'effect';

const mockFetch = (url: string | URL | Request): Promise<Response> => {
  const urlString = typeof url === 'string' ? url : url.toString();
  if (urlString.includes('/users')) {
    return Promise.resolve(
      new Response(JSON.stringify([{ id: 1, name: 'John' }]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );
  }
  return Promise.resolve(new Response('Not Found', { status: 404 }));
};

const TestFetchLayer = Layer.succeed(FetchHttpClient.Fetch, mockFetch);
const TestHttpClientLayer = FetchHttpClient.layer.pipe(
  Layer.provide(TestFetchLayer)
);

// Usage in tests
const result = await Effect.runPromise(
  program.pipe(Effect.provide(TestHttpClientLayer))
);
```

## Error Types

| Error Tag       | Cause                                            |
| --------------- | ------------------------------------------------ |
| `RequestError`  | Network failures, DNS errors, connection refused |
| `ResponseError` | Non-2xx status (when using filterStatusOk)       |
| `BodyError`     | Failed to parse response body                    |

## HttpClient Methods

| Method                   | Description                        |
| ------------------------ | ---------------------------------- |
| `execute(request)`       | Execute a custom HttpClientRequest |
| `get(url, options?)`     | GET request                        |
| `post(url, options?)`    | POST request                       |
| `put(url, options?)`     | PUT request                        |
| `patch(url, options?)`   | PATCH request                      |
| `del(url, options?)`     | DELETE request                     |
| `head(url, options?)`    | HEAD request                       |
| `options(url, options?)` | OPTIONS request                    |

## Streaming Responses

### Reading Response as Stream

```typescript
import { HttpClient, FetchHttpClient } from '@effect/platform';
import { Effect, Stream } from 'effect';

const program = Effect.gen(function* () {
  const client = yield* HttpClient.HttpClient;
  const response = yield* client.get('https://api.example.com/large-file');

  // Get response body as stream
  const bodyStream = response.stream;

  // Process chunks as they arrive
  yield* Stream.runForEach(bodyStream, (chunk) =>
    Effect.sync(() => console.log(`Received ${chunk.length} bytes`))
  );
}).pipe(Effect.scoped, Effect.provide(FetchHttpClient.layer));
```

### Consuming Server-Sent Events (SSE)

```typescript
import { HttpClient, FetchHttpClient } from '@effect/platform';
import { Effect, Stream, Chunk } from 'effect';

const program = Effect.gen(function* () {
  const client = yield* HttpClient.HttpClient;
  const response = yield* client.get('https://api.example.com/events');

  // Parse SSE stream
  const decoder = new TextDecoder();
  let buffer = '';

  yield* Stream.runForEach(response.stream, (chunk) =>
    Effect.gen(function* () {
      buffer += decoder.decode(chunk, { stream: true });

      // Parse complete SSE messages (separated by \n\n)
      const messages = buffer.split('\n\n');
      buffer = messages.pop() || ''; // Keep incomplete message in buffer

      for (const message of messages) {
        if (message.startsWith('data: ')) {
          const data = JSON.parse(message.slice(6));
          yield* Effect.logInfo(`SSE Event: ${JSON.stringify(data)}`);
        }
      }
    })
  );
}).pipe(Effect.scoped, Effect.provide(FetchHttpClient.layer));
```

### Streaming Download to File

```typescript
import { HttpClient, FetchHttpClient } from '@effect/platform';
import { FileSystem } from '@effect/platform-node';
import { Effect, Stream } from 'effect';

const downloadFile = (url: string, outputPath: string) =>
  Effect.gen(function* () {
    const client = yield* HttpClient.HttpClient;
    const fs = yield* FileSystem.FileSystem;

    const response = yield* client.get(url);

    // Stream directly to file
    yield* fs.sink(outputPath).pipe(Stream.run(response.stream));
  }).pipe(Effect.scoped);
```

## WebSocket Client

### Socket Module

```typescript
import { Socket } from '@effect/platform';
import { Effect, Duration } from 'effect';

// Create WebSocket connection
const program = Effect.gen(function* () {
  const socket = yield* Socket.makeWebSocket('wss://api.example.com/ws', {
    openTimeout: Duration.seconds(10),
    protocols: ['graphql-ws'],
    closeCodeIsError: (code) => code !== 1000 && code !== 1001,
  });

  // Get writer for sending messages
  const write = yield* socket.writer;

  // Send a message
  yield* write(JSON.stringify({ type: 'ping' }));

  // Handle incoming messages
  yield* socket.runRaw((data) => {
    if (typeof data === 'string') {
      const message = JSON.parse(data);
      console.log('Received:', message);
    }
  });
});
```

### WebSocket with Bidirectional Communication

```typescript
import { Socket } from '@effect/platform';
import { Effect, Fiber, Queue, Duration } from 'effect';

const wsClient = Effect.gen(function* () {
  const socket = yield* Socket.makeWebSocket('wss://api.example.com/ws');
  const messageQueue = yield* Queue.unbounded<unknown>();

  // Start receiver in background
  const receiver = yield* Effect.fork(
    socket.runRaw((data) =>
      Effect.gen(function* () {
        if (typeof data === 'string') {
          const message = JSON.parse(data);
          yield* Queue.offer(messageQueue, message);
        }
      })
    )
  );

  // Get writer
  const write = yield* socket.writer;

  // Send subscription message
  yield* write(
    JSON.stringify({
      type: 'subscribe',
      channel: 'updates',
    })
  );

  // Process messages from queue
  yield* Effect.gen(function* () {
    while (true) {
      const message = yield* Queue.take(messageQueue);
      yield* Effect.logInfo(`Message: ${JSON.stringify(message)}`);
    }
  }).pipe(
    Effect.timeout(Duration.minutes(5)),
    Effect.catchAll(() => Effect.void)
  );

  // Cleanup
  yield* Fiber.interrupt(receiver);
  yield* write(new CloseEvent(1000, 'Done'));
});
```

### WebSocket as Layer

```typescript
import { Socket } from '@effect/platform';
import { Effect, Layer } from 'effect';

// Create a reusable WebSocket layer
const WebSocketLive = Socket.layerWebSocket('wss://api.example.com/ws', {
  openTimeout: Duration.seconds(10),
});

// Use the socket anywhere
const program = Effect.gen(function* () {
  const socket = yield* Socket.Socket;
  const write = yield* socket.writer;

  yield* write('Hello from client');

  yield* socket.run((data) => {
    console.log('Received binary:', data.length, 'bytes');
  });
}).pipe(Effect.provide(WebSocketLive));
```

### Browser WebSocket

```typescript
import { BrowserSocket } from '@effect/platform-browser';

// Browser-specific WebSocket layer
const WebSocketLive = BrowserSocket.layerWebSocket('wss://api.example.com/ws');

// Use with same Socket interface
const program = Effect.gen(function* () {
  const socket = yield* Socket.Socket;
  // ... same as Node.js
}).pipe(Effect.provide(WebSocketLive));
```

## HttpApiClient (Type-Safe API Client)

When you have an HttpApi definition, you can generate a fully typed client.

### Basic Usage

```typescript
import {
  HttpApiClient,
  FetchHttpClient,
  HttpClient,
  HttpClientRequest,
} from '@effect/platform';
import { Effect } from 'effect';

// Assuming MyApi is defined using HttpApi.make
const program = Effect.gen(function* () {
  const client = yield* HttpApiClient.make(MyApi, {
    baseUrl: 'http://localhost:3000',
  });

  // Fully typed calls matching the API definition
  const users = yield* client.users.list();
  const user = yield* client.users.get({ path: { id: 1 } });
  const created = yield* client.users.create({
    payload: { name: 'John', email: 'john@example.com' },
  });
}).pipe(Effect.provide(FetchHttpClient.layer));
```

### With Authentication

```typescript
const program = Effect.gen(function* () {
  const client = yield* HttpApiClient.make(MyApi, {
    baseUrl: 'http://localhost:3000',
    transformClient: HttpClient.mapRequest(
      HttpClientRequest.bearerToken('your-jwt-token')
    ),
  });

  // All requests include Authorization header
  const protectedData = yield* client.protected.getData();
});
```

### With Custom Headers

```typescript
const program = Effect.gen(function* () {
  const client = yield* HttpApiClient.make(MyApi, {
    baseUrl: 'http://localhost:3000',
    transformClient: HttpClient.mapRequest(
      pipe(
        HttpClientRequest.setHeader('X-API-Key', 'your-api-key'),
        HttpClientRequest.setHeader('X-Request-ID', generateRequestId())
      )
    ),
  });
});
```

### Error Handling

```typescript
const program = Effect.gen(function* () {
  const client = yield* HttpApiClient.make(MyApi, { baseUrl });

  // Errors are typed based on API definition
  const result = yield* client.users.get({ path: { id: 1 } }).pipe(
    Effect.catchTag('NotFoundError', (error) =>
      Effect.succeed({ fallback: true, user: null })
    ),
    Effect.catchTag('UnauthorizedError', (error) =>
      Effect.fail(new Error('Please log in'))
    )
  );
});
```
