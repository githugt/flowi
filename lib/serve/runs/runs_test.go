// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/serve/runs"
)

// fakeRun is a run the test finishes by hand.
type fakeRun struct {
	ctx      context.Context
	req      runs.Request
	finish   chan runs.Outcome
	obeyStop bool // finish with "cancelled" when ctx ends
}

func (f *fakeRun) Wait() runs.Outcome {
	if f.obeyStop {
		select {
		case o := <-f.finish:
			return o
		case <-f.ctx.Done():
			return runs.Outcome{Status: "cancelled"}
		}
	}
	return <-f.finish
}

type harness struct {
	t       *testing.T
	mu      sync.Mutex
	started []*fakeRun
	fail    error
	obey    bool
	m       *runs.Manager
	clock   time.Time
}

func newHarness(t *testing.T, mutate ...func(*runs.Options)) *harness {
	t.Helper()
	h := &harness{t: t, obey: true, clock: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
	var n int
	o := runs.Options{
		Start: func(ctx context.Context, r runs.Request) (runs.Handle, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			if h.fail != nil {
				return nil, h.fail
			}
			f := &fakeRun{ctx: ctx, req: r, finish: make(chan runs.Outcome, 1), obeyStop: h.obey}
			h.started = append(h.started, f)
			return f, nil
		},
		Max:    2,
		Keep:   3,
		NewID:  func() string { n++; return fmt.Sprintf("run-%d", n) },
		Now:    func() time.Time { return h.clock },
		Logger: nil,
	}
	for _, m := range mutate {
		m(&o)
	}
	h.m = runs.NewManager(o)
	return h
}

func (h *harness) run(i int) *fakeRun {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.started[i]
}

func await(t *testing.T, m *runs.Manager, id string) runs.Record {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	rec, ok := m.Await(ctx, id)
	if !ok {
		t.Fatalf("run %s unknown", id)
	}
	return rec
}

func TestManager_should_start_a_run_and_report_its_outcome(t *testing.T) {
	h := newHarness(t)

	rec, err := h.m.Start(runs.Request{Flow: "signup", Source: "trigger"})
	h.run(0).finish <- runs.Outcome{Status: "passed", EngineRunID: "E1", Seed: 7}
	done := await(t, h.m, rec.ID)

	if err != nil || rec.Status != runs.StatusRunning || rec.ID != "run-1" || rec.Flow != "signup" || rec.Source != "trigger" {
		t.Fatalf("started: %+v %v", rec, err)
	}
	if done.Status != "passed" || done.EngineRunID != "E1" || done.Seed != 7 || done.Finished.IsZero() {
		t.Fatalf("finished: %+v", done)
	}
	if got, ok := h.m.Get(rec.ID); !ok || got.Status != "passed" {
		t.Fatalf("Get: %+v %v", got, ok)
	}
}

func TestManager_should_refuse_more_runs_than_Max_until_one_finishes(t *testing.T) {
	h := newHarness(t)
	a, _ := h.m.Start(runs.Request{Flow: "a"})
	_, _ = h.m.Start(runs.Request{Flow: "b"})

	_, errFull := h.m.Start(runs.Request{Flow: "c"})
	h.run(0).finish <- runs.Outcome{Status: "passed"}
	await(t, h.m, a.ID)
	_, errAfter := h.m.Start(runs.Request{Flow: "c"})

	if !errors.Is(errFull, runs.ErrBusy) || errAfter != nil {
		t.Fatalf("full: %v, after: %v", errFull, errAfter)
	}
}

func TestManager_should_skip_a_second_run_with_the_same_key(t *testing.T) {
	h := newHarness(t)
	first, _ := h.m.Start(runs.Request{Flow: "a", Key: "schedule:nightly"})

	_, overlap := h.m.Start(runs.Request{Flow: "a", Key: "schedule:nightly"})
	_, other := h.m.Start(runs.Request{Flow: "a", Key: "schedule:hourly"})
	h.run(0).finish <- runs.Outcome{Status: "passed"}
	await(t, h.m, first.ID)
	_, again := h.m.Start(runs.Request{Flow: "a", Key: "schedule:nightly"})

	if !errors.Is(overlap, runs.ErrOverlap) || other != nil || again != nil {
		t.Fatalf("overlap %v, other %v, again %v", overlap, other, again)
	}
}

func TestManager_Given_the_starter_fails_Then_the_error_comes_back_and_the_slot_is_free(t *testing.T) {
	h := newHarness(t)
	h.fail = &runs.StartError{Kind: runs.KindNotFound, Message: "no such flow"}

	_, err := h.m.Start(runs.Request{Flow: "x", Key: "k"})
	h.fail = nil
	_, retry := h.m.Start(runs.Request{Flow: "x", Key: "k"})

	var se *runs.StartError
	if !errors.As(err, &se) || se.Kind != runs.KindNotFound || retry != nil {
		t.Fatalf("err %v, retry %v", err, retry)
	}
	if h.m.InFlight() != 1 {
		t.Fatalf("in flight %d", h.m.InFlight())
	}
}

func TestManager_Given_the_starter_panics_Then_an_internal_error_and_the_manager_lives(t *testing.T) {
	h := newHarness(t, func(o *runs.Options) {
		o.Start = func(context.Context, runs.Request) (runs.Handle, error) { panic("boom") }
	})

	_, err := h.m.Start(runs.Request{Flow: "x"})

	var se *runs.StartError
	if !errors.As(err, &se) || se.Kind != runs.KindInternal || h.m.InFlight() != 0 {
		t.Fatalf("err %v in flight %d", err, h.m.InFlight())
	}
}

func TestManager_should_forget_the_oldest_finished_runs_beyond_Keep(t *testing.T) {
	h := newHarness(t, func(o *runs.Options) { o.Max = 10 })
	var ids []string
	for i := range 5 {
		rec, err := h.m.Start(runs.Request{Flow: "a"})
		if err != nil {
			t.Fatal(err)
		}
		h.run(i).finish <- runs.Outcome{Status: "passed"}
		await(t, h.m, rec.ID)
		ids = append(ids, rec.ID)
	}

	_, oldest := h.m.Get(ids[0])
	_, newest := h.m.Get(ids[4])

	if oldest || !newest {
		t.Fatalf("oldest kept %v, newest kept %v", oldest, newest)
	}
}

func TestManager_Shutdown_should_wait_for_runs_that_finish_in_time_and_refuse_new_ones(t *testing.T) {
	h := newHarness(t)
	rec, _ := h.m.Start(runs.Request{Flow: "a"})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)

	go func() { result <- h.m.Shutdown(ctx) }()
	waitUntil(t, h.m.Draining)
	_, afterErr := h.m.Start(runs.Request{Flow: "b"})
	select {
	case err := <-result:
		t.Fatalf("Shutdown returned %v while a run was in flight", err)
	default:
	}
	h.run(0).finish <- runs.Outcome{Status: "passed"}

	if err := <-result; err != nil || !errors.Is(afterErr, runs.ErrClosed) {
		t.Fatalf("shutdown %v, start after %v", err, afterErr)
	}
	if got, _ := h.m.Get(rec.ID); got.Status != "passed" {
		t.Fatalf("run was cut short: %+v", got)
	}
}

// waitUntil polls cond until it holds or five seconds pass.
func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		runtime.Gosched()
	}
}

func TestManager_Shutdown_Given_the_deadline_passes_Then_cancels_what_is_left(t *testing.T) {
	h := newHarness(t)
	rec, _ := h.m.Start(runs.Request{Flow: "a"})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()

	err := h.m.Shutdown(ctx)

	if !errors.Is(err, runs.ErrDrainTimeout) {
		t.Fatalf("got %v, want ErrDrainTimeout", err)
	}
	if got, _ := h.m.Get(rec.ID); got.Status != "cancelled" {
		t.Fatalf("run not cancelled: %+v", got)
	}
}

func TestManager_Shutdown_Given_a_run_that_ignores_cancel_Then_gives_up_after_CancelWait(t *testing.T) {
	h := newHarness(t, func(o *runs.Options) { o.CancelWait = 40 * time.Millisecond })
	h.obey = false
	_, _ = h.m.Start(runs.Request{Flow: "a"})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	started := time.Now()

	err := h.m.Shutdown(ctx)

	if !errors.Is(err, runs.ErrStuck) || time.Since(started) > 3*time.Second {
		t.Fatalf("got %v after %v", err, time.Since(started))
	}
	h.run(0).finish <- runs.Outcome{Status: "aborted"} // let the goroutine go
}

func TestManager_should_be_safe_under_concurrent_use(t *testing.T) {
	h := newHarness(t, func(o *runs.Options) { o.Max = 1000; o.Keep = 1000 })
	var wg sync.WaitGroup

	for range 50 {
		wg.Go(func() {
			rec, err := h.m.Start(runs.Request{Flow: "a"})
			if err != nil {
				t.Error(err)
				return
			}
			_, _ = h.m.Get(rec.ID)
		})
	}
	wg.Wait()
	for i := range 50 {
		h.run(i).finish <- runs.Outcome{Status: "passed"}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := h.m.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

// sharedFake records what the manager publishes.
type sharedFake struct {
	mu    sync.Mutex
	saved map[string][]string // id → statuses in order
	other map[string]runs.Record
	err   error
}

func (s *sharedFake) SaveRecord(_ context.Context, r runs.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	if s.saved == nil {
		s.saved = map[string][]string{}
	}
	s.saved[r.ID] = append(s.saved[r.ID], r.Status)
	return nil
}

func (s *sharedFake) LoadRecord(_ context.Context, id string) (runs.Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.other[id]
	return r, ok, s.err
}

func TestManager_should_publish_a_run_when_it_starts_and_when_it_ends(t *testing.T) {
	sh := &sharedFake{}
	h := newHarness(t, func(o *runs.Options) { o.Shared = sh })

	rec, err := h.m.Start(runs.Request{Flow: "f"})
	if err != nil {
		t.Fatal(err)
	}
	h.run(0).finish <- runs.Outcome{Status: "passed"}
	if _, ok := h.m.Await(context.Background(), rec.ID); !ok {
		t.Fatal("run lost")
	}

	sh.mu.Lock()
	defer sh.mu.Unlock()
	if got := sh.saved[rec.ID]; len(got) != 2 || got[0] != runs.StatusRunning || got[1] != "passed" {
		t.Fatalf("published %v", got)
	}
}

func TestManager_Find_should_fall_back_to_a_run_of_another_instance(t *testing.T) {
	sh := &sharedFake{other: map[string]runs.Record{"far": {ID: "far", Status: "passed"}}}
	h := newHarness(t, func(o *runs.Options) { o.Shared = sh })

	far, okFar := h.m.Find(context.Background(), "far")
	_, okNone := h.m.Find(context.Background(), "nowhere")

	if !okFar || far.Status != "passed" || okNone {
		t.Fatalf("far %+v %v, none %v", far, okFar, okNone)
	}
}

func TestManager_Given_a_failing_shared_store_Then_runs_are_unaffected(t *testing.T) {
	sh := &sharedFake{err: errors.New("down")}
	h := newHarness(t, func(o *runs.Options) { o.Shared = sh })

	rec, err := h.m.Start(runs.Request{Flow: "f"})
	h.run(0).finish <- runs.Outcome{Status: "passed"}
	done, ok := h.m.Await(context.Background(), rec.ID)
	_, found := h.m.Find(context.Background(), "unknown")

	if err != nil || !ok || done.Status != "passed" || found {
		t.Fatalf("err %v ok %v done %+v found %v", err, ok, done, found)
	}
}
