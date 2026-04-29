---
name: harness-diagnose
description: Use when go-browser-harness tests, lint, CI, static builds, CDP/browser interactions, CLI output, or integration behavior fail or behave inconsistently.
---

# Harness Diagnose

Build a fast feedback loop before changing code. A deterministic failing test or command is the debugging engine.

## Feedback Loops

Choose the narrowest loop that reproduces the symptom:

- Go behavior: `go test ./pkg/harness -run TestName -count=1`.
- CLI behavior: `go test ./cmd/go-browser-harness -run TestName -count=1` or `go run ./cmd/go-browser-harness ...`.
- Lint/config: `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 run`.
- CI YAML: `go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/ci.yml`.
- Static binary: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...` plus `file` and `ldd`.
- Browser/CDP: use a gated integration test or explicit `BU_CDP_WS`/remote-debugging endpoint.

## Workflow

1. Reproduce the exact user-visible failure.
2. Minimize the reproducer.
3. State 3 ranked hypotheses with falsifiable predictions.
4. Add only targeted instrumentation. Prefix temporary logs with `[DEBUG-harness-<id>]`.
5. Write a regression test at the correct seam before fixing when possible.
6. Fix one cause at a time.
7. Re-run the original loop and regression test.
8. Remove all debug instrumentation:

```bash
rg '\\[DEBUG-harness-' .
```

If no correct test seam exists, report that as an architecture finding.
