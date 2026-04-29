---
name: harness-research
description: Use when researching go-browser-harness behavior, the Python browser-use/browser-harness reference, chromedp/CDP APIs, Gormes integration needs, or browser automation design tradeoffs.
---

# Harness Research

Research must preserve the thin-harness contract: this repo wraps CDP through `chromedp` and exposes typed, importable Go surfaces. Agent policy, memory, planning, and workspace execution belong in `gormes-agent`.

## Sources In Order

1. Local repo files in `/home/xel/git/sages-openclaw/workspace-mineru/go-browser-harness`.
2. Local Python reference under `browser-harness/`.
3. `gormes-agent` integration surface only when the task asks about the importer.
4. Current library docs via Context7 for `chromedp`, Go tooling, GitHub Actions, and any SDK/API.
5. Web/GitHub sources only when local and Context7 docs are insufficient.

## Workflow

1. Start with `pwd`, `git rev-parse --show-toplevel`, and `git status --short`.
2. Use `rg` first. Avoid broad manual scanning.
3. Identify the public behavior being researched, not just files.
4. Compare the Python reference to the Go target at the boundary level: CLI input/output, daemon/session behavior, CDP action shape, artifacts, errors.
5. Record decisions as short bullets with evidence: file paths, functions, command output, and doc links.
6. End with what is in scope, what is out of scope, and what should be tested first.

## Research Output Shape

Use this structure:

```markdown
Finding: <one sentence>
Evidence:
- <path or doc link>
- <command and key output>
Implication:
- <how this affects the Go harness>
Next test:
- <first behavior to lock with TDD>
```
