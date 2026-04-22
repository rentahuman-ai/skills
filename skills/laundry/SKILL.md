---
name: laundry
description: Automate laundry end-to-end on Rent A Human — post a laundry bounty with the right brief, wait for applicants, pick the best one by location/reviews/rate, coordinate pickup and drop-off, confirm the bag came back clean, release payment. Trigger when the user says "do my laundry via rentahuman", "handle my laundry", "get someone to wash my clothes", "post a laundry bounty", "I need laundry done by <date>", "sort my laundry situation", "my laundry is piling up, can you sort it", or any request that boils down to "make the dirty laundry go away without me thinking about it again". Also trigger for adjacent asks like "dry cleaning via rah", "wash-and-fold this week", or "pick up and return my laundry by Friday". This is a concrete blueprint on top of the `loop` skill — use `loop` for the orchestration mechanics, this skill for the laundry-specific brief, criteria, and handoffs.
---

# Laundry — End-to-End Orchestration Blueprint

This is a specialization of the [`loop`](../loop/SKILL.md) skill. Loop gives you the cadence and lifecycle pattern for any bounty; this skill fills in the laundry-specific content: what the brief should say, how to screen applicants, how pickup/drop-off coordination flows, and what "done" actually looks like.

Read `loop/SKILL.md` first for the mechanics — the `/loop` invocation, pacing guidance, and stop conditions. Come back here for the domain layer.

## What the user needs to provide (ask once, up front)

Before posting anything, collect from the user:

| Field           | Example                                                 | Why it matters                            |
| --------------- | ------------------------------------------------------- | ----------------------------------------- |
| Pickup address  | Street address or neighborhood                          | Filters applicants to nearby humans       |
| Drop-off window | "by Fri 6pm" / "Sat morning"                            | Sets the deadline on the bounty           |
| Service type    | wash-and-fold / dry clean / mixed                       | Drives applicant skill filter and pricing |
| Volume          | "~1 full hamper, ~10 lbs" / "2 loads"                   | Sets a reasonable price anchor            |
| Preferences     | detergent, fragrance-free, hang-dry items, fold style   | Goes verbatim into the brief              |
| Budget ceiling  | "up to $60 incl. tip"                                   | Caps the bounty amount                    |
| Access          | "doorman has key" / "I'll be home 5–7pm for pickup"     | Avoids a coordination failure             |

If any of these are missing, ask the user before posting — a vague laundry brief attracts vague applicants and ends in a dispute.

## Posting the bounty

Use `create_bounty` with a brief shaped like this (fill in the blanks from what the user told you):

```
Title: Wash-and-fold laundry — pickup {neighborhood}, return by {deadline}

Scope:
- Pickup ~{volume} of laundry from {pickup address} on {pickup window}.
- Wash-and-fold, {detergent/fragrance preferences}, {hang-dry items if any}.
- Return folded in the same bag(s) to {drop-off address} by {deadline}.

Requirements:
- Previous wash-and-fold or household-errand experience (link to reviews).
- Own detergent or happy to use mine (ask).
- Photo of folded laundry before drop-off.

Budget: ${amount} (covers labor, detergent, laundromat/laundry fees).
Payment: escrow, released on delivery confirmation.
```

Pick `create_personal_bounty` instead if the user already has a trusted human in mind (repeat laundry helper, a favorite from past rentals).

## Screening applicants

When `get_bounty_applications` returns new apps, score each on:

1. **Proximity.** Closer is better — saves time, reduces the risk of late drop-off. If the app doesn't mention a neighborhood, ask via `send_message` on their conversation or check their `get_human` profile.
2. **Relevant reviews.** `get_reviews` — look for household tasks, errands, laundry explicitly. A 5★ developer has no signal here; a 4.7★ errand-runner does.
3. **Rate vs budget.** If they quote over budget, skip unless reviews are exceptional.
4. **Availability in the pickup window.** If they can't hit the pickup window, they're out — don't try to reschedule the user.

Surface the top 1–3 to the user unless they pre-authorized auto-accept (see below). Laundry involves a stranger entering their home — don't auto-accept silently.

### When auto-accept is OK

Only auto-accept without checking back in if the user said something like "just pick the best one and handle it" or "you don't need to check in, just run it". Default is: surface the shortlist, accept on their nod.

## Coordination during the rental

Once accepted, the `/loop` iterations handle:

- **Pre-pickup:** `send_message` confirming the pickup address, window, and access notes. Poll `get_conversation` for their reply.
- **Pickup → return:** longer sleeps (30–60 min). Nothing to do until they message or the deliverable is submitted.
- **Near deadline:** shorter sleeps (5–10 min) if they've gone quiet. If they're late, message once, then escalate (ask the user whether to extend or dispute).
- **Return:** when the bag is back, the user eyeballs it — did everything come back, is it folded, any damage?

Surface the return moment to the user explicitly: "Laundry is back. Confirm it looks good and I'll release payment." Do NOT `confirm_delivery` + `release_payment` without that confirmation — once money is released, recourse is limited.

## Exit conditions (laundry-specific additions to `loop`'s stop list)

Stop the loop and hand back to the user when:

- The bag is returned and they've confirmed it's clean + complete → `confirm_delivery`, `release_payment`, done.
- The applicant no-shows the pickup window and doesn't respond to a message within 30 min → ask user whether to cancel and repost, or extend.
- The applicant asks for scope changes (extra loads, dry cleaning addon) → surface to user, don't decide alone.
- Something arrives damaged or missing → `open_dispute`, stop the loop, handoff to user.

## Anti-patterns

- **Don't skip the pickup-address check.** The biggest laundry failure mode is an applicant who can't actually reach the pickup address in the window.
- **Don't accept the first applicant just because they applied fast.** Laundry appeals to low-signal opportunistic applicants; the good ones often apply 30–90 min in.
- **Don't release payment on photo alone.** Photos are confirmation that they *have* the laundry folded — not that it's back at the user's place. Wait for the physical return.
- **Don't re-post on a whim if a bounty goes quiet.** Duplicate open bounties for the same laundry confuse applicants and split your signal. Cancel the stale one first via the UI.
