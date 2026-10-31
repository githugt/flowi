// SPDX-License-Identifier: Apache-2.0

package scheduler_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/serve/cron"
	"github.com/githugt/flowi-lib/serve/runs"
	"github.com/githugt/flowi-lib/serve/scheduler"
)

// fakeClock only moves when the test says so.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time     // wall clock
	mono    time.Duration // elapsed monotonic time: sleepers wake by this, as real timers do
	waiters []*waiter
}

type waiter struct {
	at time.Duration
	ch chan struct{}
}

func newClock(start string) *fakeClock {
	t, err := time.Parse("2006-01-02 15:04", start)
	if err != nil {
		panic(err)
	}
	return &fakeClock{now: t}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	c.mu.Lock()
	w := &waiter{at: c.mono + d, ch: make(chan struct{})}
	c.waiters = append(c.waiters, w)
	c.mu.Unlock()
	select {
	case <-w.ch:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// sleeping reports how many Sleep calls are waiting.
func (c *fakeClock) sleeping() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.waiters)
}

// advance moves time forward and wakes every sleeper whose time has come.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.mono += d
	kept := c.waiters[:0]
	for _, w := range c.waiters {
		if w.at <= c.mono {
			close(w.ch)
		} else {
			kept = append(kept, w)
		}
	}
	c.waiters = kept
}

// jump moves the wall clock without elapsing monotonic time: what a
// suspended laptop or a clock correction does.
func (c *fakeClock) jump(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type recorder struct {
	fires chan runs.Request
	err   error
	mu    sync.Mutex
}

func (r *recorder) Start(req runs.Request) (runs.Record, error) {
	r.mu.Lock()
	err := r.err
	r.mu.Unlock()
	r.fires <- req
	return runs.Record{ID: "run"}, err
}

func (r *recorder) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

type rig struct {
	t     *testing.T
	clock *fakeClock
	runs  *recorder
	logs  *bytes.Buffer
	stop  context.CancelFunc
	done  chan struct{}
}

func start(t *testing.T, startAt string, schedules ...model.Schedule) *rig {
	t.Helper()
	return startWith(t, startAt, nil, schedules...)
}

// startWith is start with the scheduler's options adjusted first.
func startWith(t *testing.T, startAt string, adjust func(*scheduler.Options), schedules ...model.Schedule) *rig {
	t.Helper()
	var jobs []scheduler.Job
	for _, s := range schedules {
		j, err := scheduler.NewJob(s)
		if err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, j)
	}
	r := &rig{t: t, clock: newClock(startAt), runs: &recorder{fires: make(chan runs.Request, 16)}, logs: &bytes.Buffer{}, done: make(chan struct{})}
	opts := scheduler.Options{
		Jobs: jobs, Runs: r.runs, Now: r.clock.Now, Sleep: r.clock.Sleep,
		Logger: slog.New(slog.NewTextHandler(&lockedWriter{w: r.logs}, nil)),
	}
	if adjust != nil {
		adjust(&opts)
	}
	s := scheduler.New(opts)
	ctx, cancel := context.WithCancel(t.Context())
	r.stop = cancel
	go func() { defer close(r.done); s.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-r.done })
	r.waitSleeping()
	return r
}

func (r *rig) waitSleeping() {
	r.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for r.clock.sleeping() == 0 {
		if time.Now().After(deadline) {
			r.t.Fatal("scheduler never went to sleep")
		}
		runtime.Gosched()
	}
}

// tick moves the clock and returns the runs that started, once the
// scheduler has gone back to sleep.
func (r *rig) tick(d time.Duration, want int) []runs.Request {
	r.t.Helper()
	r.clock.advance(d)
	var got []runs.Request
	for range want {
		select {
		case req := <-r.runs.fires:
			got = append(got, req)
		case <-time.After(5 * time.Second):
			r.t.Fatalf("got %d runs, want %d", len(got), want)
		}
	}
	r.waitSleeping()
	select {
	case extra := <-r.runs.fires:
		r.t.Fatalf("unexpected extra run %+v", extra)
	default:
	}
	return got
}

type lockedWriter struct {
	mu sync.Mutex
	w  *bytes.Buffer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func every(name, expr string) model.Schedule {
	// UTC keeps the tests the same on every machine: without a zone a
	// schedule follows the machine's, whose offset may not be whole hours.
	return model.Schedule{Name: name, Flow: "signup", Collection: "shop", Cron: expr, Env: "staging", TZ: "UTC",
		Inputs: []model.KeyValue{{Key: "email", Value: "a@example.com"}}}
}

func TestScheduler_should_start_a_run_at_each_tick_with_the_schedules_settings(t *testing.T) {
	r := start(t, "2026-10-09 12:00", every("often", "*/5 * * * *"))

	first := r.tick(5*time.Minute, 1)
	second := r.tick(5*time.Minute, 1)

	got := first[0]
	if got.Flow != "signup" || got.Collection != "shop" || got.Env != "staging" || got.Inputs["email"] != "a@example.com" ||
		got.Source != "schedule:often" || got.Key != "schedule:often" {
		t.Fatalf("request %+v", got)
	}
	if len(second) != 1 {
		t.Fatalf("second tick: %+v", second)
	}
}

func TestScheduler_should_not_replay_ticks_it_slept_through(t *testing.T) {
	r := start(t, "2026-10-09 12:00", every("often", "*/5 * * * *"))

	got := r.tick(47*time.Minute, 1) // eight ticks passed; one run is enough

	if len(got) != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestScheduler_should_run_each_schedule_on_its_own_cadence(t *testing.T) {
	r := start(t, "2026-10-09 12:00", every("fast", "*/10 * * * *"), every("slow", "0 * * * *"))

	atTen := r.tick(10*time.Minute, 1)
	atTop := r.tick(50*time.Minute, 2)

	if atTen[0].Key != "schedule:fast" {
		t.Fatalf("at :10 %+v", atTen)
	}
	keys := map[string]bool{atTop[0].Key: true, atTop[1].Key: true}
	if !keys["schedule:fast"] || !keys["schedule:slow"] {
		t.Fatalf("at :00 %+v", atTop)
	}
}

func TestScheduler_should_use_the_schedules_time_zone(t *testing.T) {
	if _, err := time.LoadLocation("Europe/Berlin"); err != nil {
		t.Skip("no tz database")
	}
	s := every("morning", "0 9 * * *")
	s.TZ = "Europe/Berlin" // 09:00 CEST is 07:00 UTC in October
	r := start(t, "2026-10-09 06:00", s)

	r.tick(30*time.Minute, 0)
	got := r.tick(30*time.Minute, 1)

	if len(got) != 1 || r.clock.Now().Hour() != 7 {
		t.Fatalf("fired at %s: %+v", r.clock.Now(), got)
	}
}

func TestScheduler_should_log_a_skipped_overlap_and_keep_going(t *testing.T) {
	r := start(t, "2026-10-09 12:00", every("often", "*/5 * * * *"))
	r.runs.fail(runs.ErrOverlap)

	r.tick(5*time.Minute, 1)
	r.runs.fail(nil)
	next := r.tick(5*time.Minute, 1)

	if len(next) != 1 {
		t.Fatalf("scheduler stopped after an overlap: %+v", next)
	}
	if !bytes.Contains(r.logs.Bytes(), []byte("Event=ScheduleSkipped")) || !bytes.Contains(r.logs.Bytes(), []byte("Schedule=often")) {
		t.Fatalf("logs:\n%s", r.logs)
	}
}

func TestScheduler_should_log_a_start_failure_and_keep_going(t *testing.T) {
	r := start(t, "2026-10-09 12:00", every("often", "*/5 * * * *"))
	r.runs.fail(&runs.StartError{Kind: runs.KindNotFound, Message: "no flow \"signup\""})

	r.tick(5*time.Minute, 1)
	r.runs.fail(nil)
	next := r.tick(5*time.Minute, 1)

	if len(next) != 1 || !bytes.Contains(r.logs.Bytes(), []byte("Event=ScheduleFailed")) {
		t.Fatalf("next %+v logs:\n%s", next, r.logs)
	}
}

func TestScheduler_should_not_schedule_disabled_jobs(t *testing.T) {
	off := every("off", "* * * * *")
	off.Disabled = true
	s := scheduler.New(scheduler.Options{Jobs: mustJobs(t, off, every("on", "0 * * * *")), Now: newClock("2026-10-09 12:00").Now})

	got := s.Upcoming()

	if len(got) != 1 || got[0].Name != "on" || got[0].At.Hour() != 13 {
		t.Fatalf("%+v", got)
	}
}

func mustJobs(t *testing.T, list ...model.Schedule) []scheduler.Job {
	t.Helper()
	var jobs []scheduler.Job
	for _, s := range list {
		j, err := scheduler.NewJob(s)
		if err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, j)
	}
	return jobs
}

func TestScheduler_Run_should_return_when_its_context_ends_even_with_no_jobs(t *testing.T) {
	for name, jobs := range map[string][]scheduler.Job{"none": nil, "one": mustJobs(t, every("a", "0 0 * * *"))} {
		t.Run(name, func(t *testing.T) {
			clock := newClock("2026-10-09 12:00")
			s := scheduler.New(scheduler.Options{Jobs: jobs, Now: clock.Now, Sleep: clock.Sleep})
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			go func() { defer close(done); s.Run(ctx) }()

			cancel()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("Run did not return")
			}
		})
	}
}

func TestNewJob_should_refuse_an_invalid_schedule(t *testing.T) {
	_, err := scheduler.NewJob(model.Schedule{Name: "x", Flow: "f", Cron: "nope"})

	if !errors.Is(err, scheduler.ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

// Before: a job that never fires had a zero next time, which counted as
// "due", so it ran at every wake-up of any other job.
func TestScheduler_should_never_run_a_job_that_has_no_next_time(t *testing.T) {
	never := every("never", "0 0 30 2 *")
	never.Cron = "0 0 30 2 *"
	jobs := mustJobsUnchecked(t, every("often", "*/5 * * * *"), never)
	clock := newClock("2026-10-09 12:00")
	runs := &recorder{fires: make(chan runs.Request, 16)}
	s := scheduler.New(scheduler.Options{Jobs: jobs, Runs: runs, Now: clock.Now, Sleep: clock.Sleep})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	defer func() { cancel(); <-done }()

	for range 2 {
		for clock.sleeping() == 0 {
			runtime.Gosched()
		}
		clock.advance(5 * time.Minute)
		first := <-runs.fires
		select {
		case second := <-runs.fires:
			t.Fatalf("two runs for one tick: %s and %s", first.Key, second.Key)
		case <-time.After(50 * time.Millisecond):
		}
		if first.Key != "schedule:often" {
			t.Fatalf("ran %s", first.Key)
		}
	}
}

// mustJobsUnchecked builds jobs the way a hand-edited file can: without
// Check's refusal of expressions that never fire.
func mustJobsUnchecked(t *testing.T, list ...model.Schedule) []scheduler.Job {
	t.Helper()
	var jobs []scheduler.Job
	for _, s := range list {
		expr, err := cron.Parse(s.Cron)
		if err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, scheduler.Job{Schedule: s, Expr: expr, Loc: time.UTC})
	}
	return jobs
}

func TestCheck_should_refuse_a_cron_expression_that_never_fires(t *testing.T) {
	s := valid()
	s.Cron = "0 0 30 2 *"

	got := scheduler.Check(s)

	if len(got) != 1 || got[0].Field != "cron" || !strings.Contains(got[0].Detail, "never") {
		t.Fatalf("%+v", got)
	}
}

// Before: one long Sleep on a monotonic timer, which does not count time
// the machine was suspended, so a job due while it slept fired hours late.
func TestScheduler_should_notice_a_wall_clock_jump_within_a_minute(t *testing.T) {
	r := start(t, "2026-10-09 12:00", every("daily", "0 13 * * *"))

	r.clock.jump(2 * time.Hour) // asleep through 13:00
	got := r.tick(time.Minute, 1)

	if len(got) != 1 || got[0].Key != "schedule:daily" {
		t.Fatalf("%+v", got)
	}
}

// claims is a Claimer that records the keys and answers from a script.
type claims struct {
	mu    sync.Mutex
	keys  []string
	allow bool
	err   error
}

func (c *claims) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = append(c.keys, key)
	return c.allow, c.err
}

func TestScheduler_Given_a_claimer_Then_a_tick_runs_only_where_it_is_claimed(t *testing.T) {
	c := &claims{allow: true}
	r := startWith(t, "2026-10-09 12:00", func(o *scheduler.Options) { o.Claim = c.Claim }, every("often", "*/5 * * * *"))

	r.tick(5*time.Minute, 1) // claimed here: runs
	c.mu.Lock()
	c.allow = false
	c.mu.Unlock()
	r.tick(5*time.Minute, 0) // another instance took it: skipped

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.keys) != 2 || c.keys[0] == c.keys[1] || !strings.Contains(c.keys[0], "often") || !strings.Contains(c.keys[0], "1791547500") {
		t.Fatalf("keys %v (one per tick, naming the schedule and the tick's own time)", c.keys)
	}
	if !strings.Contains(r.logs.String(), "ScheduleClaimedElsewhere") {
		t.Fatalf("logs:\n%s", r.logs)
	}
}

func TestScheduler_Given_a_claimer_that_fails_Then_the_tick_still_runs(t *testing.T) {
	c := &claims{err: errors.New("store down")}
	r := startWith(t, "2026-10-09 12:00", func(o *scheduler.Options) { o.Claim = c.Claim }, every("often", "*/5 * * * *"))

	got := r.tick(5*time.Minute, 1)

	if len(got) != 1 || !strings.Contains(r.logs.String(), "ScheduleClaimFailed") {
		t.Fatalf("runs %v logs:\n%s", got, r.logs)
	}
}
