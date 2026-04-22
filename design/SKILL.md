---
name: design
description: 'Minimalist product design philosophy for UI decisions, feature scoping, and visual review. Use when designing new features, reviewing UI mockups, evaluating page layouts, deciding what to build, scoping product requirements, simplifying interfaces, or applying Linear/Vercel-style design principles.'
---

# Product Design Philosophy

Principles for making product design decisions. This skill governs _what_ to build and _why_ — not implementation patterns (see `/domain-frontend` for code conventions).

## Core Doctrine

### Less, but better

Every element, feature, and interaction MUST justify its existence. The default answer to "should we add this?" is no.

- MUST ask "what can I remove?" before asking "what should I add?"
- MUST solve the problem with the fewest possible elements
- NEVER add a feature because competitors have it
- NEVER add an option when a sensible default will serve 90% of users
- SHOULD prefer removing a feature over adding a toggle to disable it

### Intentionality

Nothing is accidental. Every pixel, word, and interaction exists because someone decided it should.

- MUST have a clear reason for every visible element — if you cannot articulate why it exists, remove it
- MUST design for the primary use case first, then handle edge cases without compromising the primary path
- NEVER add placeholder content, decorative elements, or filler to "make it look complete"
- SHOULD question inherited patterns — "we've always had this" is not a reason to keep it

### Restraint over abundance

Fewer things done exceptionally well beats many things done adequately.

- MUST ship one complete feature rather than three half-finished ones
- MUST say no to scope creep during design — features expand to fill available complexity
- NEVER present more than 5-7 items in a single decision surface (navigation, dropdown, action menu)
- SHOULD hide advanced functionality behind progressive disclosure, not alongside primary actions
- SHOULD prefer depth over breadth

## Visual Design

### Clarity through reduction

- MUST use whitespace as a primary design tool — space is not "empty," it is structure
- MUST establish clear visual hierarchy: one primary action, one focal point per view
- NEVER use more than two font weights on a single surface
- MUST limit the color palette to neutrals plus one accent per view
- SHOULD use monochrome before reaching for color — add color only when it carries meaning

### Typography is the interface

- MUST use type size, weight, and spacing to create hierarchy — not color, borders, or decoration
- MUST ensure body text is comfortable to read: 16px minimum, 1.5 line height for paragraphs
- SHOULD use a single typeface with weight variations rather than multiple typefaces
- NEVER use ALL CAPS for more than short labels (2-3 words maximum)

### Density and breathing room

- MUST give interactive elements generous touch targets (44px minimum)
- MUST use consistent spacing rhythms — pick a scale and stick to it
- NEVER crowd elements to "fit more on screen" — if it doesn't fit, redesign the information architecture

### Motion with purpose

- MUST use animation only to communicate state change or spatial relationship
- NEVER add animation for decoration — it becomes noise on repeated use
- MUST keep animations under 200ms for feedback, under 400ms for transitions
- SHOULD default to no animation — add only when the interface is confusing without it
- MUST limit to one prominent animation at a time per view — competing motion destroys hierarchy
- MUST dim or de-emphasize the background when presenting modal or dialog overlays
- SHOULD use ease-out for entrances, ease-in for exits, ease-in-out for view transitions

## Product Decisions

### Before adding anything

Ask these questions in order. Stop at the first "no":

1. **Does this solve a real, observed problem?** Not hypothetical. Show the evidence.
2. **Is this the simplest solution?** Could better defaults, better copy, or removing something else solve it?
3. **Does this earn its complexity?** Every addition has maintenance cost, cognitive cost, and opportunity cost.
4. **Does this make the primary workflow better or worse?** If it doesn't improve the main path, it degrades it by adding noise.
5. **Can we ship this without settings?** If it requires configuration, the design is not opinionated enough.

### Feature scoping

- MUST define the single problem being solved before exploring solutions
- MUST write "what it does" in one sentence — if it takes a paragraph, the scope is too large
- MUST design the empty state first — it reveals whether the feature stands on its own
- NEVER design for power users first — design for new users, then layer complexity
- SHOULD prototype with real content, not lorem ipsum

### When to say no

Strong signals that a feature should not be built:

- It requires explaining to users (the UI should be self-evident)
- It adds a new settings page or section
- It serves less than 20% of users
- It duplicates something that exists in a different form
- The best argument for it is "other products have it"
- It cannot be described without the word "also"

## Design Review Process

1. **Screenshot the current state.** What is the first thing you notice? Is that the most important thing?
2. **Inventory every element.** List every visible element. For each: does it earn its place?
3. **Identify the primary action.** There MUST be exactly one. Zero means the page lacks purpose. Multiple means broken hierarchy.
4. **Check information density.** Is the user seeing what they need, or everything the system knows?
5. **Remove, then evaluate.** Mentally remove elements one at a time. If the page still works without it, it should not be there.
6. **Check alignment and rhythm.** Elements MUST align to a consistent grid. Spacing MUST follow a predictable pattern.
7. **Read all copy aloud.** If it sounds bureaucratic, robotic, or redundant — rewrite it.

## Copy and Language

- MUST use plain language — no jargon, no marketing speak, no filler
- MUST use active voice and direct address ("Save your work" not "Your work will be saved")
- MUST make button labels describe the outcome ("Create team" not "Submit")
- NEVER use "Are you sure?" — describe the consequence instead ("This will permanently delete 3 files")
- SHOULD use sentence case for UI text
- MUST keep error messages actionable: what happened and what to do next
- NEVER use exclamation marks in UI copy

## Anti-patterns

| Anti-pattern                              | Why it fails                          | Alternative                            |
| ----------------------------------------- | ------------------------------------- | -------------------------------------- |
| Dashboard with 12+ cards                  | Nothing is prioritized                | 3-4 key metrics, link to detail views  |
| Settings page with 30 options             | Complexity pushed to the user         | Opinionated defaults, remove options   |
| Multi-step wizard for simple tasks        | Implies the task is harder than it is | Single-page form with smart defaults   |
| "Learn more" links everywhere             | The UI failed to communicate          | Fix the UI                             |
| Confirmation dialogs for safe actions     | Erodes trust                          | Only confirm destructive actions       |
| Toast notifications for expected outcomes | Noise — user already knows            | Silent success, notify only on failure |
| Tabs with one item                        | Structure with no benefit             | Remove the container                   |

## Quick Reference

| Principle                | Rule                                          |
| ------------------------ | --------------------------------------------- |
| Default answer           | No — prove it should exist                    |
| Primary action per view  | Exactly one                                   |
| Maximum nav items        | 5-7                                           |
| Animation duration       | Under 200ms feedback, under 400ms transitions |
| Color usage              | Neutrals + one accent per view                |
| Font weights per surface | Maximum two                                   |
| Copy tone                | Direct, plain, actionable                     |
| Settings                 | Avoid — pick the right default                |
| Empty state              | Design it first                               |

## Reference Points

| Product   | Study for                                                                 |
| --------- | ------------------------------------------------------------------------- |
| Linear    | Information density without clutter, keyboard-first, opinionated defaults |
| Vercel    | Progressive disclosure, deployment simplicity, developer clarity          |
| Apple     | Material honesty, functional form, restraint in decoration                |
| Stripe    | Documentation clarity, consistent visual language                         |
| iA Writer | Radical simplification, typography as interface                           |

---

## Additional References

- **Visual Language**: See [references/visual-language.md](references/visual-language.md) for color philosophy, borders, shadows, icons, spacing system, dark mode, and radius conventions
- **Interaction Patterns**: See [references/interaction-patterns.md](references/interaction-patterns.md) for keyboard-first design, progressive disclosure, feedback/state, navigation, and drag-and-drop
- **Visual Polish**: See [references/visual-polish.md](references/visual-polish.md) for shadow layering, concentric radius, border techniques, and button anatomy with code examples
