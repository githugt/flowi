// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

// refreshEvery coalesces run events into at most ~30 canvas updates a
// second (design §10.5).
const refreshEvery = time.Second / 30

// NodeDetail is what a run did at one node of the flow (all its instances
// for nodes inside batches or run_till bodies).
type NodeDetail struct {
	Node      string
	Status    string // the worst instance's status, or running
	Instances int
	Duration  time.Duration
	Outputs   map[string]any // the last finished instance's, masked by the engine
	Diags     []string
	Record    *engine.Record // the last instance's request and response, for a request node
}

// FlowRun runs a flow from the canvas and tracks node statuses.
type FlowRun struct {
	b        Backend
	w        *Worker
	doc      *FlowDoc
	running  bool
	cancel   context.CancelFunc
	details  map[string]*NodeDetail
	hooks    map[string]map[string]string // request node → "phase:level:index" → how that hook went
	result   engine.RunResult
	vars     map[string]any // the flow's variables when the last run ended (masked)
	err      string
	skip     []string // steps this open flow skips; not saved in the file
	hold     bool     // skipped steps keep what the last run gave them
	breaks   []string // steps to pause before when debugging; not saved in the file
	dbg      *debugSession
	paused   string // the step a debug session waits before
	onStatus []func(node, status string)
	onChange []func()
	onPrompt []func(engine.AuthPrompted)
}

// NewFlowRun returns the run view-model of doc.
func NewFlowRun(b Backend, w *Worker, doc *FlowDoc) *FlowRun {
	return &FlowRun{b: b, w: w, doc: doc, details: map[string]*NodeDetail{}, hooks: map[string]map[string]string{}}
}

// OnStatus registers f to run when a node's shown status changes.
func (r *FlowRun) OnStatus(f func(node, status string)) { r.onStatus = append(r.onStatus, f) }

// OnChange registers f to run when the run starts, ends or fails.
func (r *FlowRun) OnChange(f func()) { r.onChange = append(r.onChange, f) }

func (r *FlowRun) changed() {
	for _, f := range r.onChange {
		f()
	}
}

// Doc is the flow being run.
func (r *FlowRun) Doc() *FlowDoc { return r.doc }

// Running reports a run in progress.
func (r *FlowRun) Running() bool { return r.running }

// Err is the last problem, or "".
func (r *FlowRun) Err() string { return r.err }

// Result is the last finished run's result.
func (r *FlowRun) Result() engine.RunResult { return r.result }

// Vars are the flow's variables as the last run ended, masked; nil before a run.
func (r *FlowRun) Vars() map[string]any { return r.vars }

// ClearResults forgets what the last run did at every node.
func (r *FlowRun) ClearResults() {
	for node := range r.details {
		r.setStatus(node, "")
	}
	clear(r.details)
	clear(r.hooks)
	r.vars = nil
	r.changed()
}

// HookKey names a hook of a request's chain for HookState: its phase (pre or
// post), its level (collection, folder, request or version) and its position
// in that level.
func HookKey(phase, level string, index int) string {
	return phase + ":" + level + ":" + strconv.Itoa(index)
}

// HookState is how a hook of request node went in the last run: passed, failed
// and so on, or "" when it did not run (yet).
func (r *FlowRun) HookState(node, key string) string { return r.hooks[node][key] }

// Detail is what the last run did at node.
func (r *FlowRun) Detail(node string) (NodeDetail, bool) {
	d, ok := r.details[node]
	if !ok {
		return NodeDetail{}, false
	}
	return *d, true
}

// Run runs the saved flow with env and inputs. Unsaved edits refuse the
// run: the engine runs the file on disk.
func (r *FlowRun) Run(env string, inputs map[string]string) {
	r.start(env, inputs, nil)
}

// RunPart runs the part of the flow that scope names around node. The steps
// left out keep their last results and feed the ones that run, so there must
// be a last run for a scope that depends on it. It reports false when node is
// not in the flow.
func (r *FlowRun) RunPart(env string, inputs map[string]string, node string, scope RunScope) bool {
	only, ok := PartialNodes(r.doc.Draft().Nodes, node, scope)
	if !ok {
		return false
	}
	r.start(env, inputs, only)
	return true
}

// start runs the saved flow, all of it when only is nil.
func (r *FlowRun) start(env string, inputs map[string]string, only []string) {
	if r.doc.Dirty() {
		r.err = "save the flow first: runs use the file on disk"
		r.changed()
		return
	}
	opts := flowi.RunOptions{Env: env, Inputs: inputs, Only: only}
	if r.dbg != nil {
		opts.Seed = r.dbg.seed
	}
	r.applySkips(&opts) // before the results are cleared: a hold reads the last run's
	if only != nil {
		opts.Prior, opts.PriorVars = r.priorOutputsWith(opts.Prior, only), r.vars
		r.clearNodes(only)
	} else {
		r.ClearResults()
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.running, r.cancel, r.err = true, cancel, ""
	r.changed()
	target, own := r.doc.Target(), ownNodes(r.doc.Draft())
	Run(r.w, func() runOutcome { return r.execute(ctx, target, opts, own) }, func(o runOutcome) {
		cancel()
		r.finish(o)
	})
}

// priorOutputsWith adds to held (the skipped steps' last values, nil for none)
// the last run's outputs of the top-level steps not in only.
func (r *FlowRun) priorOutputsWith(held map[string]map[string]any, only []string) map[string]map[string]any {
	prior := held
	if prior == nil {
		prior = map[string]map[string]any{}
	}
	for _, n := range r.doc.Draft().Nodes {
		d, ran := r.details[n.ID]
		if ran && d.Outputs != nil && !slices.Contains(only, n.ID) {
			prior[n.ID] = d.Outputs
		}
	}
	return prior
}

// clearNodes forgets what the last run did at the given top-level steps and
// at the steps inside them.
func (r *FlowRun) clearNodes(ids []string) {
	for _, id := range ids {
		for _, inner := range nodeIDsWithin(r.doc.Draft().Nodes, id) {
			if _, ok := r.details[inner]; ok {
				r.setStatus(inner, "")
				delete(r.details, inner)
			}
		}
	}
	r.changed()
}

// Cancel stops the run (the engine then waits its grace period).
func (r *FlowRun) Cancel() {
	if r.cancel != nil {
		r.cancel()
	}
}

type runOutcome struct {
	result engine.RunResult
	err    error
}

// execute runs off the UI goroutine; events are posted in batches.
func (r *FlowRun) execute(ctx context.Context, t flowi.FlowTarget, o flowi.RunOptions, own ownership) runOutcome {
	running, err := r.b.RunFlow(ctx, t, o)
	if err != nil {
		return runOutcome{err: err}
	}
	var mu sync.Mutex // guards batch
	var batch []engine.Event
	flush := func() {
		mu.Lock()
		events := batch
		batch = nil
		mu.Unlock()
		if len(events) > 0 {
			r.w.Post(func() { r.apply(events) })
		}
	}
	last := time.Now()
	for e := range running.Events() {
		if !own.concerns(e) {
			continue
		}
		mu.Lock()
		batch = append(batch, e)
		mu.Unlock()
		if time.Since(last) >= refreshEvery {
			flush()
			last = time.Now()
		}
	}
	flush()
	return runOutcome{result: running.Wait()}
}

func (r *FlowRun) finish(o runOutcome) {
	r.running, r.cancel = false, nil
	if o.err != nil {
		r.err = ErrorText(o.err)
	} else {
		r.result, r.vars = o.result, o.result.Report.Vars
		if len(o.result.Diags) > 0 {
			r.err = ErrorText(o.result.Diags.Err())
		}
	}
	r.debugAfter(o)
	r.changed()
}

// statusRank orders statuses; a node shows its worst instance.
func statusRank(s string) int {
	return slices.Index([]string{"", "skipped", "passed", "running", "cancelled", "failed", "error", "aborted"}, s)
}

// OnPrompt registers f to run when a step waits for the person to sign in.
func (r *FlowRun) OnPrompt(f func(engine.AuthPrompted)) { r.onPrompt = append(r.onPrompt, f) }

func (r *FlowRun) apply(events []engine.Event) {
	hooked := false
	defer func() {
		if hooked {
			r.changed() // the hook ticks on the cards are part of the view
		}
	}()
	for _, e := range events {
		switch ev := e.(type) {
		case engine.NodeStarted:
			d := r.detail(ev.Node)
			if d.Status == "" || d.Status == "passed" || d.Status == "skipped" {
				r.setStatus(ev.Node, "running")
			}
		case engine.AuthPrompted:
			for _, f := range r.onPrompt {
				f(ev)
			}
		case engine.HookFinished:
			hooked = true
			node := nodeOfPath(ev.Path)
			if r.hooks[node] == nil {
				r.hooks[node] = map[string]string{}
			}
			r.hooks[node][HookKey(ev.Phase, ev.Level, ev.Index)] = string(ev.Status)
		case engine.RecordCaptured:
			rec := ev.Record
			r.detail(ev.Node).Record = &rec
		case engine.NodeFinished:
			d := r.detail(ev.Node)
			d.Instances++
			d.Duration += ev.Duration
			d.Outputs = ev.Outputs
			d.Diags = append(d.Diags, DiagLines(ev.Diags)...)
			status := string(ev.Status)
			if d.Status != "running" && statusRank(d.Status) > statusRank(status) {
				status = d.Status // an earlier instance did worse
			}
			r.setStatus(ev.Node, status)
		}
	}
}

func (r *FlowRun) detail(node string) *NodeDetail {
	d, ok := r.details[node]
	if !ok {
		d = &NodeDetail{Node: node}
		r.details[node] = d
	}
	return d
}

func (r *FlowRun) setStatus(node, status string) {
	if d, ok := r.details[node]; ok {
		d.Status = status
	}
	for _, f := range r.onStatus {
		f(node, status)
	}
}

// DiagLines is one line of text per diagnostic: "CODE: message".
func DiagLines(l diag.List) []string {
	var out []string
	for _, d := range l {
		msg, _ := diag.English().Message(d)
		out = append(out, string(d.Code)+": "+msg)
	}
	return out
}

// ownership knows which event paths are this flow's own nodes: not hook
// steps, and not nodes inside subflows.
type ownership struct {
	nodes    map[string]bool
	subflows map[string]bool
}

func ownNodes(f model.Flow) ownership {
	o := ownership{nodes: map[string]bool{}, subflows: map[string]bool{}}
	var walk func(list []model.Node)
	walk = func(list []model.Node) {
		for _, n := range list {
			o.nodes[n.ID] = true
			if n.Type == "subflow" {
				o.subflows[n.ID] = true
			}
			for _, child := range n.Children() {
				walk(child)
			}
		}
	}
	walk(f.Nodes)
	return o
}

// concerns reports whether an event is about one of the flow's own nodes.
// Paths look like "a", "p/b1/x", "users[2]/create", "x/pre:request:0/a"
// (a hook step) and "sub/a" (inside a subflow).
func (o ownership) concerns(e engine.Event) bool {
	var node, path string
	var depth int
	switch ev := e.(type) {
	case engine.NodeStarted:
		node, path, depth = ev.Node, ev.Path, ev.Depth
	case engine.NodeFinished:
		node, path, depth = ev.Node, ev.Path, ev.Depth
	case engine.RecordCaptured:
		node, path, depth = ev.Node, ev.Path, ev.Depth
	case engine.HookFinished:
		node, path, depth = nodeOfPath(ev.Path), ev.Path, ev.Depth // path is the request node's own
	case engine.AuthPrompted:
		return true // a person is waited for wherever in the flow (a hook flow's request too)
	default:
		return false
	}
	if depth > 0 || !o.nodes[node] {
		return false
	}
	segments := strings.Split(path, "/")
	for _, s := range segments[:len(segments)-1] {
		id, _, _ := strings.Cut(s, "[")
		if strings.Contains(s, ":") || o.subflows[id] {
			return false
		}
	}
	return true
}

// nodeOfPath is the node id at the end of an event path ("a", "users[2]/create").
func nodeOfPath(path string) string {
	last := path[strings.LastIndex(path, "/")+1:]
	id, _, _ := strings.Cut(last, "[")
	return id
}
