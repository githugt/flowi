// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"github.com/githugt/flowi-lib/serve/replay"
	"github.com/githugt/flowi-lib/serve/scheduler"
)

// ErrInvalidServeOptions is returned by Serve for options that cannot work
// (as opposed to a system failure such as a port in use).
var ErrInvalidServeOptions = errors.New("invalid serve options")

// ErrShutdownTimeout is returned by Server.Wait when requests or runs were
// still going after ServeOptions.ShutdownTimeout and had to be cancelled.
var ErrShutdownTimeout = errors.New("requests or runs were still going at the shutdown timeout and were cancelled")

// ServeOptions configure Serve. Start from ServeDefaults, which fills them
// from settings, and change what the caller was asked to.
type ServeOptions struct {
	ReplayAddr  string // host:port; "" uses serve.replay_addr; port 0 picks a free one
	TriggerAddr string // host:port; "" uses serve.trigger_addr

	Latency  time.Duration // added to every replayed answer
	Sequence bool          // replay a request's responses one call at a time
	CORS     bool          // let browsers call the replayer
	Record   string        // upstream base URL: forward calls and save the real responses
	// RecordTo is the collection (name or id) that calls matching no saved
	// request are saved in, as new requests; "" is the Scratch draft. Only
	// used with Record.
	RecordTo string

	NoReplay, NoTriggers, NoSchedules bool

	// ValkeyAddr (host:port) shares run records and schedule ticks with
	// other instances through a Valkey or Redis server; "" uses
	// serve.valkey_addr, and empty there means no sharing. The password is
	// ValkeyPassword. The server may be down: serve then runs on its own.
	ValkeyAddr string
	ValkeyTLS  bool

	// ShutdownTimeout is how long in-flight requests and runs may take to
	// finish once the context ends; what is left is then cancelled and
	// Wait returns ErrShutdownTimeout. 0 uses serve.shutdown_timeout.
	ShutdownTimeout time.Duration
	MaxRuns         int // flow runs in flight at once (serve.max_runs)
	TrafficLog      int // replayer calls kept in memory (serve.traffic_log)

	Logger *slog.Logger

	// Hooks for tests.
	Now         func() time.Time
	Sleep       func(ctx context.Context, d time.Duration) error // the scheduler's wait
	replaySleep func(ctx context.Context, d time.Duration) error // the replayer's latency wait
}

// ServeDefaults returns the options settings call for.
func (w *Workspace) ServeDefaults() ServeOptions {
	r := w.Settings(Scope{})
	str := func(key string) string { v, _ := r.Get(key); s, _ := v.Value.(string); return s }
	dur := func(key string) time.Duration { v, _ := r.Get(key); d, _ := v.Value.(time.Duration); return d }
	num := func(key string) int { v, _ := r.Get(key); n, _ := v.Value.(int64); return int(n) }
	boolean := func(key string) bool { v, _ := r.Get(key); b, _ := v.Value.(bool); return b }
	return ServeOptions{
		ReplayAddr: str("serve.replay_addr"), TriggerAddr: str("serve.trigger_addr"), Latency: dur("serve.latency"),
		ShutdownTimeout: dur("serve.shutdown_timeout"), MaxRuns: num("serve.max_runs"), TrafficLog: num("serve.traffic_log"),
		ValkeyAddr: str("serve.valkey_addr"), ValkeyTLS: boolean("serve.valkey_tls"),
	}
}

// resolve fills blanks from settings and rejects what cannot work.
func (w *Workspace) resolveServe(o ServeOptions) (ServeOptions, error) {
	def := w.ServeDefaults()
	if o.ReplayAddr == "" {
		o.ReplayAddr = def.ReplayAddr
	}
	if o.TriggerAddr == "" {
		o.TriggerAddr = def.TriggerAddr
	}
	if o.ShutdownTimeout == 0 {
		o.ShutdownTimeout = def.ShutdownTimeout
	}
	if o.MaxRuns == 0 {
		o.MaxRuns = def.MaxRuns
	}
	if o.TrafficLog == 0 {
		o.TrafficLog = def.TrafficLog
	}
	if o.ValkeyAddr == "" {
		o.ValkeyAddr, o.ValkeyTLS = def.ValkeyAddr, def.ValkeyTLS
	}
	switch {
	case o.NoReplay && o.NoTriggers && o.NoSchedules:
		return o, fmt.Errorf("%w: nothing to serve, the replayer, triggers and schedules are all turned off", ErrInvalidServeOptions)
	case o.CORS && o.Record != "":
		return o, fmt.Errorf("%w: --cors with --record would let any web page read the real API through your network position", ErrInvalidServeOptions)
	case o.Latency < 0:
		return o, fmt.Errorf("%w: the latency cannot be negative", ErrInvalidServeOptions)
	case o.ShutdownTimeout < 0:
		return o, fmt.Errorf("%w: the shutdown timeout cannot be negative", ErrInvalidServeOptions)
	case o.ValkeyAddr != "" && !validHostPort(o.ValkeyAddr):
		return o, fmt.Errorf("%w: the Valkey address needs the form host:port, got %q", ErrInvalidServeOptions, o.ValkeyAddr)
	case o.MaxRuns < 0 || o.TrafficLog < 0:
		return o, fmt.Errorf("%w: run and traffic-log limits cannot be negative", ErrInvalidServeOptions)
	}
	return o, nil
}

// Server is a running `flowi serve`.
type Server struct {
	replayAddr, triggerAddr string
	traffic                 *replay.Log
	notes                   []string
	sched                   *scheduler.Scheduler
	done                    chan struct{}
	err                     error // the shutdown result, set before done closes
}

// ReplayAddr is where the replayer listens ("" when it is off).
func (s *Server) ReplayAddr() string { return s.replayAddr }

// TriggerAddr is where HTTP triggers listen ("" when they are off).
func (s *Server) TriggerAddr() string { return s.triggerAddr }

// Traffic is the replayer's log of calls.
func (s *Server) Traffic() *replay.Log { return s.traffic }

// Notes say what is off or unusual: triggers without a token, a listener
// open to the network, skipped schedules.
func (s *Server) Notes() []string { return s.notes }

// Upcoming lists when each schedule fires next.
func (s *Server) Upcoming() []scheduler.Upcoming {
	if s.sched == nil {
		return nil
	}
	return s.sched.Upcoming()
}

// Wait blocks until the context given to Serve has ended and the server
// has shut down: listeners closed, in-flight requests answered, runs
// finished or cancelled. It returns nil for a clean drain,
// ErrShutdownTimeout if work had to be cancelled, or the failure that
// stopped the server.
func (s *Server) Wait() error {
	<-s.done
	return s.err
}

// Serve starts the replayer, the HTTP triggers and the schedules, as
// ServeOptions say. Listeners are opened before it returns, so a port in
// use is an error here. It runs until ctx ends; call Wait to shut down
// gracefully:
//
//  1. stop accepting calls and starting scheduled runs;
//  2. let in-flight requests and flow runs finish, for up to
//     ShutdownTimeout;
//  3. cancel what is left (runs stop within the run.grace setting) and
//     close the connections.
//
// Goroutine ownership: Serve starts one goroutine per listener, one for
// the scheduler, one for live reload and one supervisor; they all end
// before Wait returns.
func (w *Workspace) Serve(ctx context.Context, o ServeOptions) (*Server, error) {
	o, err := w.resolveServe(o)
	if err != nil {
		return nil, err
	}
	b := &serveBuild{w: w, o: o, s: &Server{traffic: replay.NewLog(max(o.TrafficLog, 1)), done: make(chan struct{})}}
	if err := b.assemble(); err != nil {
		b.closeListeners()
		return nil, err
	}
	b.start(ctx)
	return b.s, nil
}

// exposure says when an address reaches beyond this machine.
func exposure(name, addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	ip := net.ParseIP(host)
	switch {
	case host == "" || (ip != nil && ip.IsUnspecified()):
		return fmt.Sprintf("%s listens on all network interfaces (%s): anyone who can reach this machine can call it", name, addr)
	case host == "localhost" || (ip != nil && ip.IsLoopback()):
		return ""
	}
	return fmt.Sprintf("%s listens on %s, which other machines may be able to reach", name, addr)
}

func parseUpstream(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: the record upstream needs an http:// or https:// URL, got %q", ErrInvalidServeOptions, raw)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: put credentials in the calls' headers, not in the upstream URL", ErrInvalidServeOptions)
	}
	return u, nil
}

func validHostPort(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	return err == nil && host != "" && port != ""
}
