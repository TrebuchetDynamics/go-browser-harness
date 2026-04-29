---
name: harness-grill-me
description: Use when Juan asks to be grilled, wants to stress-test a go-browser-harness plan, or when browser harness requirements are ambiguous before implementation.
---

# Harness Grill Me

Interrogate the plan until the harness boundary is precise. Ask one question at a time. For each question, include the recommended answer and why.

If the answer can be discovered from the codebase or docs, inspect them instead of asking.

## Question Order

1. Import boundary: what must `gormes-agent` call directly?
2. CLI boundary: what should humans or CI invoke?
3. Action contract: what JSON/input shape is stable?
4. Browser lifecycle: local Chrome, remote CDP, profile reuse, or daemon?
5. Artifacts: screenshots, PDFs, DOM snapshots, downloads, logs.
6. Error evidence: what should failures return to agents?
7. Security boundary: what must never execute inside this module?
8. Test strategy: which behavior gets a fast unit test, integration test, or manual browser validation?
9. Out of scope: which tempting feature belongs in `gormes-agent` instead?

## Format

```markdown
Question: <single decision>
Recommended answer: <short recommendation>
Reason: <thin harness / testability / Gormes integration reason>
```

Stop when each branch has an explicit answer or a named blocker.
