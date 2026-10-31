// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/serve/replay"
)

// MaxReplayerEntries is how many calls the traffic view keeps.
const MaxReplayerEntries = 1000

// ReplayerState is where the replayer is in its life.
type ReplayerState int

// States.
const (
	ReplayerStopped ReplayerState = iota
	ReplayerStarting
	ReplayerRunning
	ReplayerStopping
)

// ReplayerForm is what the user chooses before starting.
type ReplayerForm struct {
	Addr     string // host:port; blank uses the serve.replay_addr setting
	Latency  string // a duration such as 250ms; blank uses the serve.latency setting
	Sequence bool
	CORS     bool
	Record   string // an upstream base URL: forward calls to it and save its responses; blank replays
	// RecordTo is the collection (name or id) that calls matching no saved
	// request are saved in as new requests; blank is the Scratch draft.
	RecordTo string
	// RecordHAR is a file the recorded calls are also written to, in order (needs Record).
	RecordHAR string
	// Faults make a share of the replayed calls fail on purpose. Percent fields are 0 to 100; blank is none.
	FaultErrorPercent string
	FaultStatus       string // the status of an injected error; blank is 503
	FaultDropPercent  string
	FaultJitter       string // up to this much extra delay, as a duration such as 300ms
}

// Replayer starts and stops the replayer of flowi serve inside the app and
// collects its traffic. Only the replayer runs here: triggers and
// schedules belong to `flowi serve`.
//
// State changes happen on the UI goroutine. Goroutine ownership: Start
// launches two goroutines, one forwarding the server's traffic to the UI
// and one waiting for the server to finish; both end when the server has
// stopped. Close waits for them.
type Replayer struct {
	b  Backend
	w  *Worker
	wg sync.WaitGroup

	state    ReplayerState
	form     ReplayerForm
	addr     string
	err      string
	notes    []string
	entries  []replay.Entry
	cancel   context.CancelFunc
	onChange []func()
}

// NewReplayer returns a stopped replayer.
func NewReplayer(b Backend, w *Worker) *Replayer { return &Replayer{b: b, w: w} }

// OnChange registers f to run (on the UI goroutine) after anything changes.
func (r *Replayer) OnChange(f func()) { r.onChange = append(r.onChange, f) }

func (r *Replayer) changed() {
	for _, f := range r.onChange {
		f()
	}
}

// State is the replayer's state.
func (r *Replayer) State() ReplayerState { return r.state }

// Addr is where the running replayer listens.
func (r *Replayer) Addr() string { return r.addr }

// Err is the last problem, or "".
func (r *Replayer) Err() string { return r.err }

// Notes say what is unusual about the running replayer.
func (r *Replayer) Notes() []string { return r.notes }

// Form is the form's current values.
func (r *Replayer) Form() ReplayerForm { return r.form }

// Entries are the calls so far, oldest first.
func (r *Replayer) Entries() []replay.Entry { return r.entries }

// Clear forgets the calls shown.
func (r *Replayer) Clear() {
	r.entries = nil
	r.changed()
}

// LoadDefaults fills the form from settings (the address to listen on).
func (r *Replayer) LoadDefaults() {
	Run(r.w, r.b.ServeDefaults, func(o flowi.ServeOptions) {
		if r.state != ReplayerStopped {
			return
		}
		r.form.Addr = o.ReplayAddr
		if o.Latency > 0 {
			r.form.Latency = o.Latency.String()
		}
		r.changed()
	})
}

// Start starts the replayer with the form's values. Starting twice, or
// while stopping, does nothing.
func (r *Replayer) Start(form ReplayerForm) {
	if r.state != ReplayerStopped {
		return
	}
	r.form, r.err = form, ""
	latency, err := parseLatency(form.Latency)
	if err != nil {
		r.err = err.Error()
		r.changed()
		return
	}
	form.Record = strings.TrimSpace(form.Record)
	if err := checkUpstream(form); err != nil {
		r.err = err.Error()
		r.changed()
		return
	}
	faults, err := parseFaults(form)
	if err != nil {
		r.err = err.Error()
		r.changed()
		return
	}
	r.state = ReplayerStarting
	r.changed()
	ctx, cancel := context.WithCancel(context.Background())
	type started struct {
		srv Serving
		err error
	}
	Run(r.w, func() started {
		o := r.b.ServeDefaults()
		if a := strings.TrimSpace(form.Addr); a != "" {
			o.ReplayAddr = a
		}
		if latency > 0 || strings.TrimSpace(form.Latency) != "" {
			o.Latency = latency
		}
		o.Sequence, o.CORS, o.Record, o.RecordTo, o.NoTriggers, o.NoSchedules = form.Sequence, form.CORS, form.Record, form.RecordTo, true, true
		o.RecordHAR, o.Faults = strings.TrimSpace(form.RecordHAR), faults
		srv, err := r.b.Serve(ctx, o)
		return started{srv, err}
	}, func(s started) {
		if s.err != nil {
			cancel()
			r.state, r.err = ReplayerStopped, ErrorText(s.err)
			r.changed()
			return
		}
		r.running(ctx, cancel, s.srv)
	})
}

// checkUpstream refuses a record upstream that is not an http(s) address, and
// CORS together with recording: any web page could then read the real API
// through the user's network position (the library refuses it too; saying so
// before starting gives a plainer message).
func checkUpstream(form ReplayerForm) error {
	if form.Record == "" && strings.TrimSpace(form.RecordHAR) != "" {
		return errors.New("a HAR file needs an address to record from: there is nothing to put in it")
	}
	if form.Record == "" {
		return nil
	}
	u, err := url.Parse(form.Record)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("the address to record from must start with http:// or https://, not %q", form.Record)
	}
	if form.CORS {
		return errors.New("recording cannot be combined with Allow browsers (CORS): any web page could read the real API through your network")
	}
	return nil
}

func parseLatency(text string) (time.Duration, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(text)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("latency must be a duration such as 250ms or 2s, not %q", text)
	}
	return d, nil
}

func (r *Replayer) running(ctx context.Context, cancel context.CancelFunc, srv Serving) {
	r.state, r.cancel, r.addr, r.notes = ReplayerRunning, cancel, srv.ReplayAddr(), srv.Notes()
	r.changed()
	traffic := srv.Traffic().Subscribe(ctx)
	r.wg.Go(func() {
		for e := range traffic {
			r.w.Post(func() { r.add(e) })
		}
	})
	r.wg.Go(func() {
		err := srv.Wait()
		r.w.Post(func() { r.stopped(err) })
	})
}

func (r *Replayer) add(e replay.Entry) {
	r.entries = append(r.entries, e)
	if over := len(r.entries) - MaxReplayerEntries; over > 0 {
		r.entries = r.entries[over:]
	}
	r.changed()
}

func (r *Replayer) stopped(err error) {
	r.state, r.cancel, r.addr, r.notes = ReplayerStopped, nil, "", nil
	// A forced stop is not a failure of the replayer: its work was cut off
	// at the shutdown timeout, which is what that setting is for.
	if err != nil && !errors.Is(err, flowi.ErrShutdownTimeout) {
		r.err = ErrorText(err)
	}
	r.changed()
}

// Stop asks the running replayer to stop; calls in progress finish first.
func (r *Replayer) Stop() {
	if r.state != ReplayerRunning || r.cancel == nil {
		return
	}
	r.state = ReplayerStopping
	r.cancel()
	r.changed()
}

// Close stops the replayer and waits for it; the app calls it on shutdown,
// off the UI goroutine.
func (r *Replayer) Close() {
	if c := r.cancel; c != nil {
		c()
	}
	r.wg.Wait()
}

// parseFaults reads the fault fields. A blank field is none; a field that is not a number in range is an error that
// names it, so the person sees which box to fix.
func parseFaults(form ReplayerForm) (replay.Faults, error) {
	var f replay.Faults
	var err error
	if f.ErrorRate, err = percent("error rate", form.FaultErrorPercent); err != nil {
		return f, err
	}
	if f.DropRate, err = percent("drop rate", form.FaultDropPercent); err != nil {
		return f, err
	}
	if s := strings.TrimSpace(form.FaultStatus); s != "" {
		if f.ErrorStatus, err = strconv.Atoi(s); err != nil {
			return f, fmt.Errorf("the error status must be a number such as 503, not %q", s)
		}
	}
	if s := strings.TrimSpace(form.FaultJitter); s != "" {
		if f.Jitter, err = time.ParseDuration(s); err != nil || f.Jitter < 0 {
			return f, fmt.Errorf("the jitter must be a duration such as 300ms, not %q", s)
		}
	}
	return f, f.Validate()
}

// percent reads "20" as 0.2; blank is zero.
func percent(what, text string) (float64, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil || v < 0 || v > 100 {
		return 0, fmt.Errorf("the %s must be a percent from 0 to 100, not %q", what, text)
	}
	return v / 100, nil
}
