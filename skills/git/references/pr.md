# Pull Request Conventions

## Required Body Sections

Always include:

- `## Context`
- `## What Changed`
- `## Validation`
- `## Risks & Rollback`

Include `## External Changelog` only when external release notes are needed.

## Template

```markdown
## Context

Why this change exists.

## What Changed

- Key implementation changes.

## Validation

- Commands run
- Manual checks performed

## Risks & Rollback

- Main risks
- Rollback approach

## External Changelog

{CHANGELOG}: <customer-facing change summary>
{CHANGELOG}: <second customer-facing summary if needed>
```

If no external changelog is needed, omit the entire `## External Changelog` section (including heading).

## {CHANGELOG} Line Format

The `{CHANGELOG}: ...` lines are machine-parseable metadata for future CI automation.

Rules:

- External/customer-facing only
- No internal implementation details, secrets, or incident context
- Prefer one user-visible change per line

## Draft-First Flow

```bash
git push -u origin <branch>
gh pr create --draft --title "<title>" --body-file /tmp/PR_BODY.md --base main
gh pr edit --body-file /tmp/PR_BODY.md
gh pr ready
```

## Review Readiness Checklist

- `npm run build && npm test` passed locally or in CI
- PR body includes validation evidence
- If `## External Changelog` exists, `{CHANGELOG}: ...` line(s) are externally safe
- Risks and rollback are documented
