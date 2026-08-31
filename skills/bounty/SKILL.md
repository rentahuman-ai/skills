---
name: bounty
description: Write a high-quality Rent A Human bounty — title, description, definition of done, evidence requirements, pricing, deadline, and application screening — before calling `create_bounty`. Trigger whenever the user wants to "post a bounty", "hire a human for <task>", "put up a task", "create a gig", or any request that will end in a `create_bounty` / `update_bounty` call via the rentahuman-mcp tools. Also trigger when refining or debugging an underperforming bounty (no applicants, low-quality applicants, disputed evidence). Do NOT trigger for browsing or booking existing services (use `browse_services` / `search_humans` / `book_service` directly).
---

# Bounty — Writing Bounties That Get Great Work

A bounty is a task posting humans apply to. Vague bounties attract bad applicants and end in disputes; well-specified ones get accepted, completed, and paid with no friction. This skill is the authoring playbook. The MCP tools come from [`rentahuman-mcp`](https://www.npmjs.com/package/rentahuman-mcp) (bundled with this plugin). For running the bounty's lifecycle after posting, use the `loop` skill.

## The non-negotiables

Every bounty must set ALL of these before posting:

1. **Title + description** — clear title (5–200 chars); description more detailed than the user's raw ask. Expand "hold a sign" into where, when, how long, what the sign says. **Never put personal info (addresses, phone numbers, real names) in the public description** — share privately in chat after acceptance.
2. **Definition of done** (`completionCriteria`) — specific, measurable criteria. "Sign held visibly at the corner of X & Y for 2 continuous hours between 12–2pm" beats "hold sign for a while".
3. **Evidence** (`evidenceTypes` + `evidenceCriteria`) — how the worker proves completion. Types: `photo`, `video`, `link`, `text` (combine freely).
4. **Requirements + skills** (`requirements`, `skillsNeeded`) — concrete qualifications inferred from the task ("own professional camera equipment", "comfortable being in public").
5. **Price** — fixed USD by default; hourly only when the user says so. Anchor to real freelancer rates for the effort, location, urgency, and skill. Underpriced bounties get low-quality applicants — say so once if the user's number is clearly too low.
6. **Deadline** — always set one, always in the future. "No deadline" → default 2 weeks out.

## Evidence rules of thumb

- **Physical / in-person**: timestamped photos AND at least one short video clip — still photos alone are easy to fake.
- **Online**: a live link PLUS a screenshot — links can 404 or get removed.
- **Location-specific**: evidence must show the location (landmark, storefront, street sign).
- Always require at least one concrete artifact: link, screenshot, photo, or video.

## Archetype cheat sheet

| Task type | Evidence | Notes |
| --- | --- | --- |
| Sign holding / street marketing | `["photo","video"]`: ≥3 timestamped photos ~15 min apart showing sign + landmark, plus a 15–30s video | Require city. **Appearance-based → live-video gate (below)** |
| Social engagement (likes/follows/comments) | `["link","photo"]`: worker profile link + screenshot per live action | Under ~$1/action expect non-local workers |
| UGC / short-form video | `["link","video"]`: live post link + raw file | Ask whose account posts. **Appearance-based → live-video gate** |
| Sponsored posts | `["link","photo"]`: live post link + 24h-later screenshot | **Appearance-based → live-video gate** |
| QA / bug testing | `["text","video","link"]`: structured report + recordings of bugs | Prefer the dedicated `qa` skill / QA run tools |
| In-person services (cleaning, delivery, errands) | `["photo"]`: timestamped before/after or handoff photos | Require city + firm deadline |

## Application screening (`applicationDetails`)

Standard application bounties support pre-acceptance screening items: `text` questions, `checkbox` acknowledgments (max 5), `file_upload` (max 3), and **one required `live_video` field** whose `label` is a script the applicant must record with the in-browser camera. Pre-recorded files are rejected server-side via camera metadata, so the recording is genuinely live.

**Appearance-based bounties MUST include the live-video gate.** When the worker appears on camera or their appearance/presence is part of the deliverable — sign holding, UGC, sponsored posts, on-camera video, brand-ambassador or GTM/promo appearances, event staffing, modeling — always add one required `live_video` item with a label like:

> "Record a short video of yourself explaining, in 2 sentences, why you'd be a great fit for this task."

This gives the poster a real look at each applicant before accepting, and it filters out spray-and-pray applicants for free.

Screening caveats:

- Not available on direct-upload bounties (`submissionMode` of `photo_upload` / `video_upload` / `document_upload`) — those flows collect proof and consent already.
- Keep screening minimal for simple tasks; every extra field costs applicants.
- **Never** ask for passwords, OTP/2FA codes, API keys, seed phrases, government IDs, bank/card details, exact home addresses, dates of birth, or other sensitive personal information.

## Posting flow

1. Call `create_bounty` with `dryRun: true` first. Show the preview and `preview.fundingTotal` to the user.
2. Only post (`dryRun: false` or omitted) after the user approves the preview and price.
3. Pass a stable `idempotencyKey` so retries can't double-post.
4. If the response includes `deposit_url`, the wallet can't cover the bounty — the account owner must complete checkout before it goes live.
5. Multi-person tasks: set `spotsAvailable` (1–50) instead of posting duplicates.
6. Time-boxed tasks: set `completionWindowHours` (1–720) so a confirmed worker who stalls doesn't strand the seat — past the window the seat auto-releases and the listing reopens (auto-reassign). Workers can request extensions; answer with `decide_extension_request` (`approve` / `deny` / `grant` with custom `hours`). Pending requests show on `get_bounty_applications` as `completionExtension` and pause the auto-release for up to 24h. Lean prompt (errands 4–8h, content/research 24–48h) — you can always extend.
7. Hands-off mode: `aiManaged: true` (beta) has the platform recruit, vet, review, and pay — fixed USD price, 1–50 spots. `completionWindowHours` overrides its default 6-hour work window.

After posting, switch to the `loop` skill to poll applications, evaluate, accept, and release payment.

## Debugging an underperforming bounty

- **No applicants**: price too low, location too narrow, or deadline too tight — fix with `update_bounty`.
- **Accepted worker stalling**: if the bounty has `completionWindowHours`, the seat auto-releases at the deadline; otherwise release it manually with `expire_application`. Change the window with `update_bounty` (affects future seats only; `null` disables).
- **Low-quality applicants**: add the live-video gate and/or targeted screening questions; tighten `requirements`.
- **Disputed evidence**: your `evidenceCriteria` were ambiguous — rewrite them measurably for the next posting.
