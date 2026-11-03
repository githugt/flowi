// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/otel"
	"github.com/githugt/flowi/lib/settings"
)

// checkTimeout bounds the connection test: a collector that does not answer is a finding, not a wait.
const checkTimeout = 15 * time.Second

// Tracer is the exporter the Tracing page controls. Apply with on false
// switches tracing off; Status reports whether it is running and how exports
// have gone.
type Tracer interface {
	Apply(cfg otel.Config, on bool) error
	Status() (otel.Status, bool)
}

// TraceTest is the outcome of the last "Send test trace".
type TraceTest struct {
	State   string // "" (not run), "testing", "ok" or "failed"
	Message string
}

// TracingView is what the Tracing page shows. It carries header names but
// never their values, which are usually tokens.
type TracingView struct {
	Endpoint    string
	HeaderNames []string
	Propagate   bool
	Problem     string
	Running     bool
	Status      otel.Status
	Test        TraceTest
}

// Tracing is the page where a person points flowi's traces at their own
// OpenTelemetry collector. The settings stay the source of truth: the page
// writes the otel.* settings, and Reload applies whatever they hold, so
// editing them on the settings page has the same effect.
type Tracing struct {
	b        Backend
	w        *Worker
	t        Tracer
	version  string
	view     TracingView
	onChange []func()
}

// NewTracing returns the page's view-model; call Load.
func NewTracing(b Backend, w *Worker, t Tracer, version string) *Tracing {
	return &Tracing{b: b, w: w, t: t, version: version}
}

// OnChange registers f to run after anything the page shows changes.
func (t *Tracing) OnChange(f func()) { t.onChange = append(t.onChange, f) }

func (t *Tracing) changed() {
	for _, f := range t.onChange {
		f()
	}
}

// View is the page as it is now, with the exporter's status read fresh.
func (t *Tracing) View() TracingView {
	v := t.view
	v.Status, v.Running = t.t.Status()
	return v
}

// Load reads the settings and applies them.
func (t *Tracing) Load() {
	Run(t.w, func() struct{} { return struct{}{} }, func(struct{}) { t.Reload() })
}

// Reload reads the otel.* settings, applies them to the exporter and
// refreshes what the page shows. A malformed setting is shown as the
// problem and leaves the running exporter as it was.
func (t *Tracing) Reload() {
	r := t.b.Settings("")
	t.view.Endpoint, t.view.HeaderNames, t.view.Propagate = readTracingSettings(r)
	cfg, on, err := otel.ConfigFrom(r, t.version)
	t.view.Problem = ""
	if err != nil {
		t.view.Problem = ErrorText(err)
	} else if err := t.t.Apply(cfg, on); err != nil {
		t.view.Problem = ErrorText(err)
	}
	t.changed()
}

// Save stores the settings, then applies them. headers are "Name=value"
// entries; none leaves the stored headers as they are, because the page
// cannot show their values to be edited (ClearHeaders removes them). An empty
// endpoint switches tracing off.
func (t *Tracing) Save(endpoint string, headers []string, propagate bool) {
	endpoint = strings.TrimSpace(endpoint)
	Run(t.w, func() error {
		if err := t.b.SetUserSetting("otel.endpoint", endpoint); err != nil {
			return err
		}
		if len(headers) > 0 {
			if err := t.b.SetUserSetting("otel.headers", settings.FormatList(headers)); err != nil {
				return err
			}
		}
		return t.b.SetUserSetting("otel.propagate", boolText(propagate))
	}, t.afterSave)
}

// ClearHeaders removes the stored collector headers.
func (t *Tracing) ClearHeaders() {
	Run(t.w, func() error { return t.b.SetUserSetting("otel.headers", "") }, t.afterSave)
}

func (t *Tracing) afterSave(err error) {
	if err != nil {
		t.view.Problem = ErrorText(err)
		t.changed()
		return
	}
	t.view.Test = TraceTest{}
	t.Reload()
}

// Test sends one small trace to the collector now configured and records
// whether it was accepted.
func (t *Tracing) Test() {
	cfg, on, err := otel.ConfigFrom(t.b.Settings(""), t.version)
	switch {
	case err != nil:
		t.view.Test = TraceTest{State: "failed", Message: ErrorText(err)}
	case !on:
		t.view.Test = TraceTest{State: "failed", Message: "Enter the collector's address first."}
	default:
		t.view.Test = TraceTest{State: "testing"}
		t.changed()
		Run(t.w, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
			defer cancel()
			return otel.Check(ctx, cfg)
		}, func(err error) {
			t.view.Test = TraceTest{State: "ok", Message: "The collector accepted the test trace."}
			if err != nil {
				t.view.Test = TraceTest{State: "failed", Message: ErrorText(err)}
			}
			t.changed()
		})
		return
	}
	t.changed()
}

// readTracingSettings returns the endpoint, the header names (never the
// values) and the propagate flag.
func readTracingSettings(r settings.Resolved) (endpoint string, headerNames []string, propagate bool) {
	if v, ok := r.Get("otel.endpoint"); ok {
		endpoint, _ = v.Value.(string)
	}
	if v, ok := r.Get("otel.headers"); ok {
		entries, _ := v.Value.([]string)
		for _, e := range entries {
			name, _, _ := strings.Cut(e, "=")
			headerNames = append(headerNames, strings.TrimSpace(name))
		}
	}
	if v, ok := r.Get("otel.propagate"); ok {
		propagate, _ = v.Value.(bool)
	}
	return endpoint, headerNames, propagate
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
