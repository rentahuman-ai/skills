---
name: fix-prod-errors
description: Fetch recent production errors and warnings from Vercel runtime logs, group them by root cause, fix each group with concurrent Opus subagents, and open a PR. Use this skill whenever the user asks to diagnose, triage, or fix production errors/warnings/runtime issues — phrases like "check prod errors", "fix the errors in prod", "what's going wrong in production", "audit recent warnings", "triage Vercel logs", "fix prod warnings", or any variation referencing production runtime health. Trigger even when the user doesn't say "Vercel" explicitly — any request about live-site errors, exceptions, 500s, or unexpected warnings qualifies. Do NOT trigger for local dev errors, build failures, or test failures — those need different workflows.
argument-hint: '[time window, e.g. "30m" or "1h"]'
---

# Fix Prod Errors

Fetch recent production errors and warnings from Vercel, group by root cause, fix each group concurrently, and open a PR. A task is not complete until the PR is merged to `main`.

The guiding reason this skill exists: prod errors decay. The longer we leave them, the more similar reports pile up and the harder it gets to separate signal from noise. This skill turns "something's wrong in prod" into a concrete, shippable PR within a single session.

## Step 1: Capture runtime logs

Use `vercel logs` directly. Take the time window from the user's request (default: `30m`).

### Prerequisite: CLI v51.0.0 or newer

The modern `vercel logs` command (post-`logsv2` rewrite, Feb 2026) supports `--since`, `--until`, `--level`, `--status-code`, and `--json` — everything needed for historical backfill. Check the version and upgrade if old:

```bash
vercel --version            # need ≥ 51.0.0
bun i -g vercel@latest      # upgrade if older (the old v50 help text still says "from now and for 5 minutes at most" — that restriction is gone in v51+)
```

### Pull the logs

```bash
vercel logs \
  --scope rent-ah-uman \
  --project human-rental-marketplace \
  --environment production \
  --no-branch \
  --since 30m \
  --level error --level warning \
  --limit 5000 \
  --json \
  --no-follow \
  > /tmp/prod-logs.jsonl
```

Flags that are easy to forget:

- `--no-branch` — the CLI auto-filters by the current git branch when run inside a repo. From a worktree on `bmg/whatever` you will get **zero** results for a production query without this flag.
- `--project human-rental-marketplace` — the project slug, not the team slug. `rent-ah-uman` is the team (`--scope`).
- `--no-follow` — forces a one-shot historical pull. Without it, a deployment-id argument would implicitly turn on streaming.
- `--limit` — defaults to 100. Raise it (e.g. 5000) for busy windows; check for truncation by comparing the line count against the limit.
- `--level` can be passed multiple times to OR-combine levels.

### JSON shape (for Step 2 parsing)

One NDJSON object per line. Fields of interest:

- `level` — `error` / `warning` / `info` / `fatal`.
- `message` — **stringified** inner JSON that carries its own `level`, `message`, `error`, `profileId`, etc. The app's bracketed tags (`[humans.PATCH]`, `[rate-limit]`, `[stripe]`) live inside this inner message — parse it before grouping.
- `requestMethod`, `requestPath`, `responseStatusCode`, `timestampInMs` — top-level request context.

A quick grouping pass with `jq` + `grep`:

```bash
# Count by bracketed tag in the inner message
jq -r '.message // ""' /tmp/prod-logs.jsonl \
  | grep -oE '\[[a-zA-Z0-9._:/-]+\]' \
  | sort | uniq -c | sort -rn | head -25

# Distribution by path and status
jq -r '.requestPath // "?"'        /tmp/prod-logs.jsonl | sort | uniq -c | sort -rn | head
jq -r '.responseStatusCode // "?"' /tmp/prod-logs.jsonl | sort | uniq -c | sort -rn
```

## Step 2: Deduplicate and group

Parse the inner message and group by bracketed tag (e.g. `[rate-limit]`, `[stripe]`, `[auth]`, `[humans.PATCH]`). For each group note:

- Log level (error vs warning)
- Affected endpoints (path + method)
- Occurrence count
- A representative log line

Discard groups that are purely informational or expected behavior (e.g. `[profile-review]` safe approvals). A single root cause can appear under multiple tags if the same error bubbles through layers — recognize and merge those.

## Step 3: Triage

For each remaining group, classify as:

- **Actionable** — a code change can fix or reduce the issue
- **Operational** — requires config, infra, quota increase, or user intervention (flag but don't try to fix here)

Print the triage summary. If nothing is actionable, stop and report "No actionable errors found."

## Step 4: Fix concurrently

For each actionable group, spawn a **general-purpose Opus subagent** (`model: "opus"`) with:

- The group's representative log lines
- The affected endpoint file paths (use `Grep` / `Glob` to locate them)
- Instructions to fix the root cause, not suppress the symptom
- Instructions to run `bun run lint && bunx tsc --noEmit` after its changes

Run subagents in parallel when their changes touch disjoint files. Serialize groups that would touch the same file — parallel edits to one file create merge churn that costs more than the parallelism saves.

## Step 5: Verify

After all subagents complete, run the full gauntlet from the worktree:

```bash
bun run lint
bunx tsc --noEmit
bun run test
```

Fix anything that fails before shipping. "Fix" means fix the underlying code — not silence the check.

## Step 6: Ship

Create a PR following `/pr` conventions. Title format: `fix: resolve prod [errors|warnings] — [summary]`.

```bash
git add -A && git commit -m "fix: resolve prod warnings — [summary of changes]"
git push -u origin HEAD
gh pr create --title "fix: resolve prod warnings" --body "$(cat <<'EOF'
## Summary
- [bullet points of what was fixed and why]

## Prod log evidence
[paste representative log lines]

## Verification
- [x] bun run lint passes
- [x] bunx tsc --noEmit passes
- [x] bun run test passes
EOF
)"
```

Then watch CI: `gh pr checks --watch --fail-fast`. Auto-merge when green: `gh pr merge --squash --auto`. Never merge a failing PR — diagnose and fix instead of leaving it hanging. Report the PR URL when done.
