# Python ↔ Go Parity Matrix

Source of truth for what `browser-harness` (Python, upstream) provides and what `go-browser-harness` (Go, this repo) covers. Pinned to the submodule SHA in `browser-harness/`.

## Conventions

- **shipped** — feature is implemented and covered by Go tests.
- **partial** — feature is implemented but missing edge cases or tests vs Python.
- **planned** — gap with a builder-ready slice in mind.
- **gap** — gap with no slice yet; needs planner pass.
- **owned** — Go-native; Python has no equivalent.

Source-line citations point at the submodule's `browser-harness/src/browser_harness/<file>.py:<line>`.

## Action contract (Gormes `gormes.browser.action.v1`)

| Feature | Python ref | Go ref | Status |
|---|---|---|---|
| Action JSON schema (kind + payload) | n/a (Gormes-owned) | `pkg/harness/action.go:ActionRequest` | shipped |
| Snapshot / click / type / scroll / back / press | `helpers.py` page primitives | `pkg/harness/chromedp_backend.go:run*` | shipped |
| Console reads | `helpers.py:console_logs` | `pkg/harness/chromedp_backend.go:runConsole` | shipped |
| `get_images` / `vision` | `helpers.py:get_images` | `pkg/harness/chromedp_backend.go:runGetImages,runVision` | shipped |
| Raw CDP passthrough | `helpers.py:cdp` | `pkg/harness/chromedp_backend.go:runCDP` | shipped |
| Dialog (alert/confirm/prompt) | `helpers.py:dialog` | `pkg/harness/chromedp_backend.go:runDialog` | shipped |
| Bounded screenshot artifacts (`@e` refs) | `helpers.py` snapshot tree | `pkg/harness/chromedp_backend.go` | shipped |
| Typed unavailable evidence | n/a (Python raises) | `pkg/harness/action.go:UnavailableBackend` | owned |

## Daemon lifecycle

| Feature | Python ref | Go ref | Status |
|---|---|---|---|
| Daemon process start | `daemon.py:Daemon`, `serve` | — | gap |
| `already_running` detection (PID file + endpoint check) | `daemon.py:already_running` | — | gap |
| Restart daemon | `admin.py:restart_daemon` | — | gap |
| Stop remote daemon | `admin.py:stop_remote_daemon` | — | gap |
| Ensure daemon (boot if absent, wait until alive) | `admin.py:ensure_daemon` | — | gap |
| Multiple daemon endpoints (`local`, `remote`) | `admin.py:_daemon_endpoint_names` | — | gap |
| Daemon active-connection inventory | `admin.py:browser_connections,active_browser_connections` | — | gap |

## Endpoint discovery & CDP wiring

| Feature | Python ref | Go ref | Status |
|---|---|---|---|
| WebSocket URL resolution from `localhost:9222` | `daemon.py:get_ws_url` | `pkg/harness/chromedp_backend.go:NewChromedpBackend` | partial — Python tries fallback hosts; Go assumes single endpoint |
| `_cdp_ws_from_url` (HTTP→WS conversion) | `admin.py:_cdp_ws_from_url` | inline in `chromedp_backend.go` | partial |
| Local Chrome mode detection | `admin.py:_is_local_chrome_mode` | — | gap |
| `_needs_chrome_remote_debugging_prompt` (operator hint) | `admin.py:_needs_chrome_remote_debugging_prompt` | — | gap |

## Profile management

| Feature | Python ref | Go ref | Status |
|---|---|---|---|
| List local profiles (cookies/storage in OS-standard dirs) | `admin.py:list_local_profiles` | — | gap |
| List cloud profiles (browser-use cloud API) | `admin.py:list_cloud_profiles` | — | gap |
| Sync cloud profile → local | `admin.py:sync_local_profile` | — | gap |
| Resolve profile name (alias resolution) | `admin.py:_resolve_profile_name` | — | gap |
| Start remote daemon attached to profile | `admin.py:start_remote_daemon` | — | gap |
| Stop cloud browser session | `admin.py:_stop_cloud_browser` | — | gap |

## Cloud browser provider (browser-use API)

| Feature | Python ref | Go ref | Status |
|---|---|---|---|
| `_browser_use(path, method, body)` API client | `admin.py:_browser_use` | — | gap |
| Live URL display (`_show_live_url`) | `admin.py:_show_live_url` | — | gap |
| Local GUI detection | `admin.py:_has_local_gui` | — | gap |

## Doctor / diagnostics

| Feature | Python ref | Go ref | Status |
|---|---|---|---|
| `run_doctor` (full env + endpoint health report) | `admin.py:run_doctor` | — | gap |
| `_doctor_short_text` (redacted display helper) | `admin.py:_doctor_short_text` | — | gap |
| `_log_tail` (recent daemon logs) | `admin.py:_log_tail` | — | gap |

## Setup / update

| Feature | Python ref | Go ref | Status |
|---|---|---|---|
| `run_setup` (first-time install flow) | `admin.py:run_setup` | — | gap |
| `run_update` / `_latest_release_tag` (self-update) | `admin.py:run_update,_latest_release_tag` | — | gap |
| `_install_mode` (pip/uv/dev) | `admin.py:_install_mode` | — | gap |
| `_version` | `admin.py:_version` | `internal/buildinfo/buildinfo.go` | partial — Go reports build SHA, Python reports installed package version |

## CLI ergonomics

| Feature | Python ref | Go ref | Status |
|---|---|---|---|
| HELP text | `run.py:HELP` | `cmd/go-browser-harness/main.go` | partial — Go HELP minimal |
| Subcommand parser (setup, doctor, update, etc.) | `run.py:main` | — | gap |
| Update banner | `admin.py:print_update_banner` | — | gap |
| `--action-json` (Gormes-only contract) | n/a | `cmd/go-browser-harness/main.go` | owned |

## Suggested first slices (smallest first)

1. **`go doctor` subcommand**: read `admin.py:run_doctor`, port to a hermetic Go subcommand that probes `CHROME_REMOTE_DEBUGGING_URL`, prints redacted env + endpoint health. Pure stdout; no daemon required. Small slice.
2. **Daemon `already_running` detection**: PID-file check + HTTP probe of `localhost:9222/json/version`. Foundation for `ensure_daemon`. Small slice.
3. **`get_ws_url` fallback resolver**: walk fallback hosts (`127.0.0.1`, `localhost`, container-aware `host.docker.internal`), match Python's behavior in `daemon.py:get_ws_url`. Small slice.
4. **List local profiles**: stat XDG/macOS/Windows profile dirs, return JSON. No CDP needed. Small slice.
5. **Subcommand CLI scaffold**: replace `--action-json`-only flag parsing with cobra-style subcommands so `setup`, `doctor`, `restart`, `list-profiles` can each have their own help. Refactor slice.

## Maintenance

When the Python submodule advances, audit changed files for new functions and add rows above. Run:

```bash
git submodule update --remote browser-harness
git -C browser-harness log --oneline HEAD@{1}..HEAD -- src/browser_harness/
```
