// SPDX-License-Identifier: AGPL-3.0-or-later

package tracing_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/otel"

	"github.com/githugt/flowi/app/internal/tracing"
)

func newHolder() *tracing.Holder {
	return tracing.NewHolder(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func countingCollector(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { n.Add(1); w.WriteHeader(http.StatusOK) }))
	t.Cleanup(srv.Close)
	return srv, &n
}

func finishedRun(h *tracing.Holder, id string) {
	h.Observe(engine.RunStarted{RunID: id, Kind: "send"})
	h.Observe(engine.RunFinished{RunID: id, Status: engine.StatusPassed})
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHolder_Given_no_config_Then_runs_are_ignored_and_nothing_is_propagated(t *testing.T) {
	h := newHolder()

	finishedRun(h, "R1")

	if _, on := h.Status(); on || h.Traceparent("R1", "") != "" {
		t.Fatal("an unconfigured holder must do nothing")
	}
}

func TestHolder_Apply_Given_a_collector_Then_runs_are_exported_and_status_shows_it(t *testing.T) {
	srv, got := countingCollector(t)
	h := newHolder()
	t.Cleanup(func() { _ = h.Close(t.Context()) })

	err := h.Apply(otel.Config{Endpoint: srv.URL, Propagate: true}, true)
	finishedRun(h, "R1")

	waitFor(t, func() bool { return got.Load() == 1 })
	st, on := h.Status()
	if err != nil || !on || h.Traceparent("R1", "") == "" {
		t.Fatalf("err %v on %v", err, on)
	}
	waitFor(t, func() bool { s, _ := h.Status(); return s.Sent == 1 })
	_ = st
}

func TestHolder_Apply_Given_the_same_config_Then_the_exporter_is_kept(t *testing.T) {
	srv, _ := countingCollector(t)
	h := newHolder()
	t.Cleanup(func() { _ = h.Close(t.Context()) })
	cfg := otel.Config{Endpoint: srv.URL, Headers: map[string]string{"A": "b"}}
	_ = h.Apply(cfg, true)
	finishedRun(h, "R1")
	waitFor(t, func() bool { s, _ := h.Status(); return s.Sent == 1 })

	_ = h.Apply(otel.Config{Endpoint: srv.URL, Headers: map[string]string{"A": "b"}}, true)

	// A new exporter would start its counts again from zero.
	if s, _ := h.Status(); s.Sent != 1 {
		t.Fatalf("exporter was replaced: %+v", s)
	}
}

func TestHolder_Apply_Given_off_Then_exporting_stops(t *testing.T) {
	srv, got := countingCollector(t)
	h := newHolder()
	t.Cleanup(func() { _ = h.Close(t.Context()) })
	_ = h.Apply(otel.Config{Endpoint: srv.URL}, true)

	_ = h.Apply(otel.Config{}, false)
	finishedRun(h, "R1")
	time.Sleep(50 * time.Millisecond) // nothing is expected; there is no event to wait for

	if _, on := h.Status(); on || got.Load() != 0 {
		t.Fatalf("on %v, exports %d", on, got.Load())
	}
}

func TestHolder_Apply_Given_a_bad_endpoint_Then_it_is_refused_and_the_old_exporter_stays(t *testing.T) {
	srv, _ := countingCollector(t)
	h := newHolder()
	t.Cleanup(func() { _ = h.Close(t.Context()) })
	_ = h.Apply(otel.Config{Endpoint: srv.URL}, true)

	err := h.Apply(otel.Config{Endpoint: "ftp://nope"}, true)

	if _, on := h.Status(); err == nil || !on {
		t.Fatalf("err %v on %v", err, on)
	}
}
