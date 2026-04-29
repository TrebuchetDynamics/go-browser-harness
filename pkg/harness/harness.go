// Package harness exposes a thin Go wrapper around Chrome DevTools Protocol
// browser sessions.
package harness

import (
	"context"

	"github.com/chromedp/chromedp"
)

// NewContext creates a new chromedp-backed browser context.
func NewContext(parent context.Context, opts ...chromedp.ContextOption) (context.Context, context.CancelFunc) {
	return chromedp.NewContext(parent, opts...)
}
