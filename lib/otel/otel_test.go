// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
)

// collector is a fake OTLP/HTTP receiver.
type collector struct {
	srv *httptest.Server

	mu      sync.Mutex
	bodies  []map[string]any
	headers []http.Header
	paths   []string
	status  int
}

func newCollector(t *testing.T) *collector {
	t.Helper()
	c := &collector{status: http.StatusOK}
	c.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		c.mu.Lock()
		c.bodies, c.headers, c.paths = append(c.bodies, body), append(c.headers, r.Header.Clone()), append(c.paths, r.URL.Path)
		status := c.status
		c.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(c.srv.Close)
	return c
}

func (c *collector) received() []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]any(nil), c.bodies...)
}

// span is the decoded form of one exported span.
type span struct {
	Name, TraceID, SpanID, ParentID string
	Start, End                      int64
	Attrs                           map[string]string
	Events                          []string
	StatusCode                      int
	StatusMessage                   string
}

func spansOf(t *testing.T, body map[string]any) []span {
	t.Helper()
	var out []span
	rs := body["resourceSpans"].([]any)
	for _, r := range rs {
		for _, ss := range r.(map[string]any)["scopeSpans"].([]any) {
			for _, raw := range ss.(map[string]any)["spans"].([]any) {
				m := raw.(map[string]any)
				s := span{Name: str(m["name"]), TraceID: str(m["traceId"]), SpanID: str(m["spanId"]), ParentID: str(m["parentSpanId"]), Attrs: map[string]string{}}
				s.Start, _ = strconv.ParseInt(str(m["startTimeUnixNano"]), 10, 64)
				s.End, _ = strconv.ParseInt(str(m["endTimeUnixNano"]), 10, 64)
				for _, a := range list(m["attributes"]) {
					am := a.(map[string]any)
					s.Attrs[str(am["key"])] = valueString(am["value"])
				}
				for _, e := range list(m["events"]) {
					s.Events = append(s.Events, str(e.(map[string]any)["name"]))
				}
				if st, ok := m["status"].(map[string]any); ok {
					code, _ := st["code"].(float64)
					s.StatusCode, s.StatusMessage = int(code), str(st["message"])
				}
				out = append(out, s)
			}
		}
	}
	return out
}

func str(v any) string { s, _ := v.(string); return s }

func list(v any) []any { l, _ := v.([]any); return l }

func valueString(v any) string {
	m, _ := v.(map[string]any)
	for _, k := range []string{"stringValue", "intValue", "boolValue"} {
		if x, ok := m[k]; ok {
			switch t := x.(type) {
			case string:
				return t
			case bool:
				return strconv.FormatBool(t)
			}
		}
	}
	return ""
}

func byName(spans []span, name string) span {
	for _, s := range spans {
		if s.Name == name {
			return s
		}
	}
	return span{}
}

// ticker is a clock that advances 1ms per reading.
type ticker struct {
	mu  sync.Mutex
	now time.Time
}

func newTicker() *ticker { return &ticker{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)} }

func (k *ticker) Now() time.Time {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.now = k.now.Add(time.Millisecond)
	return k.now
}

func newExporter(t *testing.T, c *collector, mod func(*Config)) *Exporter {
	t.Helper()
	cfg := Config{Endpoint: c.srv.URL, Service: "flowi", Version: "1.2.3", Timeout: 2 * time.Second, QueueSize: 8, Now: newTicker().Now}
	if mod != nil {
		mod(&cfg)
	}
	e, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close(context.Background()) })
	return e
}

func flowRun(e *Exporter, id string, failNode bool) {
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	e.Observe(engine.RunStarted{RunID: id, Kind: "flow", FlowPath: "user-creation", Env: "local", Seed: 7, At: start})
	e.Observe(engine.NodeStarted{RunID: id, Path: "login", Node: "login", Type: "request"})
	e.Observe(engine.HookStarted{RunID: id, Path: "login", Level: "collection", Phase: "pre", Step: "inline", Index: 0})
	e.Observe(engine.HookFinished{RunID: id, Path: "login", Level: "collection", Phase: "pre", Step: "inline", Status: engine.StatusPassed})
	e.Observe(engine.RequestSent{RunID: id, Path: "login", RequestID: "R1", Method: "POST", URL: "https://api.test/login", Attempt: 1})
	e.Observe(engine.ResponseReceived{RunID: id, Path: "login", RequestID: "R1", Status: 200, Duration: 10 * time.Millisecond, Attempt: 1})
	status := engine.StatusPassed
	if failNode {
		status = engine.StatusFailed
	}
	e.Observe(engine.NodeFinished{RunID: id, Path: "login", Node: "login", Type: "request", Status: status, Duration: 12 * time.Millisecond})
	runStatus := status
	e.Observe(engine.RunFinished{RunID: id, Status: runStatus, Duration: 30 * time.Millisecond})
}

func TestExporter_should_post_one_trace_per_run_with_a_span_per_node_and_hook(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, nil)

	flowRun(e, "RUN1", false)
	if err := e.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	bodies := c.received()
	if len(bodies) != 1 || c.paths[0] != "/v1/traces" {
		t.Fatalf("bodies %d paths %v", len(bodies), c.paths)
	}
	spans := spansOf(t, bodies[0])
	root, node := byName(spans, "flow user-creation"), byName(spans, "request login")
	hook := byName(spans, "hook collection pre inline")
	if len(spans) != 3 || root.SpanID == "" || node.SpanID == "" || hook.SpanID == "" {
		t.Fatalf("spans %+v", spans)
	}
	if root.ParentID != "" || node.ParentID != root.SpanID || hook.ParentID != node.SpanID {
		t.Fatalf("parents: root %q node %q hook %q", root.ParentID, node.ParentID, hook.ParentID)
	}
	if node.TraceID != root.TraceID || len(root.TraceID) != 32 || len(root.SpanID) != 16 {
		t.Fatalf("ids %+v", spans)
	}
	if got := strings.Join(node.Events, ","); got != "request.sent,response.received" {
		t.Fatalf("node events %q", got)
	}
	if root.Attrs["flowi.run.id"] != "RUN1" || root.Attrs["flowi.env"] != "local" || node.Attrs["flowi.node.type"] != "request" {
		t.Fatalf("attrs root %v node %v", root.Attrs, node.Attrs)
	}
	if root.StatusCode != 1 || node.StatusCode != 1 || root.Start >= node.Start || node.End > root.End {
		t.Fatalf("status/time root %+v node %+v", root, node)
	}
}

func TestExporter_Given_a_failed_node_Then_spans_carry_error_status(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, nil)

	flowRun(e, "RUN1", true)
	_ = e.Close(context.Background())

	spans := spansOf(t, c.received()[0])
	if byName(spans, "request login").StatusCode != 2 || byName(spans, "flow user-creation").StatusCode != 2 {
		t.Fatalf("spans %+v", spans)
	}
}

func TestTraceparent_should_name_the_node_span_of_the_exported_trace(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, func(cfg *Config) { cfg.Propagate = true })

	header := e.Traceparent("RUN1", "login")
	flowRun(e, "RUN1", false)
	_ = e.Close(context.Background())

	node := byName(spansOf(t, c.received()[0]), "request login")
	if want := "00-" + node.TraceID + "-" + node.SpanID + "-01"; header != want {
		t.Fatalf("traceparent %q, want %q", header, want)
	}
}

func TestTraceparent_Given_standalone_send_Then_names_the_root_span(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, func(cfg *Config) { cfg.Propagate = true })

	header := e.Traceparent("RUN1", "")
	e.Observe(engine.RunStarted{RunID: "RUN1", Kind: "send", At: time.Now()})
	e.Observe(engine.RunFinished{RunID: "RUN1", Status: engine.StatusPassed})
	_ = e.Close(context.Background())

	root := spansOf(t, c.received()[0])[0]
	if want := "00-" + root.TraceID + "-" + root.SpanID + "-01"; header != want || root.Name != "send" {
		t.Fatalf("traceparent %q want %q (root %+v)", header, want, root)
	}
}

func TestTraceparent_Given_propagation_off_Then_empty(t *testing.T) {
	e := newExporter(t, newCollector(t), nil)

	if got := e.Traceparent("RUN1", "login"); got != "" {
		t.Fatalf("traceparent %q", got)
	}
}

func TestExporter_should_keep_interleaved_runs_apart(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, nil)

	e.Observe(engine.RunStarted{RunID: "A", Kind: "flow", FlowPath: "a", At: time.Now()})
	e.Observe(engine.RunStarted{RunID: "B", Kind: "flow", FlowPath: "b", At: time.Now()})
	e.Observe(engine.NodeStarted{RunID: "A", Path: "n", Node: "n", Type: "log"})
	e.Observe(engine.NodeStarted{RunID: "B", Path: "n", Node: "n", Type: "log"})
	e.Observe(engine.NodeFinished{RunID: "B", Path: "n", Node: "n", Type: "log", Status: engine.StatusPassed})
	e.Observe(engine.RunFinished{RunID: "B", Status: engine.StatusPassed})
	e.Observe(engine.NodeFinished{RunID: "A", Path: "n", Node: "n", Type: "log", Status: engine.StatusPassed})
	e.Observe(engine.RunFinished{RunID: "A", Status: engine.StatusPassed})
	_ = e.Close(context.Background())

	bodies := c.received()
	if len(bodies) != 2 {
		t.Fatalf("%d bodies", len(bodies))
	}
	first, second := spansOf(t, bodies[0]), spansOf(t, bodies[1])
	if first[0].TraceID == second[0].TraceID || len(first) != 2 || len(second) != 2 {
		t.Fatalf("first %+v second %+v", first, second)
	}
}

func TestExporter_Given_aborted_flow_Then_open_spans_are_closed_with_an_error(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, nil)

	e.Observe(engine.RunStarted{RunID: "A", Kind: "flow", FlowPath: "a", At: time.Now()})
	e.Observe(engine.NodeStarted{RunID: "A", Path: "slow", Node: "slow", Type: "delay"})
	e.Observe(engine.RunCancelled{RunID: "A"})
	e.Observe(engine.RunAborted{RunID: "A", Reason: "grace_period_exceeded", Inflight: []string{"slow"}})
	_ = e.Close(context.Background())

	spans := spansOf(t, c.received()[0])
	slow := byName(spans, "delay slow")
	if slow.End == 0 || slow.StatusCode != 2 || byName(spans, "flow a").StatusCode != 2 {
		t.Fatalf("spans %+v", spans)
	}
}

func TestExporter_Given_cancelled_send_Then_the_trace_ends_there(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, nil)

	e.Observe(engine.RunStarted{RunID: "S", Kind: "send", At: time.Now()})
	e.Observe(engine.RunCancelled{RunID: "S"})
	_ = e.Close(context.Background())

	spans := spansOf(t, c.received()[0])
	if len(spans) != 1 || spans[0].StatusCode != 2 {
		t.Fatalf("spans %+v", spans)
	}
}

func TestExporter_Given_a_failing_collector_Then_Observe_and_Close_return(t *testing.T) {
	c := newCollector(t)
	c.status = http.StatusInternalServerError
	e := newExporter(t, c, nil)

	flowRun(e, "RUN1", false)
	err := e.Close(context.Background())

	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("Close should report the failed export, got %v", err)
	}
}

func TestExporter_Given_a_full_queue_Then_Observe_never_blocks_and_drops(t *testing.T) {
	block := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	defer slow.Close()
	defer close(block)
	e, err := New(Config{Endpoint: slow.URL, Timeout: time.Minute, QueueSize: 1, Now: newTicker().Now}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := range 20 {
			flowRun(e, "RUN"+strconv.Itoa(i), false)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Observe blocked on a full queue")
	}
	if e.Dropped() == 0 {
		t.Fatal("nothing was dropped")
	}
}

func TestExporter_should_send_configured_headers_and_json(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, func(cfg *Config) { cfg.Headers = map[string]string{"Authorization": "Bearer t0k"} })

	flowRun(e, "RUN1", false)
	_ = e.Close(context.Background())

	h := c.headers[0]
	if h.Get("Authorization") != "Bearer t0k" || h.Get("Content-Type") != "application/json" {
		t.Fatalf("headers %v", h)
	}
}

func TestExporter_Given_no_endpoint_Then_New_is_refused(t *testing.T) {
	_, err := New(Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err == nil {
		t.Fatal("empty endpoint accepted")
	}
}

func TestExporter_should_not_export_request_bodies_or_masked_values(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, nil)
	id := "RUN1"
	e.Observe(engine.RunStarted{RunID: id, Kind: "flow", FlowPath: "f", At: time.Now()})
	e.Observe(engine.NodeStarted{RunID: id, Path: "n", Node: "n", Type: "request"})
	e.Observe(engine.NodeFinished{RunID: id, Path: "n", Node: "n", Type: "request", Status: engine.StatusPassed,
		Outputs: map[string]any{"token": "hunter2"}, Diags: diag.List{}})
	e.Observe(engine.Log{RunID: id, Path: "n", Node: "n", Message: "secret message hunter2"})
	e.Observe(engine.RunFinished{RunID: id, Status: engine.StatusPassed})
	_ = e.Close(context.Background())

	data, _ := json.Marshal(c.received()[0])
	if strings.Contains(string(data), "hunter2") {
		t.Fatalf("node outputs or log text leaked: %s", data)
	}
}

// Before: the export of a trace of maxSpans spans had the default 2s test timeout, so on a busy
// machine it failed, nothing arrived and the test panicked indexing the empty list.
func TestExporter_should_not_pile_the_events_of_dropped_spans_onto_the_root(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, func(cfg *Config) { cfg.Timeout = time.Minute })
	e.Observe(engine.RunStarted{RunID: "BIG", Kind: "flow", FlowPath: "loop", At: time.Now()})

	for i := range maxSpans + 50 {
		p := "n" + strconv.Itoa(i)
		e.Observe(engine.NodeStarted{RunID: "BIG", Path: p, Node: p, Type: "request"})
		e.Observe(engine.RequestSent{RunID: "BIG", Path: p, Method: "GET", URL: "http://x", Attempt: 1})
		e.Observe(engine.NodeFinished{RunID: "BIG", Path: p, Node: p, Type: "request", Status: engine.StatusPassed})
	}
	e.Observe(engine.RunFinished{RunID: "BIG", Status: engine.StatusPassed})
	_ = e.Close(context.Background())

	spans := spansOf(t, c.received()[0])
	root := byName(spans, "flow loop")
	if len(spans) > maxSpans || len(root.Events) > 0 || root.Attrs["flowi.spans.dropped"] == "" {
		t.Fatalf("%d spans, %d root events, dropped attr %q", len(spans), len(root.Events), root.Attrs["flowi.spans.dropped"])
	}
}

func TestExporter_should_bound_the_events_of_one_span(t *testing.T) {
	c := newCollector(t)
	e := newExporter(t, c, nil)
	e.Observe(engine.RunStarted{RunID: "R", Kind: "flow", FlowPath: "f", At: time.Now()})
	e.Observe(engine.NodeStarted{RunID: "R", Path: "n", Node: "n", Type: "request"})

	for range maxSpanEvents * 3 {
		e.Observe(engine.RequestSent{RunID: "R", Path: "n", Method: "GET", URL: "http://x", Attempt: 1})
	}
	e.Observe(engine.RunFinished{RunID: "R", Status: engine.StatusPassed})
	_ = e.Close(context.Background())

	if got := len(byName(spansOf(t, c.received()[0]), "request n").Events); got != maxSpanEvents {
		t.Fatalf("%d events", got)
	}
}
