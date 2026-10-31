// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/githugt/flowi-lib/engine"
)

const (
	tracesPath         = "/v1/traces"
	defaultQueue       = 64
	defaultTimeout     = 10 * time.Second
	defaultServiceName = "flowi"
	// maxOpenRuns bounds the traces being built at once; a run that never
	// ends is evicted, oldest first, rather than kept forever.
	maxOpenRuns = 256
	// maxKeptErrors bounds the export errors Close reports.
	maxKeptErrors = 5
)

// Config configures an Exporter. Endpoint is the collector's base URL
// ("http://localhost:4318"); "/v1/traces" is appended unless it is there.
type Config struct {
	Endpoint  string
	Headers   map[string]string // sent with every export (typically a token)
	Propagate bool              // add traceparent to outgoing requests
	Service   string            // service.name; default "flowi"
	Version   string            // service.version
	Timeout   time.Duration     // per export; default 10s
	QueueSize int               // finished traces waiting to be sent; default 64
	Now       func() time.Time  // nil → time.Now
	Client    *http.Client      // nil → a client with Timeout
}

// Exporter builds traces from engine events and sends them to a collector.
//
// Goroutine ownership: New starts one worker that posts queued traces; Close
// stops accepting traces, lets the worker finish what is queued (or gives
// up when its context ends) and waits for it. Observe and Traceparent are
// safe for concurrent use; Observe never blocks.
type Exporter struct {
	cfg    Config
	log    *slog.Logger
	url    string
	client *http.Client

	mu     sync.Mutex // guards runs, order, closed and sending on queue
	runs   map[string]*trace
	order  []string
	closed bool
	queue  chan []byte

	dropped atomic.Int64
	errMu   sync.Mutex
	errs    []error
	done    chan struct{}
}

// New starts an exporter. An empty or malformed endpoint is an error:
// callers create an Exporter only when OTEL is switched on.
func New(cfg Config, log *slog.Logger) (*Exporter, error) {
	endpoint, err := tracesURL(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if cfg.Service == "" {
		cfg.Service = defaultServiceName
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueue
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	e := &Exporter{cfg: cfg, log: log, url: endpoint, client: client, runs: map[string]*trace{}, queue: make(chan []byte, cfg.QueueSize), done: make(chan struct{})}
	go e.work()
	return e, nil
}

func tracesURL(endpoint string) (string, error) {
	if endpoint == "" {
		return "", errors.New("otel: no endpoint")
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("otel: endpoint %q is not an http(s) URL", endpoint)
	}
	if !strings.HasSuffix(u.Path, tracesPath) {
		u.Path = strings.TrimRight(u.Path, "/") + tracesPath
	}
	return u.String(), nil
}

// Traceparent is the header value naming the span of the call made by node
// path ("" for a standalone send) of run runID, or "" when propagation is
// off. It matches engine.Deps.Traceparent.
func (e *Exporter) Traceparent(runID, path string) string {
	if !e.cfg.Propagate {
		return ""
	}
	span := rootSpanID(runID)
	if path != "" {
		span = nodeSpanID(runID, path)
	}
	return traceparent(traceID(runID), span)
}

// Observe folds one engine event into its run's trace and queues the trace
// when the run ends. It matches engine.Deps.Observe.
func (e *Exporter) Observe(ev engine.Event) {
	id, ok := runIDOf(ev)
	if !ok {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	t := e.traceFor(id, ev)
	if t == nil || !t.apply(ev, e.cfg.Now()) {
		return
	}
	e.forget(id)
	e.enqueue(t)
}

// traceFor returns the run's trace, starting it at RunStarted. Events of
// runs that were never seen (or already ended) are ignored.
func (e *Exporter) traceFor(id string, ev engine.Event) *trace {
	if t, ok := e.runs[id]; ok {
		return t
	}
	if _, starts := ev.(engine.RunStarted); !starts {
		return nil
	}
	if len(e.order) >= maxOpenRuns {
		e.forget(e.order[0])
		e.dropped.Add(1)
	}
	t := newTrace(id)
	e.runs[id] = t
	e.order = append(e.order, id)
	return t
}

func (e *Exporter) forget(id string) {
	delete(e.runs, id)
	for i, o := range e.order {
		if o == id {
			e.order = append(e.order[:i], e.order[i+1:]...)
			return
		}
	}
}

// enqueue encodes the trace and hands it to the worker, dropping it when
// the queue is full: a slow collector must not slow a run.
func (e *Exporter) enqueue(t *trace) {
	body, err := encodeTrace(t, e.cfg.Service, e.cfg.Version)
	if err != nil {
		e.record(fmt.Errorf("encoding trace %s: %w", t.id, err))
		return
	}
	select {
	case e.queue <- body:
	default:
		e.dropped.Add(1)
	}
}

// Dropped is the number of traces discarded because the queue was full or
// too many runs were open.
func (e *Exporter) Dropped() int64 { return e.dropped.Load() }

func (e *Exporter) work() {
	defer close(e.done)
	for body := range e.queue {
		if err := e.post(body); err != nil {
			e.record(err)
		}
	}
}

func (e *Exporter) post(body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), e.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building the export request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range e.cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("exporting a trace: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // the answer is not needed past its status
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("exporting a trace: collector answered %d", resp.StatusCode)
	}
	return nil
}

// record keeps an export error for Close and logs it once; this is the
// exporter's boundary, so it is the place that logs.
func (e *Exporter) record(err error) {
	e.log.Warn("trace export failed", "Event", "OTelExportFailed", "Error", err.Error())
	e.errMu.Lock()
	defer e.errMu.Unlock()
	if len(e.errs) < maxKeptErrors {
		e.errs = append(e.errs, err)
	}
}

// Close sends what is queued and stops the worker. It returns the export
// errors seen so far, or ctx's error when the flush did not finish in time.
// Traces of runs still open are discarded.
func (e *Exporter) Close(ctx context.Context) error {
	e.mu.Lock()
	if !e.closed {
		e.closed = true
		close(e.queue)
	}
	e.mu.Unlock()
	select {
	case <-e.done:
	case <-ctx.Done():
		return fmt.Errorf("flushing traces: %w", context.Cause(ctx))
	}
	e.errMu.Lock()
	defer e.errMu.Unlock()
	return errors.Join(e.errs...)
}

// runIDOf extracts the run id of the events a trace is built from.
func runIDOf(ev engine.Event) (string, bool) {
	switch e := ev.(type) {
	case engine.RunStarted:
		return e.RunID, true
	case engine.NodeStarted:
		return e.RunID, true
	case engine.NodeFinished:
		return e.RunID, true
	case engine.HookStarted:
		return e.RunID, true
	case engine.HookFinished:
		return e.RunID, true
	case engine.RequestSent:
		return e.RunID, true
	case engine.ResponseReceived:
		return e.RunID, true
	case engine.RunCancelled:
		return e.RunID, true
	case engine.RunAborted:
		return e.RunID, true
	case engine.RunFinished:
		return e.RunID, true
	}
	return "", false
}
