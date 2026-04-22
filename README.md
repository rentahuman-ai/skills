# Rent A Human — Claude Skills

Shared [Claude Code skills](https://docs.claude.com/en/docs/claude-code/skills) used across Rent A Human projects.

Each top-level directory is a skill. A skill is auto-loaded by Claude Code when its `description` matches what you're doing — the model decides when to read the body.

## Layout

```
<skill-name>/
  SKILL.md           # required — YAML frontmatter + instructions
  references/*.md    # optional — longer context the skill links to
  scripts/           # optional — helper scripts invoked from the skill
```

`SKILL.md` frontmatter:

```yaml
---
name: skill-name
description: One sentence on when Claude should use this. Specific triggers beat vague summaries.
---
```

## Using this repo

### As a git submodule (recommended for Rent A Human projects)

```bash
git submodule add https://github.com/rentahuman-ai/skills .agents/skills
git submodule update --init
```

Then symlink or copy each skill into the project's `.claude/skills/` so Claude Code picks them up:

```bash
for dir in .agents/skills/*/; do
  name=$(basename "$dir")
  ln -sf "../../.agents/skills/$name" ".claude/skills/$name"
done
```

### Updating

```bash
git submodule update --remote .agents/skills
git add .agents/skills
git commit -m "chore: bump skills submodule"
```

## Adding or editing skills

1. Create `<skill-name>/SKILL.md` with frontmatter (`name`, `description`).
2. Keep the body focused — link out to `references/*.md` for anything long.
3. Open a PR here. Downstream projects pick it up via `git submodule update --remote`.

## Spec

- [Skill spec](https://docs.claude.com/en/docs/claude-code/skills)
- [Plugin packaging](https://docs.claude.com/en/docs/claude-code/plugins) (if later distributed via a plugin marketplace)
