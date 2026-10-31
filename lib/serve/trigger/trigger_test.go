// SPDX-License-Identifier: Apache-2.0

package trigger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/githugt/flowi/lib/serve/runs"
	"github.com/githugt/flowi/lib/serve/trigger"
)

const token = "s3cret-token-value"

type fakeRun struct{ finish chan runs.Outcome }

func (f *fakeRun) Wait() runs.Outcome { return <-f.finish }

type rig struct {
	t         *testing.T
	h         http.Handler
	runs      *runs.Manager
	logs      *bytes.Buffer
	mu        sync.Mutex
	started   []runs.Request
	handles   []*fakeRun
	fail      error
	calls     int  // numbers the callers, so one test's many bad tokens do not lock one address out
	oneCaller bool // every call comes from the same address
}

func newRig(t *testing.T, mutate ...func(*trigger.Options, *runs.Options)) *rig {
	t.Helper()
	r := &rig{t: t, logs: &bytes.Buffer{}}
	var n int
	ro := runs.Options{
		Max:   2,
		NewID: func() string { n++; return fmt.Sprintf("run-%d", n) },
		Start: func(_ context.Context, req runs.Request) (runs.Handle, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.fail != nil {
				return nil, r.fail
			}
			h := &fakeRun{finish: make(chan runs.Outcome, 1)}
			r.started, r.handles = append(r.started, req), append(r.handles, h)
			return h, nil
		},
	}
	to := trigger.Options{Token: token, Logger: slog.New(slog.NewTextHandler(r.logs, nil))}
	for _, m := range mutate {
		m(&to, &ro)
	}
	r.runs = runs.NewManager(ro)
	to.Runs = r.runs
	h, err := trigger.New(to)
	if err != nil {
		t.Fatal(err)
	}
	r.h = h
	return r
}

type reply struct {
	status int
	header http.Header
	body   string
}

func (r *rig) do(method, target, body string, headers ...string) reply {
	r.t.Helper()
	req := httptest.NewRequestWithContext(r.t.Context(), method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if !r.oneCaller {
		r.calls++
		req.RemoteAddr = fmt.Sprintf("192.0.2.%d:4000", r.calls%250+1)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i+1] == "" {
			req.Header.Del(headers[i])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return reply{status: res.StatusCode, header: res.Header, body: string(b)}
}

func (p reply) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(p.body), &m); err != nil {
		t.Fatalf("body %q: %v", p.body, err)
	}
	return m
}

func TestTrigger_should_start_a_run_and_answer_202_with_its_id(t *testing.T) {
	r := newRig(t)

	got := r.do("POST", "/flows/signup/run", "")

	body := got.json(t)
	if got.status != 202 || body["run_id"] != "run-1" || body["status"] != "running" || body["flow"] != "signup" {
		t.Fatalf("%d %s", got.status, got.body)
	}
	if len(r.started) != 1 || r.started[0].Source != "trigger" || r.started[0].Flow != "signup" {
		t.Fatalf("started %+v", r.started)
	}
	if loc := got.header.Get("Location"); loc != "/runs/run-1" {
		t.Errorf("Location %q", loc)
	}
}

func TestTrigger_should_pass_flow_paths_collection_and_body_options_to_the_run(t *testing.T) {
	r := newRig(t)

	got := r.do("POST", "/flows/users/signup/run?collection=shop", `{"env": "staging", "inputs": {"email": "a@example.com", "count": 3, "dry": true}, "prefer_version": "2", "seed": 42}`)

	if got.status != 202 {
		t.Fatalf("%d %s", got.status, got.body)
	}
	want := runs.Request{Flow: "users/signup", Collection: "shop", Env: "staging", PreferVersion: "2", Seed: 42, Source: "trigger",
		Inputs: map[string]string{"email": "a@example.com", "count": "3", "dry": "true"}}
	g := r.started[0]
	if g.Flow != want.Flow || g.Collection != want.Collection || g.Env != want.Env || g.PreferVersion != want.PreferVersion || g.Seed != want.Seed || len(g.Inputs) != 3 {
		t.Fatalf("got %+v want %+v", g, want)
	}
	for k, v := range want.Inputs {
		if g.Inputs[k] != v {
			t.Errorf("input %s = %q, want %q", k, g.Inputs[k], v)
		}
	}
}

func TestTrigger_should_reject_bad_bodies(t *testing.T) {
	r := newRig(t)
	cases := map[string]string{
		"invalid json":       `{"env":`,
		"unknown field":      `{"envv": "x"}`,
		"nested input":       `{"inputs": {"a": {"b": 1}}}`,
		"list input":         `{"inputs": {"a": [1]}}`,
		"negative seed":      `{"seed": -1}`,
		"input name empty":   `{"inputs": {"": "x"}}`,
		"trailing content":   `{} {}`,
		"seed not a number":  `{"seed": "abc"}`,
		"inputs not mapping": `{"inputs": "x"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got := r.do("POST", "/flows/signup/run", body)

			if got.status != 400 {
				t.Fatalf("%d %s", got.status, got.body)
			}
		})
	}
	if len(r.started) != 0 {
		t.Fatalf("a bad body started a run: %+v", r.started)
	}
}

func TestTrigger_should_refuse_huge_bodies(t *testing.T) {
	r := newRig(t)

	got := r.do("POST", "/flows/signup/run", `{"inputs": {"a": "`+strings.Repeat("x", 2<<20)+`"}}`)

	if got.status != 413 {
		t.Fatalf("%d", got.status)
	}
}

// Before: it is easy to compare tokens with == and to answer 404 before
// checking who is asking, which tells a stranger which flows exist.
func TestTrigger_should_require_the_bearer_token_before_anything_else(t *testing.T) {
	r := newRig(t, func(_ *trigger.Options, ro *runs.Options) {
		inner := ro.Start
		ro.Start = func(ctx context.Context, req runs.Request) (runs.Handle, error) {
			if req.Flow == "ghost" {
				return nil, &runs.StartError{Kind: runs.KindNotFound, Message: "no flow ghost"}
			}
			return inner(ctx, req)
		}
	})
	bad := map[string][]string{
		"no header":      {"Authorization", ""},
		"wrong token":    {"Authorization", "Bearer nope"},
		"wrong scheme":   {"Authorization", "Basic " + token},
		"token only":     {"Authorization", token},
		"empty bearer":   {"Authorization", "Bearer "},
		"prefix of real": {"Authorization", "Bearer " + token[:5]},
		"longer":         {"Authorization", "Bearer " + token + "x"},
	}

	for name, h := range bad {
		for _, target := range []string{"/flows/signup/run", "/flows/ghost/run", "/runs/run-1", "/anything/else"} {
			got := r.do("POST", target, "", h...)

			if got.status != 401 || got.header.Get("WWW-Authenticate") != "Bearer" {
				t.Errorf("%s %s: %d %v", name, target, got.status, got.header)
			}
			if strings.Contains(got.body, "ghost") {
				t.Errorf("%s %s: an unauthenticated answer names the flow: %s", name, target, got.body)
			}
		}
	}
	if len(r.started) != 0 {
		t.Fatalf("an unauthenticated call started a run")
	}
	if strings.Contains(r.logs.String(), token) {
		t.Fatalf("the token was logged:\n%s", r.logs)
	}
	if !strings.Contains(r.logs.String(), "Event=TriggerDenied") {
		t.Fatalf("denials are not logged:\n%s", r.logs)
	}
}

func TestTrigger_should_not_take_the_token_from_the_query_string(t *testing.T) {
	r := newRig(t)

	got := r.do("POST", "/flows/signup/run?token="+token, "", "Authorization", "")

	if got.status != 401 {
		t.Fatalf("%d", got.status)
	}
}

func TestTrigger_should_map_start_errors_to_statuses_without_leaking_internals(t *testing.T) {
	cases := []struct {
		err    error
		status int
		shows  string
		hides  string
	}{
		{&runs.StartError{Kind: runs.KindNotFound, Message: `no flow "ghost"`}, 404, "ghost", ""},
		{&runs.StartError{Kind: runs.KindAmbiguous, Message: "2 flows match"}, 409, "2 flows", ""},
		{&runs.StartError{Kind: runs.KindInvalid, Message: `input "email" is missing`}, 400, "email", ""},
		{&runs.StartError{Kind: runs.KindInternal, Message: "the run could not be started", Err: errors.New("open /Users/me/vault: denied")}, 500, "could not be started", "/Users/me"},
		{runs.ErrBusy, 429, "too many", ""},
		{runs.ErrClosed, 503, "shutting down", ""},
		{errors.New("raw failure at /etc/secret"), 500, "could not be started", "/etc/secret"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprint(c.status), func(t *testing.T) {
			r := newRig(t)
			r.fail = c.err

			got := r.do("POST", "/flows/ghost/run", "")

			if got.status != c.status || !strings.Contains(got.body, c.shows) || (c.hides != "" && strings.Contains(got.body, c.hides)) {
				t.Fatalf("%d %s", got.status, got.body)
			}
			if c.status == 429 && got.header.Get("Retry-After") == "" {
				t.Error("429 without Retry-After")
			}
		})
	}
}

func TestTrigger_wait_should_answer_200_with_the_result_when_the_run_finishes_in_time(t *testing.T) {
	r := newRig(t)
	done := make(chan reply, 1)

	go func() { done <- r.do("POST", "/flows/signup/run?wait=5s", "") }()
	for {
		r.mu.Lock()
		n := len(r.handles)
		r.mu.Unlock()
		if n == 1 {
			break
		}
		runtime.Gosched() // wait for the handler goroutine to start the run
	}
	r.handles[0].finish <- runs.Outcome{Status: "passed", EngineRunID: "E1", Seed: 9, Messages: []string{"all good"}}
	got := <-done

	body := got.json(t)
	if got.status != 200 || body["status"] != "passed" || body["report_id"] != "E1" || body["seed"] != float64(9) || body["finished"] == nil {
		t.Fatalf("%d %s", got.status, got.body)
	}
}

func TestTrigger_wait_should_answer_202_when_the_run_is_still_going(t *testing.T) {
	r := newRig(t)

	got := r.do("POST", "/flows/signup/run?wait=20ms", "")

	if got.status != 202 || got.json(t)["status"] != "running" {
		t.Fatalf("%d %s", got.status, got.body)
	}
}

func TestTrigger_wait_should_be_a_duration(t *testing.T) {
	r := newRig(t)

	for _, w := range []string{"soon", "-1s", "0"} {
		if got := r.do("POST", "/flows/signup/run?wait="+w, ""); got.status != 400 {
			t.Errorf("wait=%s gave %d", w, got.status)
		}
	}
}

func TestTrigger_should_report_runs_by_id(t *testing.T) {
	r := newRig(t)
	r.do("POST", "/flows/signup/run", "")
	before := r.do("GET", "/runs/run-1", "")
	r.handles[0].finish <- runs.Outcome{Status: "failed", Messages: []string{"node login failed"}}
	if _, ok := r.runs.Await(t.Context(), "run-1"); !ok {
		t.Fatal("unknown run")
	}
	after := r.do("GET", "/runs/run-1", "")
	missing := r.do("GET", "/runs/run-99", "")

	if before.json(t)["status"] != "running" || after.json(t)["status"] != "failed" || missing.status != 404 {
		t.Fatalf("before %s after %s missing %d", before.body, after.body, missing.status)
	}
	if msgs, _ := after.json(t)["messages"].([]any); len(msgs) != 1 {
		t.Errorf("messages %s", after.body)
	}
}

func TestTrigger_should_answer_health_checks_without_a_token(t *testing.T) {
	r := newRig(t)

	got := r.do("GET", "/healthz", "", "Authorization", "")

	if got.status != 200 || got.json(t)["status"] != "ok" {
		t.Fatalf("%d %s", got.status, got.body)
	}
}

func TestTrigger_should_use_the_right_methods(t *testing.T) {
	r := newRig(t)

	for _, c := range [][2]string{{"GET", "/flows/signup/run"}, {"POST", "/runs/run-1"}, {"DELETE", "/flows/signup/run"}} {
		got := r.do(c[0], c[1], "")

		if got.status != 405 {
			t.Errorf("%s %s gave %d", c[0], c[1], got.status)
		}
	}
	if got := r.do("POST", "/flows//run", ""); got.status != 404 {
		t.Errorf("empty flow gave %d", got.status)
	}
}

func TestNew_should_refuse_to_run_without_a_token(t *testing.T) {
	_, err := trigger.New(trigger.Options{Runs: runs.NewManager(runs.Options{Max: 1, NewID: func() string { return "x" }, Start: nil})})

	if !errors.Is(err, trigger.ErrNoToken) {
		t.Fatalf("got %v", err)
	}
}

// Before: a caller could send bad tokens without limit. Now an address that
// sends maxFailures of them in a minute is locked out until the minute ends.
func TestTrigger_Given_repeated_bad_tokens_When_the_limit_is_hit_Then_429_until_the_window_ends(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t, func(o *trigger.Options, _ *runs.Options) { o.Now = time.Now })
		r.oneCaller = true
		bad := func() reply { return r.do("GET", "/runs/x", "", "Authorization", "Bearer wrong") }
		for range 10 {
			if got := bad(); got.status != 401 {
				t.Fatalf("status %d, want 401", got.status)
			}
		}

		locked := bad()
		lockedGood := r.do("GET", "/runs/x", "")
		time.Sleep(time.Minute)
		after := r.do("GET", "/healthz", "")
		recovered := bad()

		if locked.status != 429 || locked.header.Get("Retry-After") == "" {
			t.Fatalf("locked: %d %v", locked.status, locked.header)
		}
		if lockedGood.status != 429 {
			t.Fatalf("a valid token during the lockout got %d", lockedGood.status)
		}
		if after.status != 200 || recovered.status != 401 {
			t.Fatalf("after the window: healthz %d, bad token %d", after.status, recovered.status)
		}
	})
}
