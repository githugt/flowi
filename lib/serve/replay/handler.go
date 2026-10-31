// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/githugt/flowi-lib/model"
)

// Options configure a Handler.
type Options struct {
	// Collections returns the collections to answer for; it is called by
	// New and Refresh.
	Collections func() []model.Collection
	// DataDir returns a collection's data directory, where saved responses
	// live. Nil serves pinned examples only.
	DataDir func(model.ID) string

	Latency time.Duration // delay for every answer unless X-Flowi-Latency says otherwise
	// Faults make a share of the calls fail on purpose; the zero value injects nothing. Not applied in record mode.
	Faults Faults
	// Rand returns a number in [0, 1) for each roll (default: math/rand/v2). Tests inject a fixed one.
	Rand     func() float64
	Sequence bool // walk a request's responses one call at a time
	CORS     bool // answer browsers: Access-Control-* headers and preflights

	// Hosts, when set, are the host names (without port) a call may use in
	// its Host header; others are refused with 403. A listener on loopback
	// sets it so a web page cannot reach the replayer through a name that
	// DNS rebinding points at 127.0.0.1.
	Hosts []string

	Log    *Log         // receives every call; nil keeps none
	Logger *slog.Logger // bad response files are reported at debug level; nil discards

	// Sleep waits for d or until ctx ends (default: a timer). Tests inject a
	// recorder so they do not wait.
	Sleep func(ctx context.Context, d time.Duration) error

	// Record, when set, forwards matched calls upstream and saves the
	// answers (see record.go).
	Record *Recorder
}

// Handler is the replayer. It is safe for concurrent use.
type Handler struct {
	opts Options
	rec  *Recorder // opts.Record with defaults filled in; nil when not recording
	idx  atomic.Pointer[index]

	seqMu sync.Mutex // guards seq
	seq   map[string]int

	captureMu sync.Mutex // one call at a time is made into a request
}

// New builds a replayer over the collections now returned by
// opts.Collections.
func New(opts Options) *Handler {
	if opts.Sleep == nil {
		opts.Sleep = sleepContext
	}
	if opts.Rand == nil {
		opts.Rand = rand.Float64
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	h := &Handler{opts: opts, seq: map[string]int{}}
	if opts.Record != nil {
		rec := opts.Record.withDefaults()
		h.rec = &rec
	}
	h.Refresh()
	return h
}

// Refresh rebuilds the route table; call it after requests change.
func (h *Handler) Refresh() {
	ix := buildIndex(h.opts.Collections())
	h.idx.Store(ix)
	for _, path := range ix.skipped {
		h.opts.Logger.Warn("request not replayed: its id or version label is not a plain name", "Event", "ReplayRequestSkipped", "Request", path)
	}
}

// Reset restarts every sequence from its first response.
func (h *Handler) Reset() {
	h.seqMu.Lock()
	defer h.seqMu.Unlock()
	clear(h.seq)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// hostAllowed checks a Host header against Options.Hosts.
func (h *Handler) hostAllowed(host string) bool {
	if len(h.opts.Hosts) == 0 {
		return true
	}
	name := strings.ToLower(host)
	if hostOnly, _, err := net.SplitHostPort(name); err == nil {
		name = hostOnly
	}
	name = strings.Trim(name, "[]")
	return name != "" && slices.Contains(h.opts.Hosts, name)
}

const controlPrefix = "/__flowi/"

// crossSite reports whether a browser made the call on behalf of a page of
// another origin. The Host check stops DNS rebinding but not a page that
// calls 127.0.0.1 by its address: such a page cannot read the answer (the
// control routes send no CORS headers), yet a "simple" POST to /__flowi/reset
// would still run. Browsers always label these calls; curl and Flowi send
// neither header and pass.
func crossSite(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err != nil || !strings.EqualFold(u.Host, r.Host)
}

// ServeHTTP answers one call.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.hostAllowed(r.Host) {
		a := problem(r, http.StatusForbidden, MatchBad, "this host name is not allowed; use localhost or the address the replayer listens on")
		h.deliver(w, r, a)
		return
	}
	if strings.HasPrefix(r.URL.Path, controlPrefix) {
		if crossSite(r) {
			a := problem(r, http.StatusForbidden, MatchBad, "the control endpoints are not open to calls made by a web page from another origin")
			h.deliver(w, r, a)
			return
		}
		h.control(w, r)
		return
	}
	if h.opts.CORS && h.preflight(w, r) {
		return
	}
	if h.rec != nil {
		h.serveRecording(w, r)
		return
	}
	start := time.Now()
	a, dropped := h.applyFaults(r, h.decide(r))
	if a.wait > 0 {
		if err := h.opts.Sleep(r.Context(), a.wait); err != nil {
			return // the caller went away
		}
	}
	if dropped {
		h.record(r, a, time.Since(start))
		// ErrAbortHandler is how a handler closes the connection without an answer: net/http ends the
		// connection (or resets an HTTP/2 stream) and logs nothing.
		panic(http.ErrAbortHandler)
	}
	h.deliver(w, r, a)
	h.record(r, a, time.Since(start))
}

// applyFaults adds the jitter and turns the answer into an injected error, or marks it to be dropped,
// when the call's roll or its X-Flowi-Fault header says so. A call that did not match anything keeps its
// own answer: a fault is for calls the replayer would have answered.
func (h *Handler) applyFaults(r *http.Request, a answer) (answer, bool) {
	chosen, set, err := readFault(r.Header)
	if err != nil {
		return problem(r, http.StatusBadRequest, MatchBad, err.Error()), false
	}
	if !set && !h.opts.Faults.Active() || a.route == nil {
		return a, false
	}
	if !set {
		chosen = h.opts.Faults.roll(h.opts.Rand())
	}
	if j := h.opts.Faults.Jitter; j > 0 {
		a.wait += time.Duration(h.opts.Rand() * float64(j))
	}
	switch chosen.kind {
	case faultError:
		wait := a.wait
		a = errorAnswer(r, chosen.status)
		a.wait = wait
	case faultDrop:
		a.status, a.match = 0, MatchFault
		return a, true
	case faultNone:
	}
	return a, false
}

// answer is what the replayer decided to say, before any I/O.
type answer struct {
	status int
	match  string
	wait   time.Duration

	route *route    // the matched request version
	resp  *response // the chosen response
	// problem is the JSON body of an error answer made by the replayer.
	problem map[string]any
	allow   []string
}

// decide turns a call into an answer: which request, which response, how
// long to wait. It does no writing.
func (h *Handler) decide(r *http.Request) answer {
	p, err := readPicks(r.Header)
	if err != nil {
		return problem(r, http.StatusBadRequest, MatchBad, err.Error())
	}
	look := h.idx.Load().find(r.Method, splitPath(r.URL.Path), r.URL.Query())
	found := p.narrow(look.candidates)
	if len(found) == 0 {
		return h.notFound(r, look.allow)
	}
	rt := found[0].route
	rs := responsesFor(rt, h.opts.DataDir, func(file string, err error) {
		h.opts.Logger.Debug("response file skipped", "Event", "ReplayFileSkipped", "File", file, "Error", err)
	})
	c := chooser{sequence: h.opts.Sequence, next: func() int { return h.nextInSequence(rt) }}
	resp, ok := c.pick(rs, p)
	wait := h.opts.Latency
	if p.hasLatency {
		wait = p.latency
	}
	if !ok {
		a := h.noResponse(r, rt, rs, p)
		a.wait = wait
		return a
	}
	return answer{status: resp.status, match: MatchOK, wait: wait, route: &rt, resp: &resp}
}

func (h *Handler) nextInSequence(rt route) int {
	key := string(rt.req.ID) + "/" + rt.ver.Label
	h.seqMu.Lock()
	defer h.seqMu.Unlock()
	n := h.seq[key]
	h.seq[key] = n + 1
	return n
}

// problem is an error the replayer makes itself, as a problem details document (RFC 9457). "flowi", "error", "method"
// and "path" are the replayer's own members from before the RFC members were added, and stay.
func problem(r *http.Request, status int, match, msg string) answer {
	return answer{status: status, match: match, problem: map[string]any{
		"type": "about:blank", "title": http.StatusText(status), "status": status, "detail": msg, "instance": r.URL.Path,
		"flowi": true, "error": msg, "method": r.Method, "path": r.URL.Path,
	}}
}

func (h *Handler) notFound(r *http.Request, allow []string) answer {
	if len(allow) > 0 {
		a := problem(r, http.StatusMethodNotAllowed, MatchNone, "the path exists, but not for "+r.Method+"; allowed: "+strings.Join(allow, ", "))
		a.allow = allow
		return a
	}
	return problem(r, http.StatusNotFound, MatchNone, "no saved request matches "+r.Method+" "+r.URL.Path)
}

func (h *Handler) noResponse(r *http.Request, rt route, rs []response, p picks) answer {
	var msg string
	switch {
	case len(rs) == 0:
		msg = "request " + rt.req.Path + " has no response to give: send it once to save one, or pin an example"
	case p.example != "":
		msg = "request " + rt.req.Path + " has no response named " + strconv.Quote(p.example)
	default:
		msg = "request " + rt.req.Path + " has no response with status " + strconv.Itoa(p.status)
	}
	a := problem(r, http.StatusNotFound, MatchNoResponse, msg)
	a.route = &rt
	names := make([]string, len(rs))
	for i, x := range rs {
		names[i] = x.name + " (" + strconv.Itoa(x.status) + ")"
	}
	a.problem["available"] = names
	return a
}

// exposed lists the headers a browser may read from a replayed answer.
const exposed = "X-Flowi-Request, X-Flowi-Version, X-Flowi-Example, X-Flowi-Source, X-Flowi-Match"

func (h *Handler) preflight(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", exposed)
	if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
		return false
	}
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(append(model.StandardMethods(), model.MoreMethods()...), ", "))
	if want := r.Header.Get("Access-Control-Request-Headers"); want != "" {
		w.Header().Set("Access-Control-Allow-Headers", want)
	}
	w.Header().Set("Access-Control-Max-Age", "600")
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (h *Handler) record(r *http.Request, a answer, took time.Duration) {
	if h.opts.Log == nil {
		return
	}
	e := Entry{At: time.Now(), Method: r.Method, Path: r.URL.Path, Status: a.status, Match: a.match,
		Latency: a.wait, Duration: took, Remote: r.RemoteAddr}
	if a.route != nil {
		e.Request, e.Version = a.route.req.Path, a.route.ver.Label
	}
	if a.resp != nil {
		e.Example, e.Source = a.resp.name, a.resp.source
	}
	h.opts.Log.Add(e)
}
