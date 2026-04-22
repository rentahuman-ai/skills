# Tooling Housekeeping

Enforce conventions across internal agent tools and developer tooling.

## Scope

- `scripts/` — utility scripts (run with `npx tsx scripts/<name>.ts`)
- `.agents/skills/*/scripts/` — skill-bundled scripts
- Developer tooling configuration

## Source of Truth

CLAUDE.md conventions. Auto-generated skills are produced from `//<gen-skill>` and `#<gen-skill>` markers in source files via the fragment scanner (`npx tsx .agents/skills/skill-creator/scripts/scan-fragments.ts`).

## What to Check

| Check                | Details                                                                                                  |
| -------------------- | -------------------------------------------------------------------------------------------------------- |
| Script executability | Scripts have appropriate shebangs and permissions                                                        |
| Gen-skill metadata   | All `//<gen-skill>` and `#<gen-skill>` markers have valid YAML frontmatter with `name` and `description` |
| Script descriptions  | Help text and skill descriptions accurately describe what the tool does                                  |
| Dependency alignment | Package dependencies match what the implementations actually import                                      |

## Audit Approach

1. Scan for `//<gen-skill>` and `#<gen-skill>` markers across the codebase
2. Validate each marker has well-formed YAML frontmatter with `name` and `description`
3. Run `npx tsx .agents/skills/skill-creator/scripts/scan-fragments.ts check` to verify generated skills are fresh
4. Check scripts in `scripts/` for correct imports and working invocations

## Cross-Cutting References

- [skills.md](skills.md) — skill housekeeping and generated vs hand-written distinction
