# Codebase Review Mode

Context for reviewing specific files or modules without a diff. Use when the user points at files, directories, or modules rather than a PR or branch diff.

## Identifying Targets

The user specifies what to review:

- Specific file paths: `review packages/platform/auth/`
- Module names: `review the auth module`
- Patterns: `review all API route handlers`

If the target is a directory or module, use `Glob` and `Read` to collect all source files. Skip test files, config files, and generated files unless specifically requested.

## Issue Finder Focus

For codebase review, the Issue Finder examines full file contents (not diffs). The focus shifts toward:

- **Architectural issues** — coupling, circular dependencies, misplaced responsibilities
- **Latent bugs** — race conditions, unhandled error paths, incorrect assumptions about state
- **Security** — injection vectors, auth bypass, data exposure
- **Type safety** — loose types, missing validation at boundaries
- **Dead code** — unreachable branches, unused exports, stale imports
- **Domain convention violations** — patterns that contradict the matched domain skill conventions (from Phase 1.5)

Pass the Issue Finder the full file contents, CLAUDE.md conventions, and domain conventions from Phase 1.5.

## Output

All output is written to `.context/` files — do NOT use the DiffComment tool or post to GitHub. Each agent writes its phase output to the corresponding `.context/review-*.md` file. Use the same final review format from the main skill, written to `.context/review-verdict.md`.
