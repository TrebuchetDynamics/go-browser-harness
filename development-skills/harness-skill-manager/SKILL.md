---
name: harness-skill-manager
description: Use when working in go-browser-harness and deciding which project-local development skill should govern research, design interrogation, TDD, diagnosis, verification, or new skill creation.
---

# Harness Skill Manager

Use this first for non-trivial `go-browser-harness` work. It routes the task to the smallest useful skill set and creates a new `development-skills/<name>/SKILL.md` when no existing skill fits.

Inspired by the small, composable skill style in `mattpocock/skills`.

## Route

- Research original behavior, `chromedp`, CDP, or `gormes-agent` integration: use `harness-research`.
- Stress-test a plan or design with Juan: use `harness-grill-me`.
- Implement feature, bugfix, or behavior change: use `harness-tdd`.
- Debug failure, flaky test, CI issue, or browser/CDP problem: use `harness-diagnose`.
- Claim completion, prepare commit, or touch CI/release paths: use `harness-verify`.
- Create or update project-local skills: stay here and follow "Create A Skill".

Use multiple skills only when the task crosses phases. Example: research first, grill if requirements are unclear, then TDD, then verify.

## Create A Skill

Create a new skill when the same project-specific workflow will recur and is not covered above.

Rules:
- Directory: `development-skills/<lowercase-hyphen-name>/SKILL.md`.
- Keep one skill per concern.
- Frontmatter must include `name` and `description`.
- Description starts with `Use when` and names trigger conditions, not the workflow.
- Keep `SKILL.md` concise. Move only genuinely reusable heavy details to one-level references.
- Prefer project commands and exact paths over generic advice.

Minimum template:

```markdown
---
name: harness-example
description: Use when <specific go-browser-harness trigger>.
---

# Harness Example

Core rule in one or two sentences.

## Workflow

1. Check repo state.
2. Do the task-specific work.
3. Run the exact verification commands.
```

After creating or editing skills, run:

```bash
find development-skills -path '*/SKILL.md' -print | sort
rg -n '^name:|^description:' development-skills
git status --short
```
