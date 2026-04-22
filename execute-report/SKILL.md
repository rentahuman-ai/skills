---
name: execute-report
description: 'Use when asked to summarize daily agent output, PR activity, session history, or produce an executive report of work done across Devin, Cursor, and Codex agents.'
---

# Executive Summary Report

Generate a comprehensive daily report covering all agent activity — PRs authored, sessions run, and work done — across Devin, Cursor Background Agents, and Codex. Output is a structured storyline suitable for executive review.

## When to Use

- User asks "what did I do today?" or "summarize today's work"
- User requests a PR recap, session summary, or daily report
- User wants to see agent activity grouped by theme/storyline
- User asks for a breakdown by agent (devin vs cursor vs codex)

## Data Sources

### 1. GitHub PRs (primary)

Use `gh` CLI to pull all PRs authored by the user for a date range:

```bash
gh pr list --repo <org>/<repo> --author <username> --state all --limit 200 \
  --json number,title,state,mergedAt,createdAt,headRefName
```

**Agent attribution** is determined by branch prefix:

- `devin/` → Devin
- `cursor/` → Cursor Background Agent
- `codex/` → Codex
- anything else → manual or unknown

**Key fields:** `number`, `title`, `state` (MERGED/OPEN/CLOSED), `createdAt`, `headRefName`

### 2. Devin Sessions

Use the Devin MCP tool to list and read sessions:

```
devin_mcp → call_tool → list_sessions (filter by date)
devin_mcp → call_tool → read_session (for details)
```

Or use the `devin` CLI if available:

```bash
devin list --limit 50
devin read <session-id>
```

**Key fields:** session title, status, ACUs consumed, PRs produced, created timestamp

### 3. Cursor Background Agents API

**Base URL:** `https://api.cursor.com`
**Auth:** `Authorization: Bearer $CURSOR_API_KEY`

```bash
# List all agents (paginated, max 100 per page)
curl -s -H "Authorization: Bearer $CURSOR_API_KEY" \
  "https://api.cursor.com/v0/agents?limit=100"

# Get agent conversation (first message = prompt)
curl -s -H "Authorization: Bearer $CURSOR_API_KEY" \
  "https://api.cursor.com/v0/agents/<id>/conversation"
```

**Response shape (list):**

```json
{
  "agents": [
    {
      "id": "bc-<uuid>",
      "status": "FINISHED" | "ERROR" | "RUNNING",
      "name": "Session name",
      "createdAt": "2026-04-04T20:48:57.180Z",
      "linesAdded": 141,
      "linesRemoved": 162,
      "filesChanged": 6,
      "source": { "repository": "github.com/org/repo", "prUrl": "..." },
      "target": { "branchName": "cursor/...", "prUrl": "...", "url": "https://cursor.com/agents/<id>" }
    }
  ],
  "nextCursor": "<pagination-token>"
}
```

**Pagination:** If `nextCursor` is non-empty, fetch next page with `?cursor=<token>`.

**Conversation response:**

```json
{
  "messages": [
    { "type": "user_message", "text": "the original prompt" },
    { "type": "assistant_message", "text": "response..." }
  ]
}
```

**Filtering by date:** The API does not support date filtering. Fetch all agents and filter client-side by `createdAt`. Use ISO 8601 range comparison. Remember timezone conversion — "Apr 4 PT" = `2026-04-04T07:00:00Z` to `2026-04-05T07:00:00Z` (Pacific Daylight Time, UTC-7). Note: users often say "PST" colloquially even during daylight saving time — always check whether the date falls in PST (UTC-8, Nov–Mar) or PDT (UTC-7, Mar–Nov).

### 4. Codex (if applicable)

Codex PRs appear in GitHub with `codex/` branch prefix. No separate session API is currently available — attribute by branch name only.

## Report Structure

### 1. Top-Level Stats

```
**N PRs** · **M merged** · X open · Y closed
**D devin sessions** (~A ACUs) · **C cursor agents** · **X codex PRs**
```

Include a table:

|          | Devin | Cursor | Codex | Total |
| -------- | ----- | ------ | ----- | ----- |
| PRs      |       |        |       |       |
| Merged   |       |        |       |       |
| Open     |       |        |       |       |
| Sessions |       |        |       |       |

### 2. Storyline (acts)

Group PRs into **thematic acts** — not by agent, but by what they accomplished. Each act:

1. **Title** — short descriptive name with time range
2. **Which agents contributed** and how
3. **Key sessions** — link to Devin session URLs (`https://app.devin.ai/sessions/<id>`) and Cursor agent URLs (`https://cursor.com/agents/<id>`)
4. **PRs** — list with `#number` and short description
5. **Narrative** — what happened, what problems surfaced, how they were solved

Order acts chronologically by when the work started. Look for:

- **Cascading issues** — one PR reveals a problem that spawns 3 more
- **Parallel tracks** — devin and cursor working on same problem from different angles
- **Iterative convergence** — multiple sessions refining the same thing
- **Agent handoffs** — one agent's output becoming another's input

### 3. Throughline

End with a one-paragraph synthesis: what was the meta-narrative of the day? What capability existed at the start vs end?

## Grouping Heuristics

To identify thematic acts:

1. **PR title clustering** — look for common prefixes/keywords (IAM, bootstrap, pulumi, rivet)
2. **Session names** — Devin session titles and Cursor agent names reveal intent
3. **Conversation prompts** — read the first `user_message` from Cursor conversations to understand what was requested
4. **Temporal proximity** — PRs created within 1-2 hours on related topics likely belong together
5. **Dependency chains** — PR A fixes something that PR B depends on

## Tone and Formatting

- Casual, dense, emoji-accented (not corporate)
- Link everything — PR numbers, session URLs, agent URLs
- Tables for structured data, prose for narrative
- Use `**bold**` for key stats inline
- Each act should be skimmable in 10 seconds but have enough detail to understand what happened
- Include the user's original prompt for Cursor sessions when it adds context (especially humorous ones like "pls sir")

## Common Mistakes

- **Wrong timezone** — user says "today PST" but you filter UTC. Always convert.
- **Missing pagination** — Cursor API returns max 100 per page. Check `nextCursor`.
- **Attributing by session not branch** — a Devin session can produce PRs on non-`devin/` branches. Branch prefix is ground truth for agent attribution.
- **Flat listing** — don't just list PRs. Group into acts with narrative.
- **Forgetting conversations** — Cursor agent names alone aren't enough context. Read the first message to understand intent.
