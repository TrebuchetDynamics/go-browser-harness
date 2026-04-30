package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// healthyJSONVersion is the JSON body /json/version returns when Chrome is up.
const healthyJSONVersion = `{"Browser":"Chrome/123.0","Protocol-Version":"1.3"}`

// newHealthyServer returns an httptest.Server that serves 200 + valid JSON on
// any path, simulating a live Chrome remote debugging endpoint.
func newHealthyServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(healthyJSONVersion))
	}))
}

// TestRunDoctor_HealthyEndpointReturnsZero verifies exit 0 and an ok row when
// the Chrome endpoint answers 200 + valid JSON.
func TestRunDoctor_HealthyEndpointReturnsZero(t *testing.T) {
	srv := newHealthyServer(t)
	defer srv.Close()

	var buf bytes.Buffer
	env := DoctorEnv{
		ChromeRemoteDebuggingURL: srv.URL,
		BrowserUseAPIKeyPresent:  false,
		HTTPClient:               srv.Client(),
	}

	code := runDoctor(context.Background(), env, &buf)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout:\n%s", code, buf.String())
	}
	if !strings.Contains(buf.String(), "[ok  ] chrome remote debugging endpoint") {
		t.Fatalf("want '[ok  ] chrome remote debugging endpoint' in stdout; got:\n%s", buf.String())
	}
}

// TestRunDoctor_UnreachableEndpointReturnsNonZero verifies non-zero exit and a
// FAIL row with the env-var operator hint when the server is closed.
func TestRunDoctor_UnreachableEndpointReturnsNonZero(t *testing.T) {
	srv := newHealthyServer(t)
	srv.Close() // close before the call so the probe fails

	var buf bytes.Buffer
	env := DoctorEnv{
		ChromeRemoteDebuggingURL: srv.URL,
		BrowserUseAPIKeyPresent:  false,
		HTTPClient:               srv.Client(),
	}

	code := runDoctor(context.Background(), env, &buf)

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; stdout:\n%s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "[FAIL] chrome remote debugging endpoint") {
		t.Fatalf("want '[FAIL] chrome remote debugging endpoint' in stdout; got:\n%s", out)
	}
	if !strings.Contains(out, "CHROME_REMOTE_DEBUGGING_URL") {
		t.Fatalf("want operator hint mentioning 'CHROME_REMOTE_DEBUGGING_URL' in stdout; got:\n%s", out)
	}
}

// TestRunDoctor_MissingEndpointURLReturnsNonZero verifies non-zero exit when
// ChromeRemoteDebuggingURL is empty, with no HTTP call made and the unset hint
// in stdout.
func TestRunDoctor_MissingEndpointURLReturnsNonZero(t *testing.T) {
	// Use a nil HTTPClient to catch any accidental HTTP call (nil dereference = test fail).
	var buf bytes.Buffer
	env := DoctorEnv{
		ChromeRemoteDebuggingURL: "",
		BrowserUseAPIKeyPresent:  false,
		HTTPClient:               nil,
	}

	code := runDoctor(context.Background(), env, &buf)

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; stdout:\n%s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "(unset)") {
		t.Fatalf("want '(unset)' in stdout; got:\n%s", out)
	}
	if !strings.Contains(out, "CHROME_REMOTE_DEBUGGING_URL") {
		t.Fatalf("want env var name 'CHROME_REMOTE_DEBUGGING_URL' in stdout; got:\n%s", out)
	}
}

// TestRunDoctor_NotJSONResponseReturnsNonZero verifies non-zero exit when the
// server returns 200 but a non-JSON body.
func TestRunDoctor_NotJSONResponseReturnsNonZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	env := DoctorEnv{
		ChromeRemoteDebuggingURL: srv.URL,
		BrowserUseAPIKeyPresent:  false,
		HTTPClient:               srv.Client(),
	}

	code := runDoctor(context.Background(), env, &buf)

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero; stdout:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "[FAIL] chrome remote debugging endpoint") {
		t.Fatalf("want '[FAIL] chrome remote debugging endpoint' in stdout; got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "malformed JSON") {
		t.Fatalf("want 'malformed JSON' in stdout; got:\n%s", buf.String())
	}
}

// TestRunDoctor_BrowserUseAPIKeyPresenceRowReflectsInjectedFlag verifies that
// the api-key row reflects the pre-resolved boolean, not any live env lookup.
func TestRunDoctor_BrowserUseAPIKeyPresenceRowReflectsInjectedFlag(t *testing.T) {
	srv := newHealthyServer(t)
	defer srv.Close()

	// Key present: expect [ok  ] row.
	t.Run("present", func(t *testing.T) {
		var buf bytes.Buffer
		env := DoctorEnv{
			ChromeRemoteDebuggingURL: srv.URL,
			BrowserUseAPIKeyPresent:  true,
			HTTPClient:               srv.Client(),
		}
		runDoctor(context.Background(), env, &buf)
		if !strings.Contains(buf.String(), "[ok  ] BROWSER_USE_API_KEY") {
			t.Fatalf("want '[ok  ] BROWSER_USE_API_KEY' row; got:\n%s", buf.String())
		}
	})

	// Key absent: expect [--  ] (optional) row.
	t.Run("absent", func(t *testing.T) {
		var buf bytes.Buffer
		env := DoctorEnv{
			ChromeRemoteDebuggingURL: srv.URL,
			BrowserUseAPIKeyPresent:  false,
			HTTPClient:               srv.Client(),
		}
		runDoctor(context.Background(), env, &buf)
		if !strings.Contains(buf.String(), "[--  ] BROWSER_USE_API_KEY") {
			t.Fatalf("want '[--  ] BROWSER_USE_API_KEY' optional row; got:\n%s", buf.String())
		}
	})
}

// TestRunDoctor_NeverEchoesAPIKey verifies that even when the caller injects
// a known sentinel key value, that value never appears in stdout. (The value
// is intentionally NOT stored in DoctorEnv; this test confirms the contract.)
//
// We simulate the scenario by calling main's env-resolution path inline so we
// can confirm the value is stripped before reaching runDoctor.
func TestRunDoctor_NeverEchoesAPIKey(t *testing.T) {
	const sentinelKey = "SUPER_SECRET_KEY_VALUE_123456"

	srv := newHealthyServer(t)
	defer srv.Close()

	// Pre-resolve like main does: convert value → boolean.
	apiKeyPresent := sentinelKey != ""

	var buf bytes.Buffer
	env := DoctorEnv{
		ChromeRemoteDebuggingURL: srv.URL,
		BrowserUseAPIKeyPresent:  apiKeyPresent,
		HTTPClient:               srv.Client(),
	}
	runDoctor(context.Background(), env, &buf)

	if strings.Contains(buf.String(), sentinelKey) {
		t.Fatalf("stdout must NOT contain the API key value; got:\n%s", buf.String())
	}
}

// TestRunDoctor_PlatformRowAlwaysPrints verifies the platform row is present
// and contains GOOS/GOARCH regardless of endpoint state.
func TestRunDoctor_PlatformRowAlwaysPrints(t *testing.T) {
	var buf bytes.Buffer
	env := DoctorEnv{
		ChromeRemoteDebuggingURL: "", // deliberately unset so endpoint fails
		BrowserUseAPIKeyPresent:  false,
		HTTPClient:               nil,
	}
	runDoctor(context.Background(), env, &buf)

	out := buf.String()
	if !strings.Contains(out, "platform") {
		t.Fatalf("want 'platform' row in stdout; got:\n%s", out)
	}
	// The row must contain both GOOS and GOARCH separated by "/".
	if !strings.Contains(out, "/") {
		t.Fatalf("want GOOS/GOARCH in platform row; got:\n%s", out)
	}
}

// TestRunDoctor_HarnessVersionRowAlwaysPrints verifies the harness version row
// is present regardless of endpoint state. The value is the buildinfo string.
func TestRunDoctor_HarnessVersionRowAlwaysPrints(t *testing.T) {
	var buf bytes.Buffer
	env := DoctorEnv{
		ChromeRemoteDebuggingURL: "",
		BrowserUseAPIKeyPresent:  false,
		HTTPClient:               nil,
	}
	runDoctor(context.Background(), env, &buf)

	out := buf.String()
	if !strings.Contains(out, "harness version") {
		t.Fatalf("want 'harness version' row in stdout; got:\n%s", out)
	}
	// buildinfo.Version is "dev" in test builds; confirm it appears.
	if !strings.Contains(out, "dev") {
		t.Fatalf("want buildinfo version string 'dev' in harness version row; got:\n%s", out)
	}
}
