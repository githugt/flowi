// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"strconv"
	"time"

	"github.com/githugt/flowi-lib/engine"
)

// OTLP span status codes and kinds.
const (
	statusUnset = 0
	statusOK    = 1
	statusError = 2

	kindInternal = 1
	kindClient   = 3
)

// maxSpans bounds one trace so a huge batch cannot build an unbounded
// payload; spans past it are counted in flowi.spans.dropped.
const maxSpans = 5000

// maxSpanEvents bounds the events kept on one span (a request retried or
// looped on without end).
const maxSpanEvents = 100

type attr struct {
	key string
	val any // string, int64 or bool
}

type spanEvent struct {
	at    time.Time
	name  string
	attrs []attr
}

type spanRec struct {
	id, parent string
	name       string
	kind       int
	start, end time.Time
	attrs      []attr
	events     []spanEvent
	status     int
	message    string
	done       bool
}

// trace is the spans of one run, built from its events.
type trace struct {
	id      string
	runID   string
	kind    string // send | flow
	root    *spanRec
	spans   []*spanRec
	byKey   map[string]*spanRec
	dropped int
	ended   bool
}

func newTrace(runID string) *trace {
	return &trace{id: traceID(runID), runID: runID, byKey: map[string]*spanRec{}}
}

// apply folds one event into the trace and reports whether the trace is
// complete. now stamps spans and events.
func (t *trace) apply(ev engine.Event, now time.Time) bool {
	switch e := ev.(type) {
	case engine.RunStarted:
		t.startRoot(e, now)
	case engine.NodeStarted:
		t.startNode(e, now)
	case engine.NodeFinished:
		t.finishNode(e, now)
	case engine.HookStarted:
		t.startHook(e, now)
	case engine.HookFinished:
		t.finishHook(e, now)
	case engine.RequestSent:
		t.addEvent(nodeKey(e.Path), now, "request.sent", requestAttrs(e))
	case engine.ResponseReceived:
		t.addEvent(nodeKey(e.Path), now, "response.received", responseAttrs(e))
	case engine.RunCancelled:
		t.markCancelled(now)
		return t.kind == "send" // a flow run still ends with RunFinished
	case engine.RunAborted:
		t.root.attrs = append(t.root.attrs, attr{"flowi.abort.reason", e.Reason})
		t.finishRun(statusError, "aborted: "+e.Reason, now)
		return true
	case engine.RunFinished:
		t.finishRun(runStatus(e.Status), string(e.Status), now)
		return true
	}
	return false
}

func (t *trace) startRoot(e engine.RunStarted, now time.Time) {
	t.kind = e.Kind
	name := "send"
	if e.Kind == "flow" {
		name = "flow " + e.FlowPath
	}
	t.root = &spanRec{id: rootSpanID(t.runID), name: name, kind: kindInternal, start: now, attrs: []attr{
		{"flowi.run.id", t.runID}, {"flowi.run.kind", e.Kind}, {"flowi.env", e.Env}, {"flowi.seed", int64(e.Seed)}, //nolint:gosec // a seed is shown, not computed with
	}}
	if e.Kind == "flow" {
		t.root.attrs = append(t.root.attrs, attr{"flowi.flow.path", e.FlowPath})
	}
	t.spans = append(t.spans, t.root)
}

func nodeKey(path string) string { return "n:" + path }

func (t *trace) startNode(e engine.NodeStarted, now time.Time) {
	parent := t.root
	if p, ok := t.byKey[nodeKey(e.Parent)]; ok && e.Parent != "" {
		parent = p
	}
	s := t.add(nodeKey(e.Path), &spanRec{
		id: nodeSpanID(t.runID, e.Path), name: e.Type + " " + e.Node, kind: kindInternal, start: now,
		attrs: []attr{{"flowi.node.id", e.Node}, {"flowi.node.type", e.Type}, {"flowi.node.path", e.Path}, {"flowi.hook.depth", int64(e.Depth)}},
	}, parent)
	if s != nil && e.Lane != "" {
		s.attrs = append(s.attrs, attr{"flowi.node.lane", e.Lane})
	}
}

func (t *trace) finishNode(e engine.NodeFinished, now time.Time) {
	s, ok := t.byKey[nodeKey(e.Path)]
	if !ok {
		return
	}
	s.end, s.done = now, true
	s.attrs = append(s.attrs, attr{"flowi.status", string(e.Status)}, attr{"flowi.duration_ms", e.Duration.Milliseconds()})
	if e.Handled {
		s.attrs = append(s.attrs, attr{"flowi.handled", true})
		return // on_error let the run continue: the failure is not the trace's
	}
	s.status, s.message = runStatus(e.Status), statusText(e.Status)
}

func hookKey(path, level, phase, step string, index, depth int) string {
	return "h:" + path + "|" + level + "|" + phase + "|" + step + "|" + strconv.Itoa(index) + "|" + strconv.Itoa(depth)
}

func (t *trace) startHook(e engine.HookStarted, now time.Time) {
	key := hookKey(e.Path, e.Level, e.Phase, e.Step, e.Index, e.Depth)
	parent := t.root
	if p, ok := t.byKey[nodeKey(e.Path)]; ok && e.Path != "" {
		parent = p
	}
	t.add(key, &spanRec{
		id: spanID(t.runID, "hook", key), name: "hook " + e.Level + " " + e.Phase + " " + e.Step, kind: kindInternal, start: now,
		attrs: []attr{{"flowi.hook.level", e.Level}, {"flowi.hook.phase", e.Phase}, {"flowi.hook.step", e.Step}, {"flowi.hook.index", int64(e.Index)}},
	}, parent)
}

func (t *trace) finishHook(e engine.HookFinished, now time.Time) {
	s, ok := t.byKey[hookKey(e.Path, e.Level, e.Phase, e.Step, e.Index, e.Depth)]
	if !ok {
		return
	}
	s.end, s.done = now, true
	s.attrs = append(s.attrs, attr{"flowi.status", string(e.Status)})
	s.status, s.message = runStatus(e.Status), statusText(e.Status)
}

// add registers a span under parent, or counts it as dropped past maxSpans.
func (t *trace) add(key string, s *spanRec, parent *spanRec) *spanRec {
	if t.root == nil || len(t.spans) >= maxSpans {
		t.dropped++
		return nil
	}
	s.parent = parent.id
	t.byKey[key] = s
	t.spans = append(t.spans, s)
	return s
}

func (t *trace) addEvent(key string, now time.Time, name string, attrs []attr) {
	s := t.root
	if found, ok := t.byKey[key]; ok && key != nodeKey("") {
		s = found
	} else if key != nodeKey("") && t.dropped > 0 {
		return // the span was dropped at the cap; its events must not pile onto the root
	}
	if s == nil || len(s.events) >= maxSpanEvents {
		return
	}
	s.events = append(s.events, spanEvent{at: now, name: name, attrs: attrs})
}

func (t *trace) markCancelled(now time.Time) {
	if t.root == nil {
		return
	}
	t.root.events = append(t.root.events, spanEvent{at: now, name: "run.cancelled"})
	t.root.status, t.root.message = statusError, "cancelled"
}

// finishRun closes the root and every span still open.
func (t *trace) finishRun(status int, message string, now time.Time) {
	if t.root == nil || t.ended {
		return
	}
	t.ended = true
	for _, s := range t.spans {
		if s.done {
			continue
		}
		s.end, s.done = now, true
		if s != t.root {
			s.status, s.message = statusError, "unfinished"
		}
	}
	t.root.status, t.root.message = status, message
	if t.dropped > 0 {
		t.root.attrs = append(t.root.attrs, attr{"flowi.spans.dropped", int64(t.dropped)})
	}
}

// runStatus maps a flowi status to an OTLP status code.
func runStatus(s engine.Status) int {
	switch s {
	case engine.StatusPassed:
		return statusOK
	case engine.StatusSkipped:
		return statusUnset
	}
	return statusError
}

func statusText(s engine.Status) string {
	if runStatus(s) == statusError {
		return string(s)
	}
	return ""
}

// requestAttrs and responseAttrs carry what a trace viewer needs and
// nothing more: no bodies and no headers. The URL is the engine's masked
// URL.
func requestAttrs(e engine.RequestSent) []attr {
	return []attr{{"http.request.method", e.Method}, {"url.full", e.URL}, {"flowi.attempt", int64(e.Attempt)}, {"flowi.request.id", e.RequestID}}
}

func responseAttrs(e engine.ResponseReceived) []attr {
	a := []attr{{"http.response.status_code", int64(e.Status)}, {"flowi.attempt", int64(e.Attempt)}, {"flowi.duration_ms", e.Duration.Milliseconds()}}
	if e.Error != "" {
		a = append(a, attr{"error.message", e.Error})
	}
	return a
}
