// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"errors"
	"sync"
	"time"

	flowi "github.com/githugt/flowi/lib"
)

// ScheduleRunnerState is where the schedule runner is in its life.
type ScheduleRunnerState int

// States.
const (
	ScheduleStopped ScheduleRunnerState = iota
	ScheduleStarting
	ScheduleRunning
	ScheduleStopping
)

// UpcomingRun is when a schedule fires next.
type UpcomingRun struct {
	Name string
	At   time.Time
}

// ScheduleRunner runs the schedules (and the HTTP triggers, when a token
// exists) of `flowi serve` inside the app, so a schedule fires while the
// window is open. The replayer is not part of it: it has its own screen.
// Schedules fire only while it runs, and `flowi serve` does the same without a
// window.
//
// State changes happen on the UI goroutine. Goroutine ownership: Start
// launches one goroutine that waits for the server to finish; Close waits for
// it.
type ScheduleRunner struct {
	b  Backend
	w  *Worker
	wg sync.WaitGroup

	state    ScheduleRunnerState
	err      string
	notes    []string
	upcoming []UpcomingRun
	trigger  string
	cancel   context.CancelFunc
	onChange []func()
}

// NewScheduleRunner returns a stopped runner.
func NewScheduleRunner(b Backend, w *Worker) *ScheduleRunner { return &ScheduleRunner{b: b, w: w} }

// OnChange registers f to run (on the UI goroutine) after anything changes.
func (r *ScheduleRunner) OnChange(f func()) { r.onChange = append(r.onChange, f) }

func (r *ScheduleRunner) changed() {
	for _, f := range r.onChange {
		f()
	}
}

// State is the runner's state.
func (r *ScheduleRunner) State() ScheduleRunnerState { return r.state }

// Err is the last problem, or "".
func (r *ScheduleRunner) Err() string { return r.err }

// Notes say what is off or unusual: triggers without a token, skipped schedules.
func (r *ScheduleRunner) Notes() []string { return r.notes }

// Upcoming is when each schedule fires next, while the runner runs.
func (r *ScheduleRunner) Upcoming() []UpcomingRun { return r.upcoming }

// TriggerAddr is where HTTP triggers listen while the runner runs; "" otherwise.
func (r *ScheduleRunner) TriggerAddr() string { return r.trigger }

// Start starts the schedules. Starting twice, or while stopping, does nothing.
func (r *ScheduleRunner) Start() {
	if r.state != ScheduleStopped {
		return
	}
	r.err = ""
	r.state = ScheduleStarting
	r.changed()
	ctx, cancel := context.WithCancel(context.Background())
	type started struct {
		srv Serving
		err error
	}
	Run(r.w, func() started {
		o := r.b.ServeDefaults()
		o.NoReplay = true
		srv, err := r.b.Serve(ctx, o)
		return started{srv, err}
	}, func(s started) {
		if s.err != nil {
			cancel()
			r.state, r.err = ScheduleStopped, ErrorText(s.err)
			r.changed()
			return
		}
		r.running(cancel, s.srv)
	})
}

func (r *ScheduleRunner) running(cancel context.CancelFunc, srv Serving) {
	r.state, r.cancel, r.notes, r.trigger = ScheduleRunning, cancel, srv.Notes(), srv.TriggerAddr()
	for _, u := range srv.Upcoming() {
		r.upcoming = append(r.upcoming, UpcomingRun{Name: u.Name, At: u.At})
	}
	r.changed()
	r.wg.Go(func() {
		err := srv.Wait()
		r.w.Post(func() { r.stopped(err) })
	})
}

func (r *ScheduleRunner) stopped(err error) {
	r.state, r.cancel, r.notes, r.upcoming, r.trigger = ScheduleStopped, nil, nil, nil, ""
	// A forced stop is the shutdown timeout doing its job, not a failure.
	if err != nil && !errors.Is(err, flowi.ErrShutdownTimeout) {
		r.err = ErrorText(err)
	}
	r.changed()
}

// Stop asks the runner to stop; runs in progress finish first.
func (r *ScheduleRunner) Stop() {
	if r.state != ScheduleRunning || r.cancel == nil {
		return
	}
	r.state = ScheduleStopping
	r.cancel()
	r.changed()
}

// Close stops the runner and waits for it; the app calls it on shutdown, off
// the UI goroutine.
func (r *ScheduleRunner) Close() {
	if c := r.cancel; c != nil {
		c()
	}
	r.wg.Wait()
}
