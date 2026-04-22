---
name: workflow-finish
description: 'Use when implementation is complete and you need to create a PR and babysit it through review and CI until merge.'
---

# Finishing a Development Branch

## Overview

Verify CI passes, push branch, create PR, then babysit the PR through review comments and CI failures until it is approved and merged.

**Core principle:** Verify CI → Push + Create PR → Babysit loop → Merge + Cleanup

**Announce at start:** "I'm using the workflow-finish skill to complete this work."

## The Process

```
┌────────────────────────────────────────────────────────────────┐
│                     WORKFLOW-FINISH                             │
│                                                                │
│  Step 1: Verify CI passes locally                              │
│  Step 2: Push branch + create PR                               │
│  Step 3: Babysit loop                                          │
│    ┌──────────────────────────────────┐                        │
│    │  1. Wait for review comments     │                        │
│    │  2. Read comments                │                        │
│    │  3. Fix issues (sub-agent per    │                        │
│    │     comment group if needed)     │                        │
│    │  4. Push fixes                   │                        │
│    │  5. Wait for CI                  │                        │
│    │  6. Fix CI failures if any       │                        │
│    │  7. Repeat until:                │                        │
│    │     • All comments resolved      │                        │
│    │     • CI passes                  │                        │
│    │     • PR approved                │                        │
│    └──────────────────────────────────┘                        │
│  Step 4: Merge + cleanup                                       │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

## Step 1: Verify CI Passes

**Before creating a PR, verify tests pass locally:**

```bash
bun run ci
```

**If CI fails:**

- Fix the failures directly or dispatch a sub-agent for complex fixes
- Re-run `bun run ci` until it passes
- Do NOT proceed to Step 2 with failing CI

**If CI passes:** Continue to Step 2.

## Step 2: Push Branch and Create PR

### Determine Base Branch

The base of a PR is **whatever branch this work targets** — usually `main`, but in multi-PR mode it is the parent base branch (a sub-PR targets the base branch, not `main`).

```bash
# Detect which base to use:
# - If the orchestrator passed an explicit --base, use that
# - Otherwise default to main (or master)
BASE="${EXPLICIT_BASE:-$(git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@')}"
```

Pass `--base "$BASE"` to `gh pr create`. If you skip this flag and the work is a sub-PR, GitHub will diff against `main` and the PR will look like it contains every other sub-PR's changes too.

### Push and Create PR

```bash
# Push branch
git push -u origin <feature-branch>

# Create PR (pass --base for sub-PRs in multi-PR mode)
gh pr create --base "$BASE" --title "<title>" --body "$(cat <<'EOF'
## Summary
<2-3 bullets of what changed>

## Test Plan
- [ ] <verification steps>
EOF
)"
```

Report the PR URL to the user.

## Step 3: Babysit Loop

After creating the PR, enter the babysit loop. This loop runs until the PR is approved, all review comments are resolved, and CI passes.

```
┌──────────────────────────────────────────────────────┐
│                   BABYSIT LOOP                        │
│                                                       │
│  ┌─▶ Check for review comments (gh pr view)          │
│  │     │                                              │
│  │     ├── comments found                             │
│  │     │     │                                        │
│  │     │     ▼                                        │
│  │     │   Read + group comments                      │
│  │     │     │                                        │
│  │     │     ▼                                        │
│  │     │   Fix issues                                 │
│  │     │   (sub-agent per comment group if needed)    │
│  │     │     │                                        │
│  │     │     ▼                                        │
│  │     │   Push fixes                                 │
│  │     │                                              │
│  │     └── no comments (or all resolved)              │
│  │           │                                        │
│  │           ▼                                        │
│  │         Check CI status                            │
│  │           │                                        │
│  │           ├── failing → fix failures → push → ─┐   │
│  │           │                                    │   │
│  │           └── passing                          │   │
│  │                 │                              │   │
│  │                 ▼                              │   │
│  │           Check PR approval status             │   │
│  │                 │                              │   │
│  │                 ├── not approved → wait ────────┘   │
│  │                 │                                  │
│  │                 └── approved + CI green             │
│  │                       │                            │
│  │                       ▼                            │
│  │                     EXIT LOOP                      │
│  │                                                    │
│  └────────────────────────────────────────────────────┘
└──────────────────────────────────────────────────────┘
```

### Handling Review Comments

1. Fetch comments: `gh pr view <pr-number> --comments` and `gh api repos/<owner>/<repo>/pulls/<pr-number>/comments`
2. Group related comments (same file, same concern)
3. For each group:
   - If simple fix: fix directly
   - If complex fix: dispatch a sub-agent with the comment context and relevant files
4. Push all fixes together
5. Respond to comments on the PR indicating the fixes

### Handling CI Failures

1. Check CI status: `gh pr checks <pr-number>`
2. If failing:
   - Read the failure logs
   - Fix the issue (directly or via sub-agent)
   - Push fixes
   - Wait for CI to re-run
3. If a CI failure persists after 2 fix attempts, escalate to the user

### Loop Exit Conditions

The babysit loop exits when ALL of these are true:

- All review comments are resolved
- CI passes (all checks green)
- PR is approved

If the loop has been running for more than **5 iterations** without converging, surface the situation to the user for guidance.

## Step 4: Merge and Cleanup

Once the PR is approved and CI passes:

```bash
# Merge the PR
gh pr merge <pr-number> --squash --delete-branch

# Clean up step detail files
rm -f .context/plans/step-*.md
rm -f .context/plans/brainstorm.md
```

### Worktree Cleanup

Check if in a worktree and clean up:

```bash
# Check if in worktree
git worktree list | grep $(git branch --show-current)

# If yes, return to main worktree
git worktree remove <worktree-path>
```

## Multi-PR Mode

When invoked on a sub-PR (one that targets a base branch instead of `main`), the flow differs in three places: PR creation already covered above (`--base "$BASE"`), the merge step, and the cascade afterward. The babysit loop is identical.

### Sub-PR Merge

Sub-PRs merge into the base branch with `--squash --delete-branch` (same as single-PR mode). Each sub-PR's commits collapse to one squash commit on the base branch — that's intentional, the base branch's history is sub-PR boundaries, not internal sub-PR commits.

```bash
gh pr merge <sub-pr-number> --squash --delete-branch
```

Do **not** clean up `.context/plans/` after a sub-PR merge — later sub-PRs and the final base PR still need it. Defer cleanup until the base PR merges.

### Rebase Cascade

After a sub-PR merges into the base branch, every other open sub-PR has a stale base. Restack each one before its babysit loop continues:

```bash
# In each open sub-PR's worktree:
git fetch origin
git rebase origin/<base-branch>
git push --force-with-lease
```

Cascade in land order. If two sub-PRs touch overlapping files (they shouldn't, by the topology rule, but it happens), resolve conflicts using the rebase workflow in the `git` skill — never `-X ours/theirs`.

If juggling 4+ open sub-PRs, recommend the user adopt Graphite (`gt restack`) — manual cascade scales poorly past 3.

### Final Base PR Merge

When every sub-PR has merged into the base branch:

1. Undraft the base PR: `gh pr ready <base-pr-number>`
2. Re-run the babysit loop on the base PR (CI on the base branch validates the integrated whole, which is where sub-PRs collide)
3. Merge with `--merge` (NOT `--squash`) so the sub-PR boundaries on the base branch survive in `main`'s history:

   ```bash
   gh pr merge <base-pr-number> --merge --delete-branch
   ```

4. Now clean up:

   ```bash
   rm -f .context/plans/step-*.md
   rm -f .context/plans/brainstorm.md
   git worktree list  # remove every sub-PR worktree
   ```

## Red Flags

**Never:**

- Create a PR with failing CI
- Merge without PR approval
- Force-push without explicit user request (`--force-with-lease` during a multi-PR rebase cascade is the one exception, and only on the sub-branch the cascade owns)
- Ignore review comments
- Skip CI re-check after pushing fixes
- Merge to main/master without going through PR
- Squash the final base PR merge in multi-PR mode (use `--merge`; squash destroys sub-PR boundaries)
- Omit `--base <base-branch>` when creating a sub-PR (GitHub will diff against `main` and the PR will look enormous)

**Always:**

- Verify CI locally before creating PR
- Address every review comment
- Wait for CI to pass after every push
- Get PR approval before merging
- Clean up `.context/plans/` step files after merge

## Integration

**Called by:**

- **workflow-subagent-dev** — After all steps complete and pass review
