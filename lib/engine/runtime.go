// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"cmp"
	"context"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
)

// nodeRuntime is nodes.Runtime for one node instance: the run, the frame
// the node runs in, and the node itself.
type nodeRuntime struct {
	r    *runner
	f    *frame
	node model.Node
	path string
}

var _ nodes.Runtime = (*nodeRuntime)(nil)

// Request runs a saved request from a request node: version resolution,
// its hooks, render, send, extract and assert (M2 executor), with the
// node's with values and headers and its frame's cookie jar.
func (rt *nodeRuntime) Request(ctx context.Context, c nodes.RequestCall) nodes.Outcome {
	coll, req, ok := rt.r.deps.Lookup.Request(c.Ref, rt.f.fs.collection.Manifest.ID)
	if !ok {
		return nodes.Outcome{Diags: diag.List{diag.New(diag.CodeRefUnknownRequest, diag.Location{}, map[string]string{"id": string(c.Ref.ID)})}}
	}
	with := c.With
	if with == nil {
		with = map[string]any{} // non-nil: already rendered, never re-rendered as templates
	}
	overlay, diags := overlayOfCall(coll, req, cmp.Or(c.Overlay, rt.r.overlay))
	if diags.HasErrors() {
		return nodes.Outcome{Diags: diags}
	}
	inner := rt.r.execute(ctx, call{
		collection: coll, request: req, pinned: c.Version, flowPrefer: rt.f.fs.prefer, withValues: with, hooks: c.Hooks,
		depth: rt.f.depth, path: rt.path, headers: c.Headers, jar: rt.f.jar, noJar: rt.f.noJar, overlay: overlay,
		conn: connUse{open: c.Open, over: c.Over, closeAfter: c.Close}, bodyOver: c.Body,
	})
	if inner.record != nil {
		masked := inner.record.Masked(rt.r.secrets.current())
		rt.r.addCaptured(masked)
		rt.r.emit(RecordCaptured{RunID: rt.r.runID, Path: rt.path, Node: rt.node.ID, Depth: rt.f.depth, Record: masked})
	}
	return nodes.Outcome{Outputs: nodeOutputs(inner), Status: requestNodeStatus(inner), Diags: inner.diags}
}

// requestNodeStatus maps an execution's status to the node's (plan
// Decision 7): "error" only for transport and system failures; a request
// that could not be prepared or rendered (undefined value, missing secret)
// is a failed node. classify raises failed+transport back to error.
func requestNodeStatus(o outcome) nodes.Status {
	if o.status == StatusError && !hasTransportError(o.diags) {
		return nodes.StatusFailed
	}
	return nodes.Status(o.status)
}

// Subflow runs another flow with its declared inputs, in its own scope
// (own vars and nodes; the caller's cookies); its declared outputs become
// the node's (plan Decision 10).
func (rt *nodeRuntime) Subflow(ctx context.Context, ref model.Ref, inputs map[string]any) nodes.Outcome {
	r := rt.r
	coll, fl, ok := r.deps.Lookup.Flow(ref, rt.f.fs.collection.Manifest.ID)
	if !ok {
		return nodes.Outcome{Diags: diag.List{diag.New(diag.CodeRefUnknownFlow, diag.Location{}, map[string]string{"id": string(ref.ID)})}}
	}
	cfg := newConfig(r.deps.Lookup.Settings(coll.Manifest.ID, flowFolder(fl), nil))
	env, diags := selectEnvironment(coll, r.env, cfg.defaultEnv)
	if diags.HasErrors() {
		return nodes.Outcome{Diags: diags}
	}
	fs := &flowScope{collection: coll, flow: &fl, env: env, prefer: fl.PreferVersion, onBranchError: fl.Execution.OnBranchError}
	f := &frame{fs: fs, vars: nodes.NewVars(nil), outputs: newOutputs(nil), jar: rt.f.jar, noJar: rt.f.noJar,
		prefix: rt.path + "/", parentPath: rt.path, depth: rt.f.depth}
	resolved, diags := r.resolveInputs(fl, inputs, f.scope(r))
	if diags.HasErrors() {
		return nodes.Outcome{Diags: diags}
	}
	fs.inputs = resolved
	out := r.runList(ctx, f, fl.Nodes)
	if out.status != StatusPassed {
		return nodes.Outcome{Status: nodes.Status(out.status), Diags: out.diags}
	}
	outputs, diags := r.renderOutputs(fl.Outputs, f.scope(r))
	return nodes.Outcome{Outputs: outputs, Diags: diags}
}

// StopRun ends the run (a stop node): lists start no new node from now on.
func (rt *nodeRuntime) StopRun() { rt.r.stop.Store(true) }

// Stopped reports that a stop node ended the run.
func (rt *nodeRuntime) Stopped() bool { return rt.r.stop.Load() }

// Sleep waits d on the run's clock, or until ctx is done.
func (rt *nodeRuntime) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	select {
	case <-rt.r.after(d):
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (rt *nodeRuntime) Now() time.Time { return rt.r.deps.Clock() }

// RandomDuration picks uniformly in [lo, hi] with the run's seeded RNG.
func (rt *nodeRuntime) RandomDuration(lo, hi time.Duration) time.Duration {
	if hi <= lo {
		return lo
	}
	rt.r.rngMu.Lock()
	defer rt.r.rngMu.Unlock()
	return lo + time.Duration(rt.r.rng.Int64N(int64(hi-lo)+1))
}

// Report is the report so far ("run"), or a finished node's subtree: the
// id is looked up in the calling node's frame, then at the top.
func (rt *nodeRuntime) Report(of string) (report.Run, bool) {
	b := rt.r.builder
	if b == nil {
		return report.Run{}, false
	}
	if of == "run" {
		return b.snapshot(), true
	}
	if r, ok := b.find(rt.f.path(of)); ok {
		return r, true
	}
	return b.find(of)
}

// CheckSchema checks what request step `step` received against the response
// schema kept for its request. It finds nothing (Found false) when the step
// is not a request step of this flow, has not run, or no schema is kept.
func (rt *nodeRuntime) CheckSchema(step string) nodes.SchemaCheck {
	matcher, ok := rt.r.deps.Schemas.(SchemaMatcher)
	flow := rt.f.fs.flow
	if !ok || flow == nil {
		return nodes.SchemaCheck{}
	}
	n, ok := findStep(flow.Nodes, step)
	if !ok || n.Request == nil {
		return nodes.SchemaCheck{}
	}
	got, ok := rt.f.outputs.snapshot()[step].(map[string]any)
	if !ok {
		return nodes.SchemaCheck{}
	}
	status, _ := got["status"].(int)
	coll, req, ok := rt.r.deps.Lookup.Request(n.Request.Request, rt.f.fs.collection.Manifest.ID)
	if !ok {
		return nodes.SchemaCheck{}
	}
	// The version the step used: resolved as the send resolved it (pinned, run preference, flow preference, default).
	chosen, _, diags := resolveVersion(call{request: req, pinned: n.Request.Version, flowPrefer: rt.f.fs.prefer}, rt.r.prefer)
	if diags.HasErrors() {
		return nodes.SchemaCheck{}
	}
	version := chosen.Label
	violations, found := matcher.Match(coll, req, version, status, got["body"])
	out := nodes.SchemaCheck{Found: found, Request: req.Path, Status: status}
	for _, v := range violations {
		out.Violations = append(out.Violations, nodes.SchemaViolation{Path: v.Path, Message: v.Message})
	}
	return out
}

// findStep is the node with id among list and everything nested in it.
func findStep(list []model.Node, id string) (model.Node, bool) {
	for _, n := range list {
		if n.ID == id {
			return n, true
		}
		for _, child := range n.Children() {
			if found, ok := findStep(child, id); ok {
				return found, true
			}
		}
	}
	return model.Node{}, false
}
