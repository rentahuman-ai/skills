# PR Review Mode (Default)

Context for reviewing pull requests and local diffs.

## Gathering the Diff

```bash
# If PR number provided
gh pr view <number> --json title,body,additions,deletions,files
gh pr diff <number>

# If no PR number — local diff against main
git diff main...HEAD --stat
git log main..HEAD --oneline
git diff main...HEAD
```

## Issue Finder Focus

The Issue Finder should focus on changed lines and their immediate surrounding context. Pass it:

- The full diff
- The PR title and description (so it understands intent)
- CLAUDE.md conventions
- Domain skill conventions from Phase 1.5 (relevant `domain-*` SKILL.md contents)

Convention violations from CLAUDE.md are always categorized as critical (+10):

- Type casts (`as`, `as any`, `as unknown as`, `<Type>expr`, `expr!`)
- Barrel imports (should import from specific files)
- File extensions in imports (`.ts`, `.tsx`, `.js` — none allowed)
- Direct `process.env` reads (should use `@platform/config`)
- Co-located tests (should be in `test/` directory)
- Naming violations (PascalCase backend, kebab-case frontend)
- Section separator comments
- Dead code (unused exports, unreachable functions, test-only production code)
- Domain-specific anti-patterns (e.g., Effect-TS anti-patterns from `domain-effect`, frontend hierarchy violations from `domain-frontend`, Drizzle convention violations from `domain-database` — as documented in the matched domain SKILL.md files)

## Writing Results

All output is written to `.context/` files — do NOT use the DiffComment tool or post to GitHub.

Each agent writes its phase output:

- Issue Finder → `.context/review-findings.md`
- Adversary → `.context/review-adversary.md`
- Referee → `.context/review-referee.md`

The final compiled verdict is written to `.context/review-verdict.md`.
