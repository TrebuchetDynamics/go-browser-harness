---
name: harness-verify
description: Use when claiming go-browser-harness work is complete, committing, editing CI, changing Go dependencies, or preparing a PR.
---

# Harness Verify

No completion claim without fresh evidence from this repo.

## Required Checks

Run from `/home/xel/git/sages-openclaw/workspace-mineru/go-browser-harness`:

```bash
go test -race ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 run
mkdir -p dist && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -extldflags '-static'" -o dist/go-browser-harness ./cmd/go-browser-harness
file dist/go-browser-harness
ldd dist/go-browser-harness 2>&1 || true
go mod verify
git status --short
```

Expected static-build evidence:
- `file` includes `statically linked`.
- `ldd` prints `not a dynamic executable`.

If `.github/workflows/*.yml` changed, also run:

```bash
go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/ci.yml
```

## Report

Include:
- Repo path.
- Branch.
- Commit hash if committed.
- Exact pass/fail status for each command.
- Push status or exact blocker.

Do not say complete, fixed, green, or passing until the commands above confirm it.
