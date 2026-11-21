// SPDX-License-Identifier: Apache-2.0

// Package health is the small HTTP interface an orchestrator or a monitor uses to look after a `flowi serve` process:
//
//	GET /healthz   the process is alive (always 200 while it can answer)
//	GET /readyz    it can take work: 200, or 503 with the reasons while it drains or its run slots are full
//	GET /metrics   Prometheus text format, only when Options.Metrics is on
//
// None of these needs a token and none says anything about flows, requests or secrets: counts and states only. They
// are served on an address of their own so the probe never has to reach the trigger endpoint, and so a probe cannot be
// confused with a call that starts a run. Keep the address on loopback or inside the cluster network.
package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Source is what the handler reads. Every field may be nil; a nil one counts as zero or false.
type Source struct {
	// Draining says the server has begun to shut down.
	Draining func() bool
	// RunsInFlight is the number of flow runs that have not ended, and RunsMax the most that may run at once (0: no limit).
	RunsInFlight func() int
	RunsMax      int
	// Schedules is the number of schedules being kept.
	Schedules func() int
	// ReplayCalls is the number of calls the replayer has answered since the start; Replaying says it is on.
	ReplayCalls func() int
	Replaying   bool
	Triggering  bool
}

// Options configure the handler.
type Options struct {
	Source Source
	// Metrics turns /metrics on. Off, it is a 404 like any other path.
	Metrics bool
}

type handler struct {
	src     Source
	metrics bool
}

// New returns the health handler.
func New(o Options) http.Handler { return &handler{src: o.Source, metrics: o.Metrics} }

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/healthz":
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	case "/readyz":
		reasons := h.notReady()
		if len(reasons) > 0 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not ready", "reasons": reasons})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
	case "/metrics":
		if !h.metrics {
			http.NotFound(w, r)
			return
		}
		h.writeMetrics(w)
	default:
		http.NotFound(w, r)
	}
}

// notReady says why the server should get no new work: it is shutting down, or every run slot is taken.
func (h *handler) notReady() []string {
	reasons := []string{}
	if h.src.Draining != nil && h.src.Draining() {
		reasons = append(reasons, "draining: the server is shutting down")
	}
	if h.src.RunsMax > 0 && h.src.RunsInFlight != nil && h.src.RunsInFlight() >= h.src.RunsMax {
		reasons = append(reasons, fmt.Sprintf("all %d run slots are in use", h.src.RunsMax))
	}
	return reasons
}

func (h *handler) writeMetrics(w http.ResponseWriter) {
	var b strings.Builder
	gauge := func(name, help string, v int) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, v)
	}
	counter := func(name, help string, v int) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, v)
	}
	val := func(f func() int) int {
		if f == nil {
			return 0
		}
		return f()
	}
	ready := 1
	if len(h.notReady()) > 0 {
		ready = 0
	}
	gauge("flowi_up", "1 while the process can answer.", 1)
	gauge("flowi_ready", "1 while the server takes new work, 0 while it drains or its run slots are full.", ready)
	gauge("flowi_runs_in_flight", "Flow runs that have not ended.", val(h.src.RunsInFlight))
	gauge("flowi_runs_max", "Most flow runs allowed at once (0: no limit).", h.src.RunsMax)
	gauge("flowi_schedules", "Schedules being kept.", val(h.src.Schedules))
	if h.src.Replaying {
		counter("flowi_replay_calls_total", "Calls the replayer has answered since the start.", val(h.src.ReplayCalls))
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
