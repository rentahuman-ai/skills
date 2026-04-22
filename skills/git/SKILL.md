---
name: git
description: 'Git workflow conventions for commits, pull requests, and rebases. Use when committing, creating PRs, or rebasing.'
---

# Git Workflow

## Commit Messages

Format: `<scope>: <imperative summary>` with optional body and references. Scopes map to the monorepo structure (e.g. `api:`, `app:`, `web:`, `database:`).

For full conventions and examples, see [references/commit.md](references/commit.md).

## Pull Requests

Every PR body must include Context, What Changed, Validation, and Risks & Rollback sections.

For full conventions and template, see [references/pr.md](references/pr.md).

## Rebase

### Conflict Resolution Workflow

When conflicts arise during rebase, follow this sequence for EVERY file:

1. **Identify the merge base**

   ```bash
   git merge-base HEAD ORIG_HEAD
   ```

2. **Read all three versions** before touching anything

   ```bash
   git show HEAD:<file>          # target branch version
   git show ORIG_HEAD:<file>     # your branch version
   git show <merge-base>:<file>  # common ancestor
   ```

3. **Understand the intent** of both sides before resolving
   - What did the target branch change since the ancestor?
   - What did your branch change since the ancestor?

4. **Resolve and verify** the file compiles / parses correctly

5. **Continue** only after all conflicts in the current step are resolved
   ```bash
   git add <resolved-files>
   git rebase --continue
   ```

### Guardrails

- NEVER use `-X ours` or `-X theirs` to bulk-resolve without reading each conflict
- NEVER skip step 2 (reading all three versions) — guessing from markers alone causes silent regressions
- If unsure which side made a change, verify with the merge-base comparison
- If a file was deleted on one side and modified on the other, always pause and verify intent before choosing

### Abort Conditions

Abort the rebase (`git rebase --abort`) if:

- More than 5 consecutive conflicts suggest the branch diverged too far
- A conflict involves generated files (`openapi.json`, `generated/*`) — regenerate after rebase instead
- You cannot determine the intent of both sides

For ours/theirs terminology reference and delete-vs-modify semantics, see [references/rebase-conflict-resolution.md](references/rebase-conflict-resolution.md).

## References

| Topic                     | Reference                                                                 |
| ------------------------- | ------------------------------------------------------------------------- |
| Commit conventions        | [commit.md](references/commit.md)                                         |
| PR conventions            | [pr.md](references/pr.md)                                                 |
| Rebase conflict semantics | [rebase-conflict-resolution.md](references/rebase-conflict-resolution.md) |
