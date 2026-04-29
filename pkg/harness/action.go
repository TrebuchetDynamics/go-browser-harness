package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	ActionSchemaVersion = "gormes.browser.action.v1"

	EvidenceActionAccepted     = "go_browser_harness_action_accepted"
	EvidenceBackendUnavailable = "go_browser_harness_backend_unavailable"
	EvidenceInvalidAction      = "go_browser_harness_action_invalid"
)

// ActionRequest is the Go-native command contract consumed by Gormes. It
// deliberately describes browser intent as typed data instead of Python code.
type ActionRequest struct {
	SchemaVersion string         `json:"schema_version"`
	Kind          string         `json:"kind"`
	TaskID        string         `json:"task_id,omitempty"`
	URL           string         `json:"url,omitempty"`
	Ref           string         `json:"ref,omitempty"`
	Text          string         `json:"text,omitempty"`
	Key           string         `json:"key,omitempty"`
	Direction     string         `json:"direction,omitempty"`
	Expression    string         `json:"expression,omitempty"`
	Method        string         `json:"method,omitempty"`
	Params        map[string]any `json:"params,omitempty"`
	DialogAction  string         `json:"dialog_action,omitempty"`
	PromptText    string         `json:"prompt_text,omitempty"`
	Question      string         `json:"question,omitempty"`
	Full          bool           `json:"full,omitempty"`
	NewTab        bool           `json:"new_tab,omitempty"`
}

// ActionResult is the JSON result envelope printed by the CLI.
type ActionResult struct {
	SchemaVersion string         `json:"schema_version"`
	Evidence      string         `json:"evidence"`
	Kind          string         `json:"kind,omitempty"`
	TaskID        string         `json:"task_id,omitempty"`
	URL           string         `json:"url,omitempty"`
	Title         string         `json:"title,omitempty"`
	Text          string         `json:"text,omitempty"`
	Artifact      string         `json:"artifact,omitempty"`
	Message       string         `json:"message,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
	Interactive   []Element      `json:"interactive,omitempty"`
}

// Element is one visible page target that a caller can refer to by @eN.
type Element struct {
	Ref  string `json:"ref"`
	Tag  string `json:"tag,omitempty"`
	Role string `json:"role,omitempty"`
	Text string `json:"text,omitempty"`
	X    int    `json:"x,omitempty"`
	Y    int    `json:"y,omitempty"`
}

// Backend executes an accepted browser action. Live CDP wiring stays behind
// this interface so the CLI and package tests do not need a browser.
type Backend interface {
	RunAction(context.Context, ActionRequest) (ActionResult, error)
}

// UnavailableBackend is the safe default until a caller wires a live CDP
// backend. It returns typed evidence instead of silently pretending a browser
// action happened.
type UnavailableBackend struct {
	Reason string
}

func (b UnavailableBackend) RunAction(_ context.Context, req ActionRequest) (ActionResult, error) {
	reason := strings.TrimSpace(b.Reason)
	if reason == "" {
		reason = "backend unavailable"
	}
	result := ActionResult{
		SchemaVersion: ActionSchemaVersion,
		Evidence:      EvidenceBackendUnavailable,
		Kind:          normalizeActionKind(req.Kind),
		TaskID:        strings.TrimSpace(req.TaskID),
		Message:       reason,
	}
	return result, errors.New(reason)
}

func ParseActionJSON(raw string) (ActionRequest, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ActionRequest{}, errors.New("action json is required")
	}
	var req ActionRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return ActionRequest{}, fmt.Errorf("decode action json: %w", err)
	}
	return req, ValidateActionRequest(req)
}

func ValidateActionRequest(req ActionRequest) error {
	if req.SchemaVersion != ActionSchemaVersion {
		return fmt.Errorf("schema_version = %q, want %q", req.SchemaVersion, ActionSchemaVersion)
	}
	switch normalizeActionKind(req.Kind) {
	case "navigate":
		if strings.TrimSpace(req.URL) == "" {
			return errors.New("navigate requires url")
		}
	case "click":
		if strings.TrimSpace(req.Ref) == "" {
			return errors.New("click requires ref")
		}
	case "type":
		if strings.TrimSpace(req.Ref) == "" {
			return errors.New("type requires ref")
		}
	case "snapshot", "scroll", "back", "press", "console", "get_images", "vision", "cdp", "dialog":
		// Shape is accepted; concrete backends may enforce more detail.
	default:
		return fmt.Errorf("unsupported action kind %q", req.Kind)
	}
	return nil
}

func RunAction(ctx context.Context, req ActionRequest, backend Backend) (ActionResult, error) {
	if err := ValidateActionRequest(req); err != nil {
		return ActionResult{
			SchemaVersion: ActionSchemaVersion,
			Evidence:      EvidenceInvalidAction,
			Kind:          normalizeActionKind(req.Kind),
			TaskID:        strings.TrimSpace(req.TaskID),
			Message:       err.Error(),
		}, err
	}
	if backend == nil {
		backend = UnavailableBackend{Reason: "CDP backend not configured"}
	}
	result, err := backend.RunAction(ctx, req)
	if result.SchemaVersion == "" {
		result.SchemaVersion = ActionSchemaVersion
	}
	if result.Kind == "" {
		result.Kind = normalizeActionKind(req.Kind)
	}
	if result.TaskID == "" {
		result.TaskID = strings.TrimSpace(req.TaskID)
	}
	if result.Evidence == "" {
		result.Evidence = EvidenceActionAccepted
	}
	return result, err
}

func normalizeActionKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}
