---
name: qa
description: Run one-time or recurring human QA on a website or web app through RentAHuman.ai. Trigger when the user wants real people to test a product journey, find usability or blocking issues, return photo/video evidence, or monitor an autonomous QA run.
---

# RentAHuman QA

Use RentAHuman to have real people test a defined website or web-app journey and
return evidence-backed findings. This complements automated tests with real
devices, browsers, human interpretation, and unexpected failure paths.

## Before creating a run

Collect the target URL, the exact journey to test, what counts as a finding,
the evidence mode (`photo`, `video`, or `document`), tester count, tester pay,
cadence, and any country or qualification requirements.

QA runs spend the account wallet. Confirm the budget with the user before
creating a paid run unless they already gave clear spending authorization. The
per-run budget must cover tester pay plus the 18% platform fee for every tester.
For example, three testers paid 1,500 cents each require a budget of at least
5,310 cents.

Create an API key at
[rentahuman.ai/account/api-keys](https://rentahuman.ai/account/api-keys) and
export it as `RENTAHUMAN_API_KEY` before using authenticated tools. Never use a
production password, API key, payment secret, or other sensitive credential in
a test brief. If a tester needs access, use a disposable test account and put
its setup details in `testerStartMessage`, which is sent only after acceptance.

## MCP workflow

Use the `rentahuman-mcp` tools bundled with the RentAHuman skills plugin when
they are available.

1. Call `get_wallet_balance` when the run may exceed the current balance. If it
   is short, use `deposit_wallet` and wait for the deposit to complete.
2. Call `create_qa_run_template` with the target, instructions, cadence,
   budget, tester count, evidence mode, and optional restrictions.
3. Always pass a stable `idempotencyKey`, derived from the project, journey,
   and intended cadence. Reuse the exact key when retrying a timed-out request;
   never make a new key just because the first response was slow.
4. Record the returned template ID. Use `list_qa_runs(templateId: ...)` to see
   its runs and `get_qa_run(runId: ...)` for the full report.
5. For a run that is still in progress, poll every 20–30 minutes or use the
   `run.report_ready` webhook when the agent has a webhook endpoint. Stop when
   the run is closed, failed, or expired.
6. Present the closed report's summary, findings, reproduction steps, degraded
   status, accepted tester count, and evidence links. Call out escalations that
   need the account owner to review.

Example:

```text
create_qa_run_template(
  name: "Checkout QA",
  targetUrl: "https://example.com/checkout",
  instructions: "Complete checkout with a test account. Record every blocking issue, confusing step, and visible error. Include reproduction steps and evidence.",
  cadence: "once",
  budgetPerRunCents: 1770,
  payPerTesterCents: 1500,
  testerCount: 1,
  submissionMode: "video",
  periodCapCents: 1770,
  idempotencyKey: "checkout_qa_2026_08_12"
)

list_qa_runs(templateId: "template_01")
get_qa_run(runId: "template_01_2026-08-12")
```

For recurring schedules, use `daily`, `every_2_days`, or `weekly` and set
`periodCapCents` to the maximum amount the account may reserve in the billing
period. For `once`, `periodCapCents` must equal `budgetPerRunCents`.

## REST fallback

Agents without MCP can use the same API key against the public REST API:

```bash
curl -X POST https://rentahuman.ai/api/v1/qa/templates \
  -H "X-API-Key: $RENTAHUMAN_API_KEY" \
  -H "Idempotency-Key: checkout_qa_2026_08_12" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Checkout QA",
    "targetUrl": "https://example.com/checkout",
    "instructions": "Complete checkout with a test account. Record every blocking issue, confusing step, and visible error. Include reproduction steps and evidence.",
    "cadence": "once",
    "budgetPerRunCents": 1770,
    "payPerTesterCents": 1500,
    "testerCount": 1,
    "submissionMode": "video",
    "periodCapCents": 1770,
    "idempotencyKey": "checkout_qa_2026_08_12"
  }'
```

Then list runs and fetch a report:

```bash
curl "https://rentahuman.ai/api/v1/qa/runs?templateId=$TEMPLATE_ID" \
  -H "X-API-Key: $RENTAHUMAN_API_KEY"

curl "https://rentahuman.ai/api/v1/qa/runs/$RUN_ID" \
  -H "X-API-Key: $RENTAHUMAN_API_KEY"
```

The API returns a warning instead of rejecting template creation when the
wallet is below the run budget. Fund the wallet before the run is due. Run
details include the report, finding diff, escalation summaries, and
allowlisted evidence from accepted submissions. Video evidence is available
under `applications[].submission.evidence[]`.

See the [QA API documentation](https://rentahuman.ai/docs#api-qa) for the
complete request and response contract.
