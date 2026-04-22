---
name: workflow-subagent-dev
description: Use when executing implementation plans with independent tasks in the current session
---

# Subagent-Driven Development

Execute plans by dispatching fresh Opus sub-agents per step, with recursive decomposition for large steps and two-stage review (spec compliance + code quality) at every depth level.

**Why subagents:** You delegate steps to specialized agents with isolated context. By precisely crafting their instructions and context, you ensure they stay focused and succeed. They never inherit your session's context — you construct exactly what they need. This preserves your own context for coordination.

**Core principle:** Fresh sub-agent per step + recursive decomposition + two-stage review at every level = high quality, scalable execution

## Process Overview

```
┌────────────────────────────────────────────────────────────────────┐
│                    EXECUTION ORCHESTRATOR                          │
│                                                                    │
│  1. Read overall plan + all step detail files                      │
│  2. For each step (respecting execution order):                    │
│     a. Dispatch executor sub-agent with overall plan + step detail │
│     b. Executor assesses scope: direct execution OR decompose      │
│     c. If decomposing: executor creates sub-plan, dispatches       │
│        sub-agents (same pattern recursively)                       │
│     d. Spec review + quality review at every depth level           │
│     e. Mark step complete                                          │
│  3. After all steps: invoke workflow-finish                        │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

## The Recursive Execution Pattern

Each executor sub-agent at any depth follows this pattern:

```
┌─────────────────────────────────────────────────────────┐
│  EXECUTOR SUB-AGENT (at any level)                      │
│                                                         │
│  1. Receive: overall plan + step/sub-step detail        │
│  2. Read relevant skills for this work                  │
│  3. Assess: can I do this directly, or should I         │
│     decompose into sub-steps?                           │
│                                                         │
│     ┌──────────────┐         ┌──────────────────┐       │
│     │ DIRECT        │         │ DECOMPOSE         │       │
│     │ (small scope) │         │ (large scope)     │       │
│     │               │         │                    │       │
│     │ implement     │         │ create sub-plan    │       │
│     │ test          │         │ dispatch sub-agents│       │
│     │ commit        │         │ each sub-agent     │       │
│     │ self-review   │         │ follows this same  │       │
│     └──────┬───────┘         │ pattern recursively│       │
│            │                  └────────┬───────────┘       │
│            ▼                           ▼                  │
│     spec review              sub-step results             │
│     quality review           spec review per sub-step     │
│                              quality review per sub-step  │
└─────────────────────────────────────────────────────────┘
```

### Decomposition Decision

The **plan suggests** decomposition hints (e.g., "this step covers 50 files, consider per-package sub-steps"), but the **executor confirms and adjusts** at runtime based on what it actually finds.

| Signal               | Direct execution    | Decompose               |
| -------------------- | ------------------- | ----------------------- |
| Files affected       | < ~10 files         | 10+ files               |
| Conceptual scope     | Single concern      | Multiple concerns       |
| Package count        | 1 package           | Multiple packages       |
| Estimated complexity | Can hold in context | Too large for one agent |
| Plan hint            | "straightforward"   | "consider sub-steps"    |

These are guidelines, not rules — the sub-agent uses judgment. There is no hard depth limit — sub-agents decide if further decomposition is warranted.

### Decomposition Example

```
Orchestrator
  │
  │  Step 1: "Audit all Effect-TS packages"
  │  Plan hint: decompose per-package
  │
  ▼
Step 1 Executor (Opus sub-agent)
  │
  │  Reads: domain-effect skill, overall plan, step-1 detail
  │  Decides: 8 packages → decompose per-package
  │
  ├──▶ Sub-agent: audit packages/platform/auth/
  │    spec review ✓  quality review ✓
  │
  ├──▶ Sub-agent: audit packages/platform/server/
  │    spec review ✓  quality review ✓
  │
  ├──▶ Sub-agent: audit packages/clients/redis/
  │    Finds 40 files → decomposes further:
  │    ├──▶ Sub-sub-agent: audit src/Modules/Cache/
  │    ├──▶ Sub-sub-agent: audit src/Modules/Queue/
  │    └──▶ Sub-sub-agent: audit src/Modules/PubSub/
  │    each: spec review ✓  quality review ✓
  │
  └──▶ ... (parallel where independent)
```

## Step-by-Step Orchestration

### Step 1: Read Plan and Set Up Tracking

1. Read the overall plan from `docs/plans/YYYY-MM-DD-<name>.md`
2. Read all step detail files from `.context/plans/step-N-<name>.md`
3. Extract execution order and dependencies
4. **Read the `## Topology` section** to determine mode (single-PR vs multi-PR)
5. Create task tracking for all steps

### Step 1b: Topology Setup

**Single-PR mode (default):** Continue on the current feature branch. Each step commits to it. After all steps, hand off to `workflow-finish` for one PR.

**Multi-PR mode:** Set up the branch topology before dispatching any executors.

```bash
# 1. Create base branch off main and open a draft base PR
git checkout -b <base-branch> main
git push -u origin <base-branch>
gh pr create --base main --head <base-branch> --draft \
  --title "<feature-name> (base)" \
  --body "Base PR. Sub-PRs target this branch. See docs/plans/<plan-file>.md."

# 2. For each sub-PR group declared in the plan, create a sub-branch + worktree
for sub in <part-a> <part-b> <part-c>; do
  git worktree add ../<repo>-<sub> -b <base-branch>-<sub> <base-branch>
done
```

Then dispatch one executor per sub-branch (parallel where the land order permits independence). Each executor operates inside its own worktree on its own sub-branch — never swap branches inside a live sub-agent's worktree (the file map drifts).

When dispatching multi-PR executors, the prompt must include the full plan's `## Topology` section so each executor knows which branch it owns, the shared contracts it must honor, and the land order.

After a sub-PR's executor completes (and review passes), hand that sub-PR off to `workflow-finish` with `--base <base-branch>` (not `main`). When all sub-PRs have merged into the base branch, the orchestrator invokes `workflow-finish` one more time on the base PR itself, which merges to `main`.

### Step 2: Execute Steps (respecting order)

For each step, dispatch an executor sub-agent.

**Executor sub-agent prompt:**

> You are executing Step N of an implementation plan.
>
> ## Overall Plan
>
> [OVERALL_PLAN_CONTENT — so you understand where your step fits]
>
> ## Your Step Detail
>
> [FULL_CONTENT_OF_STEP_DETAIL_FILE]
>
> ## Relevant Skills
>
> Read these skills before starting: [LIST_OF_SKILL_NAMES]
>
> ## Recursive Execution
>
> Assess the scope of your step:
>
> **Direct execution** (small scope — fewer than ~10 files, single concern):
>
> 1. Implement exactly what the step detail specifies
> 2. Write tests (following TDD if step says to)
> 3. Verify implementation works
> 4. Commit your work
> 5. Self-review
> 6. Report back
>
> **Decompose** (large scope — many files, multiple concerns, multiple packages):
>
> 1. Create a sub-plan breaking your step into sub-steps
> 2. Dispatch sub-agents for each sub-step (parallel if independent)
> 3. Each sub-agent follows this same pattern (may decompose further)
> 4. Ensure spec review + quality review after each sub-step
> 5. Report aggregate results
>
> The step detail may include a "Decomposition hint" — use it as guidance but confirm based on what you actually find in the codebase.
>
> ## Implementer Instructions
>
> [CONTENT_OF_IMPLEMENTER_PROMPT_MD]
>
> ## Report Format
>
> - **Status:** DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
> - What you implemented (or decomposed into)
> - What you tested and test results
> - Files changed
> - Self-review findings (if any)
> - Any issues or concerns

**Parallel execution:** Independent steps (no mutual dependencies) can have their executor sub-agents dispatched simultaneously. Never have two agents edit the same file.

**Sequential execution:** Dependent steps run one at a time in dependency order.

### Step 3: Two-Stage Review (at every depth level)

After each executor sub-agent (at any depth) completes:

```
Sub-agent completes work
      │
      ▼
Spec Reviewer: "Did this match what was asked?"
      │
      ├── issues? → sub-agent fixes → re-review
      │
      ▼
Quality Reviewer: "Clean code? Tests pass? Maintainable?"
      │
      ├── issues? → sub-agent fixes → re-review
      │
      ▼
Mark complete ✓
```

Use the existing prompt templates:

- `./spec-reviewer-prompt.md` — Spec compliance review
- `./code-quality-reviewer-prompt.md` — Code quality review

**Review is mandatory at every depth level.** A sub-sub-agent that directly implements code gets spec + quality review. This ensures quality does not degrade as decomposition gets deeper.

### Step 4: Hand Off to Finish

**Single-PR mode:** After all steps are complete and reviewed, invoke `workflow-finish` once to create the PR and babysit it through review.

**Multi-PR mode:** Hand off to `workflow-finish` per sub-PR (in land order) with `--base <base-branch>`. After each sub-PR merges into the base branch, restack any open downstream sub-branches before continuing (`workflow-finish` handles the cascade — see its "Multi-PR mode" section). When every sub-PR has landed, invoke `workflow-finish` one final time on the base PR to undraft it and merge to `main`.

## Handling Executor Status

**DONE:** Proceed to spec compliance review.

**DONE_WITH_CONCERNS:** Read the concerns before proceeding. If about correctness or scope, address before review. If observations, note them and proceed.

**NEEDS_CONTEXT:** Provide the missing context and re-dispatch.

**BLOCKED:** Assess the blocker:

1. Context problem → provide more context, re-dispatch
2. Needs more reasoning → re-dispatch with more capable model
3. Task too large → tell executor to decompose
4. Plan is wrong → escalate to the user

**Never** ignore an escalation or force retry without changes.

## Prompt Templates

- `./implementer-prompt.md` — Implementer instructions (included in executor prompt)
- `./spec-reviewer-prompt.md` — Spec compliance reviewer
- `./code-quality-reviewer-prompt.md` — Code quality reviewer

## Red Flags

**Never:**

- Start implementation on main/master branch without explicit user consent
- Skip reviews (spec compliance OR code quality) at any depth level
- Proceed with unfixed issues
- Have two agents edit the same file simultaneously
- Skip review loops (reviewer found issues = implementer fixes = review again)
- Let self-review replace actual review (both are needed)
- Start code quality review before spec compliance is approved

**If reviewer finds issues:**

- Implementer (same sub-agent) fixes them
- Reviewer reviews again
- Repeat until approved

**If sub-agent asks questions:**

- Answer clearly and completely
- Provide additional context if needed

## Integration

**Required workflow skills:**

- **workflow-plan** — Creates the plan this skill executes
- **workflow-finish** — Complete development after all steps (PR + babysit)

**Sub-agents should use:**

- **practice-tdd** — Follow TDD for each task
- Domain skills as specified by the plan (domain-effect, domain-database, etc.)
