// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/githugt/flowi/lib/serve/runs"
)

// RunStarter is what the scheduler needs from the run manager.
type RunStarter interface {
	Start(runs.Request) (runs.Record, error)
}

// Options configure a Scheduler.
type Options struct {
	Jobs []Job
	Runs RunStarter
	Now  func() time.Time
	// Sleep waits for d or until ctx ends (default: a timer). Tests inject
	// a clock they move by hand.
	Sleep  func(ctx context.Context, d time.Duration) error
	Logger *slog.Logger
	// Claim, when set, is asked before each tick whether this instance may
	// run it, so that several serve instances sharing a store fire each
	// tick once. An error counts as "yes": a missed nightly job is worse
	// than a duplicate.
	Claim func(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Await waits for a started run to end and returns its record; with it, schedules that name on_failure or
	// on_success tell that address when the run ends. Without it they send nothing.
	Await func(ctx context.Context, id string) (runs.Record, bool)
	// Post sends an alert's JSON to an address (default: an HTTP POST with a ten-second limit, no redirects).
	Post func(ctx context.Context, addr string, body []byte) error
	// Getenv reads an environment variable for an env:NAME address (default: the process environment).
	Getenv func(string) string
}

const (
	// claimTTL outlasts any clock difference between instances.
	claimTTL     = 10 * time.Minute
	claimTimeout = 3 * time.Second
)

// Scheduler fires jobs at their cron times. Ticks it slept through (the
// machine was suspended) are not replayed: a job fires at most once per
// wake-up.
type Scheduler struct {
	opts     Options
	jobs     []Job // enabled jobs
	watchers sync.WaitGroup
}

// New returns a scheduler for the enabled jobs.
func New(o Options) *Scheduler {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Sleep == nil {
		o.Sleep = sleepContext
	}
	if o.Post == nil {
		o.Post = httpPost
	}
	if o.Getenv == nil {
		o.Getenv = defaultGetenv
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	jobs := slices.DeleteFunc(slices.Clone(o.Jobs), func(j Job) bool { return j.Disabled })
	return &Scheduler{opts: o, jobs: jobs}
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

// maxSleep bounds one wait. Timers run on a monotonic clock that stops while
// the machine is suspended, so a long wait would end late by the time
// slept; waking every minute to compare the wall clock keeps a job within
// a minute of its time.
const maxSleep = time.Minute

// Upcoming is when a job fires next.
type Upcoming struct {
	Name string
	At   time.Time
}

// Upcoming lists each enabled job's next fire time from now, soonest first.
func (s *Scheduler) Upcoming() []Upcoming {
	now := s.opts.Now()
	out := make([]Upcoming, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, Upcoming{Name: j.Name, At: j.Next(now)})
	}
	slices.SortStableFunc(out, func(a, b Upcoming) int { return a.At.Compare(b.At) })
	return out
}

// Run fires jobs until ctx ends. The caller owns the goroutine; Run
// returns when ctx is done.
func (s *Scheduler) Run(ctx context.Context) {
	defer s.watchers.Wait()
	next := make([]time.Time, len(s.jobs))
	now := s.opts.Now()
	for i, j := range s.jobs {
		next[i] = j.Next(now)
	}
	for {
		wait, ok := s.untilNext(next, s.opts.Now())
		if !ok {
			<-ctx.Done() // nothing to schedule
			return
		}
		if err := s.opts.Sleep(ctx, wait); err != nil {
			return
		}
		now = s.opts.Now()
		for i, j := range s.jobs {
			if next[i].IsZero() || next[i].After(now) {
				continue // a job that never fires again is not due, whatever the time
			}
			s.fire(ctx, j, next[i])
			next[i] = j.Next(now)
		}
	}
}

// untilNext is the time to the soonest fire; ok is false with no jobs, or
// when no job ever fires again (Feb 30).
func (s *Scheduler) untilNext(next []time.Time, now time.Time) (time.Duration, bool) {
	var soonest time.Time
	for _, t := range next {
		if !t.IsZero() && (soonest.IsZero() || t.Before(soonest)) {
			soonest = t
		}
	}
	if soonest.IsZero() {
		return 0, false
	}
	return min(max(soonest.Sub(now), 0), maxSleep), true
}

// claimed reports whether this instance may run the tick scheduled at at.
func (s *Scheduler) claimed(j Job, at time.Time) bool {
	if s.opts.Claim == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), claimTimeout)
	defer cancel()
	key := "tick:" + j.Name + ":" + strconv.FormatInt(at.Unix(), 10)
	ok, err := s.opts.Claim(ctx, key, claimTTL)
	if err != nil {
		s.opts.Logger.Warn("claiming a tick failed; running it anyway", "Event", "ScheduleClaimFailed", "Schedule", j.Name, "Error", err)
		return true
	}
	if !ok {
		s.opts.Logger.Info("tick taken by another instance", "Event", "ScheduleClaimedElsewhere", "Schedule", j.Name)
	}
	return ok
}

func (s *Scheduler) fire(ctx context.Context, j Job, at time.Time) {
	if !s.claimed(j, at) {
		return
	}
	inputs := make(map[string]string, len(j.Inputs))
	for _, kv := range j.Inputs {
		inputs[kv.Key] = kv.Value
	}
	source := "schedule:" + j.Name
	rec, err := s.start(runs.Request{Flow: j.Flow, Collection: j.Collection, Env: j.Env, Inputs: inputs, Source: source, Key: source})
	log := s.opts.Logger.With("Schedule", j.Name, "Flow", j.Flow)
	switch {
	case err == nil:
		log.Info("scheduled run started", "Event", "ScheduleFired", "RunID", rec.ID)
		if (j.OnFailure != "" || j.OnSuccess != "") && s.opts.Await != nil {
			s.background(func() { s.alertWhenDone(ctx, j, rec.ID) })
		}
	case errors.Is(err, runs.ErrOverlap), errors.Is(err, runs.ErrBusy), errors.Is(err, runs.ErrClosed):
		log.Warn("scheduled run skipped", "Event", "ScheduleSkipped", "Reason", err.Error())
	default:
		log.Error("scheduled run could not start", "Event", "ScheduleFailed", "Error", err)
		var se *runs.StartError
		msg := err.Error()
		if errors.As(err, &se) {
			msg = se.Message
		}
		a := Alert{Schedule: j.Name, Flow: j.Flow, Env: j.Env, Record: runs.Record{Status: "did not start", Outcome: runs.Outcome{Messages: []string{msg}}}}
		s.background(func() { s.send(ctx, j, j.OnFailure, a) })
	}
}

// start keeps one misbehaving starter from ending the loop.
func (s *Scheduler) start(r runs.Request) (rec runs.Record, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = &runs.StartError{Kind: runs.KindInternal, Message: "starting the run panicked"}
		}
	}()
	return s.opts.Runs.Start(r)
}

// alertWhenDone waits for the run and tells on_failure or on_success. A run that was cancelled (the server was
// stopping) or that was still going when the scheduler stopped tells nobody.
func (s *Scheduler) alertWhenDone(ctx context.Context, j Job, id string) {
	rec, ok := s.opts.Await(ctx, id)
	if !ok || rec.Status == runs.StatusRunning || rec.Status == statusCancelled {
		return
	}
	a := Alert{Schedule: j.Name, Flow: j.Flow, Env: j.Env, Record: rec}
	if rec.Status == statusPassed {
		s.send(ctx, j, j.OnSuccess, a)
		return
	}
	s.send(ctx, j, j.OnFailure, a)
}

// send posts one alert. A failure to deliver is logged with the host only (the path of a webhook is a secret) and
// never touches the run.
func (s *Scheduler) send(ctx context.Context, j Job, hook string, a Alert) {
	if hook == "" {
		return
	}
	log := s.opts.Logger.With("Schedule", j.Name)
	addr, err := resolveHook(hook, s.opts.Getenv)
	if err != nil {
		log.Warn("an alert could not be sent", "Event", "ScheduleAlertFailed", "Reason", err.Error())
		return
	}
	// The run's own context may have ended with the scheduler; an alert about a finished run still goes out.
	if err := s.opts.Post(context.WithoutCancel(ctx), addr, alertBody(a)); err != nil {
		log.Warn("an alert could not be sent", "Event", "ScheduleAlertFailed", "Host", hostOf(addr), "Reason", err.Error())
		return
	}
	log.Info("an alert was sent", "Event", "ScheduleAlerted", "Host", hostOf(addr), "Status", a.Record.Status)
}

// background runs an alert off the loop, so a slow receiver never holds up the next job; Run waits for them on exit.
func (s *Scheduler) background(f func()) {
	s.watchers.Add(1)
	go func() {
		defer s.watchers.Done()
		f()
	}()
}
