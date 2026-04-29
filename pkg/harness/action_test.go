package harness_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/TrebuchetDynamics/go-browser-harness/pkg/harness"
)

func TestParseActionJSONValidatesGoNativeContract(t *testing.T) {
	req, err := harness.ParseActionJSON(`{"schema_version":"gormes.browser.action.v1","kind":"navigate","task_id":"task-1","url":"https://example.com"}`)
	if err != nil {
		t.Fatalf("ParseActionJSON returned error: %v", err)
	}
	if req.Kind != "navigate" || req.URL != "https://example.com" {
		t.Fatalf("request not decoded: %#v", req)
	}

	_, err = harness.ParseActionJSON(`{"schema_version":"gormes.browser.action.v1","kind":"navigate"}`)
	if err == nil || !strings.Contains(err.Error(), "requires url") {
		t.Fatalf("missing url err = %v, want navigate requires url", err)
	}

	_, err = harness.ParseActionJSON(`{"schema_version":"legacy","kind":"snapshot"}`)
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("schema err = %v, want schema_version failure", err)
	}
}

func TestRunActionUsesBackendAndFillsEnvelope(t *testing.T) {
	backend := &recordingBackend{result: harness.ActionResult{Title: "Example"}}
	result, err := harness.RunAction(context.Background(), harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "snapshot",
		TaskID:        "task-1",
	}, backend)
	if err != nil {
		t.Fatalf("RunAction returned error: %v", err)
	}
	if result.SchemaVersion != harness.ActionSchemaVersion || result.Evidence != harness.EvidenceActionAccepted {
		t.Fatalf("result envelope missing defaults: %#v", result)
	}
	if result.Kind != "snapshot" || result.TaskID != "task-1" || result.Title != "Example" {
		t.Fatalf("result not filled from request/backend: %#v", result)
	}
	if backend.calls != 1 {
		t.Fatalf("backend calls = %d, want 1", backend.calls)
	}
}

func TestUnavailableBackendReturnsTypedEvidence(t *testing.T) {
	result, err := harness.RunAction(context.Background(), harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "snapshot",
		TaskID:        "task-1",
	}, harness.UnavailableBackend{Reason: "no cdp"})
	if err == nil {
		t.Fatalf("RunAction returned nil error, want unavailable backend error")
	}
	if result.Evidence != harness.EvidenceBackendUnavailable || result.Message != "no cdp" {
		t.Fatalf("result = %#v, want unavailable evidence", result)
	}
	raw, marshalErr := json.Marshal(result)
	if marshalErr != nil || !json.Valid(raw) {
		t.Fatalf("result did not marshal as JSON: %v %s", marshalErr, raw)
	}
}

type recordingBackend struct {
	err    error
	result harness.ActionResult
	calls  int
}

func (b *recordingBackend) RunAction(context.Context, harness.ActionRequest) (harness.ActionResult, error) {
	b.calls++
	if b.err != nil {
		return b.result, b.err
	}
	return b.result, nil
}

var _ harness.Backend = (*recordingBackend)(nil)
