// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/githugt/flowi-lib/serve/replay"
	"github.com/githugt/flowi-lib/serve/runs"
	"github.com/githugt/flowi-lib/serve/scheduler"
	"github.com/githugt/flowi-lib/serve/shared"
	"github.com/githugt/flowi-lib/serve/trigger"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
)

// serveBuild assembles the parts of one Serve call.
type serveBuild struct {
	w   *Workspace
	o   ServeOptions
	s   *Server
	log *slog.Logger

	replayLn, triggerLn net.Listener
	replayH             *replay.Handler
	triggerH            http.Handler
	mgr                 *runs.Manager
	servers             []namedServer
	store               shared.Store // nil when instances do not share state
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
		Collections: b.w.Collections, DataDir: b.w.DataDir, Latency: b.o.Latency, Sequence: b.o.Sequence, CORS: b.o.CORS,
		Log: b.s.traffic, Logger: b.log, Sleep: b.o.replaySleep, Hosts: loopbackHosts(b.s.replayAddr),
	}
	if b.o.Record != "" {
		upstream, err := parseUpstream(b.o.Record)
		if err != nil {
			return err
		}
		if upstream.Host == b.s.replayAddr {
			return fmt.Errorf("%w: the record upstream is the replayer itself; give the real API's URL", ErrInvalidServeOptions)
		}
		rec := &replay.Recorder{Upstream: upstream}
		target, err := b.w.recordCollection(b.o.RecordTo)
		if err != nil {
			return err
		}
		rec.Capture = b.w.captureTo(target)
		ro.Record = rec
	}
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
	if _, ok, err := b.w.TriggerToken(); err != nil {
		return fmt.Errorf("reading the trigger token: %w", err)
	} else if !ok {
		b.s.notes = append(b.s.notes, "triggers are off: no trigger token is set (create one with: flowi serve token)")
		return nil
	}
	ln, err := listen(b.o.TriggerAddr)
	if err != nil {
		return fmt.Errorf("triggers: %w", addrError(err))
	}
	b.triggerLn, b.s.triggerAddr = ln, ln.Addr().String()
	b.s.notes = appendNonEmpty(b.s.notes, exposure("the trigger endpoint", b.s.triggerAddr))
	return nil
}

func (b *serveBuild) assembleTriggerHandler() error {
	if b.triggerLn == nil {
		return nil
	}
	token, _, err := b.w.TriggerToken()
	if err != nil {
		return fmt.Errorf("reading the trigger token: %w", err)
	}
	b.triggerH, err = trigger.New(trigger.Options{Token: token, Runs: b.mgr, Logger: b.log})
	return err
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
	so := scheduler.Options{Jobs: jobs, Runs: b.mgr, Now: b.o.Now, Sleep: b.o.Sleep, Logger: b.log}
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
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
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
	for _, ln := range []net.Listener{b.replayLn, b.triggerLn} {
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
	return errors.Join(errs...)
}

// recordTarget names where recorded calls go, for the note.
func recordTarget(name string) string {
	if name == "" {
		return "the Scratch draft"
	}
	return "collection " + name
}
