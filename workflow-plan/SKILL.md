---
name: workflow-plan
description: 'Create high-quality implementation plans using a central agent that orchestrates sub-agents for per-step planning, then composes and critic-reviews an overall plan. Use when you have requirements for a multi-step task and need a plan before writing code.'
---

# Workflow: Plan

Hierarchical planning using a central agent that orchestrates sub-agents. The central agent reads context, determines steps, dispatches sub-agents to write detailed per-step plans, composes an overall plan, runs a lightweight critic pass, and finalizes. The result is a plan with detailed step files that executor sub-agents can work from independently.

For plan format and quality conventions, see `references/writing-plans.md`.
For the critic prompt template, see `references/critic-prompt.md`.

## Process Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                     CENTRAL PLAN AGENT                           │
│                                                                  │
│  Phase 1: Gather Context                                         │
│    Read brainstorm + skills + CLAUDE.md + source files            │
│                                                                  │
│  Phase 2: Determine Steps                                        │
│    Break work into steps, decide ordering (parallel/sequential)  │
│                                                                  │
│  Phase 3: Dispatch Step Planners                                 │
│    Sub-agent per step → .context/plans/step-N-<name>.md          │
│                                                                  │
│  Phase 4: Compose Overall Plan                                   │
│    Read all step files, compose overview + execution order        │
│                                                                  │
│  Phase 5: Critic Pass                                            │
│    Single critic sub-agent checks cross-step coherence            │
│                                                                  │
│  Phase 6: Finalize and Save                                      │
│    Address critic feedback, save to docs/plans/                  │
└──────────────────────────────────────────────────────────────────┘
```

## Modes

**Warm handoff** (from `workflow-brainstorm`): Read `.context/plans/brainstorm.md` as the primary input. Skip Phase 1 requirements clarification — the brainstorm already covers it.

**Cold start** (no prior brainstorm): Run Phase 1 (Requirements Clarification) first to establish context. The output is saved to `.context/plans/brainstorm.md` — the same handoff artifact used by the warm path — so that all downstream phases work identically regardless of entry point.

## Phase 1: Requirements Clarification (cold start only)

Do NOT spawn a sub-agent for this phase. The main agent talks to the user directly.

### Step 1: Explore

Read the relevant codebase. Identify:

- Existing patterns and conventions in affected packages
- Files that will likely need to change
- Related features and how they work
- Constraints from the architecture

### Step 2: Ask Questions

Ask clarifying questions. Batch unrelated questions together (3-5 per batch). Focus on:

- **Scope** — what exactly needs to happen? What is explicitly out of scope?
- **Constraints** — what must it integrate with? What cannot change?
- **Acceptance criteria** — how do we know it is done?
- **Edge cases** — what happens when things go wrong?

Keep going until you can describe the complete solution without guessing.

### Step 3: Produce Requirements Summary

Write a structured requirements summary to `.context/plans/brainstorm.md` (same location as the brainstorm handoff):

```markdown
# Requirements: [Feature Name]

**Goal:** [One sentence]
**Packages affected:** [List]

## Functional Requirements

1. [Specific, testable requirement]
2. ...

## Non-Functional Requirements

- [Performance, security, compatibility constraints]

## Out of Scope

- [Explicitly excluded items]

## Existing Patterns

- [Relevant conventions discovered during exploration]

## Key Decisions

- [Any decisions made during clarification]
```

Show this to the user and get confirmation before proceeding.

## Phase 2: Gather Context and Determine Steps

Read all relevant context:

- `.context/plans/brainstorm.md` (approved design or requirements)
- Relevant skill files (domain-effect, domain-database, domain-frontend, etc.)
- CLAUDE.md conventions
- Key source files identified in the brainstorm

Break the work into steps. **Granularity is your call** — decide based on complexity:

- Simple work → fewer, larger steps
- Complex work → more, smaller steps
- Each step should be a coherent unit one sub-agent can execute

Determine **ordering and parallelization** per step:

- Independent steps can be planned in parallel (sub-agents dispatched together)
- Dependent steps are planned sequentially (later sub-agents can reference earlier step plans)

### PR Topology Decision

Once steps and dependencies are known, decide how the work will ship.

**Default — single PR, many commits.** All steps land on one feature branch as separate commits. Use this unless ALL escalation criteria below are met.

**Escalation — multi-PR topology.** A draft base PR targets `main`; each sub-PR targets the base branch. The base PR merges to `main` only after every sub-PR has landed into the base branch. Use only when ALL hold:

- Two or more steps are marked `Parallel: yes` with **disjoint file sets** (no overlap in files modified)
- Total scope is large enough that a single PR would be unreviewable in one sitting
- The user opted in (or the brainstorm flagged the work as needing parallelization)

```
main
└── <base-branch>                    ← base PR, draft, --base main
    ├── <base-branch>-<part-a>        ← sub-PR, --base <base-branch>
    ├── <base-branch>-<part-b>        ← sub-PR, --base <base-branch>
    └── <base-branch>-<part-c>        ← sub-PR, --base <base-branch>
```

When choosing multi-PR mode, the plan must additionally declare:

- **Branch names** — `<base-branch>` and one sub-branch name per sub-PR group
- **Sub-PR grouping** — which steps belong to which sub-PR (one or more steps per sub-PR)
- **Land order** — dependency order in which sub-PRs must merge into the base (e.g. schema → api → ui)
- **Shared contracts** — interfaces / type signatures / file paths each sub-PR depends on from the others

These are recorded in the overall plan under a `## Topology` section (see `references/writing-plans.md`).

If single-PR mode is chosen, no topology section is required.

## Phase 3: Dispatch Step Planner Sub-Agents

For each step, dispatch a `general-purpose` sub-agent (Opus).

```
┌─────────────────────────────────────────────┐
│  STEP PLANNER SUB-AGENT                     │
│                                             │
│  Receives:                                  │
│  • Brainstorm context                       │
│  • Step description + goals                 │
│  • Relevant source files for this step      │
│  • Relevant skill files for this step       │
│  • Writing-plans format reference           │
│  • Dependencies on other steps (if any)     │
│                                             │
│  Produces:                                  │
│  • .context/plans/step-N-<name>.md          │
│  • Exact file paths, full code blocks       │
│  • Test commands + expected output           │
│  • Commit messages                          │
│  • Decomposition hints for executor          │
│                                             │
└─────────────────────────────────────────────┘
```

**Step planner prompt:**

> You are writing a detailed implementation plan for one step of a larger project.
>
> **Step:** [STEP_NUMBER] — [STEP_NAME]
> **Goal:** [STEP_GOAL]
> **Dependencies:** [DEPENDS_ON or "none"]
>
> **Brainstorm context:**
> [BRAINSTORM_CONTENT]
>
> **Relevant source files:**
> [LIST_OF_FILES_TO_READ]
>
> **Relevant skills to read:**
> [LIST_OF_SKILL_NAMES — e.g., domain-effect, domain-database]
>
> **Writing format reference:**
> [CONTENT_OF_WRITING_PLANS_MD]
>
> Write a detailed step plan following the "Step Detail File Format" from the writing reference. Save it to `.context/plans/step-[N]-[name].md`.
>
> Requirements:
>
> - Every file path must be exact and verified by reading the codebase
> - Complete code in all implementation sections, not descriptions of code
> - Include verification commands with expected output
> - Include commit messages
> - Include a decomposition hint for the executor sub-agent
> - Read the relevant domain skills listed above before writing the plan
> - Follow all CLAUDE.md conventions

**Parallel dispatch** (independent steps):

```
Central agent dispatches simultaneously:
  ├──▶ Sub-agent → step-1-schema.md
  ├──▶ Sub-agent → step-2-config.md
  └──▶ Sub-agent → step-3-types.md
```

**Sequential dispatch** (dependent steps):

```
Central agent dispatches:
  Sub-agent → step-1-schema.md
       │ completes
       ▼
  Sub-agent → step-2-service.md (reads step-1 output)
       │ completes
       ▼
  Sub-agent → step-3-api.md (reads step-1 + step-2 output)
```

**Mixed** (central agent decides):

```
  ├──▶ Sub-agent → step-1-schema.md    ┐
  └──▶ Sub-agent → step-2-config.md    ┘ parallel (independent)
            │ both complete
            ▼
       Sub-agent → step-3-service.md     sequential (depends on 1+2)
```

## Phase 4: Compose Overall Plan

Read all `step-N-*.md` files from `.context/plans/` and compose the overall plan following the "Overall Plan Format" from `references/writing-plans.md`.

The overall plan includes:

- Goal (one sentence)
- Architecture (2-3 sentences)
- Overview (how steps fit together)
- Packages & Files Affected (grouped by package — every file across all steps)
- Execution Order (dependency graph)
- Step summaries with `**Detail:**` references to the step files

## Phase 5: Lightweight Critic Pass

Dispatch a single `general-purpose` critic sub-agent using the prompt template from `references/critic-prompt.md`.

Pass it:

- The composed overall plan
- All step detail file paths
- The brainstorm context

```
┌─────────────────────────────────────────────┐
│  CRITIC SUB-AGENT                           │
│                                             │
│  Checks for:                                │
│  • Conflicts between steps                  │
│  • Missing dependencies                     │
│  • Ordering problems                        │
│  • Gaps in coverage vs brainstorm           │
│  • Convention violations                    │
│                                             │
│  Focus: cross-step coherence                │
│  NOT a full adversarial review              │
│                                             │
└─────────────────────────────────────────────┘
```

If the critic finds issues:

1. Address each issue in the overall plan and/or relevant step detail files
2. If issues are serious (dependency errors, missing requirements), re-dispatch affected step planners
3. If issues are minor (ordering tweaks, convention fixes), fix directly

If the critic loop exceeds **3 iterations**, surface remaining issues to the user for guidance.

## Phase 6: Finalize and Save

Present the final plan to the user:

```markdown
## Plan: [Feature Name]

### Planning Summary

- **Steps:** <count>
- **Parallel groups:** <count> groups of independent steps
- **Critic issues found:** <count> (all resolved)

### Final Plan

[Clean overall plan]
```

If approved:

- Save overall plan to `docs/plans/YYYY-MM-DD-<feature-name>.md`
- Commit with message `docs: add <feature-name> implementation plan`
- Step detail files stay in `.context/plans/` for execution (gitignored)

If the user wants changes, revise directly (no need to re-run agents for small adjustments).

### Execution Handoff

After saving the plan, hand off to execution:

**"Plan complete and saved. Invoking `workflow-subagent-dev` for execution."**

Invoke `workflow-subagent-dev` to execute the plan. This is the only execution path — there is no alternative.

## Agent Configuration

Step planner sub-agents use `general-purpose` (Opus) with full codebase access. They should also read relevant domain skills (domain-effect, domain-database, domain-frontend, etc.) to produce better plans.

The critic sub-agent uses `general-purpose` (Opus) with full codebase access.

**Parallelization rules:**

- Independent step planners CAN run in parallel
- Dependent step planners MUST run sequentially
- The critic runs after ALL step planners complete
- Phase sequence is strict: gather context → determine steps → dispatch planners → compose → critic → finalize

## Tone

Frame critic feedback constructively. The goal is a coherent plan, not a debate. When presenting results, highlight any cross-step issues the critic caught.
