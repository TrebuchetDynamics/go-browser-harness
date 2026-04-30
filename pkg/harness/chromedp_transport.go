package harness

// chromedp_transport.go — live CDPTransport binding over chromedp/Browser.Execute.
//
// This file provides the production wiring between ChromedpBackend and a
// chromedp.Browser reached via the operator's CHROME_REMOTE_DEBUGGING_URL.
// Tests must NOT import this file's live paths; they use fakeCDPTransport instead.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/chromedp"
)

// chromedpLiveTransport wraps a chromedp context so it satisfies CDPTransport.
// The chromedp context must have been initialised by NewRemoteAllocator so it
// points at an operator-managed Chrome instance rather than launching one itself.
type chromedpLiveTransport struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// newChromedpLiveTransport dials the operator's Chrome remote debugging endpoint
// and returns a CDPTransport. It never launches Chrome itself.
// endpoint must be either a ws:// URL or an http://host:port URL for /json/version.
func newChromedpLiveTransport(parent context.Context, endpoint string) (CDPTransport, error) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(parent, endpoint)
	ctx, cancel := chromedp.NewContext(allocCtx)

	// Ensure the browser connection is live by running an empty action set.
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		allocCancel()
		return nil, fmt.Errorf("dial %q: %w", "[redacted]", err)
	}

	return &chromedpLiveTransport{
		ctx:    ctx,
		cancel: func() { cancel(); allocCancel() },
	}, nil
}

// SendCommand dispatches one CDP method through chromedp's Browser.Execute.
func (t *chromedpLiveTransport) SendCommand(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Merge the caller's deadline into the chromedp context.
	runCtx := t.ctx
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithDeadline(t.ctx, deadline)
		defer cancel()
	}

	c := chromedp.FromContext(runCtx)
	if c == nil || c.Browser == nil {
		return nil, fmt.Errorf("chromedp browser not initialised")
	}

	var result json.RawMessage
	if err := c.Browser.Execute(runCtx, method, params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
