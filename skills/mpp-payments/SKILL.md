---
name: mpp-payments
description: How an agent pays Rent A Human over the Machine Payments Protocol (MPP). Today that means topping up the RentAHuman wallet by paying an HTTP 402 challenge with a card or Link Shared Payment Token, then funding bounties from that balance. Trigger when the user or agent wants to "pay RentAHuman with MPP", "fund my RentAHuman wallet", "top up the wallet with MPP", "pay the mppPayment", "pay with Link CLI", or answer an HTTP 402 Payment challenge from `rentahuman.ai/api/mpp/wallet/deposit`, or when a bounty is `pending_deposit` and the agent should pay instead of sending the owner a checkout link. Do NOT trigger for USDC funding (that's x402, a separate early-access flow) or for writing the bounty itself (use the `bounty` skill).
---

# MPP Payments

This skill is how your agent pays RentAHuman over the Machine Payments Protocol (MPP). Today it covers topping up the RentAHuman wallet with your agent's own card or Link wallet, with no checkout page, then funding bounties from that balance. The wallet tools come from [`rentahuman-mcp`](https://www.npmjs.com/package/rentahuman-mcp), which ships with this plugin. Live reference: [rentahuman.ai/docs/mpp](https://rentahuman.ai/docs/mpp).

## Prerequisites

- **A RentAHuman account and API key** (`rah_...`). Set `RENTAHUMAN_API_KEY` for the MCP server, or send `X-API-Key` to REST. You can get a key without a browser: `POST https://rentahuman.ai/api/agents/signup` with an email, then `/api/agents/signup/verify` with the code.
- **An MPP client that pays with a Stripe Shared Payment Token (SPT)**:
  - **Stripe Link CLI** (recommended): `npx @stripe/link-cli auth login` once. The Link account holder approves each payment.
  - **`mppx` client SDK**: `Mppx.create({ methods: [stripe({ createToken, ... })] })` from `mppx/client`. Your own backend has to create the SPT. The bare `mppx` CLI pays Stripe challenges in test mode only.
- **A credit or debit card**, used directly or through Link. Prepaid cards are refused.

## Limits

| Rule | Value |
| --- | --- |
| Amount per top-up | $1 to $10,000 (USD) |
| Payment methods | Card or Link, through an SPT. **USDC is not available over MPP.** |
| Quote lifetime | Single-use, expires 30 minutes after `deposit_wallet` |
| 3D Secure | Not supported over MPP. Cards that need it get no `mppPayment`. |

## Steps

### 1. Request a deposit

Call MCP `deposit_wallet` with `amount` in dollars, or use REST:

```bash
curl -X POST https://rentahuman.ai/api/wallet/deposit \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $RENTAHUMAN_API_KEY" \
  -d '{"amount": 25}'
```

```json
{
  "success": true,
  "checkoutUrl": "https://checkout.stripe.com/c/pay/cs_live_...",
  "amount": 25,
  "expiresAt": "2026-10-13T18:30:00.000Z",
  "mppPayment": {
    "url": "https://rentahuman.ai/api/mpp/wallet/deposit",
    "method": "POST",
    "body": { "quote": "mtq1_..." },
    "expiresAt": "2026-10-13T18:30:00.000Z"
  }
}
```

- **Pay `checkoutUrl` or `mppPayment`, never both.** Paying both charges twice.
- **No `mppPayment`?** MPP isn't available for this deposit: the card needs 3D Secure, the account pays through a different provider, or MPP is off for the account. Give the account owner the `checkoutUrl` instead.
- **Keep the quote private.** `mppPayment.body.quote` is the credential for this payment. Send it only in the request body, and never put it in a URL, a log, or a chat message.

### 2. Pay the 402 challenge

POST `mppPayment.body` to `mppPayment.url` without changes. The first response is `402` with `WWW-Authenticate: Payment` (method `stripe`, intent `charge`, USD, realm `rentahuman.ai`). The client creates an SPT for exactly the quoted amount and retries with `Authorization: Payment`. No API key goes on this request, because the quote binds it to the account.

With the Link CLI, which handles the whole 402 exchange:

```bash
npx @stripe/link-cli mpp pay https://rentahuman.ai/api/mpp/wallet/deposit \
  -X POST \
  -d '{"quote": "mtq1_..."}' \
  --context 'Top up my RentAHuman wallet with $25 so my agent can post bounties and pay people for physical-world tasks.'
```

- `--context` must be at least 100 characters. The Link account holder reads it when approving, so state the amount and the reason.
- Don't pass `--amount`. The challenge carries the quoted amount.

### 3. Confirm the credit

| Response | Meaning |
| --- | --- |
| `200` `{"status": "credited", "paymentIntentId": "pi_...", "amountCents": 2500, "currency": "usd"}` | Wallet credited. A `Payment-Receipt` header names the PaymentIntent. |
| `202` `{"status": "pending_credit", "paymentIntentId": "pi_..."}` | Paid. The credit follows shortly. **Don't pay again.** |

Then check the balance with MCP `get_wallet_balance` (REST `GET /api/wallet/balance`), or find the deposit in `list_wallet_transactions`. Posting the same quote again after a credit returns the same `credited` result and receipt. It doesn't charge again.

## Errors

| Status / `code` | What happened | Do this |
| --- | --- | --- |
| `409 payment_not_completed`, `quote_used` | Already submitted. The card may have been declined. | Don't pay this quote again. If the card was charged, the wallet is credited automatically. Check the balance before you request a new deposit. |
| `409 quote_closed` | Closed. It may be under review, and the card may have been charged. | Don't pay again. Check the balance and contact support before you request a new deposit. |
| `410 payment_failed`, `quote_expired` | Nothing was charged. | Request a new deposit. |
| `403 card_funding_refused`, `payment_method_refused`, `risk_refused`, `use_checkout`, `top_up_restricted`, `payment_refused` | Refused before any charge. | Use the `checkoutUrl` from the same deposit. |
| `404 quote_not_found` | The quote isn't valid. | Request a new deposit. |
| `404 not_found` | MPP isn't enabled for this account. | Use the `checkoutUrl`. |
| `400 invalid_request` | The body was wrong. | Send `mppPayment.body` unchanged: `{"quote": "..."}`. |
| `429 rate_limited`, `503 temporarily_unavailable` | A temporary problem. | Wait, then retry **the same quote** before it expires. A quote can be charged only once. |

If a network failure leaves the result unclear, don't request a new deposit. Retry the same quote first: it returns the settled outcome or an error from the table above.

## Fund a bounty from the wallet

1. Write the bounty with the `bounty` skill. Call `create_bounty` with `dryRun: true` and show the preview, including `preview.fundingTotal`: worker pay plus the 18% platform fee.
2. Make sure the balance covers `fundingTotal`. If it doesn't, top up the difference with the steps above (minimum $1).
3. Post with `dryRun: false` and a stable `idempotencyKey`. The total is reserved from the wallet and the bounty goes live.
4. If the response still has a `deposit_url`, the wallet was short and the bounty is `pending_deposit`. Either give the owner that link, or top up the shortfall over MPP and then call `fund_bounty` (`dryRun: true` first, then `confirm: true` once the owner approves). `fund_bounty` spends the wallet balance before anything else. Don't pay both the `deposit_url` and an MPP top-up for the same shortfall.
5. Run the lifecycle with the `loop` skill. Money for seats that are never filled returns to the wallet.

## Refunds

Refunds of MPP top-ups go through support at support@rentahuman.ai. Include the `paymentIntentId` from the receipt. Unused bounty funds return to the wallet automatically (see [escrow docs](https://rentahuman.ai/docs/escrow)).
