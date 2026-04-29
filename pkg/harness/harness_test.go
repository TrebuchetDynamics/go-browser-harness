package harness_test

import (
	"context"
	"errors"
	"testing"

	"github.com/TrebuchetDynamics/go-browser-harness/pkg/harness"
)

func TestNewContextHonorsParentCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	ctx, cancelContext := harness.NewContext(parent)
	defer cancelContext()

	cancelParent()

	if err := ctx.Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("ctx.Err() = %v, want %v", err, context.Canceled)
	}
}
