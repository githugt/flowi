// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"maps"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// errNodeTimeout is the cancel cause of a node's own timeout, telling it
// apart from the run being cancelled.
var errNodeTimeout = errors.New("node timeout")

// nodeResult is a finished node instance.
type nodeResult struct {
	status  Status
	handled bool // failed, but on_error lets its list go on
	// outcomes are what the node ended with, for the arrows that leave it: the outcomes of a request, else
	// success or failure by its status. Empty for a skipped or cancelled node.
	outcomes []string
	outputs  map[string]any
	diags    diag.List
	reason   string // why a node was skipped, when it was not by its when
}

// skippedByYou is the reason of a node the person chose not to run.
const skippedByYou = "skipped by you"

// skipSet is the set of node ids and paths in a RunInput's Skip.
func skipSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// runNode runs one node instance and reports it: NodeStarted, the node,
// its outputs (written once into the frame), NodeFinished.
func (r *runner) runNode(ctx context.Context, f *frame, n model.Node) nodeResult {
	path := f.path(n.ID)
	start := r.deps.Clock()
	r.emit(NodeStarted{RunID: r.runID, Path: path, Parent: f.parentPath, Lane: f.lane, Node: n.ID, Type: string(n.Type), Name: n.Name, Depth: f.depth})
	res := r.execNode(ctx, f, n, path)
	if res.outputs != nil {
		f.outputs.put(n.ID, res.outputs)
	}
	r.emit(NodeFinished{
		RunID: r.runID, Path: path, Parent: f.parentPath, Lane: f.lane, Node: n.ID, Type: string(n.Type),
		Status: res.status, Handled: res.handled, Duration: elapsed(r.deps.Clock, start), Depth: f.depth,
		Outputs: r.shownOutputs(n.Type, res.outputs), Diags: maskDiags(res.diags, r.secrets.current()), Reason: res.reason,
	})
	return res
}

// execNode checks skip and when, applies the node's timeout and runs it.
func (r *runner) execNode(ctx context.Context, f *frame, n model.Node, path string) nodeResult {
	if r.skip[n.ID] || r.skip[path] {
		return nodeResult{status: StatusSkipped, reason: skippedByYou, outputs: r.prior[n.ID]}
	}
	run, diags := shouldRun(n, f.scope(r))
	if diags.HasErrors() {
		return nodeResult{status: StatusFailed, diags: diags}
	}
	if !run {
		return nodeResult{status: StatusSkipped}
	}
	runner, ok := r.nodeRunner(n.Type)
	if !ok {
		return nodeResult{status: StatusFailed, diags: diag.List{diag.New(diag.CodeRunNodeType, diag.Location{}, map[string]string{"type": string(n.Type), "node": n.ID})}}
	}
	nodeCtx := ctx
	if n.Timeout > 0 {
		var cancel context.CancelFunc
		nodeCtx, cancel = context.WithTimeoutCause(ctx, n.Timeout, errNodeTimeout)
		defer cancel()
	}
	out := runner(nodeCtx, n, f.state(r, n, path))
	return classify(ctx, nodeCtx, n, out)
}

func shouldRun(n model.Node, sc template.Scope) (bool, diag.List) {
	if n.When == "" {
		return true, nil
	}
	ok, _, err := template.Check(n.When, sc)
	if err != nil {
		return false, templateFailure("when of node "+n.ID, err)
	}
	return ok, nil
}

// classify turns a node's outcome into its status (design §7.1, plan
// Decision 7): the run being cancelled wins, then the node's own timeout;
// transport failures are errors; everything else that failed is failed.
func classify(runCtx, nodeCtx context.Context, n model.Node, out nodes.Outcome) nodeResult {
	status := Status(out.Status)
	if status == "" {
		status = StatusPassed
		if out.Diags.HasErrors() {
			status = StatusFailed
		}
	}
	diags := out.Diags
	switch {
	case status == StatusPassed || status == StatusSkipped:
	case runCtx.Err() != nil:
		status = StatusCancelled
	case errors.Is(context.Cause(nodeCtx), errNodeTimeout):
		status = StatusFailed
		diags = append(diags, diag.New(diag.CodeRunNodeTimeout, diag.Location{}, map[string]string{"node": n.ID, "timeout": n.Timeout.String()}))
	case status == StatusFailed && hasTransportError(diags):
		status = StatusError
	}
	handled := (status == StatusFailed || status == StatusError) && handlesErrors(n.OnError)
	return nodeResult{status: status, handled: handled, outcomes: outcomesOf(status, out.Outcomes), outputs: out.Outputs, diags: diags}
}

// outcomesOf are the outcomes a node ended with: those its runner reported (a request does), else success
// or failure from the status. A node that timed out failed, whatever its request had matched.
func outcomesOf(status Status, reported []string) []string {
	switch {
	case status == StatusSkipped || status == StatusCancelled:
		return nil
	case reported != nil:
		return reported
	case status == StatusFailed || status == StatusError:
		return []string{model.OutcomeFailure}
	}
	return []string{model.OutcomeSuccess}
}

func hasTransportError(l diag.List) bool {
	for _, d := range l {
		if d.Code == diag.CodeHTTPFailed || d.Code == diag.CodeHTTPTimeout || d.Code == diag.CodeRunWriteFailed {
			return true
		}
	}
	return false
}

// handlesErrors reports an on_error that lets the list go on.
func handlesErrors(onError string) bool {
	return onError == "continue" || strings.HasPrefix(onError, "goto:")
}

// shownOutputs are a node's outputs as events show them: masked, without
// response bodies and headers (those are in the response record).
func (r *runner) shownOutputs(t model.NodeType, outputs map[string]any) map[string]any {
	if outputs == nil {
		return nil
	}
	shown := maps.Clone(outputs)
	if t == "request" || t == "fetch" || t == "webhook" {
		delete(shown, "body")
		delete(shown, "headers")
	}
	masked, _ := maskValue(shown, r.secrets.current()).(map[string]any)
	return masked
}

// state is what node n sees when it runs in frame f.
func (f *frame) state(r *runner, n model.Node, path string) *nodes.State {
	s := &nodes.State{
		Phase: f.fs.phase, Path: path, Vars: f.vars, Scope: func() template.Scope { return f.scope(r) },
		Emit: nodeEmitter{r: r, path: path, node: n.ID}, Masker: r.secrets, Secret: r.secrets.derived,
		Runtime: &nodeRuntime{r: r, f: f, node: n, path: path},
	}
	if jar := f.cookieJar(); jar != nil {
		s.Cookies = jar.All()
	}
	if x := f.fs.hook; x != nil {
		f.hookState(s, x)
	}
	return s
}

// hookState gives a hook node the request: pre-hooks may change it and
// read it as it would be sent now; post-hooks read the response. Warnings
// also go to the request's own diagnostics (they are part of its result).
func (f *frame) hookState(s *nodes.State, x *execution) {
	s.Request, s.Response = x.state.Request, x.state.Response
	if f.fs.phase == nodes.PhasePre {
		x.setView(x.requestView())
	}
	if len(s.Cookies) == 0 {
		s.Cookies = x.state.Cookies
	}
	s.Emit = hookEmitter{nodeEmitter: s.Emit.(nodeEmitter), x: x}
}

// hookEmitter is nodeEmitter whose warnings also land in the request's
// diagnostics.
type hookEmitter struct {
	nodeEmitter
	x *execution
}

func (e hookEmitter) Warn(d diag.Diagnostic) { emitter{e.x}.Warn(d) }

// nodeEmitter turns what flow nodes report into run events.
type nodeEmitter struct {
	r          *runner
	path, node string
}

func (e nodeEmitter) Log(_, message string) {
	e.r.emit(Log{RunID: e.r.runID, Path: e.path, Node: e.node, Message: e.r.secrets.String(message)})
}

func (e nodeEmitter) Warn(d diag.Diagnostic) { e.r.warn(d) }

func (e nodeEmitter) Digest(_, format, text string) {
	e.r.emit(Digest{RunID: e.r.runID, Path: e.path, Node: e.node, Format: format, Text: e.r.secrets.String(text)})
}
