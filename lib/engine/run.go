// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
	"github.com/githugt/flowi/lib/template"
)

// RunInput is one flow run.
type RunInput struct {
	Collection    model.Collection
	Flow          model.Flow
	Env           string            // environment name; "" → defaults.environment
	PreferVersion string            // run preference; beats the flow's prefer_version
	Inputs        map[string]string // templates for the flow's declared inputs
	Seed          uint64            // 0 → a random seed, reported in the result
	Grace         time.Duration     // 0 → the run.grace setting
	// Only, when non-nil, runs just these top-level nodes (in flow order);
	// the rest are left out of the run and its report. Prior holds the
	// outputs of the nodes left out, as an earlier run reported them
	// (masked, so a secret in them reads as the mask), for the chosen nodes'
	// templates to read as nodes.<id>. Prior is read by a plain flow only,
	// not by a flow-level run: loop.
	Only  []string
	Prior map[string]map[string]any
	// Skip lists node ids (or node paths) that do not run this time: they
	// finish as skipped with the reason "skipped by you", wherever they are,
	// in every iteration of a loop. A skipped node whose id is in Prior keeps
	// those outputs ("hold"), so later steps read the old values; the
	// flow's variables come from PriorVars as for a partial run.
	Skip []string
	// Overlay is a flowi overlay (or a stack, "vip+slow") laid over every request step that names no overlay of
	// its own. A step's own overlay wins. "" lays none.
	Overlay string
	// PriorVars are the flow's variables as an earlier run ended (masked),
	// for the chosen nodes' templates to read as vars.<name>: a node left out
	// of a partial run may have set them.
	PriorVars map[string]any
}

// RunResult is a finished flow run. Everything in it is masked.
type RunResult struct {
	RunID  string
	Seed   uint64
	Status Status
	// Diags are the unhandled failure's diagnostics and every warning.
	Diags diag.List
	Saved []string // files written (saved responses, values, digests)
	// Captured are the masked records of every call the run made, in the order they finished: what a HAR
	// file is written from. A hook flow's calls are included.
	Captured []Record
	// Report is the run's report; ReportFile where it was saved
	// (runs/<flow-id>/<run-id>.json in the workspace data dir).
	Report     report.Run
	ReportFile string
	// Err is a system failure that did not stop the run (a file could not
	// be saved).
	Err error
}

// Run is a flow run in progress.
//
// Goroutine ownership: StartRun starts the run goroutine, which owns every
// node goroutine through the scheduler; all of them exit with the run's
// context. Events is closed after RunFinished, then Wait returns. Events
// must be drained: a full buffer blocks the run.
type Run struct {
	events chan Event
	done   chan struct{}
	result RunResult
}

// StartRun starts running in.Flow. Cancelling ctx stops the run: in-flight
// requests are aborted and no new nodes start.
func StartRun(ctx context.Context, d Deps, in RunInput) *Run {
	run := &Run{events: make(chan Event, eventBuffer), done: make(chan struct{})}
	go func() {
		defer close(run.done)
		defer close(run.events)
		run.result = newFlowRunner(d, in, run.events).runFlow(ctx, in)
	}()
	return run
}

// Events streams the run's events; it is closed when the run ends.
func (r *Run) Events() <-chan Event { return r.events }

// Wait blocks until the run has ended and returns its result.
func (r *Run) Wait() RunResult {
	<-r.done
	return r.result
}

func newFlowRunner(d Deps, in RunInput, events chan<- Event) *runner {
	r := newRunState(d, in.Seed, events)
	r.kind, r.env, r.prefer, r.overlay = "flow", in.Env, in.PreferVersion, in.Overlay
	r.builder = newReportBuilder(d.Clock, in.Flow.Name, in.Collection.Manifest.Name)
	r.jar = httpx.NewJar() // one in-memory jar per run (design §6.3)
	return r
}

// runFlow is the whole run: start, the flow's nodes, finish.
func (r *runner) runFlow(ctx context.Context, in RunInput) RunResult {
	r.ctx = ctx
	defer r.conns.closeAll() // also when the run is aborted: no connection outlives its run
	start := r.deps.Clock()
	cfg := newConfig(r.deps.Lookup.Settings(in.Collection.Manifest.ID, flowFolder(in.Flow), nil))
	r.emit(RunStarted{RunID: r.runID, Kind: "flow", FlowID: string(in.Flow.ID), FlowPath: in.Flow.Path, Env: in.Env, Overlay: in.Overlay, Seed: r.seed, At: start})
	done := make(chan listOutcome, 1)
	// The flow goroutine is owned by the run: it ends when the flow's
	// nodes and background calls have ended, which cancellation makes
	// prompt. After an abort nothing waits for it; it still ends once the
	// stuck work returns, and its late events are dropped.
	go func() {
		out := r.flowOutcome(ctx, in, cfg)
		r.bg.Wait()
		r.conns.closeAll() // an aborted run does not wait for this goroutine; the connections must not wait for it either
		done <- out
	}()
	out := r.awaitFlow(ctx, done, cmpOrDuration(in.Grace, cfg.grace, defaultGrace))
	r.finalEmit(RunFinished{RunID: r.runID, Status: out.status, Duration: elapsed(r.deps.Clock, start)})
	res := r.runResult(out)
	res.Report = r.builder.snapshot()
	res.Report.Vars = r.finalVars()
	res.Report.Outputs = r.finalOutputs()
	res.ReportFile, res.Err = r.saveReport(in, res.Report, res.Err)
	return res
}

// saveReport writes the report as JSON, masked once more with every secret
// the run read (a secret read late may postdate events built earlier).
func (r *runner) saveReport(in RunInput, rep report.Run, prevErr error) (string, error) {
	var buf bytes.Buffer
	if err := report.WriteJSON(&buf, rep); err != nil {
		return "", errors.Join(prevErr, err)
	}
	file := filepath.Join(r.deps.DataDir(in.Collection.Manifest.ID), "runs", string(in.Flow.ID), r.runID+".json")
	if err := writeFileAtomic(file, []byte(r.secrets.String(buf.String()))); err != nil {
		return "", errors.Join(prevErr, fmt.Errorf("saving the run report: %w", err))
	}
	return file, prevErr
}

// defaultGrace is run.grace's default (design §7.8).
const defaultGrace = 60 * time.Second

// graceExceeded is RunAborted's reason when stuck work outlived the grace
// period.
const graceExceeded = "grace_period_exceeded"

// awaitFlow waits for the flow. Once ctx is cancelled it reports
// RunCancelled and gives the flow the grace period to stop; past it the
// run is aborted: RunAborted names the nodes still running and later
// events are dropped (design §7.8).
func (r *runner) awaitFlow(ctx context.Context, done <-chan listOutcome, grace time.Duration) listOutcome {
	select {
	case out := <-done:
		if ctx.Err() == nil {
			return out
		}
		// Finished just as the cancel landed: nothing is left to wait for.
		r.emit(RunCancelled{RunID: r.runID})
		out.status = StatusCancelled
		return out
	case <-ctx.Done():
	}
	r.emit(RunCancelled{RunID: r.runID})
	select {
	case out := <-done:
		out.status = StatusCancelled // keep what failed before the cancel
		return out
	case <-r.after(grace):
		r.finalEmit(RunAborted{RunID: r.runID, Reason: graceExceeded, Grace: grace, Inflight: r.inflight.snapshot()})
		return listOutcome{status: StatusAborted}
	}
}

func cmpOrDuration(values ...time.Duration) time.Duration {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

// flowOutcome resolves the environment and runs the flow's nodes.
func (r *runner) flowOutcome(ctx context.Context, in RunInput, cfg config) listOutcome {
	env, diags := selectEnvironment(in.Collection, in.Env, cfg.defaultEnv)
	if diags.HasErrors() {
		return listOutcome{status: StatusFailed, diags: diags}
	}
	if diags := r.applyLimits(in.Flow.Execution, cfg); diags.HasErrors() {
		return listOutcome{status: StatusFailed, diags: diags}
	}
	fs := &flowScope{collection: in.Collection, flow: &in.Flow, env: env, prefer: in.Flow.PreferVersion,
		onBranchError: in.Flow.Execution.OnBranchError}
	root := newRootFrame(fs, r.jar)
	inputs, diags := r.flowInputs(in, root.scope(r))
	if diags.HasErrors() {
		return listOutcome{status: StatusFailed, diags: diags}
	}
	fs.inputs = inputs
	r.keepVars(root.vars, in.PriorVars)
	r.skip, r.prior = skipSet(in.Skip), in.Prior
	list := in.Flow.Nodes
	if in.Only != nil {
		list = onlyNodes(list, in.Only)
		in.Flow.Nodes = list
	}
	if in.Flow.Run != nil {
		return r.runIterations(ctx, fs, root.vars, in.Flow, in.Prior)
	}
	seedPrior(root.outputs, in.Prior, list)
	out := r.runList(ctx, root, list)
	r.keepOutputs(ctx, in.Flow, root.scope(r), out.status)
	return out
}

// flowInputs renders the given --input templates and resolves every
// declared input.
func (r *runner) flowInputs(in RunInput, sc template.Scope) (map[string]any, diag.List) {
	given, diags := r.renderGiven(in.Inputs, sc)
	inputs, more := r.resolveInputs(in.Flow, given, sc)
	return inputs, append(diags, more...)
}

// flowFolder is the collection-relative folder of a flow file.
func flowFolder(f model.Flow) string { return path.Join("flows", path.Dir(f.Path)) }

func (r *runner) runResult(out listOutcome) RunResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.secrets.current()
	diags := append(maskDiags(out.diags, m), r.warnings...)
	return RunResult{RunID: r.runID, Seed: r.seed, Status: out.status, Diags: diags, Saved: r.saved, Captured: r.captured, Err: r.err}
}

// onlyNodes keeps the listed nodes in flow order. A kept node's needs on a
// left-out node are dropped: that node's outputs come from RunInput.Prior.
func onlyNodes(list []model.Node, only []string) []model.Node {
	keep := make(map[string]bool, len(only))
	for _, id := range only {
		keep[id] = true
	}
	var out []model.Node
	for _, n := range list {
		if !keep[n.ID] {
			continue
		}
		n.Needs = slices.DeleteFunc(slices.Clone(n.Needs), func(id string) bool { return !keep[id] })
		n.AfterFailureOf = slices.DeleteFunc(slices.Clone(n.AfterFailureOf), func(id string) bool { return !keep[id] })
		out = append(out, n)
	}
	return out
}

// seedPrior makes the outputs of nodes outside list readable as nodes.<id>.
func seedPrior(o *outputs, prior map[string]map[string]any, list []model.Node) {
	running := make(map[string]bool, len(list))
	for _, n := range list {
		running[n.ID] = true
	}
	for id, v := range prior {
		if !running[id] {
			o.put(id, v)
		}
	}
}
