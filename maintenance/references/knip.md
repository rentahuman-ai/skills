# Knip: Unused Code & Dependency Audit

## Commands

| Task              | Command                       |
| ----------------- | ----------------------------- |
| Full audit        | `bun run knip`                |
| Dependencies only | `bun run knip --dependencies` |
| Unused files only | `bun run knip --files`        |

## Workflow

Two phases: first audit the knip config itself, then run knip and triage findings.

### Phase 1: Audit `knip.ts` for Stale Entries

The config accumulates dead weight as the codebase evolves. Before running knip, clean the config:

1. **Dead workspace entries** — verify each workspace path still exists. Remove entries for deleted apps/packages.
2. **Stale `ignoreDependencies`** — for each ignored dep, check if it's still in the workspace's `package.json`. If the dep was removed, the ignore is dead weight — remove it.
3. **Stale `ignore` patterns** — check if the ignored files/dirs still exist. If deleted, remove the ignore.
4. **Redundant overrides** — if a specific workspace entry (e.g. `'packages/platform/auth'`) only sets `project` with the same value as its glob default (`'packages/platform/*'`), the override is unnecessary — remove it.
5. **Missing comments** — every `ignoreDependencies`, `ignore`, and `ignoreBinaries` entry MUST have a comment explaining WHY it's ignored. Flag and fix any uncommented entries.

### Phase 2: Run Knip and Triage Findings

1. Run `bun run knip`
2. Group findings by category (unused deps, unused devDeps, unused files, unlisted deps)
3. For each finding, use the [decision framework](#decision-framework-remove-vs-ignore) to classify as true or false positive
4. **True positive** — remove the unused dep/file
5. **False positive** — add to the appropriate `ignoreDependencies`, `ignoreBinaries`, or `ignore` list in `knip.ts` WITH a comment explaining why
6. Re-run `bun run knip` until clean
7. Run `bun run ci` to verify nothing broke

## Configuration Structure (`knip.ts`)

### Global Settings

```ts
exclude: ['exports', 'types']; // Too noisy for monorepo with shared packages
lefthook: false; // Fails in worktrees
drizzle: false; // Needs env vars
```

### Workspace Entry System

Two-tier system:

1. **Glob defaults** — `'packages/platform/*'`, `'packages/ui/*'`, `'packages/clients/*'`, `'packages/comcom/*'`, `'packages/ci/actions/*'` provide baseline config (default entry `index.ts`, project glob `**/*.{ts,tsx}`)
2. **Specific overrides** — `'packages/platform/auth'`, `'apps/web'`, etc. override the glob default when a workspace needs custom entry points, ignores, or ignoreDependencies

Only add a specific override when the workspace needs something beyond the glob default.

### Key Fields

| Field                | Purpose                                  | Example                    |
| -------------------- | ---------------------------------------- | -------------------------- |
| `entry`              | Files consumed externally (entry points) | `['src/app/**/page.tsx']`  |
| `project`            | All source files knip should analyze     | `['src/**/*.{ts,tsx}']`    |
| `ignore`             | Files to skip entirely                   | `['modules/**']`           |
| `ignoreDependencies` | Deps that ARE used but knip can't trace  | `['tailwindcss', 'sharp']` |
| `ignoreBinaries`     | CLI tools knip flags as missing          | `['pulumi']`               |

## Entry Point Patterns by App Type

| App type                        | Entry pattern                                                      | Example workspace                                        |
| ------------------------------- | ------------------------------------------------------------------ | -------------------------------------------------------- |
| Next.js app                     | `src/app/**/page.tsx`, `src/app/**/layout.tsx`                     | `apps/web`, `apps/admin`                                 |
| Vite/React app                  | default (`src/main.tsx` or `index.ts`)                             | `apps/app`                                               |
| Expo app                        | `app/**/_layout.tsx`, `app/**/*.tsx`                               | `apps/mobile`                                            |
| Bun service                     | default (`src/index.ts`)                                           | `apps/actors`, `apps/worker`, `apps/cli`                 |
| Effect-TS API                   | `ignore: ['**/*']` (too dynamic to trace)                          | `apps/api`                                               |
| Package with `index.ts`         | default (handled by glob)                                          | Most `packages/platform/*`, `packages/clients/*`         |
| Package with path-alias exports | `components/**/*.{ts,tsx}`, `hooks/**/*.ts`, `lib/**/*.ts`         | `packages/comcom/app-core`, `packages/comcom/app-shared` |
| Design system                   | Multiple entry globs for components, hooks, lib, providers, stores | `packages/ui/design-system`                              |
| Codegen package                 | `generated/index.ts`                                               | `packages/comcom/api-client`                             |

## Common False Positive Categories

These deps are real but knip can't trace them. When you see them flagged, add to `ignoreDependencies` (don't remove).

**CSS/PostCSS/Build tools** — used via config, not JS imports:

- `tailwindcss`, `@tailwindcss/typography`, `@tailwindcss/postcss`, `postcss`, `tw-animate-css`
- Font packages: `@fontsource-variable/inter`, `@fontsource-variable/jetbrains-mono`, `@fontsource-variable/plus-jakarta-sans`, `@fontsource/ibm-plex-sans`
- `shadcn` (CLI tool + CSS import)

**Peer dependencies** — required by a parent package, not directly imported:

- `react-dom`, `@types/react-dom` in email package (peer of react-email)
- `@tauri-apps/api` in desktop app (base SDK for `@tauri-apps/plugin-*`)

**Codegen dependencies** — used at build time by code generators:

- `@hey-api/client-next` in api-client (referenced in `hey-api.config.ts` which is itself in `ignore`)
- `tldts`, `@t3-oss/env-core` in web app (bundled into `.source/source.config.mjs` by fumadocs-mdx)

**CLI/Global tools** — installed globally or used as CLI commands:

- `pulumi` binary in infrastructure packages

**Re-exported testing utilities** — packages that re-export deps for consumers:

- `@effect/platform`, `@effect/sql`, `@effect/vitest` in `packages/tooling/testing`

**Pulumi sub-stack deps** — used by separate Pulumi stacks in subdirectories:

- `@pulumi/eks`, `@pulumi/kubernetes`, `@pulumi/github`, `@pulumi/random`, `@pulumiverse/vercel`

**Expo/React Native build tools** — used via Expo config, not imports:

- `babel-plugin-transform-inline-environment-variables`, `babel-preset-expo`
- `expo-updates`, `expo-system-ui` (Expo-managed implicit deps)

**TSConfig references** — knip can't trace `extends` in tsconfig.json:

- `next` in typescript-config (TS plugin name, not a dependency)

## Decision Framework: Remove vs Ignore

When knip flags a dependency as unused, walk through this in order:

1. **Is it imported in source files?** → Entry/project pattern is wrong. Fix the knip config, don't remove the dep.
2. **Is it referenced in a config file?** (postcss, tailwind, tsconfig, babel, hey-api, drizzle, etc.) → Add to `ignoreDependencies` with a comment naming the config file.
3. **Is it a peer dependency?** Check with `bun pm ls <dep>`. → Add to `ignoreDependencies` with a comment naming the parent package.
4. **Is it used by codegen/build?** Check for generated files (`.source/`, `generated/`), build scripts, config files. → Add to `ignoreDependencies` with a comment explaining the build step.
5. **Is it a font/CSS package?** → Add to `ignoreDependencies` with a comment noting CSS usage.
6. **None of the above?** → Genuinely unused. Remove from `package.json`, run `bun install` + `bun run ci`.

## Danger Zone: Deps That Look Unused But Aren't

Real examples from this repo where deps were incorrectly removed and had to be restored:

| Dep                    | Workspace                    | Why it looks unused                              | Why it's actually needed                                                                                                 |
| ---------------------- | ---------------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `tldts`                | `apps/web`                   | Not imported in any `.tsx` file                  | fumadocs-mdx bundles `@platform/config` into `.source/source.config.mjs`, which resolves against `apps/web/node_modules` |
| `@hey-api/client-next` | `packages/comcom/api-client` | Config file that uses it is in the `ignore` list | Codegen plugin — removing breaks `bun run generate`                                                                      |
| `sharp`                | `apps/web`                   | Never imported in source                         | Next.js uses it internally for image optimization                                                                        |
| `@tauri-apps/api`      | `apps/desktop`               | Not directly imported                            | Base SDK required at runtime by `@tauri-apps/plugin-*`                                                                   |
| Font packages          | `packages/ui/design-system`  | Never imported in JS                             | Imported in CSS via `@import`                                                                                            |

**Rule**: Before removing ANY dependency, verify all three gates:

1. `bun run ci` passes locally after removal
2. The dep is not referenced in any config, build script, or generated file
3. The dep is not a peer/transitive requirement of another dep in the same workspace

## Adding New Workspace Entries

When a new app or package is added to the monorepo:

1. Check if it matches an existing glob default (`packages/platform/*`, `packages/ui/*`, `packages/clients/*`, `packages/comcom/*`, etc.) — if so, only add a specific entry if it needs custom patterns
2. Determine the entry point pattern from the [table above](#entry-point-patterns-by-app-type)
3. Set `project` to cover all source files: `['src/**/*.{ts,tsx}']` for apps, `['**/*.{ts,tsx}']` for packages
4. Run `bun run knip` and review findings for the new workspace
5. Add `ignoreDependencies` for any false positives with a comment for each
6. Keep the entry in the correct section (Apps, Server packages, Client packages, etc.)

## Comments Convention

Every `ignoreDependencies`, `ignore`, and `ignoreBinaries` entry in `knip.ts` MUST have an inline comment explaining WHY it's ignored. This prevents future auditors from removing the ignore and re-introducing the problem.

```ts
// Good
ignoreDependencies: [
  // CSS import in globals.css
  'tw-animate-css',
  // peer dependency of react-email
  'react-dom',
],

// Bad — no one knows why these are here
ignoreDependencies: ['tw-animate-css', 'react-dom'],
```
