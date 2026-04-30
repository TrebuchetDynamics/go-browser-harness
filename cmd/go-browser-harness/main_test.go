package main

import (
	"context"
	"errors"
	"testing"

	"github.com/TrebuchetDynamics/go-browser-harness/pkg/harness"
)

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errWriteFailed
}

var errWriteFailed = errors.New("write failed")

func TestWriteLineReturnsWriterError(t *testing.T) {
	err := writeLine(failingWriter{}, "version")
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("writeLine error = %v, want %v", err, errWriteFailed)
	}
}

// TestResolveBackendFallsBackWhenNoCDPEndpoint ensures resolveBackend returns
// UnavailableBackend (not nil) when CHROME_REMOTE_DEBUGGING_URL is absent.
// No Chrome is started.
func TestResolveBackendFallsBackWhenNoCDPEndpoint(t *testing.T) {
	t.Setenv("CHROME_REMOTE_DEBUGGING_URL", "")
	backend := resolveBackend(context.Background())
	if backend == nil {
		t.Fatal("resolveBackend returned nil, want UnavailableBackend")
	}
	result, err := harness.RunAction(context.Background(), harness.ActionRequest{
		SchemaVersion: harness.ActionSchemaVersion,
		Kind:          "snapshot",
		TaskID:        "cli-test",
	}, backend)
	if err == nil {
		t.Fatal("want error from UnavailableBackend fallback, got nil")
	}
	if result.Evidence != harness.EvidenceBackendUnavailable {
		t.Fatalf("evidence = %q, want %q", result.Evidence, harness.EvidenceBackendUnavailable)
	}
}
