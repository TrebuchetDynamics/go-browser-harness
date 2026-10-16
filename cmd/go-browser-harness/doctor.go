package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/TrebuchetDynamics/go-browser-harness/internal/buildinfo"
)

// DoctorEnv holds the injectable inputs for runDoctor so that callers
// (tests and main) can supply controlled values without touching os.Getenv
// inside the core logic. The API key is pre-resolved to a boolean so the
// value is never passed through this layer.
type DoctorEnv struct {
	// ChromeRemoteDebuggingURL is the operator-supplied endpoint, typically
	// read from the CHROME_REMOTE_DEBUGGING_URL environment variable.
	ChromeRemoteDebuggingURL string

	// BrowserUseAPIKeyPresent is true when the BROWSER_USE_API_KEY environment
	// variable is non-empty. The value itself must NOT be stored here.
	BrowserUseAPIKeyPresent bool

	// HTTPClient is the client used to probe /json/version. Tests inject an
	// httptest.Server client; production callers pass http.DefaultClient.
	HTTPClient *http.Client
}

const (
	rowOK   = "[ok  ]"
	rowFail = "[FAIL]"
	rowSkip = "[--  ]"
)

// runDoctor prints a diagnostic table to w and returns 0 iff the Chrome
// remote debugging endpoint probe succeeds. All other rows are informational.
//
// Rows printed (in order):
//  1. platform        — runtime.GOOS/runtime.GOARCH
//  2. go version      — runtime.Version()
//  3. harness version — internal/buildinfo.Version
//  4. chrome remote debugging endpoint — GET <url>/json/version; OK on 200+JSON
//  5. BROWSER_USE_API_KEY — ok when present, [--  ] (optional) when absent
func runDoctor(_ context.Context, env DoctorEnv, w io.Writer) int {
	// Row 1 — platform (always ok; purely informational)
	fmt.Fprintf(w, "%s platform         %s/%s\n", rowOK, runtime.GOOS, runtime.GOARCH)

	// Row 2 — Go version (always ok; purely informational)
	fmt.Fprintf(w, "%s go version        %s\n", rowOK, runtime.Version())

	// Row 3 — harness version (always ok; purely informational)
	fmt.Fprintf(w, "%s harness version   %s\n", rowOK, buildinfo.Version)

	// Row 4 — Chrome remote debugging endpoint (the health gate)
	chromeOK := probeChromeEndpoint(env, w)

	// Row 5 — BROWSER_USE_API_KEY presence (optional — [--  ] when absent)
	if env.BrowserUseAPIKeyPresent {
		fmt.Fprintf(w, "%s BROWSER_USE_API_KEY present\n", rowOK)
	} else {
		fmt.Fprintf(w, "%s BROWSER_USE_API_KEY not set (optional for local Chrome; required for cloud browser)\n", rowSkip)
	}

	if !chromeOK {
		return 1
	}
	return 0
}

// probeChromeEndpoint attempts GET <url>/json/version and writes a result row
// to w. Returns true iff the probe succeeds (200 + valid JSON).
func probeChromeEndpoint(env DoctorEnv, w io.Writer) bool {
	if env.ChromeRemoteDebuggingURL == "" {
		fmt.Fprintf(w,
			"%s chrome remote debugging endpoint   CHROME_REMOTE_DEBUGGING_URL is (unset) — set CHROME_REMOTE_DEBUGGING_URL to enable Chrome probe\n",
			rowFail,
		)
		return false
	}

	probeURL := env.ChromeRemoteDebuggingURL + "/json/version"
	resp, err := env.HTTPClient.Get(probeURL) //nolint:noctx // context not threaded into http.Get for hermetic simplicity
	if err != nil {
		fmt.Fprintf(w,
			"%s chrome remote debugging endpoint   unreachable (%s) — check CHROME_REMOTE_DEBUGGING_URL\n",
			rowFail, env.ChromeRemoteDebuggingURL,
		)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(w,
			"%s chrome remote debugging endpoint   /json/version returned HTTP %d — check CHROME_REMOTE_DEBUGGING_URL\n",
			rowFail, resp.StatusCode,
		)
		return false
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		fmt.Fprintf(w,
			"%s chrome remote debugging endpoint   /json/version returned malformed JSON — check Chrome is actually running\n",
			rowFail,
		)
		return false
	}

	fmt.Fprintf(w, "%s chrome remote debugging endpoint   %s\n", rowOK, env.ChromeRemoteDebuggingURL)
	return true
}
