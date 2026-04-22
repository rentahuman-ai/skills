# Effect Schema Reference

## Quick Reference

| Task                  | Pattern                                                        |
| --------------------- | -------------------------------------------------------------- |
| Basic struct          | `Schema.Struct({ name: Schema.String })`                       |
| Optional field        | `Schema.optional(Schema.String)`                               |
| Optional with default | `Schema.optional(Schema.String, { default: () => "value" })`   |
| Nullable              | `Schema.NullOr(Schema.String)`                                 |
| Array                 | `Schema.Array(Schema.String)`                                  |
| Union/enum            | `Schema.Literal("a", "b", "c")`                                |
| Branded type          | `Schema.String.pipe(Schema.brand("UserId"))`                   |
| Class schema          | `class User extends Schema.Class<User>("User")({...}) {}`      |
| Tagged class          | `class Err extends Schema.TaggedClass<Err>()("Err", {...}) {}` |
| Decode unknown        | `Schema.decodeUnknownSync(MySchema)(data)`                     |
| Type inference        | `type T = Schema.Schema.Type<typeof MySchema>`                 |
| Error formatting      | `ArrayFormatter.formatErrorSync(parseError)`                   |
| Test generation       | `Arbitrary.make(MySchema)`                                     |

---

## Primitives

```typescript
import { Schema } from 'effect';

Schema.String; // any string
Schema.NonEmptyString; // length >= 1
Schema.Number; // any number
Schema.Int; // integers only
Schema.Boolean;
Schema.Literal('pending', 'active', 'completed'); // union type
```

---

## Struct (Objects)

```typescript
const UserSchema = Schema.Struct({
  id: Schema.String,
  name: Schema.String,
  nickname: Schema.optional(Schema.String), // field can be missing
  role: Schema.optional(Schema.String, { default: () => 'user' }), // with default
});

type User = Schema.Schema.Type<typeof UserSchema>;

// Extending structs
const User = Schema.Struct({ ...BaseEntity.fields, email: Schema.String });

// Pick, omit, partial
const PublicUser = User.pipe(Schema.pick('id', 'name'));
const PartialUser = Schema.partial(User);
```

---

## Validation Filters

```typescript
// String filters
const Username = Schema.String.pipe(
  Schema.minLength(3),
  Schema.maxLength(20),
  Schema.pattern(/^[a-z0-9_]+$/)
);

// Number filters
const Age = Schema.Number.pipe(Schema.int(), Schema.between(0, 150));

// Custom filter with error message
const Email = Schema.String.pipe(
  Schema.filter((s) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s) || 'Invalid email')
);
```

---

## Transformations

```typescript
Schema.NumberFromString; // "42" -> 42
Schema.DateFromString; // ISO string -> Date
Schema.Trim; // trims whitespace
Schema.parseJson(MySchema); // parse and validate JSON

// Custom transform
const BoolFromString = Schema.transform(
  Schema.Literal('true', 'false'),
  Schema.Boolean,
  { decode: (s) => s === 'true', encode: (b) => (b ? 'true' : 'false') }
);
```

---

## Branded Types

```typescript
const UserId = Schema.String.pipe(Schema.brand('UserId'));
const OrderId = Schema.String.pipe(Schema.brand('OrderId'));

type UserId = Schema.Schema.Type<typeof UserId>; // string & Brand<"UserId">
const id = UserId.make('user_123');
```

---

## Schema Classes

`Schema.Class` creates schemas that define both a TypeScript class and validation.

```typescript
class Person extends Schema.Class<Person>('Person')({
  id: Schema.Number,
  name: Schema.NonEmptyString,
  email: Schema.String,
}) {
  get displayName() {
    return `${this.name} (${this.email})`;
  }
}

// Constructor validates automatically
const person = new Person({ id: 1, name: 'Alice', email: 'alice@example.com' });

// Decoding creates class instances
const decoded = Schema.decodeUnknownSync(Person)({
  id: 1,
  name: 'Bob',
  email: 'bob@example.com',
});
console.log(decoded instanceof Person); // true
```

### Tagged Classes (Discriminated Unions)

```typescript
class Circle extends Schema.TaggedClass<Circle>()('Circle', {
  radius: Schema.Number,
}) {}

class Rectangle extends Schema.TaggedClass<Rectangle>()('Rectangle', {
  width: Schema.Number,
  height: Schema.Number,
}) {}

const Shape = Schema.Union(Circle, Rectangle);
const circle = new Circle({ radius: 5 });
console.log(circle._tag); // "Circle"
```

### Tagged Errors

```typescript
class HttpError extends Schema.TaggedError<HttpError>()('HttpError', {
  status: Schema.Number,
  message: Schema.String,
}) {}

// Usage in Effect
const fetchUser = Effect.fail(
  new HttpError({ status: 404, message: 'User not found' })
);
```

---

## Default Constructors

`Schema.withConstructorDefault` makes fields optional during construction.

```typescript
const Person = Schema.Struct({
  name: Schema.String,
  age: Schema.Number.pipe(
    Schema.propertySignature,
    Schema.withConstructorDefault(() => 0)
  ),
  createdAt: Schema.Date.pipe(
    Schema.propertySignature,
    Schema.withConstructorDefault(() => new Date())
  ),
});

const person = Person.make({ name: 'Alice' });
// { name: "Alice", age: 0, createdAt: <current date> }
```

### With Classes

```typescript
class User extends Schema.Class<User>('User')({
  id: Schema.String,
  name: Schema.String,
  role: Schema.String.pipe(
    Schema.propertySignature,
    Schema.withConstructorDefault(() => 'user')
  ),
}) {}

const user = new User({ id: '123', name: 'Alice' });
console.log(user.role); // "user"
```

---

## Error Formatters

### TreeFormatter (Default)

```typescript
import { Schema, TreeFormatter } from 'effect';

const result = Schema.decodeUnknownEither(Person)({ name: 123 });
if (result._tag === 'Left') {
  console.log(TreeFormatter.formatErrorSync(result.left));
}
// Person
// └─ ["name"]
//    └─ Expected string, actual 123
```

### ArrayFormatter (Best for APIs)

```typescript
import { Schema, ArrayFormatter } from 'effect';

const result = Schema.decodeUnknownEither(Person, { errors: 'all' })({});
if (result._tag === 'Left') {
  const errors = ArrayFormatter.formatErrorSync(result.left);
  console.log(errors);
}
// [
//   { _tag: 'Missing', path: ['name'], message: 'is missing' },
//   { _tag: 'Missing', path: ['age'], message: 'is missing' }
// ]
```

### Custom Error Messages

```typescript
const Email = Schema.String.pipe(
  Schema.pattern(/^[^\s@]+@[^\s@]+\.[^\s@]+$/),
  Schema.annotations({ message: () => 'Invalid email format' })
);

const PositiveInt = Schema.Number.pipe(
  Schema.int({ message: () => 'Must be an integer' }),
  Schema.positive({ message: () => 'Must be positive' })
);
```

---

## Arbitrary (Test Generation)

Generate random values conforming to schemas using fast-check.

```typescript
import { Schema, Arbitrary } from 'effect';
import * as FastCheck from 'fast-check';

const Person = Schema.Struct({
  name: Schema.String,
  age: Schema.Number.pipe(Schema.int, Schema.between(0, 120)),
});

const personArbitrary = Arbitrary.make(Person);

// Generate samples
const samples = FastCheck.sample(personArbitrary, 3);

// Property-based testing
FastCheck.assert(
  FastCheck.property(
    personArbitrary,
    (person) => person.age >= 0 && person.age <= 120
  )
);
```

### Custom Arbitraries

```typescript
const Name = Schema.NonEmptyString.annotations({
  arbitrary: () => (fc) => fc.constantFrom('Alice', 'Bob', 'Charlie'),
});

// With faker for realistic data
import { faker } from '@faker-js/faker';

const RealisticPerson = Schema.Struct({
  name: Schema.String.annotations({
    arbitrary: () => (fc) =>
      fc.constant(null).map(() => faker.person.fullName()),
  }),
  email: Schema.String.annotations({
    arbitrary: () => (fc) =>
      fc.constant(null).map(() => faker.internet.email()),
  }),
});
```

---

## API Schema Patterns

```typescript
// Request schemas
const CreateUserRequest = Schema.Struct({
  email: Schema.String.pipe(Schema.Trim, Schema.Lowercase),
  name: Schema.String.pipe(Schema.minLength(1)),
  password: Schema.String.pipe(Schema.minLength(8)),
});
const UpdateUserRequest = Schema.partial(
  CreateUserRequest.pipe(Schema.omit('password'))
);

// Pagination
const PaginationParams = Schema.Struct({
  page: Schema.optional(
    Schema.NumberFromString.pipe(Schema.int(), Schema.positive()),
    { default: () => 1 }
  ),
  limit: Schema.optional(
    Schema.NumberFromString.pipe(Schema.int(), Schema.between(1, 100)),
    { default: () => 20 }
  ),
});

// Error response
const ErrorResponse = Schema.Struct({
  error: Schema.Struct({
    code: Schema.String,
    message: Schema.String,
    details: Schema.optional(Schema.Unknown),
  }),
});
```

---

## Decoding Data

```typescript
// Sync (throws on error)
const user = Schema.decodeUnknownSync(UserSchema)(rawData);
Schema.decodeUnknownSync(UserSchema)(rawData, { onExcessProperty: 'error' });

// Either (returns Left on error)
const result = Schema.decodeUnknownEither(UserSchema)(rawData);

// Effect (for async pipelines)
const program = Schema.decodeUnknown(UserSchema)(rawData); // Effect<User, ParseError>
```

---

## Common Reusable Schemas

```typescript
export const Email = Schema.String.pipe(
  Schema.Trim,
  Schema.Lowercase,
  Schema.filter((s) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s)),
  Schema.brand('Email')
);
export const UUID = Schema.String.pipe(
  Schema.pattern(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
  ),
  Schema.brand('UUID')
);
export const NonEmptyString = Schema.String.pipe(Schema.minLength(1));
export const Cents = Schema.Number.pipe(
  Schema.int(),
  Schema.nonNegative(),
  Schema.brand('Cents')
);
```

---

## Schema Annotations for OpenAPI

When schemas are used in HTTP APIs, annotations control how they appear in generated OpenAPI documentation. Properly annotated schemas produce clear, self-documenting API specs.

### Core Annotations

```typescript
import { Schema, JSONSchema } from 'effect';

// identifier - Controls $ref name in OpenAPI components
const User = Schema.Struct({
  id: Schema.Number,
  name: Schema.String,
  email: Schema.String,
}).annotations({
  identifier: 'User', // Generates: "#/components/schemas/User"
});

// title - Human-readable name shown in docs
const Product = Schema.Struct({
  sku: Schema.String,
  price: Schema.Number,
}).annotations({
  title: 'Product',
});

// description - Detailed explanation of the schema
const Order = Schema.Struct({
  orderId: Schema.String,
  items: Schema.Array(Schema.String),
}).annotations({
  description: 'Represents a customer order containing one or more items',
});

// examples - Sample values shown in documentation
const Email = Schema.String.pipe(
  Schema.pattern(/^[^\s@]+@[^\s@]+\.[^\s@]+$/)
).annotations({
  examples: ['user@example.com', 'admin@company.org'],
});

// default - Default value when not provided
const PageSize = Schema.Number.pipe(
  Schema.int(),
  Schema.between(1, 100)
).annotations({
  default: 20,
});

// documentation - Extended technical documentation
const ApiKey = Schema.String.annotations({
  documentation:
    "API keys can be generated in the dashboard. Keys are prefixed with 'ak_' for production.",
});
```

### Combining Annotations

```typescript
const CreateUserRequest = Schema.Struct({
  name: Schema.String.annotations({
    title: 'Name',
    description: "The user's full name",
    examples: ['John Doe', 'Jane Smith'],
  }),
  email: Schema.String.pipe(
    Schema.pattern(/^[^\s@]+@[^\s@]+\.[^\s@]+$/)
  ).annotations({
    title: 'Email',
    description: 'A valid email address',
    examples: ['user@example.com'],
  }),
  role: Schema.optional(Schema.Literal('admin', 'user', 'guest')).annotations({
    title: 'Role',
    description: "The user's role in the system",
    default: 'user',
  }),
}).annotations({
  identifier: 'CreateUserRequest',
  title: 'Create User Request',
  description: 'Request body for creating a new user',
});
```

### Custom JSON Schema Override

For unsupported types or custom formats, override the JSON Schema output directly.

```typescript
// For types like bigint that don't map directly
const BigIntId = Schema.BigInt.annotations({
  jsonSchema: {
    type: 'string',
    pattern: '^[0-9]+$',
    description: 'A numeric ID represented as a string',
  },
});

// Adding format specifications
const DateTime = Schema.DateTimeUtc.annotations({
  jsonSchema: {
    type: 'string',
    format: 'date-time',
  },
});

// Custom extensions
const MonetaryAmount = Schema.Number.annotations({
  jsonSchema: {
    type: 'number',
    format: 'double',
    minimum: 0,
    'x-currency': 'USD',
  },
});
```

### JSONSchema.make

Convert schemas to JSON Schema format (foundation for OpenAPI generation).

```typescript
import { JSONSchema, Schema } from 'effect';

const User = Schema.Struct({
  id: Schema.Number,
  name: Schema.String,
  active: Schema.Boolean,
});

const jsonSchema = JSONSchema.make(User);
// Output:
// {
//   "$schema": "http://json-schema.org/draft-07/schema#",
//   "type": "object",
//   "required": ["id", "name", "active"],
//   "properties": {
//     "id": { "type": "number" },
//     "name": { "type": "string" },
//     "active": { "type": "boolean" }
//   },
//   "additionalProperties": false
// }
```

Schemas with `identifier` annotations generate `$ref` references for reuse:

```typescript
const Address = Schema.Struct({
  street: Schema.String,
  city: Schema.String,
}).annotations({ identifier: 'Address' });

const Person = Schema.Struct({
  name: Schema.String,
  homeAddress: Address,
  workAddress: Address,
}).annotations({ identifier: 'Person' });

const jsonSchema = JSONSchema.make(Person);
// Generates $defs with Address referenced via $ref in both fields
```

### How Annotations Flow to OpenAPI

When schemas are used in `HttpApiEndpoint`, annotations automatically populate the OpenAPI spec:

```typescript
import { HttpApiEndpoint, HttpApiSchema } from '@effect/platform';

// Well-annotated request schema
const CreateUserRequest = Schema.Struct({
  name: Schema.String.annotations({
    description: "User's full name",
    examples: ['John Doe'],
  }),
  email: Schema.String.annotations({
    description: 'Valid email address',
    examples: ['user@example.com'],
  }),
}).annotations({
  identifier: 'CreateUserRequest',
  title: 'Create User Request',
  description: 'Request body for creating a new user',
});

// Well-annotated response schema
const User = Schema.Struct({
  id: Schema.Number.annotations({ description: 'Unique user identifier' }),
  name: Schema.String.annotations({ description: "User's full name" }),
  email: Schema.String.annotations({ description: "User's email address" }),
}).annotations({
  identifier: 'User',
  title: 'User',
  description: 'A user in the system',
});

// Endpoint definition - annotations flow through automatically
const createUser = HttpApiEndpoint.post('createUser', '/users')
  .setPayload(CreateUserRequest)
  .addSuccess(User, { status: 201 });

// Generated OpenAPI will include all titles, descriptions, examples, and $refs
```

---

## Best Practices

> **All schemas exposed via API should have annotations.** Unannotated schemas produce poor documentation and make APIs harder to consume. At minimum, include `identifier`, `title`, and `description` on every schema used in endpoints.

```typescript
// BAD: Schemas without annotations
const User = Schema.Struct({
  id: Schema.Number,
  name: Schema.String,
});

// GOOD: Fully annotated schemas
const User = Schema.Struct({
  id: Schema.Number.annotations({
    description: 'Unique identifier for the user',
  }),
  name: Schema.String.annotations({
    description: "User's display name",
    examples: ['John Doe'],
  }),
}).annotations({
  identifier: 'User',
  title: 'User',
  description: 'Represents a user in the system',
});
```
