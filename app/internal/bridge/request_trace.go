// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"encoding/json"
	"time"

	"github.com/githugt/flowi/lib/engine"

	"github.com/githugt/flowi/app/internal/vm"
)

// PhaseView is one bar of the waterfall.
type PhaseView struct {
	Name    string  `json:"name"`
	StartMs float64 `json:"startMs"`
	Ms      float64 `json:"ms"`
}

// MessageView is a message a message or gRPC request sent or received.
type MessageView struct {
	Direction string `json:"direction"` // sent or received
	At        string `json:"at"`
	Body      string `json:"body"`
	Key       string `json:"key,omitempty"`
}

// PipelineRowView is a line of the pipeline: the hooks and the request, in
// the order they run, with how the last send went at each.
type PipelineRowView struct {
	Kind     string  `json:"kind"` // section, node, api or builtin (the engine's own step, see builtInRows)
	Title    string  `json:"title"`
	Detail   string  `json:"detail"`
	Method   string  `json:"method,omitempty"`
	Tip      string  `json:"tip,omitempty"`
	State    string  `json:"state"` // idle, running, passed, failed or not-run
	Reason   string  `json:"reason,omitempty"`
	Duration float64 `json:"ms"`
}

func phasesOf(t engine.Timings) []PhaseView {
	out := []PhaseView{}
	for _, p := range vm.Waterfall(t) {
		out = append(out, PhaseView{Name: p.Name, StartMs: ms(p.Start), Ms: ms(p.Duration)})
	}
	return out
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func messagesOf(in []engine.RecordMessage) []MessageView {
	out := []MessageView{}
	for _, m := range in {
		body := m.Body.Text
		if body == "" && m.Body.JSON != nil {
			body = jsonText(m.Body.JSON)
		}
		out = append(out, MessageView{Direction: m.Direction, At: m.At.Local().Format(time.TimeOnly), Body: body, Key: m.Key})
	}
	return out
}

func pipelineOf(r *vm.Request, trace *vm.Trace) []PipelineRowView {
	out := []PipelineRowView{}
	for _, row := range r.Pipeline(trace) {
		kind := map[vm.RowKind]string{vm.RowSection: "section", vm.RowNode: "node", vm.RowAPI: "api"}[row.Kind]
		state := map[vm.StepState]string{vm.StepIdle: "idle", vm.StepRunning: "running", vm.StepPassed: "passed", vm.StepFailed: "failed", vm.StepNotRun: "not-run"}[row.State]
		out = append(out, PipelineRowView{Kind: kind, Title: row.Title, Detail: row.Detail, Method: row.Method, Tip: row.Tip, State: state, Reason: row.Reason, Duration: ms(row.Duration)})
	}
	return out
}

// jsonText is v as indented JSON; text that cannot be encoded shows as "".
func jsonText(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "" // a parsed body always encodes; nothing useful to show otherwise
	}
	return string(data)
}

// liveOf is the messages a running message request has received so far.
func liveOf(t *vm.Trace) []MessageView {
	if t == nil {
		return []MessageView{}
	}
	return messagesOf(t.Messages())
}
