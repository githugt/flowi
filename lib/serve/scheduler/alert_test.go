// SPDX-License-Identifier: Apache-2.0

package scheduler_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/serve/runs"
	"github.com/githugt/flowi/lib/serve/scheduler"
)

type posted struct{ addr, text string }

// alerts is a receiver and a run manager the test controls: Await answers with the record the test chose.
type alerts struct {
	rec     runs.Record
	got     chan posted
	postErr error
	env     map[string]string
}

func newAlerts(status string, messages ...string) *alerts {
	return &alerts{rec: runs.Record{ID: "run", Status: status, Outcome: runs.Outcome{Status: status, Messages: messages}}, got: make(chan posted, 4), env: map[string]string{}}
}

func (a *alerts) adjust(o *scheduler.Options) {
	o.Await = func(context.Context, string) (runs.Record, bool) { return a.rec, true }
	o.Getenv = func(k string) string { return a.env[k] }
	o.Post = func(_ context.Context, addr string, body []byte) error {
		var m map[string]string
		if err := json.Unmarshal(body, &m); err != nil {
			panic(err)
		}
		a.got <- posted{addr, m["text"]}
		return a.postErr
	}
}

func (a *alerts) next(t *testing.T) posted {
	t.Helper()
	select {
	case p := <-a.got:
		return p
	case <-time.After(5 * time.Second):
		t.Fatal("no alert was sent")
		return posted{}
	}
}

func (a *alerts) none(t *testing.T, r *rig) {
	t.Helper()
	r.stop()
	<-r.done // Run waits for its alert goroutines, so nothing can still be on its way
	select {
	case p := <-a.got:
		t.Fatalf("unexpected alert %+v", p)
	default:
	}
}

func hooked(name, failure, success string) model.Schedule {
	s := every(name, "*/5 * * * *")
	s.OnFailure, s.OnSuccess = failure, success
	return s
}

// Before: a failed night run showed only in logs and in the app.
func TestScheduler_Given_on_failure_Then_a_failed_run_is_told_to_that_address(t *testing.T) {
	a := newAlerts("failed", "step login: expected 201, got 500")
	r := startWith(t, "2026-10-09 12:00", a.adjust, hooked("nightly", "https://hooks.example/T1", ""))

	r.tick(5*time.Minute, 1)

	got := a.next(t)
	if got.addr != "https://hooks.example/T1" ||
		!strings.Contains(got.text, "schedule nightly (flow signup) in staging failed") ||
		!strings.Contains(got.text, "• step login: expected 201, got 500") || !strings.Contains(got.text, "run run") {
		t.Fatalf("alert %+v", got)
	}
}

func TestScheduler_Given_on_success_Then_only_a_passed_run_is_told(t *testing.T) {
	a := newAlerts("passed")
	r := startWith(t, "2026-10-09 12:00", a.adjust, hooked("nightly", "https://hooks.example/fail", "https://hooks.example/ok"))

	r.tick(5*time.Minute, 1)

	got := a.next(t)
	if got.addr != "https://hooks.example/ok" || !strings.Contains(got.text, "passed") {
		t.Fatalf("alert %+v", got)
	}
	a.none(t, r)
}

func TestScheduler_should_stay_quiet_for_a_passed_run_with_only_on_failure_and_for_a_cancelled_run(t *testing.T) {
	passed := newAlerts("passed")
	r1 := startWith(t, "2026-10-09 12:00", passed.adjust, hooked("a", "https://hooks.example/x", ""))
	cancelled := newAlerts("cancelled")
	r2 := startWith(t, "2026-10-09 12:00", cancelled.adjust, hooked("b", "https://hooks.example/x", "https://hooks.example/y"))

	r1.tick(5*time.Minute, 1)
	r2.tick(5*time.Minute, 1)

	passed.none(t, r1)
	cancelled.none(t, r2)
}

func TestScheduler_should_stay_quiet_without_a_hook_or_when_the_run_is_still_going_at_shutdown(t *testing.T) {
	plain := newAlerts("failed")
	r1 := startWith(t, "2026-10-09 12:00", plain.adjust, every("plain", "*/5 * * * *"))
	going := newAlerts(runs.StatusRunning)
	r2 := startWith(t, "2026-10-09 12:00", going.adjust, hooked("c", "https://hooks.example/x", ""))

	r1.tick(5*time.Minute, 1)
	r2.tick(5*time.Minute, 1)

	plain.none(t, r1)
	going.none(t, r2)
}

func TestScheduler_Given_an_env_address_Then_it_is_read_when_the_alert_goes_out(t *testing.T) {
	a := newAlerts("error", "boom")
	a.env["SLACK_HOOK"] = "https://hooks.example/from-env"
	r := startWith(t, "2026-10-09 12:00", a.adjust, hooked("nightly", "env:SLACK_HOOK", ""))

	r.tick(5*time.Minute, 1)

	if got := a.next(t); got.addr != "https://hooks.example/from-env" || !strings.Contains(got.text, " error") {
		t.Fatalf("alert %+v", got)
	}
}

func TestScheduler_Given_an_unset_env_address_or_a_receiver_that_fails_Then_it_is_logged_without_the_address(t *testing.T) {
	unset := newAlerts("failed")
	r1 := startWith(t, "2026-10-09 12:00", unset.adjust, hooked("a", "env:NOPE", ""))
	down := newAlerts("failed")
	down.postErr = context.DeadlineExceeded
	r2 := startWith(t, "2026-10-09 12:00", down.adjust, hooked("b", "https://hooks.example/SECRET-TOKEN", ""))

	r1.tick(5*time.Minute, 1)
	r2.tick(5*time.Minute, 1)
	down.next(t)
	unset.none(t, r1)
	down.none(t, r2)

	if !strings.Contains(r1.logs.String(), "NOPE is not set") {
		t.Errorf("unset: %s", r1.logs)
	}
	if !strings.Contains(r2.logs.String(), "Event=ScheduleAlertFailed") || !strings.Contains(r2.logs.String(), "Host=hooks.example") ||
		strings.Contains(r2.logs.String(), "SECRET-TOKEN") {
		t.Errorf("down: %s", r2.logs)
	}
}

func TestScheduler_Given_a_run_that_cannot_start_Then_on_failure_is_told_why(t *testing.T) {
	a := newAlerts("passed")
	r := startWith(t, "2026-10-09 12:00", a.adjust, hooked("nightly", "https://hooks.example/x", ""))
	r.runs.fail(&runs.StartError{Kind: runs.KindNotFound, Message: `no flow "signup"`})

	r.tick(5*time.Minute, 1)

	if got := a.next(t); !strings.Contains(got.text, "did not start") || !strings.Contains(got.text, `no flow "signup"`) {
		t.Fatalf("alert %+v", got)
	}
}

func TestScheduler_should_keep_an_alert_short_when_a_run_has_many_messages(t *testing.T) {
	long := strings.Repeat("x", 500)
	a := newAlerts("failed", long, "two", "three", "four", "five")
	r := startWith(t, "2026-10-09 12:00", a.adjust, hooked("nightly", "https://hooks.example/x", ""))

	r.tick(5*time.Minute, 1)

	got := a.next(t).text
	if strings.Contains(got, long) || !strings.Contains(got, "… and 2 more") || strings.Contains(got, "four") {
		t.Fatalf("text %q", got)
	}
}

func TestCheck_should_refuse_a_hook_that_is_not_an_http_address_or_an_env_reference(t *testing.T) {
	cases := map[string]bool{
		"": false, "https://hooks.example/x": false, "http://localhost:9/x": false, "env:SLACK_HOOK": false,
		"ftp://x/y": true, "hooks.example/x": true, "https://": true, "env:": true, "env:A B": true,
	}
	for v, bad := range cases {
		s := every("n", "@daily")
		s.OnFailure = v

		problems := scheduler.Check(s)

		if (len(problems) > 0) != bad || (bad && problems[0].Field != "on_failure") {
			t.Errorf("%q: %+v", v, problems)
		}
	}
}
