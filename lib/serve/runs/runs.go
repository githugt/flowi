// SPDX-License-Identifier: Apache-2.0

// Package runs starts flow runs for triggers and schedules and keeps track
// of them: how many may be in flight, which schedule is already running,
// what happened to each, and how they end when the process shuts down.
//
// It knows nothing about flows: a Starter, supplied by the workspace,
// resolves and starts them.
package runs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

// Request asks for a flow run.
type Request struct {
	Flow          string // a flow path or id
	Collection    string // optional name or id
	Env           string
	PreferVersion string
	Inputs        map[string]string
	Seed          uint64
	Source        string // "trigger" or "schedule:<name>", for records and logs
	// Key, when set, allows one run in flight per key: a schedule uses its
	// name so a slow run is not started again on top of itself.
	Key string
}

// Outcome is how a run ended.
type Outcome struct {
	EngineRunID string // the id its report is saved under
	Status      string // the engine's status text: passed, failed, error, cancelled, aborted
	Seed        uint64
	Messages    []string // the unhandled failure and warnings, in words
}

// Handle is a run in progress.
type Handle interface {
	// Wait blocks until the run has ended. The context given to the Starter
	// cancels it; the run then ends within its own grace period.
	Wait() Outcome
}

// Starter resolves and starts a run. A request it cannot start is a
// *StartError; anything else is reported as an internal error.
type Starter func(ctx context.Context, r Request) (Handle, error)

// Kind classifies a refused start.
type Kind int

// Kinds of refused starts.
const (
	KindInternal  Kind = iota // a system failure: details are logged, not shown
	KindNotFound              // the flow (or its collection) does not exist
	KindAmbiguous             // several flows match
	KindInvalid               // the inputs or the collection cannot run
)

// StartError is a start the caller can fix. Message is safe to show.
type StartError struct {
	Kind    Kind
	Message string
	Err     error // the cause, for logs
}

func (e *StartError) Error() string { return e.Message }
func (e *StartError) Unwrap() error { return e.Err }

// Errors returned by Manager.
var (
	ErrBusy         = errors.New("too many runs in flight")
	ErrOverlap      = errors.New("a run with the same key is still in flight")
	ErrClosed       = errors.New("the run manager is shutting down")
	ErrDrainTimeout = errors.New("runs were still going at the shutdown deadline and were cancelled")
	ErrStuck        = errors.New("runs did not stop after being cancelled")
)

// StatusRunning is the status of a run that has not ended.
const StatusRunning = "running"

// Record is what the manager knows about a run.
type Record struct {
	ID       string // the manager's id, available at once
	Flow     string
	Source   string
	Status   string // StatusRunning, then the outcome's status
	Started  time.Time
	Finished time.Time
	Outcome
}

// Options configure a Manager.
type Options struct {
	Start      Starter
	Max        int           // runs in flight at once (at least 1)
	Keep       int           // finished runs remembered (default 100)
	CancelWait time.Duration // how long Shutdown waits after cancelling (default 2m)
	NewID      func() string
	Now        func() time.Time
	Logger     *slog.Logger
	// Shared, when set, publishes every run so other instances can report
	// it. It fails open: a store error is logged and the run goes on.
	Shared SharedRecords
}

// SharedRecords is where runs are published for other serve instances.
type SharedRecords interface {
	SaveRecord(ctx context.Context, r Record) error
	LoadRecord(ctx context.Context, id string) (Record, bool, error)
}

// sharedTimeout bounds one call to the shared store.
const sharedTimeout = 2 * time.Second

type entry struct {
	rec  Record
	done chan struct{}
}

// Manager is safe for concurrent use.
//
// Goroutine ownership: each accepted run gets one goroutine that waits for
// its Handle; it ends when the run ends. Shutdown cancels the context the
// runs were started with.
type Manager struct {
	opts   Options
	ctx    context.Context
	cancel context.CancelCauseFunc

	mu       sync.Mutex // guards everything below
	runs     map[string]*entry
	finished []string          // finished run ids, oldest first
	keys     map[string]string // key → id of its in-flight run
	inflight int
	closed   bool
	idle     chan struct{} // closed when closed && inflight == 0
}

// NewManager returns a manager.
func NewManager(o Options) *Manager {
	if o.Max < 1 {
		o.Max = 1
	}
	if o.Keep < 1 {
		o.Keep = 100
	}
	if o.CancelWait <= 0 {
		o.CancelWait = 2 * time.Minute
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	return &Manager{opts: o, ctx: ctx, cancel: cancel, runs: map[string]*entry{}, keys: map[string]string{}, idle: make(chan struct{})}
}

// Start begins a run. ErrBusy, ErrOverlap and ErrClosed refuse it; a
// *StartError says why the Starter could not start it.
func (m *Manager) Start(r Request) (Record, error) {
	if err := m.reserve(r.Key); err != nil {
		return Record{}, err
	}
	h, err := m.startSafely(r)
	if err != nil {
		m.release(r.Key)
		return Record{}, err
	}
	rec := m.register(r)
	m.publish(rec)
	go m.await(rec.ID, r.Key, h)
	return rec, nil
}

// reserve takes a slot (and the key) before the Starter runs, so the limit
// holds while several starts are resolving at once.
func (m *Manager) reserve(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case m.closed:
		return ErrClosed
	case m.inflight >= m.opts.Max:
		return ErrBusy
	case key != "" && m.keys[key] != "":
		return fmt.Errorf("%w: %s", ErrOverlap, key)
	}
	m.inflight++
	if key != "" {
		m.keys[key] = "pending"
	}
	return nil
}

func (m *Manager) release(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inflight--
	if key != "" {
		delete(m.keys, key)
	}
	m.signalIdle()
}

func (m *Manager) startSafely(r Request) (h Handle, err error) {
	defer func() {
		if p := recover(); p != nil {
			m.opts.Logger.Error("starting a run panicked", "Event", "RunStartPanic", "Flow", r.Flow, "Panic", fmt.Sprint(p))
			h, err = nil, &StartError{Kind: KindInternal, Message: "the run could not be started"}
		}
	}()
	h, err = m.opts.Start(m.ctx, r)
	var se *StartError
	if err != nil && !errors.As(err, &se) {
		m.opts.Logger.Error("starting a run failed", "Event", "RunStartFailed", "Flow", r.Flow, "Error", err)
		err = &StartError{Kind: KindInternal, Message: "the run could not be started", Err: err}
	}
	return h, err
}

func (m *Manager) register(r Request) Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := Record{ID: m.opts.NewID(), Flow: r.Flow, Source: r.Source, Status: StatusRunning, Started: m.opts.Now()}
	m.runs[rec.ID] = &entry{rec: rec, done: make(chan struct{})}
	if r.Key != "" {
		m.keys[r.Key] = rec.ID
	}
	return rec
}

// publish tells the shared store about rec; a failure is logged only.
func (m *Manager) publish(rec Record) {
	if m.opts.Shared == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), sharedTimeout)
	defer cancel()
	if err := m.opts.Shared.SaveRecord(ctx, rec); err != nil {
		m.opts.Logger.Warn("publishing a run failed", "Event", "RunPublishFailed", "RunID", rec.ID, "Error", err)
	}
}

// Find is Get that also asks the shared store, for runs of other instances.
func (m *Manager) Find(ctx context.Context, id string) (Record, bool) {
	if rec, ok := m.Get(id); ok {
		return rec, true
	}
	if m.opts.Shared == nil {
		return Record{}, false
	}
	rec, ok, err := m.opts.Shared.LoadRecord(ctx, id)
	if err != nil {
		m.opts.Logger.Warn("reading a shared run failed", "Event", "RunLookupFailed", "RunID", id, "Error", err)
		return Record{}, false
	}
	return rec, ok
}

func (m *Manager) await(id, key string, h Handle) {
	out := h.Wait()
	rec := m.ended(id, out)
	// Published before done closes: whoever Awaits or Shuts down then sees
	// the shared store already hold the final state, not a stale "running".
	m.publish(rec)
	m.finish(id, key, rec)
}

// ended is the record of run id once it has out, without recording it yet.
func (m *Manager) ended(id string, out Outcome) Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.runs[id].rec
	rec.Outcome, rec.Status, rec.Finished = out, out.Status, m.opts.Now()
	return rec
}

func (m *Manager) finish(id, key string, rec Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.runs[id]
	e.rec = rec
	close(e.done)
	m.inflight--
	if key != "" {
		delete(m.keys, key)
	}
	m.finished = append(m.finished, id)
	for len(m.finished) > m.opts.Keep {
		delete(m.runs, m.finished[0])
		m.finished = m.finished[1:]
	}
	m.signalIdle()
}

// signalIdle closes idle once shutdown has begun and nothing is in flight.
// Callers hold mu.
func (m *Manager) signalIdle() {
	if m.closed && m.inflight == 0 {
		select {
		case <-m.idle:
		default:
			close(m.idle)
		}
	}
}

// Get returns a run's record, if it is still remembered.
func (m *Manager) Get(id string) (Record, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.runs[id]
	if !ok {
		return Record{}, false
	}
	return e.rec, true
}

// Await waits for a run to end (or ctx to end) and returns its record.
func (m *Manager) Await(ctx context.Context, id string) (Record, bool) {
	m.mu.Lock()
	e, ok := m.runs[id]
	m.mu.Unlock()
	if !ok {
		return Record{}, false
	}
	select {
	case <-e.done:
	case <-ctx.Done():
	}
	return m.Get(id)
}

// InFlight is the number of runs that have not ended.
func (m *Manager) InFlight() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inflight
}

// Draining reports whether Shutdown has begun.
func (m *Manager) Draining() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// Shutdown stops accepting runs and waits for the ones in flight until ctx
// ends. Runs still going then are cancelled and given CancelWait to stop.
// It returns nil when everything finished by itself, ErrDrainTimeout when
// runs had to be cancelled, and ErrStuck (wrapping ErrDrainTimeout) when
// some did not stop at all.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	m.signalIdle()
	m.mu.Unlock()
	select {
	case <-m.idle:
		m.cancel(ErrClosed)
		return nil
	case <-ctx.Done():
	}
	m.cancel(ErrDrainTimeout)
	timer := time.NewTimer(m.opts.CancelWait)
	defer timer.Stop()
	select {
	case <-m.idle:
		return ErrDrainTimeout
	case <-timer.C:
		return fmt.Errorf("%w: %w", ErrStuck, ErrDrainTimeout)
	}
}
