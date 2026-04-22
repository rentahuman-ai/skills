---
name: ai-observability
description: Debug agent-run internals using the Langfuse CLI (`bunx langfuse-cli`) — pull traces, observations, scores, and sessions to see every LLM call, tool invocation, prompt, and token cost for a given run. Use this skill whenever the user wants to investigate *why* an agent did (or didn't do) something — phrases like "why did the bounty-manager do X", "pull the trace for that run", "debug the agent run", "check langfuse", "what tools did the agent call", "how much did that invocation cost", "inspect that agent session", "find the trace for run <id>", or any request to look inside a completed agent execution. Also trigger for "trace the Vercel AI SDK call", "what was the model output", "grab the langfuse session", or when the user wants to compare two runs. Do NOT trigger for Vercel runtime logs (use `fix-prod-errors`), for Firestore data inspection, or for questions about live dev-server output.
---

# AI Observability — Debugging Agent Runs via Langfuse

Every AI SDK call in this codebase with `experimental_telemetry: { isEnabled: true }` streams spans to Langfuse through `src/instrumentation.ts`. The Langfuse CLI is how you interrogate that data from the terminal: pull a trace, walk its observations, see the full prompt + response + tool calls for every step, check token costs and scores.

The guiding reason this skill exists: when an agent misbehaves — wrong decision, wrong tool, runaway loop, hallucinated output — the only complete record of *what it actually saw and did* lives in Langfuse. Firestore has the audit doc (inputs/outputs), Vercel logs have the structured log lines, but the full per-step model I/O only exists as spans. This skill turns "something weird happened in that agent run" into a concrete reproduction of the exact prompt/response/tool trajectory.

## Setup

### Credentials

Langfuse is already configured in this repo's `.env.local` via `LANGFUSE_PUBLIC_KEY` / `LANGFUSE_SECRET_KEY` (see `src/env/server.ts`). The CLI reads the same vars but calls the host `LANGFUSE_HOST`:

```bash
export LANGFUSE_PUBLIC_KEY=pk-lf-...
export LANGFUSE_SECRET_KEY=sk-lf-...
export LANGFUSE_HOST=https://us.cloud.langfuse.com   # NOTE: US region
```

**Critical: this project is on the US region.** The CLI's default is `https://cloud.langfuse.com` (EU). If you don't set `LANGFUSE_HOST` you will silently get zero results — the credentials exist on the US tenant only. Source the env from `.env.local`:

```bash
set -a; source .env.local; set +a
export LANGFUSE_HOST=https://us.cloud.langfuse.com   # .env uses LANGFUSE_BASE_URL; CLI wants LANGFUSE_HOST
```

### Running the CLI

Prefer `bunx` — matches the repo's package manager and avoids a global install:

```bash
bunx langfuse-cli api <resource> <action> [flags]
```

No install step needed. `npx langfuse-cli` works too if you're not in a Bun shell.

## Discovery — don't guess, ask the CLI

The CLI is generated from the Langfuse OpenAPI spec, so `--help` is always accurate and always current. Use it instead of memorizing flags:

```bash
bunx langfuse-cli api __schema                        # every resource + action
bunx langfuse-cli api traces --help                   # actions on a resource
bunx langfuse-cli api traces list --help              # flags for one action
bunx langfuse-cli api traces list --curl              # preview the HTTP call (no execution)
```

Use `--curl` first when you're unsure what a flag does — you see the request the CLI is about to make without sending it.

### Prefer v2 endpoints

The v1 shapes are thinner and some operations aren't supported. Use these:

- `observations-v2s` instead of `observations`
- `metrics-v2s` instead of `metrics`
- `score-v2s` instead of `scores` (v1 only supports create/delete — no list/get)

If you see a CLI example using the v1 name, swap to v2 unless you have a specific reason.

## Core debug recipes

### 1. Find the trace for a specific agent run

Agent runs tag traces with identifying metadata. The search path depends on what you know:

```bash
# By user — e.g. the requesting user of a bounty-manager run
bunx langfuse-cli api traces list --user-id <uid> --limit 20 --json

# By session — multiple traces that share a sessionId (e.g. one rental conversation)
bunx langfuse-cli api traces list --session-id <sid> --limit 50 --json

# By trace name — the OTEL span name, e.g. the function wrapped with `observe()`
bunx langfuse-cli api traces list --name "bountyManagerRun" --limit 20 --json

# By tags — if the code sets tags via propagateAttributes
bunx langfuse-cli api traces list --tags "bounty:abc123" --json

# By time window — narrow to when the incident happened
bunx langfuse-cli api traces list \
  --from-timestamp 2026-04-21T14:00:00Z \
  --to-timestamp   2026-04-21T15:00:00Z \
  --order-by "timestamp.desc" \
  --json
```

Pagination: `--limit` (default varies per endpoint, usually 50) and `--page`. On busy windows, raise the limit and paginate rather than lowering the time window — you want to see patterns, not a single trace.

Pipe to `jq` for the ID you want:

```bash
bunx langfuse-cli api traces list --name bountyManagerRun --limit 50 --json \
  | jq -r '.data[] | "\(.id)  \(.timestamp)  \(.userId // "-")  \(.name)"'
```

### 2. Pull the full trace with all observations

Once you have a trace ID, the single richest view is the trace detail — it includes every observation (LLM call, tool call, span) nested under the trace:

```bash
bunx langfuse-cli api traces get <trace-id> --json > /tmp/trace.json
```

Walk the structure:

```bash
jq '.observations | length' /tmp/trace.json                     # step count
jq '.observations[] | {type, name, startTime, latency, model}' /tmp/trace.json
jq '.observations[] | select(.type == "GENERATION")' /tmp/trace.json   # LLM calls only
jq '.observations[] | select(.type == "TOOL")'       /tmp/trace.json   # tool calls only
jq '.totalCost, .totalTokens' /tmp/trace.json                   # cost/token totals
```

### 3. Inspect one observation (LLM call or tool call)

When you need the full prompt + completion + tool args for a single step:

```bash
# List observations for a trace
bunx langfuse-cli api observations-v2s get-many --trace-id <trace-id> --json

# One observation in detail — this is where prompt/completion/toolArgs actually live
bunx langfuse-cli api observations-v2s get <observation-id> --json > /tmp/obs.json
jq '.input, .output' /tmp/obs.json                # full I/O
jq '.metadata'       /tmp/obs.json                # tags, model params
jq '.usage'          /tmp/obs.json                # prompt/completion/total tokens
```

For `GENERATION` observations (LLM calls) the `input` field holds the messages array the model saw. For `TOOL` observations it holds the tool arguments. This is the ground truth for "what did the agent actually see" — prefer it over reconstructing from the Firestore audit doc.

### 4. Sessions — all traces for one user-facing interaction

If several traces belong to the same workflow (e.g. one rental conversation triggers multiple model calls over time), they share a session:

```bash
bunx langfuse-cli api sessions list --user-id <uid> --limit 20 --json
bunx langfuse-cli api sessions get <session-id> --json
```

Session detail enumerates all member trace IDs — from there, loop back to recipe #2 for each.

### 5. Scores and evals

Quality scores (whether set programmatically via `score.create()` or by a Langfuse LLM-as-judge) attach to traces or observations:

```bash
bunx langfuse-cli api score-v2s get-scores --trace-id <trace-id> --json
bunx langfuse-cli api score-v2s get-scores --name "agent_completion" --limit 100 --json
```

Remember: `score-v2s`, not `scores`. The v1 resource is write-only.

### 6. Compare two runs

When the user asks "why did run A succeed but run B fail", pull both in parallel and diff the observation sequences:

```bash
bunx langfuse-cli api traces get <trace-a> --json > /tmp/a.json &
bunx langfuse-cli api traces get <trace-b> --json > /tmp/b.json &
wait

# Compare observation sequences
diff \
  <(jq -r '.observations[] | "\(.type)\t\(.name)"' /tmp/a.json) \
  <(jq -r '.observations[] | "\(.type)\t\(.name)"' /tmp/b.json)

# Compare first LLM prompt (same prompt? different prompt?)
diff \
  <(jq '.observations[] | select(.type=="GENERATION") | .input' /tmp/a.json | head -200) \
  <(jq '.observations[] | select(.type=="GENERATION") | .input' /tmp/b.json | head -200)
```

## Common filters, in one place

Most `list` endpoints accept these (check `--help` for the exact names — flags are kebab-cased from the OpenAPI fields):

| Flag                  | Purpose                                                             |
| --------------------- | ------------------------------------------------------------------- |
| `--user-id`           | Filter by `userId` — the attribute your code sets on the span       |
| `--session-id`        | Filter by `sessionId`                                               |
| `--name`              | Filter by trace/observation name (the OTEL span name)               |
| `--tags`              | Filter by tag(s) — CSV or repeat the flag                           |
| `--from-timestamp`    | ISO 8601 lower bound                                                |
| `--to-timestamp`      | ISO 8601 upper bound                                                |
| `--order-by`          | e.g. `timestamp.desc`, `latency.asc`                                |
| `--limit` / `--page`  | Pagination                                                          |
| `--json`              | Structured output for `jq`/scripts                                  |
| `--curl`              | Preview the HTTP request, do not execute                            |

## This repo's trace attributes

What's on the spans (from `src/instrumentation.ts` + `src/lib/bounty-manager/runner.ts`):

- Traces are emitted via `@langfuse/otel` as OpenTelemetry spans. The Vercel AI SDK's `experimental_telemetry.functionId` becomes the trace `name`.
- `observe()` (from `@langfuse/tracing`) wraps a function to become the root span; `propagateAttributes()` sets `userId`, `sessionId`, `tags`, `metadata` on the active context.
- The primary producer today is the **bounty-manager runner** — one agent run per bounty-manager invocation. Search by the `functionId` you'll see in `runner.ts`'s `generateText({ experimental_telemetry: { functionId: ... } })` call.
- Ingest is `immediate` export mode with `x-langfuse-ingestion-version: 4` — meaning spans hit the UI within seconds (no 10-min delay from the old batch path). If you don't see a trace within ~30s of a run completing, treat that as a real absence, not lag.

## Pitfalls

- **Wrong region = zero results.** Always `export LANGFUSE_HOST=https://us.cloud.langfuse.com` for this project. The CLI defaults to EU and will happily return an empty list against a misconfigured tenant with no error. This mirrors the bug that ship ff80a5f3/1c4c317f fixed in the app itself.
- **`LANGFUSE_HOST` vs `LANGFUSE_BASE_URL`.** The app uses `LANGFUSE_BASE_URL`; the CLI reads `LANGFUSE_HOST`. Sourcing `.env.local` won't set the CLI one — export it explicitly.
- **v1 vs v2.** If a command errors with "unsupported" or returns a skeletal shape, try the `-v2s` variant.
- **`scores` doesn't list.** Use `score-v2s get-scores`.
- **Pagination drift.** If you filter by `--from-timestamp` and the window is wide, use `--order-by timestamp.desc` + `--page` rather than a single huge `--limit` — the underlying API has max page sizes.
- **Serverless flush timing.** The app flushes spans per-end (`exportMode: 'immediate'`), but if a Lambda died mid-run you may see a truncated trace. A trace whose last observation has no `endTime` is a crashed run, not a CLI bug.
- **Don't paste secrets.** Trace `input`/`output` can contain API keys or PII that showed up in a prompt. Redact before sharing a trace dump.

## Golden path for "debug this agent run"

When the user hands you a bounty-manager run ID, user ID, or timestamp:

1. Identify the trace → `traces list` with the filter you have.
2. Pull full detail → `traces get <id> --json > /tmp/trace.json`.
3. Skim the observation sequence → `jq '.observations[] | {type, name, latency}' /tmp/trace.json`.
4. Read the first `GENERATION` → full prompt the model got. Is the context right?
5. Walk each `TOOL` in order → what did the model decide? What did the tool return?
6. Check `totalCost` / `totalTokens` and any `score-v2s` for this trace.
7. Summarize: at step N the model received X, decided Y, tool returned Z, so the final result is W.

That sequence is usually enough to answer "why". If not, pull an adjacent successful run and diff (recipe #6).
