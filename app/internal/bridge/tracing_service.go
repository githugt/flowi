// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"time"

	"github.com/githugt/flowi/lib/otel"

	"github.com/githugt/flowi/app/internal/buildinfo"
	"github.com/githugt/flowi/app/internal/vm"
)

// TraceTestView is the outcome of the last test trace.
type TraceTestView struct {
	State   string `json:"state"` // "", testing, ok or failed
	Message string `json:"message"`
}

// TracingView is the Tracing page. Header values are never part of it.
type TracingView struct {
	Endpoint    string        `json:"endpoint"`
	HeaderNames []string      `json:"headerNames"`
	Propagate   bool          `json:"propagate"`
	Problem     string        `json:"problem,omitempty"`
	Running     bool          `json:"running"`
	Sent        int64         `json:"sent"`
	Failed      int64         `json:"failed"`
	Dropped     int64         `json:"dropped"`
	LastError   string        `json:"lastError,omitempty"`
	LastExport  string        `json:"lastExport,omitempty"` // RFC 3339, "" before the first export
	Test        TraceTestView `json:"test"`
}

// noTracer is the exporter of an app built without one: tracing is never on.
type noTracer struct{}

func (noTracer) Apply(otel.Config, bool) error { return nil }
func (noTracer) Status() (otel.Status, bool)   { return otel.Status{}, false }

// TracingService is the page where traces are pointed at a collector.
type TracingService struct {
	app   *App
	vm    *vm.Tracing
	first *firstLoad
}

func newTracingService(a *App, t vm.Tracer) *TracingService {
	if t == nil {
		t = noTracer{}
	}
	s := &TracingService{app: a, vm: vm.NewTracing(a.backend, a.worker, t, buildinfo.Version()), first: newFirstLoad()}
	s.vm.OnChange(func() {
		s.first.mark()
		a.emit("tracing", s.view())
	})
	// The settings page can change the same keys: follow it, so both agree on what is running.
	a.settings.vm.OnChange(s.vm.Reload)
	return s
}

// State loads the page and returns it; later changes arrive as the "tracing" event.
func (s *TracingService) State() TracingView {
	var v TracingView
	s.app.actor.Do(s.vm.Load)
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// Save stores the collector's address, headers ("Name=value"; none keeps the stored ones) and the
// propagate flag, and applies them. An empty address turns tracing off.
func (s *TracingService) Save(endpoint string, headers []string, propagate bool) {
	s.app.actor.Do(func() { s.vm.Save(endpoint, headers, propagate) })
}

// ClearHeaders removes the stored collector headers.
func (s *TracingService) ClearHeaders() { s.app.actor.Do(s.vm.ClearHeaders) }

// Test sends one small trace to the collector; the outcome arrives as the "tracing" event.
func (s *TracingService) Test() { s.app.actor.Do(s.vm.Test) }

func (s *TracingService) view() TracingView {
	v := s.vm.View()
	out := TracingView{
		Endpoint: v.Endpoint, HeaderNames: v.HeaderNames, Propagate: v.Propagate, Problem: v.Problem, Running: v.Running,
		Sent: v.Status.Sent, Failed: v.Status.Failed, Dropped: v.Status.Dropped, LastError: v.Status.LastError,
		Test: TraceTestView{State: v.Test.State, Message: v.Test.Message},
	}
	if out.HeaderNames == nil {
		out.HeaderNames = []string{}
	}
	if !v.Status.LastExport.IsZero() {
		out.LastExport = v.Status.LastExport.UTC().Format(time.RFC3339)
	}
	return out
}
