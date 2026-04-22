# npm Package Updates

## Commands

| Task                   | Command             |
| ---------------------- | ------------------- |
| Interactive npm update | `bun run bump:deps` |

## Dependency Audit (Knip)

For unused dependency/file auditing with knip, see [knip.md](knip.md). Quick start: `bun run knip`.

## Catalogue Deduplication

The root `package.json` has a `workspaces.catalog` that centralizes shared dependency versions. When a dependency appears in multiple workspace `package.json` files with its own version (not using `catalog:`), it should be consolidated into the catalogue.

**Workflow:**

1. Scan all workspace `package.json` files for dependencies that appear in 2+ packages but are **not** in the root `workspaces.catalog`
2. For each duplicate:
   - Pick the highest semver-compatible version across all workspaces
   - Add it to `workspaces.catalog` in the root `package.json`
   - Replace inline versions in each workspace with `"catalog:"` (Bun catalogue protocol)
3. Also check for dependencies that **are** in the catalogue but where some workspaces still use an inline version instead of `"catalog:"` — migrate those too
4. Run `bun install` to regenerate the lockfile
5. Verify with `bun run ci`

## Dependency Ownership Rules

Some dependencies must only live in a single package to keep boundaries clean. When auditing, enforce these rules:

| Dependency    | Allowed package          | Reason                                             |
| ------------- | ------------------------ | -------------------------------------------------- |
| `better-auth` | `packages/platform/auth` | Auth logic must not leak outside the auth boundary |

If a disallowed dependency is found elsewhere:

1. Check if the consuming code should import from the owning package instead (e.g. import auth types from `@platform/auth`)
2. Remove the dependency from the wrong `package.json`
3. Adjust imports to go through the owning package's public API

## Verification

After any npm update:

1. Typecheck affected packages: `turbo run typecheck --filter=@<scope>/<package>`
2. Run tests: `turbo run test --filter=@<scope>/<package>`
3. If a package is coupled to a shadcn component (see [shadcn-components.md](shadcn-components.md) coupled packages table), retest that component visually
4. Full CI gate: `bun run ci`
