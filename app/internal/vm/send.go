// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

// Hook modes, as `flowi send --no-hooks / --skip`.
const (
	HooksAll      = "all"
	HooksNone     = "none"
	HooksSkipPre  = "skip-pre"
	HooksSkipPost = "skip-post"
)

// SendOptions are the send controls of the request screen (design §12
// parity with flowi send). The environment comes from the workspace's
// selection and the version from the request screen.
type SendOptions struct {
	PreferVersion string
	With          []model.KeyValue
	Hooks         string // HooksAll (default), HooksNone, HooksSkipPre, HooksSkipPost
	Seed          string // empty: random
	Save          string // empty: the responses.save setting; none | last | per-status
}

// Exchange is one finished send.
type Exchange struct {
	ID      int
	At      time.Time
	Title   string // "auth/login v1 · 200 · passed"
	Target  flowi.RequestTarget
	Version string
	Result  engine.SendResult
}

// Send sends requests and keeps the latest exchange.
type Send struct {
	b        Backend
	w        *Worker
	ws       *Workspace
	log      *RunLog
	history  *History
	running  bool
	target   flowi.RequestTarget // what is sending, while running
	cancel   context.CancelFunc
	last     *Exchange
	shown    map[flowi.RequestTarget]*Exchange // the response each request shows: its latest send, or the one saved on disk
	trace    *Trace                            // the events of the latest send
	traceFor flowi.RequestTarget               // the request trace belongs to
	err      string
	errFor   flowi.RequestTarget // the request err belongs to
	onChange []func()
	onResult []func(flowi.RequestTarget, diag.List)
	onPrompt []func(engine.AuthPrompted)
}

// NewSend returns the send view-model. Sends use ws's selected
// environment, log their events to log and add results to history.
func NewSend(b Backend, w *Worker, ws *Workspace, log *RunLog, history *History) *Send {
	return &Send{b: b, w: w, ws: ws, log: log, history: history, shown: map[flowi.RequestTarget]*Exchange{}}
}

// OnChange registers f to run after a send starts or ends.
func (s *Send) OnChange(f func()) { s.onChange = append(s.onChange, f) }

// OnResult registers f to run with the diagnostics of every finished send
// (empty when it passed): the Problems panel lists them.
func (s *Send) OnResult(f func(flowi.RequestTarget, diag.List)) { s.onResult = append(s.onResult, f) }

// OnPrompt registers f to run when a request waits for the person to sign in
// (an OAuth 2.0 grant that needs a browser, or a device code). It runs on the
// UI goroutine, from a send or a flow run.
func (s *Send) OnPrompt(f func(engine.AuthPrompted)) { s.onPrompt = append(s.onPrompt, f) }

// Prompted tells the listeners a request waits for the person.
func (s *Send) Prompted(p engine.AuthPrompted) {
	for _, f := range s.onPrompt {
		f(p)
	}
}

func (s *Send) changed() {
	for _, f := range s.onChange {
		f()
	}
}

// Running reports a send in progress (of any request).
func (s *Send) Running() bool { return s.running }

// RunningFor reports a send in progress of request t: the other tabs
// show their own state, not this one's.
func (s *Send) RunningFor(t flowi.RequestTarget) bool { return s.running && s.target == t }

// Err is the last send problem, or "".
func (s *Send) Err() string { return s.err }

// ErrFor is the last send problem when it belongs to request t.
func (s *Send) ErrFor(t flowi.RequestTarget) string {
	if s.errFor != t {
		return ""
	}
	return s.err
}

// Last is the latest finished exchange.
func (s *Send) Last() (Exchange, bool) {
	if s.last == nil {
		return Exchange{}, false
	}
	return *s.last, true
}

// LastFor is the response request t shows: its latest send in this session,
// or, until it is sent, the one saved on disk (see LoadSaved).
func (s *Send) LastFor(t flowi.RequestTarget) (Exchange, bool) {
	if ex, ok := s.shown[t]; ok {
		return *ex, true
	}
	return Exchange{}, false
}

// LoadSaved shows the response the last send of req's selected version saved,
// so a restarted app does not claim nothing was sent. A response sent this
// session is not replaced. It is not added to the history or the run log.
func (s *Send) LoadSaved(req *Request) {
	t, version := req.Target(), engine.ResponseKey(req.SelectedLabel(), req.OverlayName()) // read here: the worker must not touch req
	Run(s.w, func() savedLookup {
		rec, ok := s.b.LastResponse(t, version)
		return savedLookup{rec, ok}
	}, func(l savedLookup) {
		if _, sent := s.shown[t]; sent || !l.ok {
			return
		}
		ex := Exchange{At: l.rec.At, Target: t, Version: l.rec.Version, Title: fmt.Sprintf("%s %s · %d · saved", l.rec.RequestPath, versionTitle(l.rec.Version, l.rec.Overlay), l.rec.Status),
			Result: engine.SendResult{Status: engine.StatusPassed, Record: &l.rec}}
		s.shown[t] = &ex
		s.changed()
	})
}

type savedLookup struct {
	rec engine.Record
	ok  bool
}

// Send stores req's edit (or saves it, when the changes do not fit the
// edit slot), then sends its selected version.
func (s *Send) Send(req *Request, o SendOptions) {
	opts, err := s.options(req, o)
	s.errFor = req.Target()
	if err != nil {
		s.err = err.Error()
		s.changed()
		return
	}
	s.err = ""
	trace := newTraceFor(req)
	overlay := req.OverlayName()
	req.PrepareSend(func(useEdit bool) {
		opts.UseEdit = useEdit
		// A stored edit already shows the overlay (the working copy is the effective request), so it is
		// only named, for the response's folder; otherwise the saved version is sent with the overlay applied.
		if useEdit {
			opts.OverlayLabel = overlay
		} else {
			opts.Overlay = overlay
		}
		s.start(req.Target(), opts, trace)
	})
}

func (s *Send) options(req *Request, o SendOptions) (flowi.SendOptions, error) {
	opts := flowi.SendOptions{Env: s.ws.SelectedEnv(), Version: req.SelectedLabel(), PreferVersion: o.PreferVersion,
		Save: o.Save, Hooks: HookControlOf(o.Hooks), With: map[string]string{}}
	for _, kv := range o.With {
		if kv.Key != "" {
			opts.With[kv.Key] = kv.Value
		}
	}
	if o.Seed != "" {
		seed, err := strconv.ParseUint(o.Seed, 10, 64)
		if err != nil {
			return flowi.SendOptions{}, fmt.Errorf("seed %q is not a whole number", o.Seed)
		}
		opts.Seed = seed
	}
	return opts, nil
}

// HookControlOf turns a hooks mode (HooksAll, HooksNone, …) into a node's
// hook control.
func HookControlOf(mode string) model.HookControl {
	switch mode {
	case HooksNone:
		return model.HookControl{Mode: "none"}
	case HooksSkipPre:
		return model.HookControl{Mode: "all", Skip: []string{"pre"}}
	case HooksSkipPost:
		return model.HookControl{Mode: "all", Skip: []string{"post"}}
	}
	return model.HookControl{Mode: "all"}
}

// start runs the send on the worker: events go to the run log as they
// arrive, the result to history.
func (s *Send) start(t flowi.RequestTarget, opts flowi.SendOptions, trace *Trace) {
	ctx, cancel := context.WithCancel(context.Background())
	s.running, s.target, s.cancel = true, t, cancel
	s.trace, s.traceFor = trace, t
	s.changed()
	s.log.Begin(fmt.Sprintf("send %s", opts.Version))
	Run(s.w, func() sendOutcome { return s.run(ctx, t, opts) }, func(o sendOutcome) {
		cancel()
		s.finish(t, opts, o)
	})
}

type sendOutcome struct {
	result engine.SendResult
	err    error
}

// run blocks until the send ends; it runs off the UI goroutine.
func (s *Send) run(ctx context.Context, t flowi.RequestTarget, opts flowi.SendOptions) sendOutcome {
	sending, err := s.b.Send(ctx, t, opts)
	if err != nil {
		return sendOutcome{err: err}
	}
	trace := s.trace // this send's, even if another starts before the events are drained
	for e := range sending.Events() {
		if line := EventLine(e); line != "" {
			s.w.Post(func() { s.log.Add(line) })
		}
		if p, ok := e.(engine.AuthPrompted); ok {
			s.w.Post(func() { s.Prompted(p) })
		}
		s.w.Post(func() { trace.Apply(e); s.changed() })
	}
	return sendOutcome{result: sending.Wait()}
}

// TraceFor is what the events of the latest send of t said about each step of
// its pipeline, or nil if t was not the latest request sent.
func (s *Send) TraceFor(t flowi.RequestTarget) *Trace {
	if s.trace == nil || s.traceFor != t {
		return nil
	}
	return s.trace
}

func (s *Send) finish(t flowi.RequestTarget, opts flowi.SendOptions, o sendOutcome) {
	s.running, s.cancel = false, nil
	s.trace.Finish()
	if o.err != nil {
		s.err = ErrorText(o.err)
		s.log.Add("✗ " + s.err)
		s.changed()
		return
	}
	ex := s.history.Add(t, opts.Version, o.result)
	s.last = &ex
	s.shown[t] = &ex
	if len(o.result.Diags) > 0 {
		s.err = diagsText(o.result.Diags)
	}
	for _, f := range s.onResult {
		f(t, o.result.Diags)
	}
	s.changed()
}

// Cancel stops the running send.
func (s *Send) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Pin saves rec (a response shown in the viewer) as a pinned example
// named name (empty: the status code); done gets the file written.
func (s *Send) Pin(rec engine.Record, name string, done func(file string, err error)) {
	type pinned struct {
		file string
		err  error
	}
	Run(s.w, func() pinned {
		f, err := s.b.PinExample(rec, name)
		return pinned{f, err}
	}, func(p pinned) { done(p.file, p.err) })
}

// EventLine renders an engine event for the run log, like the CLI's plain
// output; "" means the event gets no line. Events are already masked by
// the engine.
func EventLine(e engine.Event) string {
	switch ev := e.(type) {
	case engine.RequestSent:
		return fmt.Sprintf("%s→ %s %s", indent(ev.Depth), ev.Method, ev.URL)
	case engine.ResponseReceived:
		if ev.Error != "" {
			return fmt.Sprintf("%s← ✗ %s (%s)", indent(ev.Depth), ev.Error, ev.Duration.Round(time.Millisecond))
		}
		if ev.Outcome != "" {
			return fmt.Sprintf("%s← %s (%s)", indent(ev.Depth), ev.Outcome, ev.Duration.Round(time.Millisecond))
		}
		return fmt.Sprintf("%s← %d (%s)", indent(ev.Depth), ev.Status, ev.Duration.Round(time.Millisecond))
	case engine.MessageReceived:
		return fmt.Sprintf("%s← message %d", indent(ev.Depth), ev.Seq)
	case engine.HookStarted:
		return fmt.Sprintf("%s%s hook %s · %s …", indent(ev.Depth), ev.Phase, ev.Level, ev.Step)
	case engine.HookFinished:
		return fmt.Sprintf("%s%s hook %s · %s %s", indent(ev.Depth), ev.Phase, ev.Level, ev.Step, ev.Status)
	case engine.NodeStarted:
		return "" // the finish line says enough
	case engine.NodeFinished:
		return fmt.Sprintf("%s· %s (%s) %s", indent(ev.Depth), ev.Node, ev.Type, ev.Status)
	case engine.Log:
		return "log: " + ev.Message
	case engine.Warning:
		return "⚠ " + ErrorText(diag.List{ev.Diag}.Err())
	case engine.AuthPrompted:
		return "waiting for you: " + promptLine(ev)
	case engine.RunCancelled:
		return "cancelled"
	case engine.RunFinished:
		return fmt.Sprintf("%s (%s)", ev.Status, ev.Duration.Round(time.Millisecond))
	}
	return e.EventName()
}

func indent(depth int) string {
	out := ""
	for range depth {
		out += "  "
	}
	return out
}

// RunLog is the run log panel: lines of every send this session.
type RunLog struct {
	lines    []string
	at       []time.Time // when each line was added; zero for a line restored from an earlier session
	onChange []func()
}

// maxLogLines bounds the run log; older lines are dropped.
const maxLogLines = 5000

// NewRunLog returns an empty run log.
func NewRunLog() *RunLog { return &RunLog{} }

// OnChange registers f to run after lines are added.
func (l *RunLog) OnChange(f func()) { l.onChange = append(l.onChange, f) }

// Begin starts a new section.
func (l *RunLog) Begin(title string) { l.Add("── " + time.Now().Format("15:04:05") + " " + title) }

// Add appends a line.
func (l *RunLog) Add(line string) {
	l.lines = append(l.lines, line)
	l.at = append(l.at, time.Now())
	if over := len(l.lines) - maxLogLines; over > 0 {
		l.lines, l.at = l.lines[over:], l.at[over:]
	}
	for _, f := range l.onChange {
		f()
	}
}

// Clear empties the log.
func (l *RunLog) Clear() {
	l.lines, l.at = nil, nil
	for _, f := range l.onChange {
		f()
	}
}

// Lines are the log's lines, oldest first.
func (l *RunLog) Lines() []string { return l.lines }

// LogSection is the lines of one send, under its "── time title" line.
type LogSection struct {
	Title   string // "" for lines before the first send
	Lines   []string
	Entries []LogEntry // the same lines, split into columns
}

// LogEntry is a log line as the run log panel's columns show it.
type LogEntry struct {
	At      time.Time // zero when the line came from an earlier session
	Level   string    // INFO, WARN or ERROR
	Step    string    // Request, Response, or "-"
	Message string
	Tone    Tone
}

// entryOf splits a line into its columns.
func entryOf(line string, at time.Time) LogEntry {
	e := LogEntry{At: at, Message: line, Tone: LineTone(line), Level: "INFO", Step: "-"}
	switch text := strings.TrimSpace(line); {
	case e.Tone == ToneServerError:
		e.Level = "ERROR"
	case e.Tone == ToneClientError && strings.HasPrefix(text, "⚠"):
		e.Level = "WARN"
	}
	switch text := strings.TrimSpace(line); {
	case strings.HasPrefix(text, "→"):
		e.Step = "Request"
	case strings.HasPrefix(text, "←"):
		e.Step = "Response"
	}
	return e
}

const sectionMark = "── "

// Sections groups the lines by send, oldest first.
func (l *RunLog) Sections() []LogSection {
	var out []LogSection
	for i, line := range l.lines {
		if strings.HasPrefix(line, sectionMark) {
			out = append(out, LogSection{Title: line})
			continue
		}
		if len(out) == 0 {
			out = append(out, LogSection{})
		}
		last := &out[len(out)-1]
		last.Lines = append(last.Lines, line)
		last.Entries = append(last.Entries, entryOf(line, l.at[i]))
	}
	return out
}

// LineTone is the colour class of a log line: requests pending, responses
// by status, failures red, cancellations gray.
func LineTone(line string) Tone {
	text := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(text, "→"):
		return TonePending
	case strings.HasPrefix(text, "← ✗"), strings.HasPrefix(text, "✗"):
		return ToneServerError
	case strings.HasPrefix(text, "←"):
		var status int
		if _, err := fmt.Sscanf(strings.TrimPrefix(text, "←"), "%d", &status); err == nil {
			return ToneOfStatus(status)
		}
	case strings.HasPrefix(text, "⚠"):
		return ToneClientError
	case text == "cancelled":
		return ToneCancelled
	case strings.HasPrefix(text, "passed"):
		return ToneSuccess
	case strings.HasPrefix(text, "failed"), strings.HasPrefix(text, "error"):
		return ToneServerError
	}
	return ToneNone
}

// History is this session's exchanges, newest first.
type History struct {
	items    []Exchange
	nextID   int
	selected int
	onChange []func()
	onAdd    []func()
}

// NewHistory returns an empty history.
func NewHistory() *History { return &History{} }

// OnChange registers f to run after an exchange is added or opened.
func (h *History) OnChange(f func()) { h.onChange = append(h.onChange, f) }

// OnAdd registers f to run after an exchange is added (not when one is
// only opened).
func (h *History) OnAdd(f func()) { h.onAdd = append(h.onAdd, f) }

func (h *History) changed() {
	for _, f := range h.onChange {
		f()
	}
}

// Add records an exchange and selects it.
func (h *History) Add(t flowi.RequestTarget, version string, r engine.SendResult) Exchange {
	h.nextID++
	title := fmt.Sprintf("v%s · %s", version, r.Status)
	if r.Record != nil {
		title = fmt.Sprintf("%s %s · %d · %s", historyName(r.Record), versionTitle(version, r.Record.Overlay), r.Record.Status, r.Status)
	}
	ex := Exchange{ID: h.nextID, At: time.Now(), Title: title, Target: t, Version: version, Result: r}
	h.items = append([]Exchange{ex}, h.items...)
	h.selected = ex.ID
	h.changed()
	for _, f := range h.onAdd {
		f()
	}
	return ex
}

// Clear forgets every exchange of the session.
func (h *History) Clear() {
	h.items, h.selected = nil, 0
	h.changed()
}

// Remove forgets one exchange; one that is not there changes nothing.
func (h *History) Remove(id int) {
	before := len(h.items)
	h.items = slices.DeleteFunc(h.items, func(ex Exchange) bool { return ex.ID == id })
	if len(h.items) == before {
		return
	}
	if h.selected == id {
		h.selected = 0
	}
	h.changed()
}

// Items are the exchanges, newest first.
func (h *History) Items() []Exchange { return h.items }

// Open selects an exchange to show.
func (h *History) Open(id int) {
	h.selected = id
	h.changed()
}

// Selected is the exchange to show.
func (h *History) Selected() Exchange {
	for _, ex := range h.items {
		if ex.ID == h.selected {
			return ex
		}
	}
	return Exchange{}
}

// historyName names a request in the history: its path, or for an untitled
// draft (whose name says nothing) what was sent.
func historyName(rec *engine.Record) string {
	if !strings.HasPrefix(rec.RequestPath, "untitled") || rec.Request.URL == "" {
		return rec.RequestPath
	}
	url := strings.TrimPrefix(strings.TrimPrefix(rec.Request.URL, "https://"), "http://")
	return strings.TrimSpace(rec.Request.Method + " " + url)
}

// versionTitle is "v2", or "v2 + vip" for a send with the flowi overlay vip.
func versionTitle(version, overlay string) string {
	if overlay == "" {
		return "v" + version
	}
	return "v" + version + " + " + overlay
}

// promptLine says what a waiting request asks of the person.
func promptLine(p engine.AuthPrompted) string {
	if p.Kind == "device" {
		return "open " + p.URL + " and enter the code " + p.Code
	}
	return "open " + p.URL + " to sign in"
}
