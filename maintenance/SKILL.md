---
name: maintenance
description: 'Codebase maintenance umbrella organized by area (skills, effect, frontend, cli, ci, infra, tooling, codebase) and cross-cutting tasks (npm packages, knip, shadcn). Use when enforcing conventions, cleaning up patterns, updating dependencies, auditing unused code, or running scheduled maintenance.'
---

# Housekeeping

Codebase maintenance organized into **code quality** (incremental, per-file) and **structural tasks** (dependency audits, config maintenance).

## Code Quality (Incremental)

The primary housekeeping workflow. Uses the `housekeeping` CLI tool to track which files have been audited via git blob SHAs, so only changed or never-audited files get re-checked.

### CLI Tool

```bash
housekeeping status                     # summary: clean/dirty/total counts
housekeeping dirty                      # list all files needing audit
housekeeping dirty 'apps/api/**'        # scoped to a path glob
housekeeping mark 'apps/api/**'         # record files as clean after audit
housekeeping reset 'apps/api/**'        # force re-audit
housekeeping reset --all                # clear all tracking state
```

Implementation: `packages/agent-dev/housekeeping/`
State: `packages/agent-dev/housekeeping/state.json` (committed, JSON map of path → blob SHA)

### Audit Categories

Every file is checked for all 4 categories in a single pass:

1. **Code style** — biome compliance, naming conventions, file organization
2. **Code quality** — complexity, function structure, error handling, early returns
3. **Documentation** — JSDoc with `@param`, `@returns`, `@template` on all exports
4. **Type safety** — no casts (`as`, `as any`), derived types, no enums

Rules come from the `practice-code-quality` skill (universal) plus the domain-specific skill for the file's area.

### Skill Routing

Based on file path, load the appropriate domain skill alongside `practice-code-quality`:

| Path Pattern                                                                  | Domain Skill                                             |
| ----------------------------------------------------------------------------- | -------------------------------------------------------- |
| `apps/{api,worker,rpc,actors,electric-proxy}/**`                              | `domain-effect`                                          |
| `packages/{platform,clients}/**`                                              | `domain-effect`                                          |
| `packages/comcom/{api-core,agent-harness,agents,email,rate-limit,billing}/**` | `domain-effect`                                          |
| `apps/{app,web,admin,design,mobile,desktop}/**`                               | `domain-frontend`                                        |
| `packages/ui/**`                                                              | `domain-frontend`                                        |
| `packages/comcom/{app-core,app-shared,sync,api-client}/**`                    | `domain-frontend`                                        |
| `apps/cli/**`                                                                 | CLI conventions ([references/cli.md](references/cli.md)) |
| Everything else                                                               | `practice-code-quality` only                             |

### Code Quality Workflow

1. **Discover** — run `housekeeping dirty [scope]` to get the dirty file list
2. **Group** — sort dirty files by domain skill (effect / frontend / cli / general)
3. **Audit + Fix** — spawn parallel **foreground** Opus sub-agents per group. Each agent:
   - Loads `practice-code-quality` + the group's domain skill
   - Works through the domain checklist (e.g. [effect.md](references/effect.md), [frontend.md](references/frontend.md)) **rule by rule**, P1 first
   - Also enforces all rules in [code-quality-checklist.md](references/code-quality-checklist.md)
   - Reports findings with rule IDs (e.g. `E-P1-3`, `F-P2-1`, `Q-P1-2`)
   - Fixes violations directly
   - No two agents edit the same file
4. **Verify** — run `bun run ci` (lint + typecheck + tests)
5. **Commit** — commit fixes
6. **Mark** — run `housekeeping mark [scope]` to record clean state (must run after commit so blob SHAs match HEAD)
7. **Commit** — commit updated state.json

## Structural Tasks

Cross-cutting operations that don't track per-file state.

### Area Runs

Each area defines its own scope, conventions to enforce, and audit approach.

| Area     | Reference                                        | Scope                                                                     |
| -------- | ------------------------------------------------ | ------------------------------------------------------------------------- |
| skills   | [references/skills.md](references/skills.md)     | `.agents/`, `CLAUDE.md` — skill/command consistency                       |
| ci       | [references/ci.md](references/ci.md)             | `packages/ci/`, `.github/workflows/`                                      |
| infra    | [references/infra.md](references/infra.md)       | `packages/infra/`, `apps/*/infra/`                                        |
| tooling  | [references/tooling.md](references/tooling.md)   | `bin/`, `packages/agent-dev/` — bin scripts, agent CLI tools              |
| codebase | [references/codebase.md](references/codebase.md) | Cross-area patterns, `packages/platform/*`, `packages/ui/*`, root configs |

### Task Runs

| Task                                                                 | Reference                                                          |
| -------------------------------------------------------------------- | ------------------------------------------------------------------ |
| Update npm packages, deduplicate to catalogue, enforce dep ownership | [references/npm-packages.md](references/npm-packages.md)           |
| Run knip, audit unused deps/files, fix knip config                   | [references/knip.md](references/knip.md)                           |
| Update shadcn/base-nova components                                   | [references/shadcn-components.md](references/shadcn-components.md) |

### Structural Workflow

Area runs follow this workflow:

1. **Discover** — find files/modules in the area's scope (see area reference)
2. **Audit** — spawn parallel **foreground** Opus sub-agents to scan for violations. Each agent loads the relevant skill and returns structured violations with `file:line`, current code, and fix
3. **Present** — aggregate findings. Ask user which categories to fix via `AskUserQuestion`
4. **Fix** — spawn **foreground** Opus sub-agents to apply fixes. No two agents edit the same file
5. **Verify** — run `bun run ci`

Task runs follow the workflow defined in their own reference file.

## Dispatch

| Trigger                                                         | Action                                                          |
| --------------------------------------------------------------- | --------------------------------------------------------------- |
| `housekeeping` or `housekeeping code`                           | Code quality pass (incremental, uses CLI tool)                  |
| `housekeeping code <glob>`                                      | Scoped code quality pass (e.g. `housekeeping code apps/api/**`) |
| `housekeeping skills` / `ci` / `infra` / `tooling` / `codebase` | Structural area run                                             |
| `housekeeping npm` / `knip`                                     | Structural task run                                             |
| `housekeeping shadcn`                                           | shadcn component update (only when requested)                   |
| `housekeeping all`                                              | Code quality first, then structural tasks (order below)         |

### Full Housekeeping Order

1. Code quality (incremental — dirty files only)
2. Skills
3. CI
4. Infra
5. Tooling
6. Codebase
7. npm packages
8. Knip audit

shadcn component updates are **not** included in full housekeeping.

## Quick Reference

```bash
housekeeping status       # check tracking coverage
housekeeping dirty        # see what needs auditing
bun run ci                # full CI gate — always run after any housekeeping
```
