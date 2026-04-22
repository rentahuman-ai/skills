# Universal Code Quality Checklist

Rules from `practice-code-quality` that apply to **all** domains. Every housekeeping subagent must enforce these alongside the domain-specific checklist.

Rule IDs use prefix `Q` (quality): `Q-P1-1`, `Q-P2-3`, etc.

## P1 — Must Fix

| #   | Rule                    | Violation                                                                                         | Correct                                                                                                        |
| --- | ----------------------- | ------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| 1   | No type casts           | `as any`, `as unknown as T`, `as Type`, `<Type>expr`, `expr!`                                     | Fix the underlying type; use Schema.decode or type guards                                                      |
| 2   | No enums                | `enum Status { Active, Inactive }`                                                                | `type Status = 'active' \| 'inactive'` or `const Status = { Active: 'active', Inactive: 'inactive' } as const` |
| 3   | Exported function JSDoc | Exported function without `@param` for every param, `@returns`, and `@template` for every generic | Full JSDoc on every export, no exceptions                                                                      |

## P2 — Should Fix

| #   | Rule                           | Violation                                                            | Correct                                                                     |
| --- | ------------------------------ | -------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| 1   | Boolean naming prefixes        | `const loading = ...`, `const valid = ...`, `const permission = ...` | `const isLoading = ...`, `const isValid = ...`, `const hasPermission = ...` |
| 2   | No negative boolean names      | `const isNotReady = ...`, `const isNotLoading = ...`                 | `const isReady = ...` (invert the condition)                                |
| 3   | Readonly array params          | `function process(items: Item[])` (function doesn't mutate)          | `function process(items: readonly Item[])`                                  |
| 4   | Readonly object params         | `function format(user: User)` (function doesn't mutate)              | `function format(user: Readonly<User>)`                                     |
| 5   | No boolean function params     | `sendNotification(userId, true)`                                     | `sendNotification(userId, { immediate: true })`                             |
| 6   | Max 2 positional params        | `fn(a, b, c)` or `fn(id, input, options)`                            | `fn(a, { b, c })` or `fn(id, input)` with options merged into input         |
| 7   | Delegate not duplicate         | `canPublish()` and `publish()` both implement same validation checks | `publish()` calls `canPublish()` internally                                 |
| 8   | Early returns / guard clauses  | Happy path nested inside `if (valid) { if (!cancelled) { ... } }`    | Guard clauses at top (`if (!valid) return`), happy path at bottom           |
| 9   | Exported function return types | `export function getValue() { return ... }` (inferred)               | `export function getValue(): Config { return ... }` (annotated)             |
| 10  | Required data uses `input`     | `create(data: ...)`, `create(payload: ...)`                          | `create(input: BookmarkCreateInput)`                                        |
| 11  | Optional config uses `options` | `get(id, config?: ...)`, `list(filters?: ...)`                       | `get(id, options?: BookmarkGetOptions)`                                     |

## P3 — Recommended

| #   | Rule                       | Violation                                                                               | Correct                                                                                           |
| --- | -------------------------- | --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| 1   | Explaining variables       | `if (user.createdAt > subDays(now, 7) && !user.emailVerifiedAt && user.loginCount < 3)` | Extract: `const isRecentSignup = ...`, `const isUnverified = ...`, `const hasLowEngagement = ...` |
| 2   | Variables near first use   | All declarations stacked at top of function                                             | Declare each variable right before it's first used                                                |
| 3   | No section separators      | `// ─── Methods ───`, `// --- Section ---`                                              | Remove; use blank lines between logical groups                                                    |
| 4   | One concept per file       | File exports unrelated service + utility                                                | Split into separate files                                                                         |
| 5   | Cognitive complexity       | Biome flags `noExcessiveCognitiveComplexity`                                            | Extract helper functions to reduce nesting                                                        |
| 6   | Constants casing           | `const maxRetries = 3` (module-level config constant)                                   | `const MAX_RETRIES = 3` (UPPER_SNAKE_CASE for true constants)                                     |
| 7   | Non-exported function docs | Complex internal function with no hint                                                  | Single-line `/** Extracts initials (max 2 chars). */` when name alone isn't enough                |
