---
name: humanize-text
description: Hire a Rent A Human worker to rewrite private text without generative AI, with a final DOCX/TXT, continuous screen recording, and advisory Pangram results. Trigger when the user asks for a human rewrite, human editing, proof-of-human writing evidence, or the RentAHuman humanization API.
---

# Humanize Text

Use RentAHuman's text-only humanization workflow when the user wants a real
person to rewrite text without generative AI. The source stays private until a
worker is accepted. The worker must attest to no AI use and submit both a final
DOCX or UTF-8 TXT document and a continuous MP4, MOV, or WebM screen recording.

Pangram is an advisory signal. It does not prove who wrote the text and never
authorizes an automatic approval, rejection, redo, or payment release.

## Before spending

Collect:

- the source text (1–100,000 characters);
- one goal: `paraphrase`, `tone`, `clarity`, `shorten`, `expand`, or
  `general_rewrite`;
- optional instructions (up to 5,000 characters);
- turnaround in whole minutes (5–10,080);
- fixed worker pay in cents (300–100,000,000);
- `USD` or `EUR` (default `USD`).

Optionally collect applicant-vetting requirements:

- an AI-generated text similar in subject and difficulty to the real source;
- the percentage of the real source to test (1–25%, default 5%) and maximum
  sample size (50–500 words, default 500); RentAHuman truncates the supplied
  sample to the calculated size;
- optional sample-specific instructions;
- a language and minimum proficiency (`conversational`, `professional`,
  `fluent`, or `native`);
- whether every applicant must privately upload a PDF, DOCX, or TXT resume.

When screening is enabled, generate the representative sample yourself and
provide enough words for the requested calculated size. Do not copy the
requester's private source into the sample. Each applicant must humanize the
sample with the same no-AI attestation, document, continuous screen recording,
and advisory Pangram analysis before they can be accepted. Treat resume
contents as private application evidence.

The account pays worker pay plus the platform fee: 18% of worker pay, rounded
to the nearest cent. Confirm that total and receive clear spending
authorization before calling `create_humanization`, unless the user already
authorized that exact amount.

Do not create a request intended to falsify credentials, impersonate a
professor/employer/recommender, violate academic-integrity rules, or evade an
institution's required AI disclosure. Legitimate editing is allowed. If the
intent is ambiguous, ask what the text will be used for before spending.

## MCP workflow

Prefer the `rentahuman-mcp` tools when available.

1. Build a stable idempotency key from the user's project, the source's stable
   identifier or hash, and the authorized revision. Reuse it for every retry of
   the same order; never mint a new key because a request timed out.
2. Call `create_humanization` with all required inputs and the idempotency key.
3. If the response says funding is pending, give the deposit URL to the account
   owner. The public listing does not open until funding succeeds.
4. Save the returned humanization ID and poll `get_humanization` at a cadence
   appropriate to the deadline (normally every 2–5 minutes; more frequently
   near a short deadline).
5. Report assignment changes, the exact worker deadline, expired/reopened
   attempts, applicant screening results, and submitted document/video links.
   If screening is configured, present every applicant's sample and Pangram
   result before asking the requester whom to accept.
6. When analysis is terminal (`complete`, `partial`, or `unavailable`), present
   Pangram's classification, fractions, status, and model version. Explicitly
   call out detector unavailability.
7. Ask the requester to review the document, recording, attestation, and
   detector evidence. Use the existing submission-review tool only after their
   decision to approve, request a redo, or reject.

Example:

```text
create_humanization(
  format: "text",
  sourceText: "...",
  transformation: "clarity",
  instructions: "Keep every factual claim and make the tone warmer.",
  turnaroundMinutes: 60,
  priceCents: 2500,
  currency: "USD",
  applicantScreening: {
    aiGeneratedSourceText: "...representative AI-generated sample...",
    percentage: 5,
    maximumWords: 500,
    instructions: "Preserve the claims while improving clarity."
  },
  languageRequirement: {
    language: "English",
    minimumProficiency: "fluent"
  },
  requireResume: true,
  idempotencyKey: "launch-post-clarity-v1"
)

get_humanization(humanizationId: "...")
```

## REST fallback

Use an API key from
[rentahuman.ai/account/api-keys](https://rentahuman.ai/account/api-keys). Put the
request in a local JSON file so private source text is not exposed through shell
history or process arguments.

```bash
curl -X POST https://rentahuman.ai/api/v1/humanizations \
  -H "X-API-Key: $RENTAHUMAN_API_KEY" \
  -H "Idempotency-Key: launch-post-clarity-v1" \
  -H "Content-Type: application/json" \
  --data-binary @humanization-request.json
```

`humanization-request.json`:

```json
{
  "format": "text",
  "sourceText": "...",
  "transformation": "clarity",
  "instructions": "Keep every factual claim and make the tone warmer.",
  "turnaroundMinutes": 60,
  "priceCents": 2500,
  "currency": "USD",
  "applicantScreening": {
    "aiGeneratedSourceText": "...representative AI-generated sample...",
    "percentage": 5,
    "maximumWords": 500
  },
  "languageRequirement": {
    "language": "English",
    "minimumProficiency": "fluent"
  },
  "requireResume": true
}
```

Poll without placing any private text in the URL:

```bash
curl https://rentahuman.ai/api/v1/humanizations/HUMANIZATION_ID \
  -H "X-API-Key: $RENTAHUMAN_API_KEY"
```

Delete the local request file when it is no longer needed, following the user's
data-retention expectations.
