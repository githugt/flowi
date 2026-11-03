// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tracing keeps the window's OpenTelemetry exporter, which can be
// switched on, off or pointed elsewhere while the app runs. The workspace
// takes its Observe and Traceparent hooks once, at open; the Holder is what
// they call, and it forwards to whichever exporter is current.
package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/otel"
)

// retireTimeout is how long a replaced exporter gets to send what it queued.
const retireTimeout = 5 * time.Second

type active struct {
	cfg otel.Config
	exp *otel.Exporter
}

// Holder forwards engine events to the current exporter, if there is one.
// Observe, Traceparent and Status are safe from any goroutine; Apply and
// Close take turns.
type Holder struct {
	log *slog.Logger
	cur atomic.Pointer[active]

	mu      sync.Mutex // serialises Apply and Close
	retired sync.WaitGroup
}

// NewHolder returns a holder that exports nothing until Apply switches it on.
func NewHolder(log *slog.Logger) *Holder { return &Holder{log: log} }

// Observe matches flowi.WorkspaceOptions.Observe.
func (h *Holder) Observe(ev engine.Event) {
	if a := h.cur.Load(); a != nil {
		a.exp.Observe(ev)
	}
}

// Traceparent matches flowi.WorkspaceOptions.Traceparent.
func (h *Holder) Traceparent(runID, path string) string {
	if a := h.cur.Load(); a != nil {
		return a.exp.Traceparent(runID, path)
	}
	return ""
}

// Status is the current exporter's progress; on is false when tracing is off.
func (h *Holder) Status() (st otel.Status, on bool) {
	a := h.cur.Load()
	if a == nil {
		return otel.Status{}, false
	}
	return a.exp.Status(), true
}

// Apply makes cfg the current configuration: an exporter for it when on, none
// when off. The same configuration again keeps the running exporter, so its
// counts survive a settings page that saves unrelated values. A config the
// exporter refuses leaves the running one in place and returns the reason.
// The exporter it replaces finishes sending in the background.
func (h *Holder) Apply(cfg otel.Config, on bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	old := h.cur.Load()
	if !on {
		h.swap(old, nil)
		return nil
	}
	if old != nil && sameConfig(old.cfg, cfg) {
		return nil
	}
	exp, err := otel.New(cfg, h.log)
	if err != nil {
		return fmt.Errorf("starting tracing: %w", err)
	}
	h.swap(old, &active{cfg: cfg, exp: exp})
	return nil
}

func (h *Holder) swap(old, next *active) {
	h.cur.Store(next)
	if old == nil {
		return
	}
	// Owned here: ends when the old exporter has flushed, or after retireTimeout.
	h.retired.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), retireTimeout)
		defer cancel()
		if err := old.exp.Close(ctx); err != nil {
			h.log.Debug("old trace exporter did not finish cleanly", "Event", "TraceExporterRetired", "Error", err.Error())
		}
	})
}

// Close stops exporting and gives what is queued until ctx ends to be sent.
func (h *Holder) Close(ctx context.Context) error {
	h.mu.Lock()
	h.swap(h.cur.Load(), nil)
	h.mu.Unlock()
	done := make(chan struct{})
	go func() { h.retired.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("flushing traces: %w", context.Cause(ctx))
	}
}

func sameConfig(a, b otel.Config) bool {
	return a.Endpoint == b.Endpoint && a.Propagate == b.Propagate && a.Version == b.Version && maps.Equal(a.Headers, b.Headers)
}
