---
name: github-actions
description: 'Use when creating or editing GitHub Actions workflows (.github/workflows/*.yml), composite actions, CI scripts (scripts/ci/*.ts), or reusable workflows. Covers CI/CD pipeline design, job dependency graphs, caching, secrets handling, and workflow maintainability.'
---

# GitHub Actions

Best practices for writing maintainable, fast, and secure GitHub Actions workflows.

## Project-Specific Architecture

This project uses **Effect-TS CLI scripts** for CI logic instead of bash. Workflows are thin YAML that delegates to type-safe scripts:

```
scripts/ci/
├── lib/
│   ├── errors.ts            # Structured errors (CmdError, EnvError, GitError)
│   ├── github-actions.ts    # GithubActions service (logs, outputs, masking)
│   ├── exec.ts              # Type-safe shell execution via @effect/platform
│   └── run.ts               # CiLayer + runCi() entry point
├── deploy-safety.ts         # Pre-deploy checks
├── verify.ts                # lint/typecheck/test/build gates
├── firebase.ts              # Firestore rules/indexes deploy
├── cloudrun.ts              # Cloud Run build/push/deploy
└── comment.ts               # PR status comment

.github/actions/
├── setup-bun/action.yml     # Bun + cache + install
└── setup-gcp/action.yml     # GCP auth + Cloud SDK + Docker
```

### Adding CI Logic

New CI logic goes in `scripts/ci/` as an Effect CLI script, not inline YAML:

```ts
import { Command, Options } from '@effect/cli';
import { NodeRuntime } from '@effect/platform-node';
import { Effect } from 'effect';
import { exec } from './lib/exec.js';
import { GithubActions } from './lib/github-actions.js';
import { CiLayer } from './lib/run.js';

const myCommand = Command.make('my-cmd', {}, () =>
  Effect.gen(function* () {
    const ga = yield* GithubActions;
    yield* ga.group('Step name', exec('some-tool', ['--flag']));
    yield* ga.summary('### Result\n\nDone.');
  })
);

const cli = Command.run(myCommand, { name: 'my-cmd', version: '1.0.0' });
cli(process.argv).pipe(Effect.provide(CiLayer), NodeRuntime.runMain);
```

Run from workflow: `bunx tsx scripts/ci/my-cmd.ts`

### GithubActions Service

The `GithubActions` service in `scripts/ci/lib/github-actions.ts` detects CI vs local:

| Method                | CI behavior                       | Local behavior        |
| --------------------- | --------------------------------- | --------------------- |
| `setOutput(k, v)`     | Writes to `$GITHUB_OUTPUT`        | Prints `[output] k=v` |
| `mask(v)`             | `::add-mask::`                    | No-op                 |
| `group(name, effect)` | `::group::`/`::endgroup::`        | `▸ name`              |
| `logInfo/Warn/Error`  | `::notice/warning/error::`        | Console with icons    |
| `summary(md)`         | Appends to `$GITHUB_STEP_SUMMARY` | Prints markdown       |

## Core Principles

1. **Thin YAML, fat scripts** — Workflows are orchestration. Logic lives in `scripts/ci/*.ts`.
2. **Locally runnable** — Every CI script works outside CI via the GithubActions service fallback.
3. **Parallel by default** — Independent jobs run concurrently. Use `needs` only for real dependencies.
4. **Cache aggressively** — Dependencies and build artifacts via composite actions.
5. **Use bun** as the JS/TS runtime and package manager.

## Job Dependency Graph

Jobs run in parallel by default. Use `needs` to express real dependencies:

```yaml
jobs:
  install:
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/setup-bun

  lint:
    needs: install
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/setup-bun
      - run: bunx tsx scripts/ci/verify.ts lint

  build:
    needs: install
    steps:
      - uses: actions/checkout@v4
      - uses: ./.github/actions/setup-bun
      - run: bunx tsx scripts/ci/verify.ts build
```

## Passing Data Between Steps and Jobs

### Between steps (same job)

```yaml
steps:
  - id: compute
    run: echo "version=1.2.3" >> "$GITHUB_OUTPUT"
  - run: echo "Got ${{ steps.compute.outputs.version }}"
```

### Between jobs

```yaml
jobs:
  prepare:
    outputs:
      version: ${{ steps.compute.outputs.version }}
    steps:
      - id: compute
        run: echo "version=1.2.3" >> "$GITHUB_OUTPUT"

  build:
    needs: prepare
    steps:
      - run: echo "${{ needs.prepare.outputs.version }}"
```

## Security

### Permissions — least privilege

```yaml
permissions:
  contents: read

jobs:
  deploy:
    permissions:
      contents: read
      deployments: write
```

Never use `permissions: write-all`. Jobs that need no token: `permissions: {}`.

### Secrets — always via env, never inline

```yaml
# GOOD
- run: curl -H "Authorization: Bearer $TOKEN" ...
  env:
    TOKEN: ${{ secrets.TOKEN }}

# BAD — shell expansion risks
- run: curl -H "Authorization: Bearer ${{ secrets.TOKEN }}" ...
```

## Performance

### Composite actions for setup

Always use `setup-bun` and `setup-gcp` composite actions — never inline the setup steps:

```yaml
- uses: ./.github/actions/setup-bun # bun + cache + install
- uses: ./.github/actions/setup-gcp # GCP auth + Cloud SDK + Docker
  with:
    credentials_json: ${{ secrets.GCP_SA_KEY }}
```

### Concurrency — cancel redundant runs

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true # false for deploy workflows
```

### Timeouts — always set

```yaml
jobs:
  test:
    timeout-minutes: 15 # default is 360
```

### Shallow clone

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 1
```

### Node.js version

All workflows must set `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true` at the top-level `env` to avoid deprecation warnings.

## Maintainability

### Thin workflows

If a `run:` block exceeds ~5 lines, move it to a script in `scripts/ci/`.

### Composite actions for setup

Use `.github/actions/setup-bun/` and `.github/actions/setup-gcp/` for shared setup sequences. Never duplicate the bun install + cache pattern inline.

### When to use what

| Scope                    | Mechanism                                       |
| ------------------------ | ----------------------------------------------- |
| Repeated steps in a repo | Composite action `.github/actions/*/action.yml` |
| Repeated jobs in a repo  | Reusable workflow `workflow_call`               |
| Complex logic (>5 lines) | Effect CLI script `scripts/ci/*.ts`             |

## Dependabot

Configure Dependabot with grouping to reduce noise:

```yaml
# .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: monthly
    groups:
      gh-actions:
        patterns: ['*']

  - package-ecosystem: npm
    directory: /
    schedule:
      interval: weekly
    groups:
      production:
        dependency-type: production
        update-types: [minor, patch]
      dev:
        dependency-type: development
        update-types: [minor, patch]
    ignore:
      - dependency-name: '*'
        update-types: ['version-update:semver-major']
```

## Checklist

When writing or reviewing a workflow, verify:

- [ ] `permissions` set explicitly (least privilege)
- [ ] Secrets passed via `env:`, never interpolated in `run:`
- [ ] `concurrency` with `cancel-in-progress` on CI workflows
- [ ] `timeout-minutes` on every job
- [ ] Uses `setup-bun` composite action (not inline bun setup)
- [ ] `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true` in top-level env
- [ ] Independent jobs parallelized (no unnecessary `needs`)
- [ ] Thin YAML — logic in `scripts/ci/*.ts`
- [ ] `fetch-depth: 1` unless history needed
