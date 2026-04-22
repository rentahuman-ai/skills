---
name: code-quality
description: 'Universal TypeScript code quality patterns. Use when writing or reviewing any TypeScript code — covers comments, naming, function structure, return types, and file organization.'
---

# Code Quality

Universal patterns for clean, readable, maintainable TypeScript. These apply to all code in the monorepo.

## Comments

### Exported Functions

JSDoc with a description, `@param` for every parameter, `@returns`, and `@template` for every generic type parameter:

```typescript
/**
 * Formats a byte count into a human-readable string
 * using binary units (KB, MB, GB).
 *
 * @param bytes - Raw byte count.
 * @param decimals - Decimal places to display (default: 2).
 * @returns Formatted string like "1.5 MB" or "320 KB".
 */
export function formatBytes(bytes: number, decimals = 2): string {

/**
 * Groups items by a key extracted from each item.
 *
 * @template T - The item type.
 * @template K - The grouping key type.
 * @param items - Array of items to group.
 * @param getKey - Extracts the grouping key from an item.
 * @returns Map from key to array of matching items.
 */
export function groupBy<T, K>(items: readonly T[], getKey: (item: T) => K): Map<K, T[]> {
```

No exceptions — every exported function gets `@param` for every parameter and `@template` for every type parameter, even when the name seems obvious. Consistency over judgment calls.

### Non-Exported Functions

Single-line `/** */` when the name alone isn't enough. Skip for obvious helpers:

```typescript
/** Extracts initials from a full name (max 2 characters). */
function getInitials(name: string): string {

// No comment needed — obvious from name and context
function toggleSidebar() {
```

### Inline Comments

Explain **why**, never **what**. If code needs a "what" comment, rename or restructure instead:

```typescript
// Good — explains a non-obvious decision
// Offset accounts for the sticky header height
const scrollOffset = HEADER_HEIGHT + 16;

// Good — explains a workaround
// Wait for close animation before clearing form state
setTimeout(() => form.reset(), 300);

// Bad — restates what the code does
// Set the offset to header height plus 16
const scrollOffset = HEADER_HEIGHT + 16;
```

### Early Return Guards

Comment only when the condition is non-obvious:

```typescript
// No comment needed — standard guard
if (!user) {
  return null;
}

// Comment needed — non-obvious business rule
// Skip processing for system-generated events to avoid feedback loops
if (event.actorId === SYSTEM_ACTOR_ID) {
  return;
}
```

### What NOT to Comment

- Imports
- Type aliases and interfaces
- Self-explanatory one-liners
- Test files
- Standard framework patterns
- Code that is clear from its types and names

## Naming

### Booleans

Always use a verb prefix. No bare adjectives or nouns:

| Prefix    | Meaning                 | Examples                                        |
| --------- | ----------------------- | ----------------------------------------------- |
| `is*`     | Current state           | `isLoading`, `isOpen`, `isValid`, `isDisabled`  |
| `has*`    | Existence / possession  | `hasError`, `hasChildren`, `hasPermission`      |
| `can*`    | Capability / permission | `canEdit`, `canDelete`, `canSubmit`             |
| `should*` | Conditional behavior    | `shouldValidate`, `shouldRetry`, `shouldRender` |

Rules:

- Never use negative names (`isNotReady`). Invert the condition: `isReady`.
- Derived booleans keep the prefix: `const isValid = name.length > 0`, not `const valid = ...`.

```typescript
// Good
const isLoading = query.status === 'pending';
const hasPermission = user.role === 'admin';
const canSubmit = isValid && !isLoading;

// Bad
const loading = query.status === 'pending';
const permission = user.role === 'admin';
```

### Constants

- `UPPER_SNAKE_CASE` for true environment constants, config values, and fixed domain constants: `MAX_RETRIES`, `API_BASE_URL`, `DEFAULT_PAGE_SIZE`.
- Regular `camelCase` for all other `const` declarations — computed values, references, instances.

## Function Structure

### Early Returns

Guard clauses at the top, happy path at the bottom. Never nest the main logic inside an `if/else`. All `if` statements use braces:

```typescript
/**
 * Processes an order through checkout after validating
 * stock availability and order status.
 *
 * @param order - The order to process.
 * @returns Result indicating success or the specific failure reason.
 */
function processOrder(order: Order): Result {
  // Reject invalid orders before any processing
  if (!order.isValid) {
    return Result.invalid(order.id);
  }

  // Cancelled orders should not proceed to checkout
  if (order.isCancelled) {
    return Result.cancelled(order.id);
  }

  if (!inventory.hasStock(order.itemId)) {
    return Result.outOfStock(order.itemId);
  }

  const receipt = checkout(order);
  notify(order.customerId, receipt);
  return Result.success(receipt);
}
```

Never do this — nested conditionals bury the happy path:

```typescript
// Bad
function processOrder(order: Order): Result {
  if (order.isValid) {
    if (!order.isCancelled) {
      if (inventory.hasStock(order.itemId)) {
        // ...
      }
    }
  }
}
```

### Single Responsibility

A function should do one thing. Split by **responsibility**, not by line count. A 70-line function doing one focused thing is fine. A 40-line function doing three things should be split.

Biome enforces cognitive complexity via `noExcessiveCognitiveComplexity`. If biome flags a function, that's a strong signal to extract.

### Delegate, Don't Duplicate

When two functions share logic, the action function should delegate to the check function — not re-implement the same conditions:

```typescript
// Bad — validation duplicated, will drift out of sync
export function canPublish(post: Post): boolean {
  /* checks */
}
export function publish(post: Post): Result {
  /* same checks again, then action */
}

// Good — action delegates to check
export function publish(post: Post): Result {
  if (!canPublish(post)) {
    return Result.error('Cannot publish');
  }
  return doPublish(post);
}
```

### Parameters

- **Max 2 positional parameters**: `(id, input)` or `(id, options?)`. 3+ values go in an object.
- **Destructure parameters** when using 2+ properties from the same object.
- Don't destructure when you only access one property once — just use `obj.prop`.

**Standard signatures:**

| Pattern          | When                      | Example                                      |
| ---------------- | ------------------------- | -------------------------------------------- |
| `(input)`        | Create with required data | `create(input: CreateUserInput)`             |
| `(id)`           | Single-entity read/delete | `get(id: UserId)`                            |
| `(id, input)`    | Update with required data | `update(id: UserId, input: UpdateUserInput)` |
| `(id, options?)` | Read with optional config | `get(id: UserId, options?: GetUserOptions)`  |
| `(options?)`     | List/query with filters   | `list(options?: ListUsersOptions)`           |

Required data uses `input` (noun — the data going in). Optional config uses `options` (noun — tuning knobs). Domain skills may refine this further (e.g. `params` in API modules, `payload` for events).

## Return Types

| Context                | Annotate?    | Why                                                   |
| ---------------------- | ------------ | ----------------------------------------------------- |
| Exported functions     | Always       | Documents the contract, speeds up type checking       |
| Non-exported functions | When complex | Infer for simple returns, annotate for objects/unions |

```typescript
// Good — exported, explicit return type
export function parseConfig(raw: string): AppConfig {

// Good — internal, simple return, inferred
function double(n: number) {
  return n * 2
}
```

Specific frameworks may define additional return type rules for their patterns (hooks, components, services).

## Immutability

### Readonly Parameters

Default to `readonly` for array and object parameters that the function does not mutate. This applies to **all** object types — domain entities, configs, DTOs — not just configuration objects. Zero-cost (compile-time only) and prevents accidental mutation:

```typescript
// Good — function only reads the array
export function summarize(items: readonly Item[]): Summary {

// Good — function only reads the user (a domain entity)
function formatDisplayName(user: Readonly<User>): string {

// Good — function only reads the config
function applyConfig(config: Readonly<Config>): void {

// No readonly needed — function intentionally mutates
function shuffle(items: Item[]): void {
```

Rules:

- Use `readonly T[]` (or `ReadonlyArray<T>`) for array parameters that are not mutated.
- Use `Readonly<T>` for object parameters that are not mutated — including domain entities, DTOs, and any object you only read from.
- Omit `readonly` when the function intentionally mutates the input.

## File Organization

### One Concept Per File

A "concept" is a component, a service, a hook, or a utility group. Compound families (tightly coupled parts always imported together) are the exception.

### Ordering Within a File

```
1. Imports
2. Types / interfaces / constants (whichever reads most naturally for the module)
3. Main export(s)
4. Internal helpers
```

Blank lines between logical groups. No section separator comments (`// ─── Section ───`).

### Split Signals

Time to split when:

- Function exceeds **100 lines**
- File name requires "and" (doing two things)
- Tests require **complex setup** for a single unit
- You can't describe the file's purpose in one sentence

## Readability

### Explaining Variables

Extract complex conditions into named `const` variables, even if used only once. The name documents the intent:

```typescript
// Bad — dense compound condition
if (
  user.createdAt > subDays(new Date(), 7) &&
  !user.emailVerifiedAt &&
  user.loginCount < 3
) {
  sendOnboardingNudge(user);
}

// Good — each condition is self-documenting
const isRecentSignup = user.createdAt > subDays(new Date(), 7);
const isUnverified = !user.emailVerifiedAt;
const hasLowEngagement = user.loginCount < 3;

if (isRecentSignup && isUnverified && hasLowEngagement) {
  sendOnboardingNudge(user);
}
```

### Declare Variables Near First Use

Minimize the gap between declaration and usage. Don't stack all declarations at the top — declare each variable right before it's needed.

### Avoid Boolean Parameters

Boolean parameters hide meaning at call sites. Use an options object instead:

```typescript
// Bad — what does `true` mean?
await sendNotification(userId, true);

// Good — meaning is visible at the call site
await sendNotification(userId, { immediate: true });
```

## Error Handling

- Every async operation must handle errors with user feedback. Never fire-and-forget an async call.
- Typed, explicit errors over generic catches. Handle the specific failure, not a broad `catch (e)`.
- See framework-specific skills for detailed error handling patterns.
