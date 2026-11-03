// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/otel"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

type recordingTracer struct {
	cfg    otel.Config
	on     bool
	status otel.Status
}

func (r *recordingTracer) Apply(cfg otel.Config, on bool) error { r.cfg, r.on = cfg, on; return nil }
func (r *recordingTracer) Status() (otel.Status, bool)          { return r.status, r.on }

func TestTracingService_Save_should_apply_the_settings_and_report_the_exporter_status(t *testing.T) {
	tracer := &recordingTracer{status: otel.Status{Sent: 3, Failed: 1, LastError: "collector answered 500", LastExport: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)}}
	pub := &published{}
	app := bridge.NewApp(&vmtest.Backend{}, bridge.NewInlineActor(), pub.emit, bridge.WithTracer(tracer))
	svc := app.Tracing()

	svc.Save("http://localhost:4318", []string{"Authorization=Bearer s3cret"}, true)
	v := svc.State()

	if !tracer.on || tracer.cfg.Endpoint != "http://localhost:4318" {
		t.Fatalf("tracer %+v on %v", tracer.cfg, tracer.on)
	}
	if !v.Running || v.Endpoint != "http://localhost:4318" || !v.Propagate || len(v.HeaderNames) != 1 || v.HeaderNames[0] != "Authorization" {
		t.Fatalf("view %+v", v)
	}
	if v.Sent != 3 || v.Failed != 1 || v.LastError != "collector answered 500" || v.LastExport != "2026-11-03T10:00:00Z" {
		t.Fatalf("status in view %+v", v)
	}
}

func TestTracingService_should_follow_a_change_made_on_the_settings_page(t *testing.T) {
	tracer := &recordingTracer{}
	app := bridge.NewApp(&vmtest.Backend{}, bridge.NewInlineActor(), (&published{}).emit, bridge.WithTracer(tracer))

	app.Settings().Set("otel.endpoint", "http://collector:4318")

	if !tracer.on || tracer.cfg.Endpoint != "http://collector:4318" {
		t.Fatalf("tracer %+v on %v", tracer.cfg, tracer.on)
	}
}

func TestTracingService_Given_no_tracer_Then_the_page_still_answers_and_says_it_is_not_running(t *testing.T) {
	app := bridge.NewApp(&vmtest.Backend{}, bridge.NewInlineActor(), (&published{}).emit)

	v := app.Tracing().State()

	if v.Running {
		t.Fatalf("%+v", v)
	}
}
