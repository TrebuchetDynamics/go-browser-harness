package harness

// chromedp_backend.go — ChromedpBackend: the live CDP backend for the harness.
//
// Production path: NewChromedpBackend reads CHROME_REMOTE_DEBUGGING_URL and
// dials the operator's running Chrome over the DevTools Protocol.
// Test path: NewChromedpBackendFromTransport accepts a CDPTransport fake so
// the full dispatch loop can be exercised without a real browser.
//
// NEVER auto-launch Chrome. The operator starts Chrome with
//   --remote-debugging-port=9222
// and exports CHROME_REMOTE_DEBUGGING_URL=http://localhost:9222
// (or the full websocket ws://... URL from /json/version).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ScreenshotMaxBytes is the hard limit on embedded screenshot data in
// ActionResult.Artifact. Data above this size is replaced with a
// truncation marker. Vision callers should save large captures to disk
// and return only the artifact path.
const ScreenshotMaxBytes = 8 * 1024 // 8 KiB base64 ceiling for context safety

// Additional evidence constants specific to the chromedp backend.
const (
	// EvidenceActionTimeout is returned when the CDP context deadline fires.
	EvidenceActionTimeout = "go_browser_harness_action_timeout"
	// EvidenceScreenshotFailed is returned when Page.captureScreenshot fails.
	EvidenceScreenshotFailed = "go_browser_harness_screenshot_failed"
	// EvidenceActionInvalid is returned for stale/unknown @e refs.
	// (Re-exported alias — same value as EvidenceInvalidAction for callers
	// that import only this package's public surface.)
	EvidenceActionInvalid = EvidenceInvalidAction
)

// CDPTransport is the narrow interface ChromedpBackend uses to dispatch CDP
// commands. The live binding uses the chromedp package; test code uses a fake.
type CDPTransport interface {
	// SendCommand sends one CDP method call with the given params and returns
	// the raw JSON result. Params must be JSON-serialisable or nil.
	SendCommand(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
}

// ChromedpBackend dispatches ActionRequests through a CDPTransport.
type ChromedpBackend struct {
	transport CDPTransport
}

// NewChromedpBackend creates a production ChromedpBackend that dials the
// operator-provided endpoint. endpoint must be non-empty; use the
// CHROME_REMOTE_DEBUGGING_URL environment variable as the canonical source.
// Returns an error without attempting any network connection if endpoint is empty.
func NewChromedpBackend(ctx context.Context, endpoint string) (*ChromedpBackend, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, errors.New("go_browser_harness_backend_unavailable: CHROME_REMOTE_DEBUGGING_URL is not set")
	}
	transport, err := newChromedpLiveTransport(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("go_browser_harness_backend_unavailable: %w", err)
	}
	return &ChromedpBackend{transport: transport}, nil
}

// NewChromedpBackendFromEnv is a convenience constructor that reads
// CHROME_REMOTE_DEBUGGING_URL from the process environment.
func NewChromedpBackendFromEnv(ctx context.Context) (*ChromedpBackend, error) {
	return NewChromedpBackend(ctx, os.Getenv("CHROME_REMOTE_DEBUGGING_URL"))
}

// NewChromedpBackendFromTransport creates a ChromedpBackend using the supplied
// CDPTransport. Intended for tests; transport must not be nil.
func NewChromedpBackendFromTransport(transport CDPTransport) *ChromedpBackend {
	if transport == nil {
		panic("NewChromedpBackendFromTransport: transport must not be nil")
	}
	return &ChromedpBackend{transport: transport}
}

// RunAction dispatches req through the CDPTransport and returns an ActionResult.
// It implements the Backend interface.
func (b *ChromedpBackend) RunAction(ctx context.Context, req ActionRequest) (ActionResult, error) {
	kind := normalizeActionKind(req.Kind)

	// Detect context cancellation / deadline before dispatching.
	if err := ctx.Err(); err != nil {
		return b.timeoutResult(req), fmt.Errorf("%s: context already done: %w", kind, err)
	}

	switch kind {
	case "navigate":
		return b.runNavigate(ctx, req)
	case "snapshot":
		return b.runSnapshot(ctx, req)
	case "click":
		return b.runClick(ctx, req)
	case "type":
		return b.runType(ctx, req)
	case "scroll":
		return b.runScroll(ctx, req)
	case "back":
		return b.runBack(ctx, req)
	case "press":
		return b.runPress(ctx, req)
	case "console":
		return b.runConsole(ctx, req)
	case "get_images":
		return b.runGetImages(ctx, req)
	case "vision":
		return b.runVision(ctx, req)
	case "cdp":
		return b.runCDP(ctx, req)
	case "dialog":
		return b.runDialog(ctx, req)
	default:
		return ActionResult{
			SchemaVersion: ActionSchemaVersion,
			Evidence:      EvidenceInvalidAction,
			Kind:          kind,
			TaskID:        req.TaskID,
			Message:       fmt.Sprintf("unsupported action kind %q", kind),
		}, fmt.Errorf("unsupported action kind %q", kind)
	}
}

// ---------------------------------------------------------------------------
// Action handlers
// ---------------------------------------------------------------------------

func (b *ChromedpBackend) runNavigate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// browser_navigate MUST open a new tab via Target.createTarget so the
	// operator's active tab is never clobbered. Then activate it and navigate.
	createParams := map[string]interface{}{"url": "about:blank"}
	createResp, err := b.send(ctx, "Target.createTarget", createParams)
	if err != nil {
		return b.wrapErr(req, err)
	}
	var createResult struct {
		TargetID string `json:"targetId"`
	}
	_ = json.Unmarshal(createResp, &createResult)

	if createResult.TargetID != "" {
		activateParams := map[string]interface{}{"targetId": createResult.TargetID}
		if _, err := b.send(ctx, "Target.activateTarget", activateParams); err != nil {
			return b.wrapErr(req, err)
		}
	}

	navParams := map[string]interface{}{"url": req.URL}
	if _, err := b.send(ctx, "Page.navigate", navParams); err != nil {
		return b.wrapErr(req, err)
	}

	return b.takeSnapshot(ctx, req)
}

func (b *ChromedpBackend) runSnapshot(ctx context.Context, req ActionRequest) (ActionResult, error) {
	return b.takeSnapshot(ctx, req)
}

func (b *ChromedpBackend) runClick(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Resolve the @e ref to x,y coordinates via JS evaluation.
	coords, err := b.resolveRef(ctx, req.Ref)
	if err != nil {
		return b.invalidRefResult(req, req.Ref), err
	}
	// Dispatch mouse click.
	mouseParams := map[string]interface{}{
		"type":       "mousePressed",
		"x":          coords[0],
		"y":          coords[1],
		"button":     "left",
		"clickCount": 1,
	}
	if _, err := b.send(ctx, "Input.dispatchMouseEvent", mouseParams); err != nil {
		return b.wrapErr(req, err)
	}
	mouseParams["type"] = "mouseReleased"
	if _, err := b.send(ctx, "Input.dispatchMouseEvent", mouseParams); err != nil {
		return b.wrapErr(req, err)
	}
	return b.takeSnapshot(ctx, req)
}

func (b *ChromedpBackend) runType(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Focus the element via ref resolution, then dispatch key events.
	_, err := b.resolveRef(ctx, req.Ref)
	if err != nil {
		return b.invalidRefResult(req, req.Ref), err
	}
	for _, ch := range req.Text {
		keyParams := map[string]interface{}{
			"type": "keyDown",
			"key":  string(ch),
			"text": string(ch),
		}
		if _, err := b.send(ctx, "Input.dispatchKeyEvent", keyParams); err != nil {
			return b.wrapErr(req, err)
		}
		keyParams["type"] = "keyUp"
		if _, err := b.send(ctx, "Input.dispatchKeyEvent", keyParams); err != nil {
			return b.wrapErr(req, err)
		}
	}
	return b.takeSnapshot(ctx, req)
}

func (b *ChromedpBackend) runScroll(ctx context.Context, req ActionRequest) (ActionResult, error) {
	dy := 600.0
	if strings.ToLower(strings.TrimSpace(req.Direction)) == "up" {
		dy = -600.0
	}
	mouseParams := map[string]interface{}{
		"type":      "mouseWheel",
		"x":         500.0,
		"y":         400.0,
		"deltaX":    0.0,
		"deltaY":    dy,
	}
	if _, err := b.send(ctx, "Input.dispatchMouseEvent", mouseParams); err != nil {
		return b.wrapErr(req, err)
	}
	return b.takeSnapshot(ctx, req)
}

func (b *ChromedpBackend) runBack(ctx context.Context, req ActionRequest) (ActionResult, error) {
	evalParams := map[string]interface{}{
		"expression": "history.back(); void 0",
	}
	if _, err := b.send(ctx, "Runtime.evaluate", evalParams); err != nil {
		return b.wrapErr(req, err)
	}
	return b.takeSnapshot(ctx, req)
}

func (b *ChromedpBackend) runPress(ctx context.Context, req ActionRequest) (ActionResult, error) {
	keyParams := map[string]interface{}{
		"type": "keyDown",
		"key":  req.Key,
	}
	if _, err := b.send(ctx, "Input.dispatchKeyEvent", keyParams); err != nil {
		return b.wrapErr(req, err)
	}
	keyParams["type"] = "keyUp"
	if _, err := b.send(ctx, "Input.dispatchKeyEvent", keyParams); err != nil {
		return b.wrapErr(req, err)
	}
	return b.takeSnapshot(ctx, req)
}

func (b *ChromedpBackend) runConsole(ctx context.Context, req ActionRequest) (ActionResult, error) {
	expr := strings.TrimSpace(req.Expression)
	if expr == "" {
		expr = "void 0"
	}
	evalParams := map[string]interface{}{
		"expression":    expr,
		"returnByValue": true,
	}
	raw, err := b.send(ctx, "Runtime.evaluate", evalParams)
	if err != nil {
		return b.wrapErr(req, err)
	}
	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionAccepted,
		Kind:          "console",
		TaskID:        req.TaskID,
		Data:          map[string]any{"result": string(raw)},
	}, nil
}

func (b *ChromedpBackend) runGetImages(ctx context.Context, req ActionRequest) (ActionResult, error) {
	imagesJS := `Array.from(document.images).slice(0,200).map((img,i)=>({index:i,src:img.currentSrc||img.src||'',alt:img.alt||'',width:img.naturalWidth||img.width||0,height:img.naturalHeight||img.height||0}))`
	evalParams := map[string]interface{}{
		"expression":    imagesJS,
		"returnByValue": true,
	}
	raw, err := b.send(ctx, "Runtime.evaluate", evalParams)
	if err != nil {
		return b.wrapErr(req, err)
	}
	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionAccepted,
		Kind:          "get_images",
		TaskID:        req.TaskID,
		Data:          map[string]any{"result": string(raw)},
	}, nil
}

func (b *ChromedpBackend) runVision(ctx context.Context, req ActionRequest) (ActionResult, error) {
	ssParams := map[string]interface{}{
		"format": "jpeg",
		"quality": 70,
	}
	raw, err := b.send(ctx, "Page.captureScreenshot", ssParams)
	if err != nil {
		return ActionResult{
			SchemaVersion: ActionSchemaVersion,
			Evidence:      EvidenceScreenshotFailed,
			Kind:          "vision",
			TaskID:        req.TaskID,
			Message:       "Page.captureScreenshot failed",
		}, fmt.Errorf("Page.captureScreenshot: %w", err)
	}
	// Extract the base64 data field.
	var ssResult struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(raw, &ssResult)

	const truncSuffix = "...[truncated]"
	artifact := ssResult.Data
	if len(artifact) > ScreenshotMaxBytes {
		cutAt := ScreenshotMaxBytes - len(truncSuffix)
		if cutAt < 0 {
			cutAt = 0
		}
		artifact = artifact[:cutAt] + truncSuffix
	}
	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionAccepted,
		Kind:          "vision",
		TaskID:        req.TaskID,
		Artifact:      artifact,
		Message:       req.Question,
	}, nil
}

func (b *ChromedpBackend) runCDP(ctx context.Context, req ActionRequest) (ActionResult, error) {
	method := strings.TrimSpace(req.Method)
	if method == "" {
		return ActionResult{
			SchemaVersion: ActionSchemaVersion,
			Evidence:      EvidenceInvalidAction,
			Kind:          "cdp",
			TaskID:        req.TaskID,
			Message:       "cdp requires method",
		}, errors.New("cdp requires method")
	}
	raw, err := b.send(ctx, method, req.Params)
	if err != nil {
		return b.wrapErr(req, err)
	}
	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionAccepted,
		Kind:          "cdp",
		TaskID:        req.TaskID,
		Data:          map[string]any{"result": string(raw)},
	}, nil
}

func (b *ChromedpBackend) runDialog(ctx context.Context, req ActionRequest) (ActionResult, error) {
	accept := true
	if strings.ToLower(strings.TrimSpace(req.DialogAction)) == "dismiss" {
		accept = false
	}
	dialogParams := map[string]interface{}{"accept": accept}
	if req.PromptText != "" {
		dialogParams["promptText"] = req.PromptText
	}
	if _, err := b.send(ctx, "Page.handleJavaScriptDialog", dialogParams); err != nil {
		return b.wrapErr(req, err)
	}
	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionAccepted,
		Kind:          "dialog",
		TaskID:        req.TaskID,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// snapshotJS returns the JS expression that produces a compact page snapshot.
const snapshotJS = `(() => {
const selector = 'a,button,input,textarea,select,[role="button"],[role="link"],[onclick],[tabindex]:not([tabindex="-1"])';
const visible = (el) => {
  const r = el.getBoundingClientRect();
  const s = getComputedStyle(el);
  return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none';
};
const label = (el) => (el.innerText || el.getAttribute('aria-label') || el.getAttribute('title') || el.getAttribute('placeholder') || el.value || el.href || '').trim().replace(/\s+/g, ' ').slice(0, 180);
const nodes = Array.from(document.querySelectorAll(selector)).filter(visible).slice(0, 100);
const interactive = nodes.map((el, i) => {
  const r = el.getBoundingClientRect();
  return {ref: '@e' + (i + 1), tag: el.tagName.toLowerCase(), role: el.getAttribute('role') || '', text: label(el), x: Math.round(r.left + r.width / 2), y: Math.round(r.top + r.height / 2)};
});
const bodyText = ((document.body && document.body.innerText) || '').trim().replace(/\n{3,}/g, '\n\n').slice(0, 4000);
return {url: location.href, title: document.title, text: bodyText, interactive};
})()`

// refCenterJS resolves an @eN ref to {x, y} or null.
const refCenterJS = `((wanted) => {
const n = Number(String(wanted).replace(/^@?e/, '')) - 1;
const selector = 'a,button,input,textarea,select,[role="button"],[role="link"],[onclick],[tabindex]:not([tabindex="-1"])';
const visible = (el) => {
  const r = el.getBoundingClientRect();
  const s = getComputedStyle(el);
  return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none';
};
const nodes = Array.from(document.querySelectorAll(selector)).filter(visible).slice(0, 100);
const el = nodes[n];
if (!el) return null;
el.scrollIntoView({block: 'center', inline: 'center'});
const r = el.getBoundingClientRect();
return {x: Math.round(r.left + r.width / 2), y: Math.round(r.top + r.height / 2)};
})`

func (b *ChromedpBackend) takeSnapshot(ctx context.Context, req ActionRequest) (ActionResult, error) {
	evalParams := map[string]interface{}{
		"expression":    snapshotJS,
		"returnByValue": true,
	}
	raw, err := b.send(ctx, "Runtime.evaluate", evalParams)
	if err != nil {
		return b.wrapErr(req, err)
	}

	// Parse Runtime.evaluate result envelope: {"result":{"value":{...}}}
	var evalResult struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	if jsonErr := json.Unmarshal(raw, &evalResult); jsonErr != nil || len(evalResult.Result.Value) == 0 {
		// Fallback: maybe the fake returns the value directly.
		evalResult.Result.Value = raw
	}

	var snapshot struct {
		URL         string    `json:"url"`
		Title       string    `json:"title"`
		Text        string    `json:"text"`
		Interactive []Element `json:"interactive"`
	}
	_ = json.Unmarshal(evalResult.Result.Value, &snapshot)

	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionAccepted,
		Kind:          normalizeActionKind(req.Kind),
		TaskID:        req.TaskID,
		URL:           snapshot.URL,
		Title:         snapshot.Title,
		Text:          snapshot.Text,
		Interactive:   snapshot.Interactive,
	}, nil
}

func (b *ChromedpBackend) resolveRef(ctx context.Context, ref string) ([2]float64, error) {
	expr := refCenterJS + "(" + jsonStringLiteral(ref) + ")"
	evalParams := map[string]interface{}{
		"expression":    expr,
		"returnByValue": true,
	}
	raw, err := b.send(ctx, "Runtime.evaluate", evalParams)
	if err != nil {
		return [2]float64{}, err
	}

	// Parse {"result":{"value":{x,y}}} or value directly.
	var evalResult struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	valueRaw := raw
	if jsonErr := json.Unmarshal(raw, &evalResult); jsonErr == nil && len(evalResult.Result.Value) > 0 {
		valueRaw = evalResult.Result.Value
	}

	// null → ref not found.
	if string(valueRaw) == "null" || len(valueRaw) == 0 {
		return [2]float64{}, fmt.Errorf("ref %q not found in current snapshot", ref)
	}

	var coords struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := json.Unmarshal(valueRaw, &coords); err != nil {
		return [2]float64{}, fmt.Errorf("ref %q: unexpected coords shape", ref)
	}
	return [2]float64{coords.X, coords.Y}, nil
}

func (b *ChromedpBackend) send(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	raw, err := b.transport.SendCommand(ctx, method, params)
	if err != nil {
		if isTimeoutError(ctx, err) {
			return nil, fmt.Errorf("timeout: %w", err)
		}
		return nil, err
	}
	return raw, nil
}

func (b *ChromedpBackend) wrapErr(req ActionRequest, err error) (ActionResult, error) {
	if isTimeoutError(nil, err) {
		return b.timeoutResult(req), err
	}
	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionInvalid,
		Kind:          normalizeActionKind(req.Kind),
		TaskID:        req.TaskID,
		Message:       sanitizeCDPError(err),
	}, err
}

func (b *ChromedpBackend) timeoutResult(req ActionRequest) ActionResult {
	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionTimeout,
		Kind:          normalizeActionKind(req.Kind),
		TaskID:        req.TaskID,
		Message:       "action timed out",
	}
}

func (b *ChromedpBackend) invalidRefResult(req ActionRequest, ref string) ActionResult {
	return ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceActionInvalid,
		Kind:          normalizeActionKind(req.Kind),
		TaskID:        req.TaskID,
		Message:       fmt.Sprintf("ref %q not found; take a fresh snapshot", sanitizeCDPRef(ref)),
	}
}

func isTimeoutError(ctx context.Context, err error) bool {
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// sanitizeCDPError strips URLs, credentials, and CDP endpoint details from an
// error message before surfacing it in ActionResult.Message.
func sanitizeCDPError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Replace ws:// / http:// URLs.
	if idx := strings.Index(msg, "ws://"); idx >= 0 {
		msg = msg[:idx] + "[redacted]"
	}
	if idx := strings.Index(msg, "http://"); idx >= 0 {
		msg = msg[:idx] + "[redacted]"
	}
	if len(msg) > 512 {
		msg = msg[:512] + "...[truncated]"
	}
	return msg
}

// sanitizeCDPRef returns the ref value for error messages — kept as-is since
// @eN refs contain no secret data, only structural identifiers.
func sanitizeCDPRef(ref string) string {
	if len(ref) > 32 {
		return ref[:32]
	}
	return ref
}

func jsonStringLiteral(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
