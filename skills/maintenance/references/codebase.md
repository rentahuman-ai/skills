# Codebase Housekeeping

Find and resolve cross-area pattern inconsistencies and CLAUDE.md convention violations.

**Also enforce all rules in [code-quality-checklist.md](code-quality-checklist.md)** — universal code quality rules that apply to every domain.

## Scope

Entire repo cross-area consistency. Focuses on areas not covered by dedicated housekeeping areas (effect, frontend, cli, ci, infra):

- `packages/platform/*` (auth, config, database, effect, server), `packages/tooling/*` (testing, typescript-config), and `packages/ui/*` (seo, next-config)
- Root configs (biome.json, tsconfig, knip.ts, package.json)
- `packages/comcom/*` (billing, email, rate-limit) — product-specific packages not covered by other areas
- Cross-area pattern consistency across all scopes

## Source of Truth

CLAUDE.md is the sole canonical reference.

## Audit Checklist

Rule IDs use prefix `C` (codebase): `C-P1-1`, `C-P2-3`, etc. Work through **P1 first**, then P2, then P3.

### P1 — Must Fix

| #   | Rule                          | Violation                                         | Correct                                                                                           |
| --- | ----------------------------- | ------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| 1   | No type casts                 | `as any`, `as unknown as T`, `as Type`, `expr!`   | Fix the underlying type; use type guards or Schema.decode                                         |
| 2   | No enums                      | `enum Status { Active, Inactive }`                | `type Status = 'active' \| 'inactive'` or const object                                            |
| 3   | Path alias imports            | `import { X } from '../../platform/config/foo'`   | `import { X } from '@platform/config/foo'` — use `@/*`, `@platform/*`, `@comcom/*`, `@ui/*`, etc. |
| 4   | No barrel/index imports       | `import { Button } from '@ui/design-system'`      | `import { Button } from '@ui/design-system/components/ui/button'`                                 |
| 5   | No file extensions in imports | `import { X } from './foo.ts'`, `from './bar.js'` | `import { X } from './foo'`                                                                       |
| 6   | No direct process.env         | `process.env.DATABASE_URL` in app code            | Use `@platform/config` — `Config.string("DATABASE_URL")` or env access through config layer       |

### P2 — Should Fix

| #   | Rule                        | Violation                                                            | Correct                                                                       |
| --- | --------------------------- | -------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| 1   | Type derivation from source | `type User = { id: string; name: string; ... }` (manually rewritten) | `type User = typeof users.$inferSelect` or `Schema.Type<typeof UserSchema>`   |
| 2   | No duplicated types         | Same type defined in multiple packages                               | Export from source module, import elsewhere                                   |
| 3   | Dependency ownership        | `better-auth` imported in `apps/api`                                 | `better-auth` only in `packages/platform/auth` — respect ownership boundaries |
| 4   | Named exports everywhere    | `export default` in non-page file                                    | `export function MyThing()` or `export const MyThing = ...`                   |
| 5   | Test location               | `src/Bookmarks/Service.test.ts` (co-located)                         | `test/Bookmarks/Service.test.ts` — `test/` directory at package level         |
| 6   | Test file pattern           | `test/bookmarks.spec.ts`, `test/Bookmarks.tests.ts`                  | `test/**/*.test.ts`                                                           |
| 7   | PascalCase backend modules  | `apps/api/src/modules/bookmarks/service.ts`                          | `apps/api/src/Modules/Bookmarks/Service.ts`                                   |
| 8   | kebab-case frontend files   | `apps/app/src/components/UserProfile.tsx`                            | `apps/app/src/components/user-profile.tsx`                                    |
| 9   | snake_case SQL columns      | `createdAt` in SQL schema                                            | `created_at` in SQL; camelCase in TypeScript via Drizzle mapping              |

### P3 — Recommended

| #   | Rule                          | Violation                                                           | Correct                                   |
| --- | ----------------------------- | ------------------------------------------------------------------- | ----------------------------------------- |
| 1   | Config extends tooling base   | `tsconfig.json` without `extends: "@tooling/typescript-config/..."` | Extend from `@tooling/typescript-config`  |
| 2   | Package.json in catalogue     | Dependency version inline instead of `"catalog:"`                   | Use `"catalog:"` protocol for shared deps |
| 3   | No section separator comments | `// ─── Section ───`, `// --- Utils ---`                            | Remove; use blank lines                   |
| 4   | No barrel/index.ts files      | `index.ts` with re-exports                                          | Import directly from source modules       |

## Discovery

```bash
# Cross-area pattern scan
grep -r "as any" --include="*.ts" --include="*.tsx" -l | grep -v node_modules
grep -r "as unknown" --include="*.ts" --include="*.tsx" -l | grep -v node_modules
grep -r "enum " --include="*.ts" --include="*.tsx" -l | grep -v node_modules
grep -r "process\.env\." --include="*.ts" --include="*.tsx" -l | grep -v node_modules | grep -v "@platform/config"
```

## Audit Approach

Spawn parallel Opus sub-agents scoped to areas **not covered by dedicated housekeeping areas**:

- **Agent 1**: `packages/platform/*`, `packages/ui/*`, `packages/comcom/{billing,email,rate-limit}` — config, seo, next-config, testing, typescript-config, billing, email, rate-limit
- **Agent 2**: Root configs + cross-area grep scans — run the discovery greps above and flag violations repo-wide

Each agent:

1. Works through this checklist **rule by rule**, P1 first, then P2, then P3
2. Also enforces all rules in [code-quality-checklist.md](code-quality-checklist.md)
3. Returns violations referencing rule IDs:

```
Area: {path}
- [C-P1-3] Path alias import — auth/src/client.ts:5
  Current: `import { X } from '../../config/env'`
  Should be: `import { X } from '@platform/config/env'`
- [Q-P1-2] Enum usage — billing/src/types.ts:12
  Current: `enum PlanType { Free, Pro }`
  Should be: `type PlanType = 'free' | 'pro'`
```

Then cross-reference findings to identify inconsistencies across platform/ui/comcom packages.

## Cross-Cutting References

- [knip.md](knip.md) — knip.ts config audit
- [npm-packages.md](npm-packages.md) — catalogue deduplication
