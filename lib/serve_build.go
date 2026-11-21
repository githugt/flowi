// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/githugt/flowi/lib/serve/health"
	"github.com/githugt/flowi/lib/serve/replay"
	"github.com/githugt/flowi/lib/serve/runs"
	"github.com/githugt/flowi/lib/serve/scheduler"
	"github.com/githugt/flowi/lib/serve/shared"
	"github.com/githugt/flowi/lib/serve/trigger"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
)

// tape is the session's HAR tape, shared by the replayed and the forwarded calls so one file holds them in order;
// nil when no HAR file was asked for.
func (b *serveBuild) tape() *replay.Tape {
	if b.o.RecordHAR == "" {
		return nil
	}
	if b.harTape == nil {
		b.harTape = &replay.Tape{Path: b.o.RecordHAR}
	}
	return b.harTape
}

// serveBuild assembles the parts of one Serve call.
type serveBuild struct {
	w   *Workspace
	o   ServeOptions
	s   *Server
	log *slog.Logger

	replayLn, triggerLn, healthLn net.Listener
	healthSrv                     *http.Server // shut down last, so a probe sees the drain
	draining                      atomic.Bool
	replayH                       *replay.Handler
	triggerH                      http.Handler
	mgr                           *runs.Manager
	servers                       []namedServer
	store                         shared.Store // nil when instances do not share state
	harTape                       *replay.Tape // the one tape of the session, made on first use
}

type namedServer struct {
	name string
	srv  *http.Server
	ln   net.Listener
}

// assemble opens the listeners and builds the handlers. It starts nothing.
func (b *serveBuild) assemble() error {
	b.log = b.o.Logger
	if b.log == nil {
		b.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if err := b.assembleReplay(); err != nil {
		return err
	}
	if err := b.assembleTriggers(); err != nil {
		return err
	}
	if err := b.assembleShared(); err != nil {
		return err
	}
	if err := b.assembleHealthListener(); err != nil {
		return err
	}
	if b.replayLn != nil || b.triggerLn != nil || !b.o.NoSchedules {
		opts := runs.Options{
			Start: b.w.startRun, Max: b.o.MaxRuns, NewID: func() string { return string(b.w.newID()) },
			Now: b.o.Now, Logger: b.log,
		}
		if b.store != nil {
			opts.Shared = shared.Records{Store: b.store}
		}
		b.mgr = runs.NewManager(opts)
	}
	if err := b.assembleTriggerHandler(); err != nil {
		return err
	}
	b.assembleSchedules()
	b.assembleHealth()
	return nil
}

func listen(addr string) (net.Listener, error) {
	return (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
}

// addrError marks an address that cannot be parsed as the caller's mistake.
// A port in use or a missing permission stays a system error.
func addrError(err error) error {
	var addrErr *net.AddrError
	if errors.As(err, &addrErr) {
		return fmt.Errorf("%w: %w", ErrInvalidServeOptions, err)
	}
	return err
}

func (b *serveBuild) assembleReplay() error {
	if b.o.NoReplay {
		return nil
	}
	ln, err := listen(b.o.ReplayAddr)
	if err != nil {
		return fmt.Errorf("replayer: %w", addrError(err))
	}
	b.replayLn, b.s.replayAddr = ln, ln.Addr().String()
	ro := replay.Options{
		Tape: b.tape(), Collections: b.w.Collections, DataDir: b.w.DataDir, Latency: b.o.Latency, Faults: b.o.Faults, Sequence: b.o.Sequence, CORS: b.o.CORS, Capture: b.o.CaptureCalls,
		Log: b.s.traffic, Logger: b.log, Sleep: b.o.replaySleep, Hosts: loopbackHosts(b.s.replayAddr),
	}
	if b.o.Record != "" {
		upstream, err := parseUpstream(b.o.Record)
		if err != nil {
			return err
		}
		if replay.SameHost(upstream.Host, b.s.replayAddr) {
			return fmt.Errorf("%w: the record upstream is the replayer itself; give the real API's URL", ErrInvalidServeOptions)
		}
		rec := &replay.Recorder{Upstream: upstream}
		if b.o.RecordHAR != "" {
			rec.Tape = &replay.Tape{Path: b.o.RecordHAR}
		}
		target, err := b.w.recordCollection(b.o.RecordTo)
		if err != nil {
			return err
		}
		folder, err := recordFolder(b.o.RecordFolder)
		if err != nil {
			return err
		}
		rec.Capture = b.w.captureTo(target, folder)
		ro.Record = rec
	}
	fwd, err := b.forward()
	if err != nil {
		return err
	}
	ro.Forward = fwd
	b.replayH = replay.New(ro)
	b.s.notes = appendNonEmpty(b.s.notes, exposure("the replayer", b.s.replayAddr))
	if b.o.Record != "" {
		b.s.notes = append(b.s.notes, "record mode: calls go to "+b.o.Record+"; each call becomes a request in "+recordTarget(b.o.RecordTo)+" (when none matches it yet) and its response is saved in the workspace data folder")
	}
	return nil
}

// assembleShared connects to the Valkey server, if one is configured. The
// connection is lazy and fails open, so a server that is down only costs a
// note and the sharing.
func (b *serveBuild) assembleShared() error {
	if b.o.ValkeyAddr == "" {
		return nil
	}
	password, _, err := b.w.ValkeyPassword()
	if err != nil {
		return fmt.Errorf("reading the Valkey password: %w", err)
	}
	v, err := shared.NewValkey(shared.ValkeyConfig{
		Addr: b.o.ValkeyAddr, Password: password, TLS: b.o.ValkeyTLS,
		Prefix: "flowi:" + string(b.w.Model().ID) + ":", Logger: b.log,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidServeOptions, err)
	}
	b.store = v
	b.s.notes = append(b.s.notes, "sharing run records and schedule ticks through Valkey at "+b.o.ValkeyAddr+" (if it is unreachable, this instance works alone)")
	return nil
}

func (b *serveBuild) assembleTriggers() error {
	if b.o.NoTriggers {
		return nil
	}
	if has, err := b.w.anyTriggerToken(); err != nil {
		return fmt.Errorf("reading the trigger tokens: %w", err)
	} else if !has {
		b.s.notes = append(b.s.notes, "triggers are off: no trigger token is set (create one with: flowi serve tokens new <name>)")
		return nil
	}
	cfg, err := triggerTLS(b.o)
	if err != nil {
		return err
	}
	if note := exposure("the trigger endpoint", b.o.TriggerAddr); note != "" && cfg == nil && !b.o.InsecureListen {
		return fmt.Errorf("%w: %s, and without TLS its bearer token would cross the network in clear text; "+
			"give --tls-cert and --tls-key, listen on 127.0.0.1, or pass --insecure-listen when a proxy in front terminates TLS", ErrInvalidServeOptions, note)
	}
	ln, err := listen(b.o.TriggerAddr)
	if err != nil {
		return fmt.Errorf("triggers: %w", addrError(err))
	}
	b.s.triggerAddr = ln.Addr().String()
	if cfg != nil {
		ln, b.s.triggerTLS = tls.NewListener(ln, cfg), true
	}
	b.triggerLn = ln
	note := exposure("the trigger endpoint", b.s.triggerAddr)
	if cfg == nil && note != "" { // only with --insecure-listen: say that the operator took the risk
		note += " (plain HTTP: --insecure-listen says TLS ends in front of it)"
	}
	b.s.notes = appendNonEmpty(b.s.notes, note)
	return nil
}

// triggerTLS loads the certificate the trigger endpoint serves HTTPS with, nil when none was given.
func triggerTLS(o ServeOptions) (*tls.Config, error) {
	if o.TLSCert == "" && o.TLSKey == "" {
		return nil, nil
	}
	if o.TLSCert == "" || o.TLSKey == "" {
		return nil, fmt.Errorf("%w: --tls-cert and --tls-key go together", ErrInvalidServeOptions)
	}
	pair, err := tls.LoadX509KeyPair(o.TLSCert, o.TLSKey)
	if err != nil {
		return nil, fmt.Errorf("%w: reading the TLS certificate and key: %w", ErrInvalidServeOptions, err)
	}
	return &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}, nil
}

func (b *serveBuild) assembleTriggerHandler() error {
	if b.triggerLn == nil {
		return nil
	}
	token, _, err := b.w.TriggerToken()
	if err != nil {
		return fmt.Errorf("reading the trigger token: %w", err)
	}
	// The named tokens are read from their file on each call, so one made, revoked or rotated while this runs takes
	// effect at once; the single token is read once, so replacing it needs a restart.
	b.triggerH, err = trigger.New(trigger.Options{Token: token, Tokens: b.w.triggerTokens(), Runs: b.mgr, Logger: b.log})
	return err
}

// assembleHealthListener opens the health address, when one was asked for.
func (b *serveBuild) assembleHealthListener() error {
	if b.o.HealthAddr == "" {
		return nil
	}
	ln, err := listen(b.o.HealthAddr)
	if err != nil {
		return fmt.Errorf("health: %w", addrError(err))
	}
	b.healthLn, b.s.healthAddr = ln, ln.Addr().String()
	b.s.notes = appendNonEmpty(b.s.notes, exposure("the health endpoint", b.s.healthAddr))
	return nil
}

// assembleHealth builds the health handler over what the other parts know. It is served until the very end of the
// shutdown, so /readyz can say "draining" while runs finish.
func (b *serveBuild) assembleHealth() {
	if b.healthLn == nil {
		return
	}
	src := health.Source{
		Draining:   b.draining.Load,
		RunsMax:    b.o.MaxRuns,
		Schedules:  func() int { return len(b.s.Upcoming()) },
		Replaying:  b.replayLn != nil,
		Triggering: b.triggerLn != nil,
	}
	if b.mgr != nil {
		src.RunsInFlight = b.mgr.InFlight
	}
	if b.replayLn != nil {
		src.ReplayCalls = b.s.traffic.Total
	}
	b.healthSrv = newHTTPServer(health.New(health.Options{Source: src, Metrics: b.o.Metrics}))
}

func (b *serveBuild) assembleSchedules() {
	if b.o.NoSchedules {
		return
	}
	var jobs []scheduler.Job
	for _, sc := range b.w.Schedules() {
		job, err := scheduler.NewJob(sc)
		if err != nil {
			b.s.notes = append(b.s.notes, fmt.Sprintf("schedule %q skipped: %v", sc.Name, err))
			continue
		}
		if _, err := b.w.ResolveSchedule(sc); err != nil && !sc.Disabled {
			b.s.notes = append(b.s.notes, fmt.Sprintf("schedule %q: %s; its runs fail until the flow can be found", sc.Name, flowProblem(err)))
		}
		jobs = append(jobs, job)
	}
	so := scheduler.Options{Jobs: jobs, Runs: b.mgr, Await: b.mgr.Await, Now: b.o.Now, Sleep: b.o.Sleep, Logger: b.log}
	if b.store != nil {
		so.Claim = b.store.Claim
	}
	b.s.sched = scheduler.New(so)
}

// loopbackHosts are the Host header names a loopback listener accepts, or
// nil for a listener open to the network, where the operator has chosen to
// be reachable by other names.
func loopbackHosts(addr string) []string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}
	// "localhost" is a name, not an address, and is what a listener asked for by name carries.
	if ip := net.ParseIP(host); !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return nil
	}
	return []string{"localhost", "127.0.0.1", "::1"}
}

func appendNonEmpty(list []string, s string) []string {
	if s == "" {
		return list
	}
	return append(list, s)
}

func (b *serveBuild) closeStore() {
	if b.store != nil {
		_ = b.store.Close() // the connection is only a cache; nothing is lost
	}
}

func (b *serveBuild) closeListeners() {
	b.closeStore()
	for _, ln := range []net.Listener{b.replayLn, b.triggerLn, b.healthLn} {
		if ln != nil {
			_ = ln.Close() // setup failed: the original error is the one to report
		}
	}
}

func newHTTPServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: readHeaderTimeout, IdleTimeout: idleTimeout}
}

// start runs everything assembled. Called once; ctx ends the server.
func (b *serveBuild) start(ctx context.Context) {
	runCtx, stop := context.WithCancelCause(ctx)
	schedCtx, stopSched := context.WithCancel(context.Background())
	watchCtx, stopWatch := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	if b.replayLn != nil {
		b.servers = append(b.servers, namedServer{"replayer", newHTTPServer(b.replayH), b.replayLn})
	}
	if b.triggerLn != nil {
		b.servers = append(b.servers, namedServer{"triggers", newHTTPServer(b.triggerH), b.triggerLn})
	}
	for _, ns := range b.servers {
		wg.Go(func() {
			if err := ns.srv.Serve(ns.ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				stop(fmt.Errorf("%s: %w", ns.name, err))
			}
		})
	}
	if b.healthSrv != nil {
		wg.Go(func() {
			if err := b.healthSrv.Serve(b.healthLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
				stop(fmt.Errorf("health: %w", err))
			}
		})
	}
	if b.s.sched != nil {
		wg.Go(func() { b.s.sched.Run(schedCtx) })
	}
	b.startReload(watchCtx, &wg)

	go func() {
		<-runCtx.Done()
		stopSched()
		stopWatch()
		err := b.shutdown()
		wg.Wait()
		b.closeStore()
		if cause := context.Cause(runCtx); !errors.Is(cause, context.Canceled) && !errors.Is(cause, context.DeadlineExceeded) {
			err = errors.Join(cause, err)
		}
		stop(nil) // releases the context; nothing is listening to it any more
		b.s.err = err
		close(b.s.done)
	}()
}

// startReload keeps the replayer in step with files changed on disk.
func (b *serveBuild) startReload(ctx context.Context, wg *sync.WaitGroup) {
	if b.replayH == nil {
		return
	}
	changes, err := b.w.Watch(ctx)
	if err != nil {
		b.s.notes = append(b.s.notes, "live reload is off: "+err.Error())
		return
	}
	wg.Go(func() {
		for ch := range changes {
			if ch.Err != nil {
				continue // a failing watcher ends the channel
			}
			if _, err := b.w.Reload(ch); err != nil {
				b.log.Warn("reloading a changed file failed", "Event", "ServeReloadFailed", "File", ch.File, "Error", err)
				continue
			}
			b.replayH.Refresh()
		}
	})
}

// shutdown stops accepting, drains, then cancels what is left, all within
// ShutdownTimeout.
func (b *serveBuild) shutdown() error {
	b.draining.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), b.o.ShutdownTimeout)
	defer cancel()
	var mu sync.Mutex
	var errs []error
	fail := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}
	var wg sync.WaitGroup
	for _, ns := range b.servers {
		wg.Go(func() {
			if err := ns.srv.Shutdown(ctx); err != nil {
				_ = ns.srv.Close() // forced: the deadline passed with requests still open
				fail(fmt.Errorf("%w: %s: %w", ErrShutdownTimeout, ns.name, err))
			}
		})
	}
	if b.mgr != nil {
		wg.Go(func() {
			if err := b.mgr.Shutdown(ctx); err != nil {
				fail(fmt.Errorf("%w: %w", ErrShutdownTimeout, err))
			}
		})
	}
	wg.Wait()
	if b.healthSrv != nil {
		// Last, so a probe during the drain gets "not ready" rather than a refused connection.
		if err := b.healthSrv.Shutdown(ctx); err != nil {
			_ = b.healthSrv.Close()
		}
	}
	return errors.Join(errs...)
}

// recordTarget names where recorded calls go, for the note.
func recordTarget(name string) string {
	if name == "" {
		return "the Scratch draft"
	}
	return "collection " + name
}
