// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
)

// StepState is how far a step of the request's pipeline got in the last send.
type StepState int

// Step states.
const (
	StepIdle    StepState = iota // not started: nothing sent yet, or still waiting its turn
	StepRunning                  // started and not finished
	StepPassed
	StepFailed
	StepNotRun // the send ended without reaching it (an earlier step failed)
)

// RowKind is what a row of the pipeline is.
type RowKind int

// Row kinds.
const (
	RowSection RowKind = iota // "Pre-hooks" or "Post-hooks"
	RowNode                   // a node of a hook
	RowAPI                    // the request itself
)

// PipelineRow is one line of the Flow tab: the hooks and the request in the
// order they run.
type PipelineRow struct {
	Kind   RowKind
	Key    string // what the engine's events name it by
	Title  string // a node's name, the request's name, a section's heading
	Detail string // what it is: "log · request hook"; the status for the request
	Method string // RowAPI: the HTTP method
	Tip    string // RowAPI: the URL
	State  StepState
	Reason string // why it failed
	// Duration is how long the step took, once it finished (zero before).
	Duration time.Duration
}

// Trace records what the events of one send said about each step. Keys are
// "<phase>:<level>:<index>/<node id>" for hook nodes, the same without the
// node for a whole hook step, and "api" for the request.
type Trace struct {
	shape    string // the keys of the pipeline it was recorded for
	results  map[string]stepResult
	apiURL   string
	messages []engine.RecordMessage // the messages an async request has received so far
	finished bool
	warnings int
}

type stepResult struct {
	state  StepState
	reason string
	detail string
	took   time.Duration
}

// NewTrace returns an empty trace for a send about to begin.
func NewTrace() *Trace { return &Trace{results: map[string]stepResult{}} }

// newTraceFor is a trace for a send of req as it is now: it is shown only
// while the pipeline still has that shape, since its keys name steps by place
// and a different version or other hooks would put the results on other rows.
func newTraceFor(req *Request) *Trace {
	t := NewTrace()
	t.shape = req.pipelineShape()
	return t
}

// Messages are the messages an async request has received so far, while it
// is still waiting for the rest.
func (t *Trace) Messages() []engine.RecordMessage { return slices.Clone(t.messages) }

// Finish says no more events are coming: steps without a result were not run.
func (t *Trace) Finish() { t.finished = true }

// hookNodeDepth is the depth of the nodes of a hook of the request being sent
// (the request is at 0; requests inside hook flows are deeper).
const hookNodeDepth = 1

// Apply records an event of the send.
func (t *Trace) Apply(e engine.Event) {
	switch ev := e.(type) {
	case engine.NodeStarted:
		if isHookLane(ev.Lane) && ev.Depth == hookNodeDepth {
			t.results[ev.Lane+"/"+ev.Node] = stepResult{state: StepRunning}
		}
	case engine.NodeFinished:
		if isHookLane(ev.Lane) && ev.Depth == hookNodeDepth {
			res := resultOf(ev.Status, ev.Diags.Err())
			res.took = ev.Duration
			t.results[ev.Lane+"/"+ev.Node] = res
		}
	case engine.HookFinished:
		if ev.Depth == 0 && ev.Status != engine.StatusPassed {
			t.results[ev.Phase+":"+ev.Level+":"+strconv.Itoa(ev.Index)] = resultOf(ev.Status, ev.Diags.Err())
		}
	case engine.Warning:
		t.warnings++
	case engine.MessageReceived:
		if ev.Depth == 0 {
			t.messages = append(t.messages, ev.Message)
		}
	case engine.RequestSent:
		if ev.Depth == 0 {
			t.apiURL = ev.URL
			t.messages = nil
			t.results["api"] = stepResult{state: StepRunning}
		}
	case engine.ResponseReceived:
		if ev.Depth == 0 && ev.Error != "" {
			t.results["api"] = stepResult{state: StepFailed, reason: ev.Error}
		} else if ev.Depth == 0 {
			detail := strconv.Itoa(ev.Status)
			if ev.Outcome != "" {
				detail = ev.Outcome // an async request has no status
			}
			t.results["api"] = stepResult{state: StepPassed, detail: detail, took: ev.Duration}
		}
	}
}

func isHookLane(lane string) bool {
	return strings.HasPrefix(lane, "pre:") || strings.HasPrefix(lane, "post:")
}

func resultOf(status engine.Status, err error) stepResult {
	switch status {
	case engine.StatusPassed:
		return stepResult{state: StepPassed}
	case engine.StatusFailed, engine.StatusError:
		r := stepResult{state: StepFailed}
		if err != nil {
			r.reason = ErrorText(err)
		}
		return r
	}
	return stepResult{state: StepNotRun}
}

// FlowTone is the colour of the dot on the Flow tab: nothing before a send,
// yellow while it runs, then green, orange when something warned, red when a
// step failed. It is nothing too when the trace belongs to other hooks.
func (r *Request) FlowTone(t *Trace) Tone {
	if t == nil || (t.shape != "" && t.shape != r.pipelineShape()) {
		return ToneNone
	}
	for _, res := range t.results {
		if res.state == StepFailed {
			return ToneServerError
		}
	}
	switch {
	case !t.finished:
		return TonePending
	case t.warnings > 0:
		return ToneClientError
	}
	return ToneSuccess
}

// state is the result for key; before the send ends a step without one is idle,
// after it the step was not run.
func (t *Trace) state(key string) stepResult {
	if t == nil {
		return stepResult{}
	}
	if r, ok := t.results[key]; ok {
		return r
	}
	if t.finished {
		return stepResult{state: StepNotRun}
	}
	return stepResult{}
}

// Pipeline lists what runs when the selected version is sent, in order: the
// pre-hook nodes, the request, the post-hook nodes. trace, if not nil, says
// how the last send went.
func (r *Request) Pipeline(trace *Trace) []PipelineRow {
	if trace != nil && trace.shape != "" && trace.shape != r.pipelineShape() {
		trace = nil
	}
	chain := engine.ChainFor(r.coll, r.draft, r.Current())
	var rows []PipelineRow
	rows = append(rows, r.phaseRows("Pre-hooks", "pre", chain.Pre, trace)...)
	rows = append(rows, r.requestRow(trace))
	rows = append(rows, r.phaseRows("Message hooks (each message)", "message", r.messageHooks(chain), trace)...)
	return append(rows, r.phaseRows("Post-hooks", "post", chain.Post, trace)...)
}

// pipelineShape names the steps of the pipeline in order, so two pipelines
// with the same shape have the same keys.
func (r *Request) pipelineShape() string {
	chain := engine.ChainFor(r.coll, r.draft, r.Current())
	var keys []string
	for _, rows := range [][]PipelineRow{r.phaseRows("", "pre", chain.Pre, nil), r.phaseRows("", "message", r.messageHooks(chain), nil), r.phaseRows("", "post", chain.Post, nil)} {
		for _, row := range rows {
			keys = append(keys, row.Key)
		}
	}
	return strings.Join(keys, "|")
}

func (r *Request) phaseRows(heading, phase string, refs []engine.HookRef, trace *Trace) []PipelineRow {
	if len(refs) == 0 {
		return nil
	}
	rows := []PipelineRow{{Kind: RowSection, Title: heading}}
	for i, ref := range refs {
		stepKey := phase + ":" + ref.Level + ":" + strconv.Itoa(i)
		nodes, resolved := r.hookNodes(ref)
		if !resolved {
			rows = append(rows, rowFor(stepKey, "flow "+ref.Label(), ref.Level+" hook, not shown here", trace))
			continue
		}
		for _, n := range nodes {
			rows = append(rows, rowFor(stepKey+"/"+n.ID, nodeTitle(n), string(n.Type)+" · "+ref.Level+" hook", trace))
		}
	}
	return rows
}

func rowFor(key, title, detail string, trace *Trace) PipelineRow {
	res := trace.state(key)
	return PipelineRow{Kind: RowNode, Key: key, Title: title, Detail: detail, State: res.state, Reason: res.reason, Duration: res.took}
}

func nodeTitle(n model.Node) string {
	if n.Name != "" {
		return n.Name
	}
	return n.ID
}

// hookNodes are the top-level nodes of a hook step; false when it names a
// flow the request's collection does not have.
func (r *Request) hookNodes(ref engine.HookRef) ([]model.Node, bool) {
	if ref.Step.Flow == nil {
		return ref.Step.Inline, true
	}
	if ref.Step.Flow.Collection != "" {
		return nil, false // another collection's flow: the engine finds it, this view does not look there
	}
	for _, f := range r.coll.Flows {
		if f.ID == ref.Step.Flow.ID || (ref.Step.Flow.ID == "" && f.Path == ref.Step.Flow.Path) {
			return f.Nodes, true
		}
	}
	return nil, false
}

func (r *Request) requestRow(trace *Trace) PipelineRow {
	cur := r.Current()
	name := r.draft.Name
	if name == "" {
		name = r.draft.Path
	}
	res := trace.state("api")
	row := PipelineRow{Kind: RowAPI, Key: "api", Title: name, Method: Verb(cur), Tip: asyncTip(cur), Detail: res.detail, State: res.state, Reason: res.reason, Duration: res.took}
	if trace != nil && trace.apiURL != "" {
		row.Tip = trace.apiURL
	}
	return row
}

// messageHooks are the hooks that run for each message: only a request that
// receives messages runs them.
func (r *Request) messageHooks(chain engine.HookChain) []engine.HookRef {
	if r.IsAsync() || r.IsGRPC() {
		return chain.Message
	}
	return nil
}
