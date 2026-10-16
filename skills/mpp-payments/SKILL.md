---
name: mpp-payments
description: Pay RentAHuman over MPP by topping up the wallet through an HTTP 402 challenge with a card or Link, then fund bounties. Trigger on "pay RentAHuman with MPP", "top up wallet", or HTTP 402 from /api/mpp/wallet/deposit. Not for x402/USDC.
---

# MPP Payments

Top up the RentAHuman wallet over the Machine Payments Protocol (MPP) with a card or Link, with no checkout page, then fund bounties from the balance. Wallet tools come from [`rentahuman-mcp`](https://www.npmjs.com/package/rentahuman-mcp), which ships with this plugin. Full reference: [rentahuman.ai/docs/mpp](https://rentahuman.ai/docs/mpp).

## Prerequisites

- A RentAHuman API key (`rah_...`) in `RENTAHUMAN_API_KEY`, or as `X-API-Key` for REST. Get one without a browser: `POST https://rentahuman.ai/api/agents/signup` with an email, then `/api/agents/signup/verify` with the code.
- An MPP client that pays with a Stripe Shared Payment Token (SPT): the Stripe Link CLI (`npx @stripe/link-cli auth login` once), or `mppx/client` with your own backend creating the SPT.
- A credit or debit card, used directly or through Link. Prepaid cards are refused.

## Steps

1. **Request a deposit.** Call MCP `deposit_wallet` with `amount` in dollars, or `POST https://rentahuman.ai/api/wallet/deposit` with `{"amount": 25}`. The response has a `checkoutUrl` and an `mppPayment` (`url`, `method`, `body: {"quote": "mtq1_..."}`, `expiresAt`). Pay one, never both: paying both charges twice. No `mppPayment` means MPP isn't available for this deposit, so give the account owner the `checkoutUrl`. Keep the quote out of URLs, logs and chat.
2. **Pay the 402 challenge.** POST `mppPayment.body` unchanged to `mppPayment.url`, with no API key. The client answers the `402` `WWW-Authenticate: Payment` challenge with an SPT for exactly the quoted amount.
3. **Confirm the credit.** `200` `credited` means the wallet is credited. `202` `pending_credit` means paid and the credit follows shortly: don't pay again. Check with MCP `get_wallet_balance`.
4. **Fund a bounty.** Run `create_bounty` with `dryRun: true` and show `preview.fundingTotal` (worker pay plus the 18% platform fee). Top up any shortfall, then post with `dryRun: false` and a stable `idempotencyKey`. If a `deposit_url` comes back, either give the owner that link or top up the shortfall over MPP and call `fund_bounty`, never both.

## Example

```bash
npx @stripe/link-cli mpp pay https://rentahuman.ai/api/mpp/wallet/deposit \
  -X POST \
  -d '{"quote": "mtq1_..."}' \
  --context 'Top up my RentAHuman wallet with $25 so my agent can post bounties and pay people for physical-world tasks.'
```

`--context` must be at least 100 characters; the Link account holder reads it when approving. Don't pass `--amount`.

## Limits

$1 to $10,000 USD per top-up; card or Link only (no USDC over MPP); quotes are single-use and expire after 30 minutes; no 3D Secure.

## Common errors

- `409 payment_not_completed`, `quote_used`, `quote_closed`: already submitted. Don't pay this quote again; check the balance before requesting a new deposit.
- `410 payment_failed`, `quote_expired`, `404 quote_not_found`: nothing was charged. Request a new deposit.
- `403` refusals (`card_funding_refused`, `risk_refused`, ...) or `404 not_found`: use the `checkoutUrl` from the same deposit.
- `429`, `503`, or a network failure with an unclear result: retry the same quote before it expires. A quote can be charged only once.

All error codes, refunds and response shapes: [rentahuman.ai/docs/mpp](https://rentahuman.ai/docs/mpp).
