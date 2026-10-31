// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/serve/replay"
	"github.com/githugt/flowi/lib/serve/shared/sharedtest"
)

// serveOnFreePorts starts a server on ephemeral loopback ports and stops
// it (and waits) when the test ends.
func serveOnFreePorts(t *testing.T, w *Workspace, mutate ...func(*ServeOptions)) *Server {
	t.Helper()
	o := ServeOptions{ReplayAddr: "127.0.0.1:0", TriggerAddr: "127.0.0.1:0", ShutdownTimeout: 5 * time.Second}
	for _, m := range mutate {
		m(&o)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s, err := w.Serve(ctx, o)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = s.Wait() })
	return s
}

func call(t *testing.T, method, url, token, body string, headers ...string) (int, http.Header, string) {
	t.Helper()
	status, header, text, err := tryCall(method, url, token, body, headers...)
	if err != nil {
		t.Fatal(err)
	}
	return status, header, text
}

// tryCall is call for goroutines, which cannot fail the test directly.
func tryCall(method, url, token, body string, headers ...string) (int, http.Header, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return 0, nil, "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, string(b), nil
}

func TestServe_should_replay_pinned_examples_of_mounted_collections(t *testing.T) {
	w := demoWorkspace(t)
	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.NoTriggers = true })

	status, header, body := call(t, "POST", "http://"+s.ReplayAddr()+"/v1/login", "", "{}")
	v2Status, _, v2Body := call(t, "POST", "http://"+s.ReplayAddr()+"/v2/login", "", "{}")

	if status != 200 || header.Get("X-Flowi-Request") != "auth/login" || !strings.Contains(body, "usr_demo.signature") {
		t.Fatalf("v1: %d %v %s", status, header, body)
	}
	if v2Status != 200 || !strings.Contains(v2Body, `"user"`) {
		t.Fatalf("v2: %d %s", v2Status, v2Body)
	}
	if entries := s.Traffic().Entries(); len(entries) != 2 || entries[0].Request != "auth/login" {
		t.Fatalf("traffic %+v", entries)
	}
}

func TestServe_should_pick_up_new_requests_without_a_restart(t *testing.T) {
	w := demoWorkspace(t)
	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.NoTriggers = true })
	if status, _, _ := call(t, "GET", "http://"+s.ReplayAddr()+"/v1/fresh", "", ""); status != 404 {
		t.Fatalf("before: %d", status)
	}
	root := w.Collections()[0].Root
	req := "id: 01JA00000000000000000000F1\nversions:\n  \"1\":\n    method: GET\n    url: \"{{env.base_url}}/v1/fresh\"\n"
	example := `{"status": 200, "body": {"fresh": true}}`
	write := func(file, text string) {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "requests", "fresh.examples", "ok.json"), example)
	write(filepath.Join(root, "requests", "fresh.req.yaml"), req)

	deadline := time.Now().Add(10 * time.Second)
	for {
		status, _, body := call(t, "GET", "http://"+s.ReplayAddr()+"/v1/fresh", "", "")
		if status == 200 && strings.Contains(body, "fresh") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("new request never served; last %d %s", status, body)
		}
		runtime.Gosched()
	}
}

func TestServe_triggers_should_run_a_flow_for_a_caller_with_the_token(t *testing.T) {
	w := demoWorkspace(t)
	token, err := w.NewTriggerToken()
	if err != nil {
		t.Fatal(err)
	}
	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.NoReplay = true })
	url := "http://" + s.TriggerAddr() + "/flows/shared/login/run?wait=30s"
	body := `{"env": "local", "inputs": {"email": "ada@example.com"}}`

	denied, _, _ := call(t, "POST", url, "", body)
	status, _, answer := call(t, "POST", url, token, body)
	missing, _, _ := call(t, "POST", "http://"+s.TriggerAddr()+"/flows/no-such-flow/run", token, "")
	noInput, _, noInputBody := call(t, "POST", "http://"+s.TriggerAddr()+"/flows/shared/login/run", token, `{"env": "local", "inputs": {"nope": "1"}}`)

	var got map[string]any
	_ = json.Unmarshal([]byte(answer), &got)
	if denied != 401 || status != 200 || got["status"] != "passed" || got["report_id"] == "" {
		t.Fatalf("denied %d, run %d %s", denied, status, answer)
	}
	if missing != 404 || noInput != 400 || !strings.Contains(noInputBody, "nope") {
		t.Fatalf("missing %d, bad input %d %s", missing, noInput, noInputBody)
	}
}

func TestServe_Given_no_token_Then_triggers_are_off_and_say_so(t *testing.T) {
	w := demoWorkspace(t)

	s := serveOnFreePorts(t, w)

	if s.TriggerAddr() != "" || s.ReplayAddr() == "" {
		t.Fatalf("trigger %q replay %q", s.TriggerAddr(), s.ReplayAddr())
	}
	if !containsNote(s.Notes(), "triggers are off") {
		t.Fatalf("notes %v", s.Notes())
	}
}

func containsNote(notes []string, sub string) bool {
	for _, n := range notes {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

func TestServe_should_run_schedules_and_skip_invalid_ones(t *testing.T) {
	w := demoWorkspace(t)
	for _, s := range []model.Schedule{
		{Name: "every-minute", Flow: "shared/login", Cron: "* * * * *", Env: "local", Inputs: []model.KeyValue{{Key: "email", Value: "a@b.c"}}, TZ: "UTC"},
		{Name: "lost", Flow: "no-such-flow", Cron: "* * * * *", Env: "local"},
	} {
		if err := w.AddSchedule(s); err != nil {
			t.Fatal(err)
		}
	}
	// A hand-edited file can hold what AddSchedule refuses.
	w.mu.Lock()
	w.model.Schedules = append(w.model.Schedules, model.Schedule{Name: "broken", Flow: "x", Cron: "nope"})
	w.mu.Unlock()
	var ticks atomic.Int32
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	clock := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	s := serveOnFreePorts(t, w, func(o *ServeOptions) {
		o.NoReplay, o.NoTriggers = true, true
		o.Now = func() time.Time { return clock }
		o.Sleep = func(ctx context.Context, _ time.Duration) error {
			if ticks.Add(1) == 1 {
				clock = clock.Add(time.Minute) // the first sleep ends at once: a tick
				return nil
			}
			select {
			case <-hold:
				return nil
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		}
	})

	reports := filepath.Join(w.DataDir(w.Collections()[0].Manifest.ID), "runs")
	deadline := time.Now().Add(30 * time.Second)
	for {
		if found, _ := filepath.Glob(filepath.Join(reports, "*", "*.json")); len(found) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the schedule never ran the flow")
		}
		runtime.Gosched()
	}
	if !containsNote(s.Notes(), `schedule "broken"`) {
		t.Errorf("an invalid schedule was not reported: %v", s.Notes())
	}
	if !containsNote(s.Notes(), `schedule "lost"`) {
		t.Errorf("a schedule with no flow was not reported: %v", s.Notes())
	}
	if len(s.Upcoming()) != 2 {
		t.Errorf("upcoming %+v", s.Upcoming())
	}
}

func TestServe_Shutdown_should_let_an_in_flight_request_finish(t *testing.T) {
	w := demoWorkspace(t)
	entered, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	s, err := w.Serve(ctx, ServeOptions{ReplayAddr: "127.0.0.1:0", NoTriggers: true, ShutdownTimeout: 10 * time.Second,
		replaySleep: func(context.Context, time.Duration) error { close(entered); <-release; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		status int
		body   string
	}
	answered := make(chan result, 1)
	go func() {
		status, _, body, _ := tryCall("POST", "http://"+s.ReplayAddr()+"/v1/login", "", "{}", "X-Flowi-Latency", "1s")
		answered <- result{status, body}
	}()
	<-entered

	cancel()
	waited := make(chan error, 1)
	go func() { waited <- s.Wait() }()
	select {
	case err := <-waited:
		t.Fatalf("Wait returned %v while a request was in flight", err)
	default:
	}
	close(release)

	got := <-answered
	if err := <-waited; err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got.status != 200 || !strings.Contains(got.body, "usr_demo") {
		t.Fatalf("the in-flight request was cut: %d %s", got.status, got.body)
	}
	dialer := net.Dialer{Timeout: time.Second}
	if conn, err := dialer.DialContext(context.Background(), "tcp", s.ReplayAddr()); err == nil {
		_ = conn.Close()
		t.Fatal("the replay port is still open")
	}
}

func TestServe_Shutdown_Given_a_request_that_never_ends_Then_gives_up_after_the_timeout(t *testing.T) {
	w := demoWorkspace(t)
	entered := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	s, err := w.Serve(ctx, ServeOptions{ReplayAddr: "127.0.0.1:0", NoTriggers: true, ShutdownTimeout: 100 * time.Millisecond,
		replaySleep: func(c context.Context, _ time.Duration) error { close(entered); <-c.Done(); return context.Cause(c) }})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _, _, _ = tryCall("POST", "http://"+s.ReplayAddr()+"/v1/login", "", "{}", "X-Flowi-Latency", "1s")
	}() // fails when the server gives up on it
	<-entered
	started := time.Now()

	cancel()
	err = s.Wait()

	if !errors.Is(err, ErrShutdownTimeout) || time.Since(started) > 10*time.Second {
		t.Fatalf("got %v after %v", err, time.Since(started))
	}
}

func TestServe_should_fail_fast_on_a_port_in_use_and_on_bad_options(t *testing.T) {
	w := demoWorkspace(t)
	taken, err := listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	spare, err := listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	spareAddr := spare.Addr().String()
	_ = spare.Close() // free again: the replayer binds it and a --record URL points back at it
	invalid := map[string]bool{"bad address": true, "record not a URL": true, "record not http": true, "record to itself": true,
		"nothing to serve": true, "negative latency": true, "negative shutdown": true, "fault rate over one": true, "faults with record": true, "tape without record": true}
	cases := map[string]ServeOptions{
		"port in use":         {ReplayAddr: taken.Addr().String()},
		"bad address":         {ReplayAddr: "not an address"},
		"record not a URL":    {ReplayAddr: "127.0.0.1:0", Record: "::"},
		"record not http":     {ReplayAddr: "127.0.0.1:0", Record: "ftp://example.com"},
		"record to itself":    {ReplayAddr: spareAddr, Record: "http://" + spareAddr},
		"nothing to serve":    {NoReplay: true, NoTriggers: true, NoSchedules: true},
		"negative latency":    {ReplayAddr: "127.0.0.1:0", Latency: -time.Second},
		"negative shutdown":   {ReplayAddr: "127.0.0.1:0", ShutdownTimeout: -time.Second},
		"tape without record": {ReplayAddr: "127.0.0.1:0", RecordHAR: "x.har"},
		"fault rate over one": {ReplayAddr: "127.0.0.1:0", Faults: replay.Faults{ErrorRate: 1.5}},
		"faults with record":  {ReplayAddr: "127.0.0.1:0", Record: "http://127.0.0.1:1", Faults: replay.Faults{DropRate: 0.5}},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			s, err := w.Serve(ctx, o)

			if err == nil {
				cancel()
				_ = s.Wait()
				t.Fatal("accepted")
			}
			if invalid[name] != errors.Is(err, ErrInvalidServeOptions) {
				t.Fatalf("got %v; invalid options expected: %v", err, invalid[name])
			}
			if invalid[name] != errors.Is(err, ErrInvalidServeOptions) {
				t.Fatalf("got %v; invalid options expected: %v", err, invalid[name])
			}
		})
	}
}

func TestServe_should_note_a_listener_that_is_open_to_the_network(t *testing.T) {
	w := demoWorkspace(t)

	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.ReplayAddr = "0.0.0.0:0"; o.NoTriggers = true })

	if !containsNote(s.Notes(), "all network interfaces") {
		t.Fatalf("notes %v", s.Notes())
	}
}

func TestServe_defaults_should_come_from_settings(t *testing.T) {
	w := demoWorkspace(t)

	o := w.ServeDefaults()

	if o.ReplayAddr != "127.0.0.1:8525" || o.TriggerAddr != "127.0.0.1:8526" || o.ShutdownTimeout != 30*time.Second || o.MaxRuns != 4 || o.TrafficLog != 1000 {
		t.Fatalf("%+v", o)
	}
}

func TestServe_replayer_should_refuse_foreign_Host_headers_on_loopback(t *testing.T) {
	w := demoWorkspace(t)
	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.NoTriggers = true })
	ok, _, _ := call(t, "POST", "http://"+s.ReplayAddr()+"/v1/login", "", "{}")
	req, err := http.NewRequestWithContext(context.Background(), "POST", "http://"+s.ReplayAddr()+"/v1/login", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "evil.example"

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()

	if ok != 200 || res.StatusCode != 403 {
		t.Fatalf("own host %d, foreign host %d", ok, res.StatusCode)
	}
}

func TestServe_should_refuse_cors_together_with_record(t *testing.T) {
	w := demoWorkspace(t)

	_, err := w.Serve(context.Background(), ServeOptions{ReplayAddr: "127.0.0.1:0", CORS: true, Record: "http://example.com"})

	if !errors.Is(err, ErrInvalidServeOptions) {
		t.Fatalf("got %v", err)
	}
}

func TestServe_Given_Valkey_Then_a_run_started_on_one_instance_can_be_read_from_another(t *testing.T) {
	w := demoWorkspace(t)
	token, err := w.NewTriggerToken()
	if err != nil {
		t.Fatal(err)
	}
	kv := sharedtest.Start(t, "")
	withKV := func(o *ServeOptions) { o.NoReplay = true; o.NoSchedules = true; o.ValkeyAddr = kv.Addr }
	a := serveOnFreePorts(t, w, withKV)
	b := serveOnFreePorts(t, w, withKV)

	status, _, answer := call(t, "POST", "http://"+a.TriggerAddr()+"/flows/shared/login/run?wait=30s", token, `{"env": "local", "inputs": {"email": "ada@example.com"}}`)
	var started map[string]any
	_ = json.Unmarshal([]byte(answer), &started)
	id, _ := started["run_id"].(string)
	seen, _, seenBody := call(t, "GET", "http://"+b.TriggerAddr()+"/runs/"+id, token, "")

	if status != 200 || id == "" || seen != 200 || !strings.Contains(seenBody, `"passed"`) {
		t.Fatalf("started %d %s; read on the other instance %d %s", status, answer, seen, seenBody)
	}
	if !containsNote(a.Notes(), "Valkey") {
		t.Fatalf("notes %v", a.Notes())
	}
}

func TestServe_Given_an_unreachable_Valkey_Then_serve_and_runs_work_alone(t *testing.T) {
	w := demoWorkspace(t)
	token, err := w.NewTriggerToken()
	if err != nil {
		t.Fatal(err)
	}
	dead := sharedtest.Start(t, "")
	addr := dead.Addr
	dead.Close()
	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.NoReplay = true; o.NoSchedules = true; o.ValkeyAddr = addr })

	status, _, answer := call(t, "POST", "http://"+s.TriggerAddr()+"/flows/shared/login/run?wait=30s", token, `{"env": "local", "inputs": {"email": "ada@example.com"}}`)

	if status != 200 || !strings.Contains(answer, `"passed"`) {
		t.Fatalf("%d %s", status, answer)
	}
}

func TestServe_should_refuse_a_malformed_Valkey_address(t *testing.T) {
	w := demoWorkspace(t)

	_, err := w.Serve(t.Context(), ServeOptions{ReplayAddr: "127.0.0.1:0", ValkeyAddr: "no-port"})

	if !errors.Is(err, ErrInvalidServeOptions) {
		t.Fatalf("err %v", err)
	}
}

func TestValkeyPassword_should_round_trip_through_the_vault(t *testing.T) {
	w := demoWorkspace(t)

	_, before, _ := w.ValkeyPassword()
	err := w.SetValkeyPassword("pw-1")
	got, after, getErr := w.ValkeyPassword()

	if before || err != nil || getErr != nil || !after || got != "pw-1" {
		t.Fatalf("before=%v err=%v got=%q after=%v/%v", before, err, got, after, getErr)
	}
}

// Behaviour change: record mode saved a response only for a call that matched
// a request defined beforehand, so recording an API meant writing every request
// first. A call that matches none is now saved as a request, in the collection
// named by RecordTo, and its response is saved under it.
func TestServe_Record_should_save_calls_no_request_matches_as_requests(t *testing.T) {
	w := demoWorkspace(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(rw, `{"path": "`+r.URL.Path+`"}`)
	}))
	t.Cleanup(upstream.Close)
	collection := w.Collections()[0]
	s := serveOnFreePorts(t, w, func(o *ServeOptions) {
		o.NoTriggers, o.NoSchedules, o.Record, o.RecordTo = true, true, upstream.URL, string(collection.Manifest.ID)
	})

	status, header, _ := call(t, "GET", "http://"+s.ReplayAddr()+"/v1/pokemon/25?limit=1", "", "", "Authorization", "Bearer sk-live", "X-Flowi-Latency", "1ms")
	again, _, _ := call(t, "GET", "http://"+s.ReplayAddr()+"/v1/pokemon/26", "", "")

	if status != 200 || again != 200 || header.Get("X-Flowi-Match") != "recorded" {
		t.Fatalf("answers %d %d %v", status, again, header)
	}
	root := w.Collections()[0].Root
	files, _ := filepath.Glob(filepath.Join(root, "requests", "recorded", "*", "*.req.yaml"))
	if len(files) != 1 {
		t.Fatalf("one request for both ids, got %v", files)
	}
	text, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "{{id}}") || !strings.Contains(string(text), "{{secret.authorization}}") || strings.Contains(string(text), "sk-live") || strings.Contains(string(text), "X-Flowi") {
		t.Fatalf("the request file:\n%s", text)
	}
}
