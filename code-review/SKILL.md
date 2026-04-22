---
name: code-review
description: 'Adversarial multi-agent code review with domain-aware conventions. Use when reviewing code changes, analyzing diffs, giving PR feedback, checking for bugs, or evaluating code quality. Automatically identifies relevant domain skills (Effect-TS, frontend, database, etc.) and incorporates their conventions.'
---

# Code Review

Adversarial review using three agents with opposing incentives. Exploits sycophancy bias — each agent is scored so it tries hard in its designated direction, producing higher-fidelity results than a single-pass review.

- **Issue Finder** — rewarded for finding issues → produces the superset of all possible issues
- **Adversary** — rewarded for disproving issues, penalized for wrong disprovals → filters to the subset of real issues
- **Referee** — told the human has ground truth, scored on accuracy → makes the final call

## Review Modes

**PR review** (default): reviews a PR diff or local diff against main. See `references/pr-review.md`.

**Codebase review**: reviews specific files or modules without a diff. See `references/codebase-review.md`.

## Workflow

### Phase 1: Gather Context

Determine the review mode and collect the code to review.

For PR review (default):

```bash
# If PR number provided
gh pr view <number> --json title,body,additions,deletions,files
gh pr diff <number>

# If no PR number — local diff
git diff main...HEAD --stat
git log main..HEAD --oneline
git diff main...HEAD
```

For codebase review: read the specified files/modules.

For large diffs (20+ files), spawn an `Explore` agent first to group changes by module and summarize what changed.

### Phase 1.5: Identify Domain Context

Look at which files and packages are touched, then identify which `domain-*` skills under `.agents/skills/` are relevant (e.g., `domain-effect` for Effect-TS backend code, `domain-frontend` for React components, `domain-database` for schema/query changes). Read the SKILL.md for each relevant domain (cap at 3 most relevant). These domain conventions are passed to all three agents alongside CLAUDE.md as additional context for evaluating correctness.

### Phase 2: Issue Finder

Spawn a `general-purpose` sub-agent with `model: "opus"`. Pass it the full diff or file contents, the project's CLAUDE.md conventions, and domain conventions from Phase 1.5.

**Prompt:**

> You are reviewing code for issues. Your goal is to maximize your score.
>
> **Scoring:** +1 for each low-impact issue found, +5 for each medium-impact issue, +10 for each critical issue.
>
> Examine every line carefully. Look for: bugs, logic errors, security vulnerabilities, performance problems, convention violations (CLAUDE.md and domain skill conventions), domain-specific anti-patterns, missing edge cases, type safety gaps, race conditions, error handling gaps, incorrect assumptions.
>
> For each issue, report in this exact format:
>
> ```
> ### F-<number>: <short title>
> - **File:** <path>:<line>
> - **Severity:** low (+1) | medium (+5) | critical (+10)
> - **Category:** bug | security | performance | convention | edge-case | type-safety | logic | race-condition
> - **Description:** <what the issue is>
> - **Evidence:** <the specific code and why it's problematic>
> ```
>
> After all findings, report: **Total score: <sum>**
>
> Be thorough — you are rewarded for finding real issues. Read surrounding code for context, not just the changed lines.

### Phase 3: Adversary

Wait for the Issue Finder to complete. Spawn a `general-purpose` sub-agent with `model: "opus"`. Pass it the full diff/files, the same domain conventions from Phase 1.5, AND the Issue Finder's complete output.

**Prompt:**

> You are an adversarial reviewer. Your goal is to disprove as many of the following findings as possible.
>
> **Scoring:** You earn the finding's point value for each one you successfully disprove. But if you incorrectly disprove a real issue, you lose **2x** that finding's point value. Choose your battles carefully.
>
> For each finding below, you MUST either:
>
> - **DISPROVE** — provide concrete evidence why this is not actually an issue. Cite specific code paths, control flow, type guarantees, framework behavior, or documentation that invalidates the concern.
> - **CONCEDE** — acknowledge the issue is real and briefly explain why you could not disprove it.
>
> **Rules:**
>
> - You must read the actual source code before making any determination. Do not guess.
> - A disproval requires evidence, not just disagreement. "This seems fine" is not a disproval.
> - If you're unsure, concede. The 2x penalty makes guessing expensive.
>
> Report in this format for each finding:
>
> ```
> ### F-<number>: <original title>
> - **Verdict:** DISPROVE | CONCEDE
> - **Argument:** <your evidence or concession>
> ```
>
> After all findings, report: **Total score: <sum>**

### Phase 4: Referee

Wait for the Adversary to complete. Spawn a `general-purpose` sub-agent with `model: "opus"`. Pass it the diff/files, domain conventions from Phase 1.5, the Issue Finder's findings, AND the Adversary's responses.

**Prompt:**

> You are the referee. The human has the actual correct answer for each finding and will compare your judgments against ground truth. You will be scored: **+1** for each correct judgment, **-1** for each incorrect judgment.
>
> For each finding, you have two arguments:
>
> - The Issue Finder's case (why it IS an issue)
> - The Adversary's case (why it is NOT an issue, or their concession)
>
> For each finding, determine:
>
> ```
> ### F-<number>: <title>
> - **Verdict:** CONFIRMED | DISMISSED
> - **Severity:** blocking | important | nit
> - **Reasoning:** <which argument is stronger and why>
> - **Suggested fix:** <concrete fix if CONFIRMED, omit if DISMISSED>
> ```
>
> **Guidelines:**
>
> - If the Adversary conceded, lean toward CONFIRMED but still verify independently.
> - If the Adversary disproved with strong evidence, lean toward DISMISSED.
> - When evidence is ambiguous, consider: does this issue cause incorrect behavior, or is it merely stylistic? Err toward CONFIRMED for correctness issues, DISMISSED for debatable style.
> - Convention violations documented in CLAUDE.md or domain skill conventions are always **blocking** if confirmed.
>
> Be precise. The human will compare your judgments against ground truth.

### Phase 5: Write Results to .context

Each agent writes its output to a file in the `.context` directory so other agents and the user can read the results. Instruct each agent to write its output using the Write tool:

- **Issue Finder** writes to `.context/review-findings.md`
- **Adversary** writes to `.context/review-adversary.md`
- **Referee** writes to `.context/review-referee.md`

After the Referee completes, compile the final review from the Referee's output and write it to `.context/review-verdict.md`. Include only CONFIRMED findings.

```markdown
## Verdict: [Approve | Request Changes | Needs Discussion]

### Summary

[1-2 sentences: what was reviewed and the overall assessment]

### Blocking

- [ ] [F-<n>: description — file:line — reasoning]

### Important

- [ ] [F-<n>: description — file:line — reasoning]

### Nits

- [F-<n>: description — file:line — reasoning]

### Dismissed (<count> findings filtered)

[One-line summary of each dismissed finding for transparency]
```

Include the dismissed count so the human can see how much filtering happened.

Do NOT use the DiffComment tool or post comments to GitHub. All output goes into `.context/` files.

## Agent Configuration

All three agents use `subagent_type: "general-purpose"` with `model: "opus"`.

Each agent writes its output to a `.context/` file (see Phase 5). Instruct each agent to use the Write tool to create its output file. Do NOT use the DiffComment tool or any GitHub comment APIs.

Do NOT parallelize the agents. Each one depends on the previous output. The sequential dependency is what makes the adversarial dynamic work:

1. Issue Finder runs first → writes `.context/review-findings.md`
2. Adversary runs second (reads findings) → writes `.context/review-adversary.md`
3. Referee runs third (reads both) → writes `.context/review-referee.md`
4. Main agent compiles final verdict → writes `.context/review-verdict.md`

## Tone

Frame confirmed findings constructively — offer concrete fixes, not just criticism. Use severity prefixes on every finding: `[blocking]`, `[important]`, `[nit]`.
