// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/serve/health"
)

func get(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, path, nil))
	return rec
}

func TestHealthz_Given_no_token_Then_it_answers_ok(t *testing.T) {
	// Before: the only health route lived on the trigger endpoint, beside routes that need a token.
	h := health.New(health.Options{})

	rec := get(t, h, http.MethodGet, "/healthz")

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestReadyz_Given_a_healthy_server_Then_ready_and_while_draining_or_full_not_ready_with_reasons(t *testing.T) {
	draining, inFlight := false, 0
	h := health.New(health.Options{Source: health.Source{
		Draining:     func() bool { return draining },
		RunsInFlight: func() int { return inFlight },
		RunsMax:      2,
	}})

	ready := get(t, h, http.MethodGet, "/readyz")
	inFlight = 2
	full := get(t, h, http.MethodGet, "/readyz")
	inFlight, draining = 0, true
	drain := get(t, h, http.MethodGet, "/readyz")

	if ready.Code != http.StatusOK || !strings.Contains(ready.Body.String(), "ready") {
		t.Errorf("ready: %d %s", ready.Code, ready.Body)
	}
	if full.Code != http.StatusServiceUnavailable || !strings.Contains(full.Body.String(), "all 2 run slots are in use") {
		t.Errorf("full: %d %s", full.Code, full.Body)
	}
	if drain.Code != http.StatusServiceUnavailable || !strings.Contains(drain.Body.String(), "draining") {
		t.Errorf("draining: %d %s", drain.Code, drain.Body)
	}
}

func TestReadyz_Given_no_run_limit_Then_any_number_of_runs_is_ready(t *testing.T) {
	h := health.New(health.Options{Source: health.Source{RunsInFlight: func() int { return 500 }}})

	if rec := get(t, h, http.MethodGet, "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestMetrics_Given_metrics_off_Then_404_and_on_Then_prometheus_text(t *testing.T) {
	src := health.Source{RunsInFlight: func() int { return 3 }, RunsMax: 8, Schedules: func() int { return 2 }, ReplayCalls: func() int { return 41 }, Replaying: true}

	off := get(t, health.New(health.Options{Source: src}), http.MethodGet, "/metrics")
	on := get(t, health.New(health.Options{Source: src, Metrics: true}), http.MethodGet, "/metrics")

	if off.Code != http.StatusNotFound {
		t.Errorf("off: %d", off.Code)
	}
	body := on.Body.String()
	for _, want := range []string{"# TYPE flowi_up gauge", "flowi_up 1", "flowi_ready 1", "flowi_runs_in_flight 3", "flowi_runs_max 8", "flowi_schedules 2", "# TYPE flowi_replay_calls_total counter", "flowi_replay_calls_total 41"} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics miss %q:\n%s", want, body)
		}
	}
	if !strings.HasPrefix(on.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("content type %q", on.Header().Get("Content-Type"))
	}
}

func TestMetrics_Given_the_replayer_off_Then_no_replay_counter(t *testing.T) {
	rec := get(t, health.New(health.Options{Metrics: true}), http.MethodGet, "/metrics")

	if strings.Contains(rec.Body.String(), "flowi_replay_calls_total") || !strings.Contains(rec.Body.String(), "flowi_ready 1") {
		t.Fatalf("%s", rec.Body)
	}
}

func TestHealth_Given_other_methods_or_paths_Then_405_and_404(t *testing.T) {
	h := health.New(health.Options{})

	if rec := get(t, h, http.MethodPost, "/healthz"); rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") == "" {
		t.Errorf("post: %d", rec.Code)
	}
	if rec := get(t, h, http.MethodGet, "/flows/x/run"); rec.Code != http.StatusNotFound {
		t.Errorf("path: %d", rec.Code)
	}
	if rec := get(t, h, http.MethodHead, "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("head: %d", rec.Code)
	}
}
