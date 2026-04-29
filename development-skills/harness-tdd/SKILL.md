---
name: harness-tdd
description: Use when implementing or changing go-browser-harness behavior, package APIs, CLI commands, CDP action contracts, tests, or bug fixes.
---

# Harness TDD

Use vertical red-green-refactor. One behavior, one failing test, minimal code, repeat. Do not write broad batches of imagined tests before the implementation path is proven.

## Test Seams

Prefer public seams:
- `pkg/harness` exported functions and typed contracts.
- `cmd/go-browser-harness` CLI behavior through small testable helpers.
- Backend interfaces with fake implementations for no-browser tests.
- Integration tests only when a real CDP endpoint is available and explicitly gated.

Avoid testing private implementation details, internal call order, or `chromedp` itself.

## Cycle

1. Name one observable behavior.
2. Write the smallest failing Go test for that behavior.
3. Run the narrow test and confirm the expected failure.
4. Add the minimum implementation.
5. Re-run the narrow test.
6. Run `go test ./...`.
7. Refactor only while green.

For final verification, hand off to `harness-verify`.

## Good Harness Tests

- Validate JSON contracts, defaults, evidence strings, and error messages.
- Use `context.Context` cancellation in tests where lifecycle matters.
- Use fakes for CDP backend behavior unless the test is explicitly integration-gated.
- Keep tests robust across internal refactors.

## Commands

```bash
go test ./pkg/harness -run TestName
go test ./cmd/go-browser-harness -run TestName
go test ./...
```
