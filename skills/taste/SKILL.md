---
name: taste
description: Run paid Rent A Human taste panels for human aesthetic judgment on designs, images, branding, landing pages, copy, music, fashion, video, or other creative alternatives. Trigger when the user asks what looks, sounds, reads, or feels better; wants a preference vote, vibe check, visual critique, human taste test, creative ranking, or statistically broader aesthetic feedback; wants 1–100 vetted humans to compare linked artifacts; or wants an asynchronous taste run managed until its report is ready. Use browse_taste_humans instead when the user wants to hire one specific creative professional to make or iteratively critique work.
---

# Taste — Human Aesthetic Judgment

Taste runs collect independent votes from a panel of vetted creative humans.
Use the Rent A Human MCP tools bundled with this plugin when available.

## Choose the right workflow

Use `create_taste_run` when the user has 2–6 existing, publicly reachable
artifacts and wants aggregate judgment: which option wins, how decisive the
vote is, why people chose it, and representative quotes.

Use `browse_taste_humans` plus a direct hire when the user needs one person to
create work, inspect private files, collaborate through revisions, or provide a
deep expert critique rather than independent panel votes.

## Run lifecycle

1. Check `get_wallet_balance` when the panel budget may exceed the available
   wallet balance. Use `deposit_wallet` if needed.
2. Call `create_taste_run` with the question, 2–6 `{label, url}` artifacts,
   respondent count, pay per respondent in cents, and optional taste-category
   or country targeting.
3. **Always pass `idempotencyKey`.** Derive a stable key from the user's project
   and decision, then reuse it for every retry. Never generate a fresh key just
   because a request timed out; that can create a second paid run.
4. Record the returned run ID. Call `get_taste_run` until `status` is `closed`,
   or subscribe to the `run.report_ready` webhook.
5. Present the closed report: `summary`, `tally`, `winner`, `quotes`, and
   `degraded`. Explain uncertainty when shares are close or the report is
   degraded.

If creation reports `insufficient_wallet_balance`, tell the user the current
and needed cents, call `deposit_wallet`, and retry with the **same**
`idempotencyKey`. The blocked run retries automatically after funding.

## Pacing with `/loop`

Taste runs close over hours, not seconds. For autonomous monitoring, use a
self-paced `/loop` and poll `get_taste_run` every 20–30 minutes. Stop polling
when the run is closed, failed, or expired, or when the user interjects.

Avoid rapid polling. Prefer `run.report_ready` webhooks when the agent has a
webhook endpoint.

## Example: 50-person landing-page vote

```text
create_taste_run(
  title: "Landing page trust vote",
  question: "Which landing page feels more trustworthy and makes you more likely to start checkout?",
  artifacts: [
    { label: "A", url: "https://example.com/landing-a" },
    { label: "B", url: "https://example.com/landing-b" }
  ],
  respondentCount: 50,
  payPerRespondentCents: 200,
  targetCategories: ["design"],
  idempotencyKey: "acme_landing_trust_vote_2026_07"
)
```

This pays 50 people × $2 = $100 total respondent pay. Poll the returned run ID
with `get_taste_run`, normally every 20–30 minutes.

## REST fallback

For agents without MCP, use the same API key and idempotency key over REST:

```bash
curl -X POST https://rentahuman.ai/api/v1/taste/runs \
  -H "X-API-Key: $RENTAHUMAN_API_KEY" \
  -H "Idempotency-Key: acme_landing_trust_vote_2026_07" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Landing page trust vote",
    "question": "Which landing page feels more trustworthy and makes you more likely to start checkout?",
    "artifacts": [
      {"label":"A","url":"https://example.com/landing-a"},
      {"label":"B","url":"https://example.com/landing-b"}
    ],
    "respondentCount": 50,
    "payPerRespondentCents": 200,
    "targetCategories": ["design"]
  }'
```

Then poll the run:

```bash
curl -H "X-API-Key: $RENTAHUMAN_API_KEY" \
  https://rentahuman.ai/api/v1/taste/runs/TASTE_RUN_ID
```

Use the wallet REST endpoints when MCP wallet tools are unavailable:

```bash
curl -H "X-API-Key: $RENTAHUMAN_API_KEY" \
  https://rentahuman.ai/api/wallet/balance

curl -X POST https://rentahuman.ai/api/wallet/deposit \
  -H "X-API-Key: $RENTAHUMAN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"amount":100}'
```
