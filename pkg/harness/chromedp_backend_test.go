package harness_test

// chromedp_backend_test.go — RED/GREEN tests for the chromedp action backend.
//
// All tests are hermetic: no real browser is started, no DNS is resolved, no
// Chrome process is launched. A fakeCDPTransport stands in for the live CDP
// websocket so the full dispatch loop can be exercised without a running browser.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TrebuchetDynamics/go-browser-harness/pkg/harness"
)

// ---------------------------------------------------------------------------
// Fake CDPTransport
// ---------------------------------------------------------------------------

// fakeCDPTransport records every SendCommand call and returns pre-loaded
// responses keyed by CDP method name.
type fakeCDPTransport struct {
	mu        sync.Mutex
	calls     []fakeCDPCall
	responses map[string]json.RawMessage
	errors    map[string]error
	hangOn    string // if non-empty, block forever on this method
}

type fakeCDPCall struct {
	Method string
	Params json.RawMessage
}

func newFakeCDP(responses map[string]json.RawMessage) *fakeCDPTransport {
	if responses == nil {
		responses = map[string]json.RawMessage{}
	}
	return &fakeCDPTransport{responses: responses, errors: map[string]error{}}
}

func (f *fakeCDPTransport) SendCommand(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	f.mu.Lock()
	raw, _ := json.Marshal(params)
	f.calls = append(f.calls, fakeCDPCall{Method: method, Params: raw})
	hangOn := f.hangOn
	resp := f.responses[method]
	errVal := f.errors[method]
	f.mu.Unlock()

	if hangOn == method {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if errVal != nil {
		return nil, errVal
	}
	if resp != nil {
		return resp, nil
	}
	// Default: return empty object so callers do not fail on missing fields.
	return json.RawMessage(`{}`), nil
}

func (f *fakeCDPTransport) CalledMethods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	for i, c := range f.calls {
		out[i] = c.Method
	}
	return out
}

func (f *fakeCDPTransport) CalledOnce(method string) bool {
	for _, m := range f.CalledMethods() {
		if m == method {
			return true
		}
	}
	return false
}

var _ harness.CDPTransport = (*fakeCDPTransport)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func navigateResult() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"Target.createTarget":   json.RawMessage(`{"targetId":"fake-target-1"}`),
		"Target.activateTarget": json.RawMessage(`{}`),
		"Page.navigate":         json.RawMessage(`{"frameId":"frame-1"}`),
		"Runtime.evaluate":      json.RawMessage(`{"result":{"value":{"url":"https://example.com","title":"Example","text":"hello","interactive":[{"ref":"@e1","tag":"a","role":"","text":"Docs","x":100,"y":200}]}}}`),
	}
}

func snapshotResult() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"Runtime.evaluate": json.RawMessage(`{"result":{"value":{"url":"https://example.com","title":"Example","text":"page content","interactive":[{"ref":"@e1","tag":"button","role":"","text":"Submit","x":50,"y":100}]}}}`),
	}
}

// ---------------------------------------------------------------------------
// TestAction_BackendUnavailableEvidence_NoCDPEndpoint
// ---------------------------------------------------------------------------

func TestAction_BackendUnavailableEvidence_NoCDPEndpoint(t *testing.T) {
	// No CHROME_REMOTE_DEBUGGING_URL → NewChromedpBackend must fail with a
	// typed unavailable result without attempting to connect.
	backend, err := harness.NewChromedpBackend(context.Background(), "")
	if err == nil {
		t.Fatal("NewChromedpBackend with empty endpoint: want error, got nil")
	}
	if backend != nil {
		t.Fatal("NewChromedpBackend with empty endpoint: want nil backend")
	}

	// Running an action through UnavailableBackend (the zero-endpoint sentinel)
	// must surface go_browser_harness_backend_unavailable without starting Chrome.
	req := harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "snapshot",
		TaskID:        "task-nocdp",
	}
	result, runErr := harness.RunAction(context.Background(), req, harness.UnavailableBackend{Reason: "no CHROME_REMOTE_DEBUGGING_URL"})
	if runErr == nil {
		t.Fatal("RunAction through UnavailableBackend: want error, got nil")
	}
	if result.Evidence != harness.EvidenceBackendUnavailable {
		t.Fatalf("evidence = %q, want %q", result.Evidence, harness.EvidenceBackendUnavailable)
	}
	if strings.Contains(result.Message, "ws://") || strings.Contains(result.Message, "http://") {
		t.Fatalf("result.Message leaks a URL: %q", result.Message)
	}
	raw, _ := json.Marshal(result)
	if !json.Valid(raw) {
		t.Fatalf("result is not valid JSON: %s", raw)
	}
}

// ---------------------------------------------------------------------------
// TestAction_NavigateOpensNewTab_NotClobbersOperatorTab
// ---------------------------------------------------------------------------

func TestAction_NavigateOpensNewTab_NotClobbersOperatorTab(t *testing.T) {
	fake := newFakeCDP(navigateResult())
	backend := harness.NewChromedpBackendFromTransport(fake)

	req := harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "navigate",
		TaskID:        "task-newtab",
		URL:           "https://example.com",
		NewTab:        true,
	}
	result, err := harness.RunAction(context.Background(), req, backend)
	if err != nil {
		t.Fatalf("RunAction navigate: %v", err)
	}
	// Must have called Target.createTarget (new tab) — not navigate on existing.
	if !fake.CalledOnce("Target.createTarget") {
		t.Fatalf("expected Target.createTarget call; got methods: %v", fake.CalledMethods())
	}
	if result.Evidence != harness.EvidenceActionAccepted {
		t.Fatalf("evidence = %q, want %q", result.Evidence, harness.EvidenceActionAccepted)
	}
	if result.TaskID != "task-newtab" {
		t.Fatalf("TaskID = %q, want task-newtab", result.TaskID)
	}
}

// ---------------------------------------------------------------------------
// TestAction_BU_NAMETaskNamespacePreserved
// ---------------------------------------------------------------------------

func TestAction_BU_NAMETaskNamespacePreserved(t *testing.T) {
	tasks := []string{"task-alpha", "task-beta", "task-gamma"}
	for _, tid := range tasks {
		fake := newFakeCDP(snapshotResult())
		backend := harness.NewChromedpBackendFromTransport(fake)
		req := harness.ActionRequest{
			SchemaVersion: harness.ActionSchemaVersion,
			Kind:          "snapshot",
			TaskID:        tid,
		}
		result, err := harness.RunAction(context.Background(), req, backend)
		if err != nil {
			t.Fatalf("[%s] RunAction: %v", tid, err)
		}
		if result.TaskID != tid {
			t.Fatalf("[%s] result.TaskID = %q, want %q", tid, result.TaskID, tid)
		}
	}
}

// ---------------------------------------------------------------------------
// TestAction_SnapshotClickTypeScrollBackPress_DispatchToBackend
// ---------------------------------------------------------------------------

func TestAction_SnapshotClickTypeScrollBackPress_DispatchToBackend(t *testing.T) {
	type testCase struct {
		kind       string
		req        harness.ActionRequest
		wantMethod string
	}
	tests := []testCase{
		{
			kind: "snapshot",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "snapshot",
				TaskID:        "t1",
			},
			wantMethod: "Runtime.evaluate",
		},
		{
			kind: "click",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "click",
				TaskID:        "t1",
				Ref:           "@e1",
			},
			wantMethod: "Runtime.evaluate",
		},
		{
			kind: "type",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "type",
				TaskID:        "t1",
				Ref:           "@e1",
				Text:          "hello",
			},
			wantMethod: "Input.dispatchKeyEvent",
		},
		{
			kind: "scroll",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "scroll",
				TaskID:        "t1",
				Direction:     "down",
			},
			wantMethod: "Input.dispatchMouseEvent",
		},
		{
			kind: "back",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "back",
				TaskID:        "t1",
			},
			wantMethod: "Runtime.evaluate",
		},
		{
			kind: "press",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "press",
				TaskID:        "t1",
				Key:           "Enter",
			},
			wantMethod: "Input.dispatchKeyEvent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			fake := newFakeCDP(map[string]json.RawMessage{
				"Runtime.evaluate":      json.RawMessage(`{"result":{"value":{"url":"https://example.com","title":"t","text":"","interactive":[]}}}`),
				"Input.dispatchMouseEvent": json.RawMessage(`{}`),
				"Input.dispatchKeyEvent": json.RawMessage(`{}`),
			})
			backend := harness.NewChromedpBackendFromTransport(fake)
			result, err := harness.RunAction(context.Background(), tc.req, backend)
			if err != nil {
				t.Fatalf("[%s] RunAction: %v", tc.kind, err)
			}
			if !fake.CalledOnce(tc.wantMethod) {
				t.Fatalf("[%s] expected CDP method %q; called: %v", tc.kind, tc.wantMethod, fake.CalledMethods())
			}
			if result.Evidence != harness.EvidenceActionAccepted {
				t.Fatalf("[%s] evidence = %q, want %q", tc.kind, result.Evidence, harness.EvidenceActionAccepted)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestAction_ConsoleGetImagesVisionCDPDialog_DispatchToBackend
// ---------------------------------------------------------------------------

func TestAction_ConsoleGetImagesVisionCDPDialog_DispatchToBackend(t *testing.T) {
	type testCase struct {
		kind       string
		req        harness.ActionRequest
		wantMethod string
	}
	tests := []testCase{
		{
			kind: "console",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "console",
				TaskID:        "t1",
				Expression:    "document.title",
			},
			wantMethod: "Runtime.evaluate",
		},
		{
			kind: "get_images",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "get_images",
				TaskID:        "t1",
			},
			wantMethod: "Runtime.evaluate",
		},
		{
			kind: "vision",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "vision",
				TaskID:        "t1",
				Question:      "what is on screen?",
			},
			wantMethod: "Page.captureScreenshot",
		},
		{
			kind: "cdp",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "cdp",
				TaskID:        "t1",
				Method:        "Target.getTargets",
				Params:        map[string]any{"discover": true},
			},
			wantMethod: "Target.getTargets",
		},
		{
			kind: "dialog",
			req: harness.ActionRequest{
				SchemaVersion: harness.ActionSchemaVersion,
				Kind:          "dialog",
				TaskID:        "t1",
				DialogAction:  "accept",
			},
			wantMethod: "Page.handleJavaScriptDialog",
		},
	}

	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			fake := newFakeCDP(map[string]json.RawMessage{
				"Runtime.evaluate":          json.RawMessage(`{"result":{"value":"test-result"}}`),
				"Page.captureScreenshot":    json.RawMessage(`{"data":"aGVsbG8="}`),
				"Target.getTargets":         json.RawMessage(`{"targetInfos":[]}`),
				"Page.handleJavaScriptDialog": json.RawMessage(`{}`),
			})
			backend := harness.NewChromedpBackendFromTransport(fake)
			result, err := harness.RunAction(context.Background(), tc.req, backend)
			if err != nil {
				t.Fatalf("[%s] RunAction: %v", tc.kind, err)
			}
			if !fake.CalledOnce(tc.wantMethod) {
				t.Fatalf("[%s] expected CDP method %q; called: %v", tc.kind, tc.wantMethod, fake.CalledMethods())
			}
			if result.Evidence != harness.EvidenceActionAccepted {
				t.Fatalf("[%s] evidence = %q, want %q", tc.kind, result.Evidence, harness.EvidenceActionAccepted)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestAction_ResultEnvelope_ContainsAtERefsAndBoundedScreenshot
// ---------------------------------------------------------------------------

func TestAction_ResultEnvelope_ContainsAtERefsAndBoundedScreenshot(t *testing.T) {
	fake := newFakeCDP(map[string]json.RawMessage{
		"Runtime.evaluate": json.RawMessage(`{"result":{"value":{"url":"https://example.com","title":"Example","text":"page content","inter active":[],"interactive":[{"ref":"@e1","tag":"a","role":"link","text":"Docs","x":100,"y":200},{"ref":"@e2","tag":"button","role":"button","text":"Submit","x":200,"y":300}]}}}`),
	})
	backend := harness.NewChromedpBackendFromTransport(fake)
	req := harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "snapshot",
		TaskID:        "envelope-test",
	}
	result, err := harness.RunAction(context.Background(), req, backend)
	if err != nil {
		t.Fatalf("RunAction: %v", err)
	}
	// Result must have @e refs.
	if len(result.Interactive) == 0 {
		t.Fatal("result.Interactive is empty, want @e refs")
	}
	if !strings.HasPrefix(result.Interactive[0].Ref, "@e") {
		t.Fatalf("first ref = %q, want @e prefix", result.Interactive[0].Ref)
	}

	// Screenshot bounding: vision action must not embed > screenshotMaxBytes raw data.
	screenshotFake := newFakeCDP(map[string]json.RawMessage{
		"Page.captureScreenshot": json.RawMessage(`{"data":"` + strings.Repeat("A", 10000) + `"}`),
	})
	screenshotBackend := harness.NewChromedpBackendFromTransport(screenshotFake)
	visionReq := harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "vision",
		TaskID:        "screenshot-bound-test",
		Question:      "describe page",
	}
	visionResult, err := harness.RunAction(context.Background(), visionReq, screenshotBackend)
	if err != nil {
		t.Fatalf("vision RunAction: %v", err)
	}
	// The raw base64 data must NOT be embedded verbatim — it must be bounded/artifact-referenced.
	if len(visionResult.Artifact) > harness.ScreenshotMaxBytes {
		t.Fatalf("screenshot artifact len %d exceeds ScreenshotMaxBytes %d", len(visionResult.Artifact), harness.ScreenshotMaxBytes)
	}
}

// ---------------------------------------------------------------------------
// TestAction_InvalidAtERefReturnsTypedEvidence
// ---------------------------------------------------------------------------

func TestAction_InvalidAtERefReturnsTypedEvidence(t *testing.T) {
	// A stale/unknown @e ref: Runtime.evaluate returns null for ref resolution.
	fake := newFakeCDP(map[string]json.RawMessage{
		"Runtime.evaluate": json.RawMessage(`{"result":{"value":null}}`),
	})
	backend := harness.NewChromedpBackendFromTransport(fake)
	req := harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "click",
		TaskID:        "task-stale-ref",
		Ref:           "@e999",
	}
	result, err := harness.RunAction(context.Background(), req, backend)
	if err == nil {
		t.Fatal("RunAction with stale ref: want error, got nil")
	}
	if result.Evidence != harness.EvidenceActionInvalid {
		t.Fatalf("evidence = %q, want %q", result.Evidence, harness.EvidenceActionInvalid)
	}
	// Must not leak the ref-tree contents or a CDP URL.
	if strings.Contains(result.Message, "ws://") || strings.Contains(result.Message, "http://") {
		t.Fatalf("result.Message leaks a URL: %q", result.Message)
	}
}

// ---------------------------------------------------------------------------
// TestAction_TimeoutReturnsTypedEvidence
// ---------------------------------------------------------------------------

func TestAction_TimeoutReturnsTypedEvidence(t *testing.T) {
	fake := newFakeCDP(nil)
	fake.hangOn = "Runtime.evaluate" // hang the first call

	backend := harness.NewChromedpBackendFromTransport(fake)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "snapshot",
		TaskID:        "task-timeout",
	}
	result, err := harness.RunAction(ctx, req, backend)
	if err == nil {
		t.Fatal("RunAction with timeout: want error, got nil")
	}
	if result.Evidence != harness.EvidenceActionTimeout {
		t.Fatalf("evidence = %q, want %q", result.Evidence, harness.EvidenceActionTimeout)
	}
}

// ---------------------------------------------------------------------------
// TestAction_ScreenshotFailureReturnsTypedEvidence
// ---------------------------------------------------------------------------

func TestAction_ScreenshotFailureReturnsTypedEvidence(t *testing.T) {
	fake := newFakeCDP(nil)
	fake.errors["Page.captureScreenshot"] = errors.New("synthetic screenshot failure")

	backend := harness.NewChromedpBackendFromTransport(fake)
	req := harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "vision",
		TaskID:        "task-screenshot-fail",
		Question:      "what is visible?",
	}
	result, err := harness.RunAction(context.Background(), req, backend)
	if err == nil {
		t.Fatal("RunAction with screenshot failure: want error, got nil")
	}
	if result.Evidence != harness.EvidenceScreenshotFailed {
		t.Fatalf("evidence = %q, want %q", result.Evidence, harness.EvidenceScreenshotFailed)
	}
	if strings.Contains(result.Message, "ws://") || strings.Contains(result.Message, "http://") {
		t.Fatalf("result.Message leaks a URL: %q", result.Message)
	}
}
