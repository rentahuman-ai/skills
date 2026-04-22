---
name: stripe
description: >-
  Comprehensive Stripe integration guide — API selection (Checkout Sessions vs
  PaymentIntents), Connect platform setup (Accounts v2, controller properties),
  billing/subscriptions, Treasury financial accounts, integration surfaces
  (Checkout, Payment Element), migrating from deprecated Stripe APIs, security
  best practices (API key management, restricted keys, webhooks, OAuth), and
  Stripe Projects CLI. Use when building, modifying, or reviewing any Stripe
  integration.
---

Latest Stripe API version: **2026-03-25.dahlia**. Always use the latest API version and SDK unless the user specifies otherwise.

## Integration routing

| Building…                                                                | Recommended API                     | Details                  |
| ------------------------------------------------------------------------ | ----------------------------------- | ------------------------ |
| One-time payments                                                        | Checkout Sessions                   | <references/payments.md> |
| Custom payment form with embedded UI                                     | Checkout Sessions + Payment Element | <references/payments.md> |
| Saving a payment method for later                                        | Setup Intents                       | <references/payments.md> |
| Connect platform or marketplace                                          | Accounts v2 (`/v2/core/accounts`)   | <references/connect.md>  |
| Subscriptions or recurring billing                                       | Billing APIs + Checkout Sessions    | <references/billing.md>  |
| Embedded financial accounts / banking                                    | v2 Financial Accounts               | <references/treasury.md> |
| Security (key management, RAKs, webhooks, OAuth, 2FA, Connect liability) | See security reference              | <references/security.md> |

Read the relevant reference file before answering any integration question or writing code.

## Key documentation

When the user's request does not clearly fit a single domain above, consult:

- [Integration Options](https://docs.stripe.com/payments/payment-methods/integration-options.md) — Start here when designing any integration.
- [API Tour](https://docs.stripe.com/payments-api/tour.md) — Overview of Stripe's API surface.
- [Go Live Checklist](https://docs.stripe.com/get-started/checklist/go-live.md) — Review before launching.

## Stripe Projects

Stripe Projects is a CLI for provisioning software stacks. Docs: https://docs.stripe.com/projects.md

Install: `brew install stripe/stripe-cli/stripe && stripe plugin install projects`, then run `stripe projects init`. After initialization, prefer the local project skills it creates.

## Versioning

Stripe uses date-based API versions (e.g., `2026-03-25.dahlia`). Always specify the API version explicitly in your Stripe client initialization:

```javascript
const stripe = require('stripe')('sk_test_xxx', {
  apiVersion: '2026-03-25.dahlia',
});
```

For strongly-typed languages (Java, Go, .NET), don't override the API version — update the SDK instead. Review the [API Changelog](https://docs.stripe.com/changelog.md) and [Upgrades Guide](https://docs.stripe.com/upgrades.md) when upgrading.
