---
name: garbage-collect
description: 'Find and remove dead code using knip. Run `npx knip` to detect unused files, dependencies, exports, and types, then triage findings, remove true positives, and ship a cleanup PR. Use when doing periodic dead code sweeps or before major refactors.'
---

# Garbage Collect

Detect and remove dead code using knip. Ships removals as a single PR.

## Prerequisites

- knip is installed (`npx knip` works)
- Config at `knip.ts` in repo root

## Workflow

### 1. Setup

Create a dedicated branch:

```bash
git checkout -b chore/garbage-collect-$(date +%Y%m%d)
```

### 2. Audit the knip config

Before running knip, verify `knip.ts` isn't stale:

- Check that barrel file entries still exist
- Check that `ignoreDependencies` entries are still in `package.json`
- Remove any stale entries, commit separately

### 3. Run knip and triage

```bash
npx knip
```

Group findings by category:

| Category              | Command                               | Risk                                   |
| --------------------- | ------------------------------------- | -------------------------------------- |
| Unused files          | `npx knip --files`                    | Low — file is not imported anywhere    |
| Unused dependencies   | `npx knip --dependencies`             | Medium — may be used in config/scripts |
| Unused exports        | `npx knip --exports`                  | Low — export not consumed              |
| Unused types          | `npx knip --exports` (includes types) | Low — type not referenced              |
| Unlisted dependencies | `npx knip --dependencies`             | Info — should be added to package.json |

For each finding, classify as **true positive** or **false positive**:

#### Decision framework

1. **Is it imported in source files?** The entry/project pattern is wrong. Fix `knip.ts`, don't remove the code.
2. **Is it referenced in a config file?** (postcss, tailwind, tsconfig, etc.) Add to `ignoreDependencies` with a comment naming the config.
3. **Is it a peer dependency?** Check with `bun pm ls <dep>`. Add to `ignoreDependencies`.
4. **Is it used in scripts/?** Scripts are excluded from knip's scan. If the dep/file is only used there, it's a false positive for the main app but may still be removable if the script is dead.
5. **Is it used dynamically?** Check for `dynamic(() => import(...))`, `require()`, or string-based lookups. Add to ignore if so.
6. **None of the above?** Genuinely unused. Remove it.

### 4. Remove true positives

Work in this order (lowest risk first):

**Phase A — Unused files**
Delete files flagged as unused. Run `bunx tsc --noEmit` after each batch to catch breakage early.

**Phase B — Unused exports and types**
Remove the `export` keyword (or delete the entire declaration if the value is also unused internally). Run `bunx tsc --noEmit` after each batch.

**Phase C — Unused dependencies**
Remove from `package.json`, run `bun install`, then `bun run build` to verify.

**Phase D — Unlisted dependencies**
Add genuinely needed deps to `package.json`. For transitive deps that happen to work (like `server-only` via Next.js), add them explicitly.

After each phase: `bunx tsc --noEmit && bun run lint`

### 5. Update knip config

Add any new false positives to `ignoreDependencies` or `ignore` in `knip.ts` WITH a comment explaining why.

### 6. Final verification

```bash
npx knip              # should be clean or reduced
bun run build         # still works
bunx tsc --noEmit     # types clean
bun run lint          # lint clean
bun run test          # tests pass
```

### 7. Ship

Commit all changes and create a PR:

```bash
git add -A && git commit -m "chore: remove dead code detected by knip"
git push -u origin "$(git branch --show-current)"
gh pr create --base main --title "chore: garbage collect dead code" --body "$(cat <<'EOF'
## Summary
- Removed unused files, exports, types, and dependencies detected by knip
- Updated knip config for new false positives

## Verification
- `npx knip` — clean (or reduced findings listed below)
- `bun run build` — passes
- `bunx tsc --noEmit` — passes
- `bun run test` — passes
EOF
)"
```

## Guardrails

- **Never remove barrel file exports** without checking all consumers (barrel files are knip entry points)
- **Never remove a dependency** without verifying it's not used in config files, scripts, or as a peer dep
- **Never remove types** that are part of the public API surface (exported from `src/types/` or `src/schemas/`)
- **Commit each phase separately** so reversions are surgical
- **If in doubt, leave it** — a false negative is better than a broken build
