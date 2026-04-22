---
name: brainstorm
description: 'You MUST use this before any creative work - creating features, building components, adding functionality, or modifying behavior. Explores user intent, requirements and design before implementation.'
---

# Brainstorming Ideas Into Designs

Collaborative ideation using plan mode as the medium. Explore the problem, ask thoughtful questions, present solutions with architectural context and ASCII art, iterate with the user until the idea is solid, then hand off to `workflow-plan`.

<HARD-GATE>
Do NOT invoke any implementation skill, write any code, scaffold any project, or take any implementation action until the brainstorm is complete and the user has approved. This applies to EVERY project regardless of perceived simplicity. This is for pure ideation.
</HARD-GATE>

## Anti-Pattern: "This Is Too Simple To Need A Design"

Every project goes through this process. A todo list, a single-function utility, a config change — all of them. "Simple" projects are where unexamined assumptions cause the most wasted work. The design can be short (a few sentences for truly simple projects), but you MUST present it and get approval.

## Process

```
┌──────────────────────────────────────────────────┐
│              workflow-brainstorm                 │
│                                                  │
│  1. Enter plan mode (if not in it already)       │
│  2. Explore context (codebase, patterns)         │
│  3. Ask clarifying questions (batched)           │
│  4. Write brainstorm to plan file                │
│  5. ExitPlanMode → user reviews                  │
│     ↕ automatic loop: user feedback re-engages   │
│     plan mode, update file, ExitPlanMode again   │
│  6. Save to .context/plans/brainstorm.md         │
│  7. Hand off to workflow-plan                    │
└──────────────────────────────────────────────────┘
```

**The terminal state is invoking workflow-plan.** Do NOT invoke any implementation or domain skill directly. The ONLY skill you invoke after brainstorming is workflow-plan.

## Step 1: Explore Context

Read the relevant codebase before asking anything. Identify:

- Existing patterns and conventions in affected packages
- Files that will likely need to change
- Related features and how they work
- Constraints from the architecture

Form a mental model of the current state. Use Explore subagents for broader searches.

## Step 2: Ask Clarifying Questions

Interview the user about every aspect of the design until you reach shared understanding. Walk down each branch of the decision tree, resolving dependencies between decisions. **Batch unrelated questions together** — if answering question A doesn't change what you'd ask for question B, ask them in the same message. Only serialize questions where one answer shapes the next.

**For each question, provide your recommended answer conversationally** — e.g., "I'd lean toward X here because Y — sound right?" This gives the user something concrete to react to instead of answering from scratch. Even when uncertain, give your best take and explain why.

**Keep probing across multiple rounds.** Don't stop after one batch. When answers open new branches in the decision tree, follow them in the next round. The signal to stop is when all design decisions are resolved and you have shared understanding — not a question count.

Rules:

- 3-5 questions max per batch (avoid survey fatigue)
- Prefer multiple choice when the options are knowable
- Open-ended is fine when the space is genuinely open
- Ask thoughtful, specific questions — not generic checklists
- Focus on: purpose, constraints, scope, edge cases, success criteria, design

**Scope check first:** If the request describes multiple independent subsystems, flag this immediately. Help decompose into sub-projects before diving into details. Each sub-project gets its own brainstorm → plan → implementation cycle.

## Step 3: Write the Brainstorm to Plan File

Once you understand the problem, write your proposed solution(s) to the plan file. This is the core artifact of brainstorming.

### Plan File Format

```markdown
# [Feature/Change Name]

## What we're building

One paragraph — the problem, what we're solving, and why.

## Architectural context

How this fits into the existing system. Where in the stack
this lives. What packages and layers are involved.

┌──────────┐ ┌──────────┐ ┌──────────┐
│ Package A │────▶│ Package B │────▶│ Package C │
└──────────┘ └──────────┘ └──────────┘
│ ▲
└─────────────────────────────────┘

(Scale this section to complexity — skip for trivial changes,
expand with diagrams for complex ones. Adjust for the scope of the task)

## Approach A: [Name]

Core idea in a few sentences.

┌─────────────┐
│ Component X │
│ ┌────────┐ │
│ │ Part 1 │──┼──▶ External
│ └────────┘ │
│ ┌────────┐ │
│ │ Part 2 │ │
│ └────────┘ │
└─────────────┘

**Key files:**

- Create: `path/to/new-thing.ts` — purpose
- Modify: `path/to/existing.ts` — what changes

**Trade-offs:** [pros and cons]

## Approach B: [Name]

(Only if there's a genuine tradeoff worth discussing.
Skip if one approach is clearly right.)

...

## Recommendation

Which approach and why. If only one approach was presented,
this section summarizes the key design decisions.
```

### When to Present Multiple Approaches

**Multiple approaches** when:

- There are genuine architectural tradeoffs (performance vs. simplicity, etc.)
- The user needs to make a design decision (layout, UX flow, data model)
- Different approaches serve different priorities

**Single approach** when:

- There's an established pattern to follow
- The solution is constrained enough that alternatives are contrived
- One approach is clearly superior on all dimensions

Be dynamic — don't force 2-3 approaches when one is obvious, don't collapse to one when there's a real choice.

### ASCII Diagrams

Use box-drawing characters for architectural and flow diagrams. These render well in plan mode and give the user a quick visual understanding of how pieces connect.

**Component relationships:**

```
┌─────────┐     ┌─────────┐
│ Service  │────▶│  Store  │
└─────────┘     └─────────┘
      │
      ▼
┌─────────┐
│   API   │
└─────────┘
```

**Data flow:**

```
User Input ──▶ Validation ──▶ API Call ──▶ Store Update ──▶ UI Re-render
```

**Component hierarchy:**

```
Page
├── Header
├── Content
│   ├── Sidebar
│   └── Main
│       ├── Section A
│       └── Section B
└── Footer
```

Scale diagram complexity to the problem. Simple changes might need just a tree. Complex features might need multiple diagrams showing different aspects.

### Architectural Context Depth

**Trivial changes** (rename, small fix, config): skip architectural context or keep to one sentence.

**Moderate changes** (new component, new hook, modify existing feature): show where it fits in the component/package hierarchy and what it interacts with.

**Complex changes** (new feature, new package, cross-cutting concern): full architectural overview — how it fits into the overall system, which packages/layers are involved, data flow from user action to storage and back.

## Step 4: ExitPlanMode

Present the brainstorm to the user via ExitPlanMode. The user reads through the plan file — the architectural context, the approach(es), the key files — and gives feedback.

## Step 5: Iterate

When the user gives feedback, plan mode re-engages automatically. Update the plan file based on their input:

- Adjust the approach
- Add/remove considerations
- Refine scope
- Explore alternatives they suggest

Then ExitPlanMode again. Repeat until the user is satisfied with the design.

## Step 6: Save Brainstorm Artifact

Once the user approves the brainstorm, save the final brainstorm content to `.context/plans/brainstorm.md`. This file is the handoff artifact — it becomes the primary input to the plan agent.

```bash
mkdir -p .context/plans
# Write the approved brainstorm content to the handoff file
```

This file lives in `.context/plans/` (gitignored) and is consumed by `workflow-plan`.

## Step 7: Hand Off to Implementation

Invoke `workflow-plan` to create a detailed implementation plan. The brainstorm artifact (`.context/plans/brainstorm.md`) is the input for the planning phase.

Do NOT invoke any other skill. `workflow-plan` is the next step.

## Design Principles

**Design for isolation and clarity:**

- Break the system into smaller units with one clear purpose each
- Each unit communicates through well-defined interfaces
- Can someone understand what a unit does without reading its internals?
- Can you change internals without breaking consumers?

**Work with the existing codebase:**

- Explore current structure before proposing changes — follow existing patterns
- Where existing code has problems that affect the work, include targeted improvements
- Don't propose unrelated refactoring — stay focused on what serves the current goal

**YAGNI ruthlessly:**

- Remove unnecessary features from all designs
- Build what's needed, not what might be needed
- Simple > clever

## Key Principles

- **Batch unrelated questions** — don't make the user wait through serial Q&A for independent questions
- **Dynamic solution count** — multiple approaches when there's a real choice, single when obvious
- **ASCII diagrams for understanding** — visual structure helps more than paragraphs of description
- **Scale context to complexity** — minimal for trivial, full architectural picture for complex
- **Plan mode is the medium** — the plan file is the artifact, ExitPlanMode presents it, iteration is automatic
- **Be flexible** — go back and clarify when something doesn't make sense
