# go-browser-harness

`go-browser-harness` is a fast, Go-native browser harness for LLM agents. It is a direct Go port of the upstream Python `browser-use/browser-harness` project, intended to be imported by `gormes-agent` while also shipping a small CLI binary.

The project is intentionally thin. Browser control stays close to Chrome DevTools Protocol through `chromedp`; higher-level agent behavior belongs in the importing agent runtime, not in this harness.

## Repository Layout

- `cmd/go-browser-harness/` — CLI binary (`--action-json`, `--version`).
- `pkg/harness/` — exported package consumed by `gormes-agent`.
- `internal/` — internal helpers and build info.
- `browser-harness/` — **git submodule** pinned to upstream `browser-use/browser-harness` (Python). Read-only parity reference.
- `docs/parity-matrix.md` — tracked parity gaps between Python and Go.
- `development-skills/` — repo-local skills for builder agents.

## Cloning

This repo uses a git submodule for the Python parity reference. Clone with:

```bash
git clone --recurse-submodules https://github.com/TrebuchetDynamics/go-browser-harness.git
```

If you already cloned without `--recurse-submodules`:

```bash
git submodule update --init --recursive
```

To pull the latest upstream Python code into the submodule:

```bash
git submodule update --remote browser-harness
git add browser-harness
git commit -m "chore(submodule): bump browser-harness to <sha>"
```

## Current Go Feature Surface

| Surface | Status |
|---|---|
| `ActionRequest` / `ActionResult` types (Gormes `gormes.browser.action.v1` schema) | shipped |
| `Backend` interface (live CDP / unavailable fallback) | shipped |
| `ChromedpBackend` over `CDPTransport` interface | shipped |
| Operator-provided endpoint via `CHROME_REMOTE_DEBUGGING_URL` | shipped |
| `browser_navigate` opens a new tab via `Target.createTarget` (does not clobber active tab) | shipped |
| `browser_snapshot/click/type/scroll/back/press/console/get_images/vision/cdp/dialog` action dispatch | shipped |
| Bounded screenshot/artifact envelope with `@e` ref maps | shipped |
| Typed unavailable evidence (`go_browser_harness_backend_unavailable`, `_action_invalid`, `_action_timeout`, `_screenshot_failed`) | shipped |
| `--action-json` CLI for Gormes integration | shipped |
| Daemon lifecycle (start/stop/restart/ensure) | **gap** |
| Profile management (local/cloud list, sync) | **gap** |
| Cloud browser provider integration (browser-use API) | **gap** |
| Doctor / diagnostics CLI | **gap** |
| Setup / update CLI | **gap** |

See `docs/parity-matrix.md` for the row-level breakdown of remaining work.

## Requirements

- Go 1.26 or newer.
- Chrome (or another CDP-compatible browser) for end-to-end smoke tests. The unit tests use a fake `CDPTransport` and need no live browser.

## Branching Strategy

- `main` is the protected production branch. It must remain green and should only receive reviewed merges.
- `development` is the default branch for active work.
- All changes must enter `development` through pull requests.
- Pull requests targeting `development` or `main` must pass CI before merge.
- Direct pushes to `main` are not allowed. Enforce this with GitHub branch protection rules.

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

Run the Gormes action JSON contract against the live operator-provided CDP endpoint:

```bash
export CHROME_REMOTE_DEBUGGING_URL=http://localhost:9222
go run ./cmd/go-browser-harness --action-json '{"schema_version":"gormes.browser.action.v1","kind":"snapshot"}'
```

When `CHROME_REMOTE_DEBUGGING_URL` is unset, the command returns typed unavailable evidence instead of falling back to Python or auto-launching a browser.

## Parity Discipline

When porting a feature from `browser-harness/`:

1. **Read the Python source first** — `browser-harness/src/browser_harness/{admin,daemon,helpers,run,_ipc}.py` is the source of truth for behavior parity. Do not guess; cite line numbers.
2. **Decide the Go-native shape** — keep `pkg/harness` close to CDP; put higher-level lifecycle in `cmd/go-browser-harness` or `internal/`.
3. **TDD** — write failing tests that pin behavior parity before implementing.
4. **Hermetic by default** — unit tests use a fake `CDPTransport` (or fake `http.Client` for cloud-provider tests). Live Chrome is documented as optional smoke, not required CI.
5. **Update `docs/parity-matrix.md`** with the row's status when a feature lands.

## License

See `LICENSE`.
