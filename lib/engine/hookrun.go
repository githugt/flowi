// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"strconv"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/ratelimit"
)

// runPhase runs hook steps in order and stops at the first that fails.
// It reports whether every step passed.
func (x *execution) runPhase(ctx context.Context, refs []HookRef, phase nodes.Phase) bool {
	return x.runPhaseNamed(ctx, refs, phase, map[nodes.Phase]string{nodes.PhasePre: "pre", nodes.PhasePost: "post"}[phase])
}

// runPhaseNamed is runPhase for a phase with its own name in events and
// reports: "message" hooks run with the rules of the post phase (the request
// has gone) and are named apart.
func (x *execution) runPhaseNamed(ctx context.Context, refs []HookRef, phase nodes.Phase, name string) bool {
	x.state.Phase = phase
	for i, ref := range refs {
		if ctx.Err() != nil {
			return false
		}
		start := x.r.deps.Clock()
		x.r.emit(HookStarted{RunID: x.r.runID, Path: x.c.path, Level: ref.Level, Phase: name, Step: ref.Label(), Index: i, Depth: x.c.depth})
		status, diags := x.runStep(ctx, ref, name+":"+ref.Level+":"+strconv.Itoa(i))
		x.r.emit(HookFinished{RunID: x.r.runID, Path: x.c.path, Level: ref.Level, Phase: name, Step: ref.Label(), Index: i, Depth: x.c.depth, Status: status,
			Duration: elapsed(x.r.deps.Clock, start), Diags: maskDiags(diags, x.r.secrets.current())})
		if status != StatusPassed {
			x.fail(StatusFailed, diags)
			return false
		}
	}
	return true
}

// runStep runs one hook step — a hook flow or inline nodes — on the flow
// scheduler, in a frame whose vars are the request's (hooks set request
// vars, design §4.5). A hook step is failed whatever went wrong inside it
// (M2 rule); cancellation is the run's to report, so it passes here.
func (x *execution) runStep(ctx context.Context, ref HookRef, label string) (Status, diag.List) {
	fs := &flowScope{collection: x.c.collection, env: x.env, hook: x, phase: x.state.Phase}
	list := ref.Step.Inline
	if f := ref.Step.Flow; f != nil {
		coll, flow, ok := x.r.deps.Lookup.Flow(*f, x.c.collection.Manifest.ID)
		if !ok {
			return StatusFailed, diag.List{diag.New(diag.CodeRefUnknownFlow, diag.Location{}, map[string]string{"id": string(f.ID)})}
		}
		fs.collection, fs.flow, fs.prefer, fs.onBranchError = coll, &flow, flow.PreferVersion, flow.Execution.OnBranchError
		list = flow.Nodes
	}
	out := x.r.runList(ctx, x.hookFrame(fs, label), list)
	if out.status == StatusPassed || out.status == StatusCancelled {
		return StatusPassed, nil
	}
	return StatusFailed, out.diags
}

// hookFrame is the frame a hook step's nodes run in: the request's vars
// and cookie jar (for a standalone send, the persistent one), fresh
// nodes.<id>, one level deeper.
func (x *execution) hookFrame(fs *flowScope, label string) *frame {
	prefix := label + "/"
	if x.c.path != "" {
		prefix = x.c.path + "/" + label + "/"
	}
	return &frame{fs: fs, vars: x.state.Vars, outputs: newOutputs(nil), jar: x.jar(), noJar: x.c.noJar,
		prefix: prefix, parentPath: x.c.path, lane: label, depth: x.c.depth + 1}
}

// nodeOutputs are a request node's outputs: extracted values, then
// status, headers, body, time, the quota the server reported (ratelimit) and the vars its hooks set.
func nodeOutputs(o outcome) map[string]any {
	out := map[string]any{"vars": o.vars}
	if o.record == nil {
		return out
	}
	for k, v := range o.record.Outputs {
		out[k] = v
	}
	headers := map[string]any{}
	for _, h := range o.record.Header {
		headers[h.Name] = h.Value
	}
	var body any = o.record.Body.Text
	if o.record.Body.JSON != nil {
		body = scopeValue(o.record.Body.JSON)
	}
	out["status"], out["headers"], out["body"], out["time"] = o.record.Status, headers, body, o.record.Timings.Total
	if quota, found := ratelimit.Read(headerOf(o.record), time.Now()); found {
		out["ratelimit"] = quota.Values()
	}
	return out
}
