// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"math/rand/v2"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// runner is one run (a standalone send or a flow run): the state shared by
// every node, request and hook flow in it. Node goroutines share it, so
// what they change is guarded: mu guards saved, err and warnings; emitMu
// guards closed and serialises sends on events; rngMu guards rng; the
// secret tracker and generators lock themselves.
type runner struct {
	deps    Deps
	in      SendInput // standalone sends only
	kind    string    // "send" or "flow"
	env     string    // requested environment name ("" → defaults.environment)
	prefer  string    // run version preference
	events  chan<- Event
	runID   string
	seed    uint64
	gen     template.GeneratorFunc
	secrets *secretTracker
	jar     *httpx.Jar // sends: the persistent jar; flows: the run's in-memory jar
	jarFile string     // "" for flow runs (their jar is not saved)

	stop     atomic.Bool // a stop node ended the run: no list starts another node
	mu       sync.Mutex
	saved    []string
	err      error
	warnings diag.List

	emitMu sync.Mutex
	closed bool // set when the run was aborted: later events are dropped

	rngMu sync.Mutex
	rng   *rand.Rand

	// ctx is the run's context, for work that outlives the node starting
	// it (background webhooks); bg counts that work.
	ctx context.Context
	bg  sync.WaitGroup

	inflight inflight       // top-level leaf nodes running, for RunAborted
	builder  *reportBuilder // flow runs: the report, fed by emit

	limit    int     // run-wide concurrency limit (plan Decision 2)
	limits   *limits // in-flight HTTP cap and rate limits, for every attempt
	laneJars string  // http.cookies for lanes: isolated | shared | off
}

// newRunState is what every run starts with.
func newRunState(d Deps, seed uint64, events chan<- Event) *runner {
	if seed == 0 {
		seed = template.NewSeed()
	}
	limits, _ := newLimits(defaultRunLimit, "", nil, d.Clock, d.After) // no rates: cannot fail
	return &runner{
		deps: d, events: events, runID: d.NewID(), seed: seed, limits: limits, ctx: context.Background(),
		gen:      lockedGenerator(template.NewGeneration(seed, d.Clock).Func()),
		secrets:  &secretTracker{vault: d.Vault, environ: d.Environ},
		rng:      rand.New(rand.NewPCG(seed, seed^delaySeedSalt)), //nolint:gosec // delays need reproducibility, not unpredictability
		limit:    defaultRunLimit,
		laneJars: "isolated",
	}
}

// defaultRunLimit is run.max_concurrency's default, for runs (sends) that
// resolve no flow settings.
const defaultRunLimit = 8

// applyLimits sets the run-wide limits from the flow and its settings:
// the flow's execution.max_concurrency wins over run.max_concurrency. A
// bad rate is FLW-FLOW-001 (validate reports it before a run).
func (r *runner) applyLimits(ex model.Execution, cfg config) diag.List {
	r.limit = cmpOrInt(ex.MaxConcurrency, cfg.maxConcurrency, defaultRunLimit)
	if cfg.cookies != "" {
		r.laneJars = cfg.cookies
	}
	limits, err := newLimits(r.limit, ex.RateLimit, ex.HostRateLimits, r.deps.Clock, r.deps.After)
	if err != nil {
		return diag.List{diag.New(diag.CodeFlowOption, diag.Location{}, map[string]string{"field": "execution.rate_limit", "detail": err.Error()})}
	}
	r.limits = limits
	return nil
}

func cmpOrInt(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

// delaySeedSalt keeps the delay RNG's stream apart from the generators'.
const delaySeedSalt = 0x9e3779b97f4a7c15

func newRunner(d Deps, in SendInput, events chan<- Event) *runner {
	r := newRunState(d, in.Seed, events)
	r.in, r.kind, r.env, r.prefer = in, "send", in.Env, in.PreferVersion
	r.jarFile = filepath.Join(d.DataDir(in.Collection.Manifest.ID), "cookies", "jar.json")
	jar, err := httpx.LoadJar(r.jarFile, d.Clock())
	if err != nil {
		r.err, jar = err, httpx.NewJar() // a damaged jar must not block sending; saving replaces it
	}
	r.jar = jar
	return r
}

// lockedGenerator serialises a generator: gofakeit's source is not safe
// for concurrent use, and nodes in different lanes generate at once.
func lockedGenerator(gen template.GeneratorFunc) template.GeneratorFunc {
	var mu sync.Mutex
	return func(name string, args []any) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		return gen(name, args)
	}
}

// nodeRunner is the runner for a node type (tests inject extra types).
func (r *runner) nodeRunner(t model.NodeType) (nodes.Runner, bool) {
	if r.deps.Nodes != nil {
		return r.deps.Nodes(t)
	}
	return nodes.Lookup(t)
}

// addSaved records files the run wrote; the first error is kept.
func (r *runner) addSaved(files []string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = append(r.saved, files...)
	if err != nil && r.err == nil {
		r.err = err
	}
}

func (r *runner) after(d time.Duration) <-chan time.Time {
	if r.deps.After != nil {
		return r.deps.After(d)
	}
	return time.After(d)
}

// run is the whole send: start, the request and its hooks, finish.
func (r *runner) run(ctx context.Context) SendResult {
	r.ctx = ctx
	start := r.deps.Clock()
	r.emit(RunStarted{RunID: r.runID, Kind: "send", Env: r.in.Env, Seed: r.seed, At: start})
	out := r.execute(ctx, call{
		collection: r.in.Collection, request: r.in.Request, pinned: r.in.PinnedVersion,
		with: r.in.With, hooks: r.in.Hooks, save: r.in.Save,
	})
	// Background webhooks started by hooks belong to the send: it waits
	// for them (they end with their call or with ctx) before its stream
	// closes, so none emits after the close.
	r.bg.Wait()
	if ctx.Err() != nil {
		out.status = StatusCancelled
		r.finalEmit(RunCancelled{RunID: r.runID})
		return r.result(out)
	}
	r.saveJar()
	r.finalEmit(RunFinished{RunID: r.runID, Status: out.status, Duration: r.deps.Clock().Sub(start)})
	return r.result(out)
}

func (r *runner) saveJar() {
	r.addSaved(nil, r.jar.Save(r.jarFile))
}

// result masks the outcome for the caller.
func (r *runner) result(out outcome) SendResult {
	m := r.secrets.current()
	res := SendResult{RunID: r.runID, Seed: r.seed, Status: out.status, Saved: r.saved, Err: r.err,
		Diags: maskDiags(out.diags, m)}
	if out.record != nil {
		masked := out.record.Masked(m)
		res.Record = &masked
	}
	if out.vars != nil {
		res.Vars, _ = maskValue(out.vars, m).(map[string]any)
	}
	return res
}

// emit sends an event. Sends are serialised and stop once the run is
// closed (aborted), so stragglers never write to a closed stream.
func (r *runner) emit(ev Event) {
	r.emitMu.Lock()
	defer r.emitMu.Unlock()
	if r.closed {
		return
	}
	r.record(ev)
	r.events <- ev
}

func (r *runner) record(ev Event) {
	if r.deps.Observe != nil {
		r.deps.Observe(ev)
	}
	if r.builder != nil {
		r.builder.add(ev)
	}
}

// finalEmit sends one of the run's closing events (RunAborted,
// RunFinished) and closes the run to every other event.
func (r *runner) finalEmit(ev Event) {
	r.emitMu.Lock()
	defer r.emitMu.Unlock()
	r.closed = true
	r.record(ev)
	r.events <- ev
}

func (r *runner) warn(d diag.Diagnostic) {
	masked := maskDiag(d, r.secrets)
	r.mu.Lock()
	r.warnings = append(r.warnings, masked)
	r.mu.Unlock()
	r.emit(Warning{RunID: r.runID, Diag: masked})
}

// call is one request to execute: the request being sent, or a request
// node inside a hook flow (depth > 0).
type call struct {
	collection model.Collection
	request    model.Request
	pinned     string
	flowPrefer string            // prefer_version of the hook flow calling it
	with       map[string]string // templates
	withValues map[string]any    // already rendered (request nodes in hook flows)
	hooks      model.HookControl
	save       string
	depth      int
	path       string            // the request node's path; "" for a standalone send
	headers    map[string]string // a request node's headers
	jar        *httpx.Jar        // the node's frame jar; nil → the run's
	noJar      bool              // the node's lane has cookies off
}

// outcome is what executing one request produced.
type outcome struct {
	status Status
	record *Record
	vars   map[string]any
	diags  diag.List
}

func maskDiags(l diag.List, m interface{ String(string) string }) diag.List {
	out := make(diag.List, len(l))
	for i, d := range l {
		out[i] = maskDiag(d, m)
	}
	return out
}

func maskDiag(d diag.Diagnostic, m interface{ String(string) string }) diag.Diagnostic {
	if len(d.Args) == 0 {
		return d
	}
	args := make(map[string]string, len(d.Args))
	for k, v := range d.Args {
		args[k] = m.String(v)
	}
	d.Args = args
	return d
}

func elapsed(clock func() time.Time, since time.Time) time.Duration { return clock().Sub(since) }

// traceparentName is the W3C Trace Context request header.
const traceparentName = "traceparent"

// addTraceparent adds the trace context of the call made by node path,
// unless the request already names one (the user's header wins).
func (r *runner) addTraceparent(h http.Header, nodePath string) {
	if r.deps.Traceparent == nil || h.Get(traceparentName) != "" {
		return
	}
	if v := r.deps.Traceparent(r.runID, nodePath); v != "" {
		h.Set(traceparentName, v)
	}
}
