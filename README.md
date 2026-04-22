# Rent A Human — Claude Skills

Public [Claude Code skills](https://docs.claude.com/en/docs/claude-code/skills) maintained by [Rent A Human](https://rentahuman.ai). Packaged as a [Claude Code plugin marketplace](https://docs.claude.com/en/docs/claude-code/plugin-marketplaces) so agents and contributors can install the full collection with one command.

## Install as a plugin marketplace (recommended)

Inside Claude Code:

```
/plugin marketplace add rentahuman-ai/skills
/plugin install rentahuman-skills@rentahuman-skills
```

Update later with `/plugin marketplace update rentahuman-skills`.

## Use as a git submodule

For projects that want to vendor the skills alongside source (e.g. the [rentahuman.ai](https://github.com/rentahuman-ai) monorepo):

```bash
git submodule add https://github.com/rentahuman-ai/skills rentahuman-skills
git submodule update --init
```

Bump to latest:

```bash
git submodule update --remote rentahuman-skills
git add rentahuman-skills
git commit -m "chore: bump rentahuman-skills submodule"
```

## Layout

```
.claude-plugin/
  marketplace.json    # marketplace catalog
  plugin.json         # plugin manifest
skills/
  <skill-name>/
    SKILL.md          # required — YAML frontmatter + instructions
    references/*.md   # optional — longer context the skill links to
    scripts/          # optional — helper scripts invoked from the skill
```

Each directory under `skills/` is a standalone skill that Claude Code auto-loads when its `description` matches what you're doing.

`SKILL.md` frontmatter:

```yaml
---
name: skill-name
description: One sentence on when Claude should use this. Specific triggers beat vague summaries.
---
```

## Adding or editing skills

1. Create `skills/<skill-name>/SKILL.md` with frontmatter (`name`, `description`).
2. Keep the body focused — link out to `references/*.md` for anything long.
3. Open a PR. Downstream consumers pick it up via `/plugin marketplace update` or `git submodule update --remote`.

## Spec

- [Skill spec](https://docs.claude.com/en/docs/claude-code/skills)
- [Plugin marketplace spec](https://docs.claude.com/en/docs/claude-code/plugin-marketplaces)
