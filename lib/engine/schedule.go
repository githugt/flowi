// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"slices"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Cancel causes the scheduler uses, so cancelled work can tell why.
var (
	errBranchFailed = errors.New("a sibling failed (on_branch_error: fail_fast)")
	errLaneDone     = errors.New("the wait was settled by another branch")
)

// Branch-error policies (design §7.2).
const (
	policyFailFast      = "fail_fast"
	policyFinishRunning = "finish_running"
	policyContinueAll   = "continue_all"
)

// listOutcome is how a node list ended.
type listOutcome struct {
	status Status
	diags  diag.List // the unhandled failures' diagnostics
}

type nodeState int

const (
	statePending nodeState = iota
	stateRunning
	stateFinished // ran, or was skipped
)

// listRun schedules one node list (design §7.2, plan Decision 1). The
// coordinator — the goroutine calling runList — owns every field; node
// goroutines only send completions on done, so nothing here is locked.
type listRun struct {
	r       *runner
	f       *frame
	list    []model.Node
	deps    [][]int
	state   []nodeState
	results []nodeResult
	blocked []bool // a dependency failed (or was blocked): never runs
	forced  []bool // a goto target: ready whatever its dependencies say
	policy  string
	ctx     context.Context
	cancel  context.CancelCauseFunc
	done    chan completion
	running int
	stopped bool // start nothing new
	failed  []nodeResult
	// interrupted: cancellation cut the list short (a node was cancelled
	// or never started).
	interrupted bool
}

type completion struct {
	i   int
	res nodeResult
}

// runList runs a node list in frame f: serially in list order unless
// nodes opt into concurrency, honouring needs, on_error and the flow's
// on_branch_error.
func (r *runner) runList(ctx context.Context, f *frame, list []model.Node) listOutcome {
	lr := newListRun(ctx, r, f, list)
	defer lr.cancel(nil)
	// The errgroup owns the node goroutines; each reports on done before
	// returning, so Wait only reaps goroutines that have already finished.
	var g errgroup.Group
	for {
		for lr.advance(&g) {
		}
		if lr.running == 0 {
			break
		}
		lr.finish(<-lr.done)
	}
	_ = g.Wait() // node goroutines never return errors; results come through done
	lr.skipRest()
	return lr.outcome()
}

func newListRun(ctx context.Context, r *runner, f *frame, list []model.Node) *listRun {
	lctx, cancel := context.WithCancelCause(ctx)
	n := len(list)
	lr := &listRun{
		r: r, f: f, list: list, deps: dependencies(list), state: make([]nodeState, n), results: make([]nodeResult, n),
		blocked: make([]bool, n), forced: make([]bool, n), policy: f.fs.onBranchError, ctx: lctx, cancel: cancel,
		done: make(chan completion, n),
	}
	if lr.policy == "" {
		lr.policy = policyFailFast
	}
	return lr
}

// dependencies are each node's needs, or the previous node without them.
func dependencies(list []model.Node) [][]int {
	index := make(map[string]int, len(list))
	for i, n := range list {
		index[n.ID] = i
	}
	out := make([][]int, len(list))
	for i, n := range list {
		for _, id := range n.Needs {
			if j, ok := index[id]; ok {
				out[i] = append(out[i], j)
			}
		}
		if len(n.Needs) == 0 && i > 0 {
			out[i] = []int{i - 1}
		}
	}
	return out
}

// advance starts or skips one pending node and reports whether it did.
func (lr *listRun) advance(g *errgroup.Group) bool {
	if lr.stopped || lr.ctx.Err() != nil || lr.r.stop.Load() {
		return false
	}
	for i := range lr.list {
		if lr.state[i] != statePending {
			continue
		}
		if lr.dependencyBlocked(i) {
			lr.skip(i, true)
			return true
		}
		if lr.ready(i) {
			lr.start(g, i)
			return true
		}
	}
	return false
}

// ready: every dependency finished, and — unless the node is concurrent —
// every earlier node too.
func (lr *listRun) ready(i int) bool {
	if !lr.forced[i] {
		for _, d := range lr.deps[i] {
			if lr.state[d] != stateFinished {
				return false
			}
		}
	}
	if lr.list[i].Concurrent {
		return true
	}
	for j := range i {
		if lr.state[j] != stateFinished {
			return false
		}
	}
	return true
}

func (lr *listRun) dependencyBlocked(i int) bool {
	if lr.forced[i] {
		return false
	}
	for _, d := range lr.deps[i] {
		if lr.blocked[d] || (lr.state[d] == stateFinished && lr.unhandledFailure(lr.results[d])) {
			return true
		}
	}
	return false
}

func (lr *listRun) unhandledFailure(res nodeResult) bool {
	return !res.handled && (res.status == StatusFailed || res.status == StatusError || res.status == StatusCancelled)
}

func (lr *listRun) start(g *errgroup.Group, i int) {
	lr.state[i] = stateRunning
	lr.running++
	n := lr.list[i]
	if lr.f.depth == 0 && !isContainer(n.Type) {
		lr.r.inflight.add(lr.f.path(n.ID))
	}
	g.Go(func() error {
		lr.done <- completion{i: i, res: lr.r.runNode(lr.ctx, lr.f, n)}
		return nil
	})
}

// finish records a completed node and applies on_error and
// on_branch_error.
func (lr *listRun) finish(c completion) {
	lr.state[c.i], lr.results[c.i] = stateFinished, c.res
	lr.running--
	n := lr.list[c.i]
	lr.r.inflight.remove(lr.f.path(n.ID))
	switch {
	case c.res.handled:
		lr.r.warn(diag.New(diag.CodeRunHandled, diag.Location{}, map[string]string{"node": n.ID, "on_error": n.OnError}))
		lr.jump(c.i, n.OnError)
	case c.res.status == StatusCancelled && lr.ctx.Err() != nil:
		lr.interrupted = true
	case lr.unhandledFailure(c.res):
		lr.failed = append(lr.failed, c.res)
		lr.applyPolicy()
	}
}

// jump applies goto:<id>: the nodes between are skipped and the target
// runs next. Validation guarantees the target is a later sibling.
func (lr *listRun) jump(from int, onError string) {
	target, isGoto := strings.CutPrefix(onError, "goto:")
	if !isGoto {
		return
	}
	j := slices.IndexFunc(lr.list, func(n model.Node) bool { return n.ID == target })
	if j <= from {
		return
	}
	for k := from + 1; k < j; k++ {
		if lr.state[k] == statePending {
			lr.skip(k, false)
		}
	}
	lr.forced[j] = true
}

func (lr *listRun) applyPolicy() {
	switch lr.policy {
	case policyContinueAll:
	case policyFinishRunning:
		lr.stopped = true
	default:
		lr.stopped = true
		lr.cancel(errBranchFailed)
	}
}

// skip reports a node that will not run; blocked ones block their
// dependants too.
func (lr *listRun) skip(i int, blocked bool) {
	lr.state[i], lr.blocked[i] = stateFinished, blocked
	lr.results[i] = nodeResult{status: StatusSkipped}
	lr.r.reportSkipped(lr.f, lr.list[i])
}

// skipRest reports every node that never started. When nothing stopped or
// failed the list and nodes are still pending, they were waiting for each
// other (a needs on a later node): that is a failure of the run, not a pass
// with steps left out.
func (lr *listRun) skipRest() {
	if stuck := lr.pendingIDs(); len(stuck) > 0 && !lr.stopped && !lr.interrupted && !lr.r.stop.Load() && lr.ctx.Err() == nil && len(lr.failed) == 0 {
		lr.failed = append(lr.failed, nodeResult{status: StatusError, diags: diag.List{
			diag.New(diag.CodeCycleNeeds, diag.Location{}, map[string]string{"cycle": strings.Join(stuck, " → ")}),
		}})
	}
	for i := range lr.list {
		if lr.state[i] == statePending {
			lr.interrupted = lr.interrupted || lr.ctx.Err() != nil
			lr.skip(i, false)
		}
	}
}

// pendingIDs are the nodes that have not started or been skipped.
func (lr *listRun) pendingIDs() []string {
	var ids []string
	for i, n := range lr.list {
		if lr.state[i] == statePending {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

// outcome: unhandled failures decide (an error outranks a failure);
// otherwise a list cut short by cancellation is cancelled.
func (lr *listRun) outcome() listOutcome {
	if len(lr.failed) > 0 {
		out := listOutcome{status: StatusFailed}
		for _, f := range lr.failed {
			if rank(f.status) > rank(out.status) {
				out.status = f.status
			}
			out.diags = append(out.diags, f.diags...)
		}
		return out
	}
	if lr.interrupted {
		return listOutcome{status: StatusCancelled}
	}
	return listOutcome{status: StatusPassed}
}

// reportSkipped emits a node that did not run.
func (r *runner) reportSkipped(f *frame, n model.Node) {
	path := f.path(n.ID)
	r.emit(NodeStarted{RunID: r.runID, Path: path, Parent: f.parentPath, Lane: f.lane, Node: n.ID, Type: string(n.Type), Name: n.Name, Depth: f.depth})
	r.emit(NodeFinished{RunID: r.runID, Path: path, Parent: f.parentPath, Lane: f.lane, Node: n.ID, Type: string(n.Type), Status: StatusSkipped, Depth: f.depth})
}
