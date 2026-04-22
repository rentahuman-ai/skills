# CI Housekeeping

Enforce conventions across CI tooling and code generation.

## Scope

- `.github/workflows/` (GitHub Actions workflows)
- Fragment scanning (gen-skills markers in source files)

## Source of Truth

CLAUDE.md conventions, `.agents/skills/skill-creator/references/fragment-scanning.md`.

## What to Check

| Check                    | Details                                                                       |
| ------------------------ | ----------------------------------------------------------------------------- |
| gen-skills freshness     | `npx tsx .agents/skills/skill-creator/scripts/scan-fragments.ts check` passes |
| Comment marker validity  | All `#<gen-skill>` and `//<gen-skill>` markers in source are well-formed      |
| Frontmatter completeness | All skill fragments have required `name` and `description` fields             |
| Generated file integrity | Auto-generated SKILL.md files have `AUTO-GENERATED` header, no manual edits   |
| Workflow YAML validity   | `.github/workflows/*.yml` files parse as valid YAML                           |
| Template variable usage  | `$$file` and `$$directory` expand correctly in fragments                      |

## Discovery

```bash
# Generated skill markers
grep -r "#<gen-skill>" --include="*.sh" --include="*.bash" --include="*.py" -l
grep -r "//<gen-skill>" --include="*.ts" --include="*.js" -l

# Workflows
ls .github/workflows/*.yml 2>/dev/null
```

## Audit Approach

1. Run `npx tsx .agents/skills/skill-creator/scripts/scan-fragments.ts check` to detect drift
2. Scan all source files for comment markers, validate frontmatter
3. Verify auto-generated SKILL.md files haven't been manually edited
4. Check template variable expansion
