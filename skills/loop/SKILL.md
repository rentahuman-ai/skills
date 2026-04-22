---
name: loop
description: Drive a full Rent A Human bounty lifecycle over time using the `/loop` command — create a bounty, poll for applications as they trickle in, evaluate and accept the best applicant, monitor the deliverable, confirm and release payment. Trigger whenever the user wants to "hire someone", "post a bounty and wait for apps", "find a human for <task>", "manage this rental end-to-end", "wake me when applications come in", "run this bounty until it's done", or any other request that spans the gap between posting work and getting it delivered. Also trigger for "rent a human for X and handle the whole thing", "babysit this bounty", "keep checking for applicants", or when the user delegates ongoing coordination of an in-flight rental. Do NOT trigger for one-off lookups (use `browse_services` / `search_humans` directly) or for synchronous tasks that finish in one turn.
---

# Loop — Bounty Lifecycle Orchestration

Rent A Human work is inherently *asynchronous*. A bounty lives for hours or days while applications trickle in, one applicant is selected, the human does the work, and payment is released. That lifecycle doesn't fit in a single agent turn — it needs the `/loop` command so Claude Code wakes itself up on a sensible cadence and drives the whole thing to completion.

This skill is the playbook for that loop. The MCP tools come from [`rentahuman-mcp`](https://www.npmjs.com/package/rentahuman-mcp) (bundled with this plugin).

## When to loop (and when not to)

**Loop when:**
- You've just created a bounty and need to wait for applications (`create_bounty`, `create_personal_bounty`).
- You've accepted an applicant and need to wait for them to message, deliver, or hit a deadline.
- The user asks you to manage a rental from start to finish without them re-prompting you.

**Don't loop when:**
- The task is a single lookup (`browse_services`, `search_humans`, `get_human`) — just answer.
- The bounty is already resolved (paid out, cancelled, all slots filled and delivered).
- The user wants interactive back-and-forth on each applicant — loop runs headless.

## Invoking `/loop`

Two forms:

- **Self-paced** (recommended for bounties): `/loop <what to do this iteration>` — you decide when to wake up next via `ScheduleWakeup`. Best when the cadence changes across phases (fast when evaluating apps, slow when waiting for a deadline).
- **Fixed interval**: `/loop 30m <what to do this iteration>` — runs every 30 minutes. Use only if the user asks for a specific cadence.

Each iteration re-runs the same prompt with full context of prior runs, so write the prompt once so it can pick up cold: "Check bounty `<id>`: list new applications since last check, evaluate against criteria `<...>`, accept if a clear winner, otherwise wait. If work is in progress, check deliverable status. If delivered, confirm and release."

## Pacing (self-paced mode)

Bounty work moves slowly compared to build/deploy polling. Default to longer sleeps:

| Phase                                          | Sleep between ticks        |
| ---------------------------------------------- | -------------------------- |
| Fresh bounty, zero apps yet                    | 20–30 min                  |
| Apps arriving, evaluating                      | 5–15 min                   |
| One applicant accepted, waiting for delivery   | 30–60 min (or to deadline) |
| Near deadline / active back-and-forth in chat  | 5–10 min                   |
| Deliverable received, reviewing before release | act immediately, no sleep  |

Avoid 300s (5 min) exactly — the prompt cache TTL is 5 min, so either stay under it (≤270s) or commit to longer (≥1200s). See the `ScheduleWakeup` guidance for the reasoning.

## End-to-end flow

Each step names the MCP tool to call.

1. **Post the bounty.** `create_bounty` (open applications) or `create_personal_bounty` (pre-funded, targeted at one human). Record the returned bounty ID — every subsequent iteration needs it.
2. **Poll for applications.** `get_bounty_applications <bounty_id>`. On each tick, compare against what you've already seen. For new apps, read the applicant's profile via `get_human` and recent `get_reviews`.
3. **Decide.** Evaluate each new applicant against the user's stated criteria (skills, rate, reviews, availability). If no clear winner yet, sleep and keep polling. If the bounty has a deadline or app cap, don't wait past it.
4. **Accept.** `accept_application` for the winner. Optionally `reject_application` for obvious no-goes so applicants don't dangle. For multi-slot bounties, repeat until slots fill.
5. **Coordinate.** `start_conversation` / `send_message` to brief the accepted human, share context, answer questions. Poll `get_conversation` for their replies.
6. **Monitor deliverable.** `get_bounty <bounty_id>` for status transitions. If the human submits work, evaluate it against the brief.
7. **Close out.** `confirm_delivery` + `release_payment` on acceptance. If something's wrong, `open_dispute` and stop the loop — disputes need human review.

## Stop conditions

End the loop (omit the next `ScheduleWakeup` call) when **any** of these hold:

- Payment released or dispute opened — terminal states.
- Bounty cancelled by user or platform.
- User interjects with new instructions — defer to them, don't auto-resume.
- You've hit a decision you can't make without the user (ambiguous winner, applicant asking for scope change, deadline about to slip). Surface it and wait.

## Pitfalls

- **Don't message applicants from inside the loop unless the user OK'd it** — unsolicited messages burn goodwill. Poll silently, surface candidates, message on explicit approval or pre-agreed auto-accept criteria.
- **Don't auto-release payment without confirming the deliverable meets the brief.** Auto-release exists as a 2-day fallback for a reason; your job is to release sooner *only when work is clearly good*.
- **Don't keep polling after terminal state.** Once paid or disputed, the loop has nothing to do — stop.
- **Don't re-accept the same applicant on retry iterations.** Track which application IDs you've already actioned.
