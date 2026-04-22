# Commit Conventions

## Format

```
<scope>: <imperative summary>

<body: why this matters + implementation notes>

<optional references>
```

## Scopes

Match the monorepo structure:

| Type            | Scopes                                   |
| --------------- | ---------------------------------------- |
| **App**         | `app:`, `api:`, `components:`, `lib:`    |
| **Integration** | `mcp:`, `firebase:`, `stripe:`, `slack:` |
| **Infra**       | `infra:`, `ci:`, `build:`, `deps:`       |
| **Other**       | `docs:`, `test:`, `scripts:`, `tooling:` |

For cross-cutting changes: `refactor:`, `chore:`, `fix:`, `feat:`

## Summary Line Rules

- Imperative mood: "add", "fix", "update", "remove" (not "added", "fixes")
- Under 72 characters
- Lowercase after scope
- No period at end

## Body

Answer these questions (skip if obvious from summary):

1. Why is this change needed?
2. What approach did you take and why?
3. What are the key changes?

Keep it scannable — use bullets for multiple points.

## References

- Link Linear issues: `Fixes ADAPT-123` or `Relates to ADAPT-456`
- Don't link for trivial changes

## Execution

Use a heredoc for multi-line messages:

```bash
git commit -m "$(cat <<'EOF'
scope: summary line

Body explaining why and how.
EOF
)"
```

Don't use `--no-verify` to skip hooks — fix the issue instead.

## Examples

### Simple fix

```
app: fix sidebar collapse on mobile

The sidebar wasn't respecting the mobile breakpoint due to
a missing `md:` prefix on the visibility class.
```

### Feature with context

```
api: add rate limiting for public endpoints

Public API endpoints were vulnerable to abuse. Added sliding
window rate limiting with configurable limits per endpoint.

- Default: 100 requests/minute for authenticated users
- Stricter: 20 requests/minute for anonymous requests
- Uses Redis for distributed state

Relates to ADAPT-234
```

### Refactor

```
refactor: extract shared validation logic to lib/validation

Validation logic was duplicated across multiple API routes.
Consolidated into a shared utility module to ensure consistency
and reduce maintenance burden.
```

### Dependency update

```
deps: upgrade next.js to 16.1

- Fixes hydration mismatch warnings in development
- Enables new caching behavior (see migration notes)
- Required updating next-config package for new options

https://nextjs.org/blog
```
