# go-browser-harness

`go-browser-harness` is a fast, Go-native browser harness for LLM agents. It is a direct Go port foundation for the Python `browser-use/browser-harness` project, intended to be imported by `gormes-agent` while also shipping a small CLI binary.

The project is intentionally thin. Browser control should stay close to Chrome DevTools Protocol through `chromedp`; higher-level agent behavior belongs in the importing agent runtime, not in this harness.

## Current Scope

This repository currently provides:

- A Go module at `github.com/TrebuchetDynamics/go-browser-harness`.
- A minimal CLI under `cmd/go-browser-harness` with a Go-native
  `--action-json` contract for Gormes.
- An exported package under `pkg/harness` for `gormes-agent` integration.
- CI checks for linting, race-enabled tests, and a static Linux binary build.

The Python reference checkout may exist locally at `browser-harness/`, but it is ignored by this Go repository and should only be used for behavior reference during later porting work.

## Requirements

- Go 1.26 or newer.
- Chrome or another CDP-compatible browser for future integration tests.

## Branching Strategy

- `main` is the protected production branch. It must remain green and should only receive reviewed merges.
- `development` is the default branch for active work.
- All changes must enter `development` through pull requests.
- Pull requests targeting `development` or `main` must pass CI before merge.
- Direct pushes to `main` are not allowed. Enforce this with GitHub branch protection rules in the remote repository settings.

## Development

Run the full local verification set:

```bash
go test -race ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -extldflags '-static'" -o dist/go-browser-harness ./cmd/go-browser-harness
```

Run the CLI scaffold:

```bash
go run ./cmd/go-browser-harness --version
```

Run the Gormes action JSON contract:

```bash
go run ./cmd/go-browser-harness --action-json '{"schema_version":"gormes.browser.action.v1","kind":"snapshot"}'
```

Until the live CDP backend is wired, the command returns typed unavailable
evidence instead of falling back to Python or pretending a browser action ran.
