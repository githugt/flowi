// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/otel"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// fakeTracer records what the page asked the exporter to be.
type fakeTracer struct {
	cfg    otel.Config
	on     bool
	err    error
	status otel.Status
}

func (f *fakeTracer) Apply(cfg otel.Config, on bool) error {
	if f.err != nil {
		return f.err
	}
	f.cfg, f.on = cfg, on
	return nil
}

func (f *fakeTracer) Status() (otel.Status, bool) { return f.status, f.on }

func newTracing(b *vmtest.Backend) (*vm.Tracing, *fakeTracer, *uiQueue) {
	q, w := newUIQueue()
	ft := &fakeTracer{}
	return vm.NewTracing(b, w, ft, "1.0.0"), ft, q
}

func TestTracing_Load_should_read_the_otel_settings(t *testing.T) {
	b := &vmtest.Backend{Values: map[string]any{"otel.endpoint": "http://localhost:4318", "otel.headers": []string{"Authorization=Bearer x"}, "otel.propagate": true}}
	tr, _, q := newTracing(b)

	tr.Load()
	q.run(1)

	v := tr.View()
	if v.Endpoint != "http://localhost:4318" || !v.Propagate || len(v.HeaderNames) != 1 || v.HeaderNames[0] != "Authorization" {
		t.Fatalf("%+v", v)
	}
}

func TestTracing_View_should_never_carry_a_header_value(t *testing.T) {
	b := &vmtest.Backend{Values: map[string]any{"otel.endpoint": "http://x", "otel.headers": []string{"Authorization=Bearer s3cret"}}}
	tr, _, q := newTracing(b)

	tr.Load()
	q.run(1)

	if got := strings.Join(tr.View().HeaderNames, ","); strings.Contains(got, "s3cret") {
		t.Fatalf("a header value reached the view: %s", got)
	}
}

func TestTracing_Save_should_store_the_settings_and_switch_the_exporter_on(t *testing.T) {
	b := &vmtest.Backend{}
	tr, ft, q := newTracing(b)

	tr.Save("http://localhost:4318", []string{"X-Key=abc"}, true)
	q.run(1)

	if !ft.on || ft.cfg.Endpoint != "http://localhost:4318" || ft.cfg.Headers["X-Key"] != "abc" || !ft.cfg.Propagate {
		t.Fatalf("tracer %+v on %v", ft.cfg, ft.on)
	}
	if !b.Called("SetUserSetting otel.endpoint=http://localhost:4318") || tr.View().Problem != "" {
		t.Fatalf("calls %v problem %q", b.Calls, tr.View().Problem)
	}
}

func TestTracing_Save_Given_an_empty_endpoint_Then_tracing_is_off(t *testing.T) {
	b := &vmtest.Backend{Values: map[string]any{"otel.endpoint": "http://x"}}
	tr, ft, q := newTracing(b)
	ft.on = true

	tr.Save("", nil, false)
	q.run(1)

	if ft.on || tr.View().Running {
		t.Fatal("an empty endpoint keeps tracing off")
	}
}

func TestTracing_Save_Given_a_malformed_header_Then_the_problem_is_shown_and_the_exporter_untouched(t *testing.T) {
	b := &vmtest.Backend{}
	tr, ft, q := newTracing(b)

	tr.Save("http://localhost:4318", []string{"no equals sign"}, false)
	q.run(1)

	if ft.on || !strings.Contains(tr.View().Problem, "otel.headers") {
		t.Fatalf("on %v problem %q", ft.on, tr.View().Problem)
	}
}

func TestTracing_Test_should_report_a_collector_that_answers_and_one_that_refuses(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
	t.Cleanup(srv.Close)
	b := &vmtest.Backend{Values: map[string]any{"otel.endpoint": srv.URL}}
	tr, _, q := newTracing(b)

	tr.Test()
	q.run(1)
	okResult := tr.View().Test
	status = http.StatusUnauthorized
	tr.Test()
	q.run(1)
	refused := tr.View().Test

	if okResult.State != "ok" || refused.State != "failed" || !strings.Contains(refused.Message, "401") {
		t.Fatalf("ok %+v refused %+v", okResult, refused)
	}
}

func TestTracing_Test_Given_no_endpoint_Then_asks_for_one(t *testing.T) {
	tr, _, q := newTracing(&vmtest.Backend{})

	tr.Test()
	q.run(0)

	if v := tr.View().Test; v.State != "failed" || !strings.Contains(v.Message, "address") {
		t.Fatalf("%+v", v)
	}
}

func TestTracing_Reload_should_apply_a_change_made_on_the_settings_page(t *testing.T) {
	b := &vmtest.Backend{Values: map[string]any{"otel.endpoint": "http://localhost:4318"}}
	tr, ft, _ := newTracing(b)

	tr.Reload()

	if !ft.on || ft.cfg.Endpoint != "http://localhost:4318" {
		t.Fatalf("%+v on %v", ft.cfg, ft.on)
	}
}

// The page cannot show a header's value (it is a token), so a save that names no headers must leave the stored ones alone.
func TestTracing_Save_Given_no_headers_Then_the_stored_ones_are_kept(t *testing.T) {
	b := &vmtest.Backend{Values: map[string]any{"otel.headers": []string{"Authorization=Bearer x"}}}
	tr, ft, q := newTracing(b)

	tr.Save("http://localhost:4318", nil, false)
	q.run(1)

	if ft.cfg.Headers["Authorization"] != "Bearer x" || b.Called("SetUserSetting otel.headers=") {
		t.Fatalf("headers %v calls %v", ft.cfg.Headers, b.Calls)
	}
}

func TestTracing_ClearHeaders_should_remove_the_stored_headers(t *testing.T) {
	b := &vmtest.Backend{Values: map[string]any{"otel.endpoint": "http://localhost:4318", "otel.headers": []string{"Authorization=Bearer x"}}}
	tr, ft, q := newTracing(b)

	tr.ClearHeaders()
	q.run(1)

	if len(ft.cfg.Headers) != 0 || len(tr.View().HeaderNames) != 0 {
		t.Fatalf("headers %v names %v", ft.cfg.Headers, tr.View().HeaderNames)
	}
}

// The headers are stored as a comma-separated list, so a comma inside a value is escaped on the way in; without
// that, "a,b" came back as a second, malformed header.
func TestTracing_Save_Given_a_comma_in_a_header_value_Then_it_is_stored_escaped_and_applied_whole(t *testing.T) {
	b := &vmtest.Backend{}
	tr, _, q := newTracing(b)

	tr.Save("http://localhost:4318", []string{"X-Tags=a,b"}, false)
	q.run(1)

	if !b.Called(`SetUserSetting otel.headers=X-Tags=a\,b`) || tr.View().Problem != "" {
		t.Fatalf("calls %v problem %q", b.Calls, tr.View().Problem)
	}
}
