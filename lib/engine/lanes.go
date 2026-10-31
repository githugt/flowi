// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/internal/nodes"
)

// Lane waits (parallel.wait).
const (
	waitAll          = "all"
	waitAny          = "any"
	waitFirstSuccess = "first-success"
)

// Lanes runs a container's lanes, at most min(o.Concurrency, run limit) at
// once, each in its own child frame. Results are in lane order; lanes that
// never started are skipped.
func (rt *nodeRuntime) Lanes(ctx context.Context, lanes []nodes.Lane, o nodes.LaneOptions) []nodes.LaneResult {
	lctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	set := &laneSet{wait: o.Wait, policy: rt.f.fs.onBranchError, cancel: cancel, results: make([]nodes.LaneResult, len(lanes))}
	if set.wait == "" {
		set.wait = waitAll
	}
	slots := make(chan struct{}, max(1, min(o.Concurrency, rt.r.limit)))
	// The errgroup owns the lane goroutines; Wait returns when every
	// started lane has settled.
	var g errgroup.Group
	started := make([]bool, len(lanes))
	for i, l := range lanes {
		if !set.acquire(lctx, slots) {
			break
		}
		started[i] = true
		g.Go(func() error {
			defer func() { <-slots }()
			set.settle(i, rt.runLane(lctx, l, o.LaneJars))
			return nil
		})
	}
	_ = g.Wait() // lanes report through set, never as errors
	for i := range lanes {
		if !started[i] {
			set.results[i] = nodes.LaneResult{Status: nodes.StatusSkipped}
		}
	}
	return set.results
}

// runLane runs one lane's nodes in a child frame.
func (rt *nodeRuntime) runLane(ctx context.Context, l nodes.Lane, laneJars bool) nodes.LaneResult {
	child := rt.laneFrame(l, laneJars)
	res := rt.r.runList(ctx, child, l.Nodes)
	out := nodes.LaneResult{Status: nodes.Status(res.status), Diags: res.diags}
	if l.OwnVars {
		out.Vars = child.vars.Own()
	}
	if l.OwnNodes {
		out.Nodes = child.outputs.own()
	}
	return out
}

// laneSet collects lane results and decides when a wait has settled or a
// failure stops the rest. mu guards every field below it.
type laneSet struct {
	wait, policy string
	cancel       context.CancelCauseFunc

	mu      sync.Mutex
	results []nodes.LaneResult
	settled bool // a wait of any / first-success has its winner
	stopped bool // start no more lanes
}

// acquire takes a slot for the next lane, unless the lanes were cancelled
// or stopped meanwhile.
func (s *laneSet) acquire(ctx context.Context, slots chan struct{}) bool {
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || ctx.Err() != nil {
		<-slots
		return false
	}
	return true
}

// settle records lane i's result and applies the wait mode and the
// on_branch_error policy.
func (s *laneSet) settle(i int, res nodes.LaneResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[i] = res
	failed := res.Status == nodes.StatusFailed || res.Status == nodes.StatusError
	switch {
	case s.settled:
	case s.wait == waitAny || (s.wait == waitFirstSuccess && res.Status == nodes.StatusPassed):
		s.settled, s.stopped = true, true
		s.results[i].First = true
		s.cancel(errLaneDone)
	case s.wait == waitAll && failed && s.policy == policyFinishRunning:
		s.stopped = true
	case s.wait == waitAll && failed && s.policy != policyContinueAll:
		s.stopped = true
		s.cancel(errBranchFailed)
	}
}

// laneFrame is the frame for one lane. Lanes of parallel and batch get
// cookies per http.cookies (plan Decision 4): isolated — a clone of the
// enclosing jar taken now; shared — the enclosing jar; off — none.
func (rt *nodeRuntime) laneFrame(l nodes.Lane, laneJars bool) *frame {
	child := rt.f.child(l, nil, rt.path)
	if !laneJars {
		return child
	}
	switch rt.r.laneJars {
	case "off":
		child.noJar = true
	case "shared":
	default:
		if jar := rt.f.cookieJar(); jar != nil {
			child.jar = jar.Clone()
		} else {
			child.jar = httpx.NewJar()
		}
		child.noJar = false
	}
	return child
}
