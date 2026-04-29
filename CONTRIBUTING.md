# Contributing

`go-browser-harness` follows a protected-branch workflow.

## Branches

- `development` is the default branch for active work.
- `main` is production and must always remain green.
- Do not push directly to `main`.
- Open pull requests into `development` for normal work.
- Open pull requests into `main` only for reviewed releases from `development`.

## Pull Request Requirements

Every pull request targeting `development` or `main` must pass CI:

- `golangci-lint` with the repository lint configuration.
- `go test -race ./...`.
- Static CLI build with `CGO_ENABLED=0`.

Keep changes focused. This repository is a thin harness over CDP via `chromedp`; avoid adding agent orchestration, policy logic, memory, or workspace execution behavior here.

## Local Checks

Before opening a pull request, run:

```bash
go test -race ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -extldflags '-static'" -o dist/go-browser-harness ./cmd/go-browser-harness
```

If you have `golangci-lint` v2 installed locally, also run:

```bash
golangci-lint run
```
