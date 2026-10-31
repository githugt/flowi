// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/settings"
)

var updateReference = flag.Bool("update", false, "rewrite docs/reference/telemetry.md") //nolint:gochecknoglobals // the standard golden-file flag; flags must be package-level to register before the test runs

func testClient() Client { return Client{App: "cli", Version: "1.2.3", OS: "linux", Arch: "arm64"} }

func fixedNow() time.Time { return time.Date(2026, 10, 9, 23, 59, 0, 0, time.UTC) }

func newCollector(level Level) *Collector { return NewCollector(level, testClient(), fixedNow) }

func TestCollector_Given_off_Then_nothing_to_send(t *testing.T) {
	c := newCollector(LevelOff)
	c.Started()
	_ = c.Record(EventCommandRun, map[string]string{"command": "send", "exit": "ok"})

	_, ok := c.Payload()

	if ok {
		t.Fatal("payload built while telemetry is off")
	}
}

func TestCollector_Given_minimal_Then_only_the_start_event_and_client(t *testing.T) {
	c := newCollector(LevelMinimal)
	c.Started()
	_ = c.Record(EventCommandRun, map[string]string{"command": "send", "exit": "ok"})

	p, ok := c.Payload()

	if !ok || p.Level != "minimal" || p.Date != "2026-10-09" || p.Client != testClient() {
		t.Fatalf("payload %+v", p)
	}
	if len(p.Events) != 1 || p.Events[0].Name != EventAppStarted {
		t.Fatalf("events %+v", p.Events)
	}
}

func TestCollector_Given_full_Then_command_and_run_events_are_included(t *testing.T) {
	c := newCollector(LevelFull)
	c.Started()
	if err := c.Record(EventCommandRun, map[string]string{"command": "serve", "exit": "cancelled"}); err != nil {
		t.Fatal(err)
	}

	p, _ := c.Payload()

	if len(p.Events) != 2 || p.Events[1].Fields["command"] != "serve" || p.Events[1].Fields["exit"] != "cancelled" {
		t.Fatalf("events %+v", p.Events)
	}
}

func TestCollector_Record_should_refuse_anything_outside_the_catalogue(t *testing.T) {
	tests := map[string]struct {
		name   string
		fields map[string]string
	}{
		"url as command":      {EventCommandRun, map[string]string{"command": "send https://api.example.com/v1/users", "exit": "ok"}},
		"unknown field":       {EventCommandRun, map[string]string{"command": "send", "exit": "ok", "path": "/home/me/secret"}},
		"missing field":       {EventCommandRun, map[string]string{"command": "send"}},
		"unknown event":       {"request_sent", map[string]string{}},
		"bad status":          {EventRunFinished, map[string]string{"kind": "flow", "status": "weird", "duration": "<1s", "nodes": "1-5"}},
		"free text in bucket": {EventRunFinished, map[string]string{"kind": "flow", "status": "passed", "duration": "37s", "nodes": "1-5"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := newCollector(LevelFull)

			err := c.Record(tt.name, tt.fields)

			if !errors.Is(err, ErrNotInCatalogue) {
				t.Fatalf("err %v", err)
			}
			if p, _ := c.Payload(); len(p.Events) != 0 {
				t.Fatalf("event kept: %+v", p.Events)
			}
		})
	}
}

func TestCollector_Observe_should_report_a_finished_run_in_buckets(t *testing.T) {
	c := newCollector(LevelFull)

	c.Observe(engine.RunStarted{RunID: "R", Kind: "flow", FlowPath: "secret-flow-name", Env: "prod-eu"})
	for range 7 {
		c.Observe(engine.NodeFinished{RunID: "R", Path: "n", Node: "n", Type: "request"})
	}
	c.Observe(engine.RunFinished{RunID: "R", Status: engine.StatusFailed, Duration: 4 * time.Second})

	p, _ := c.Payload()
	if len(p.Events) != 1 {
		t.Fatalf("events %+v", p.Events)
	}
	got := p.Events[0]
	want := map[string]string{"kind": "flow", "status": "failed", "duration": "<10s", "nodes": "6-20"}
	for k, v := range want {
		if got.Fields[k] != v {
			t.Errorf("%s = %q, want %q", k, got.Fields[k], v)
		}
	}
	data, _ := json.Marshal(p)
	if strings.Contains(string(data), "secret-flow-name") || strings.Contains(string(data), "prod-eu") {
		t.Fatalf("names leaked: %s", data)
	}
}

func TestBuckets(t *testing.T) {
	durations := map[time.Duration]string{0: "<1s", 999 * time.Millisecond: "<1s", time.Second: "<10s", 9 * time.Second: "<10s", 10 * time.Second: "<1m", 59 * time.Second: "<1m", time.Minute: "<10m", 10 * time.Minute: ">=10m"}
	for d, want := range durations {
		if got := durationBucket(d); got != want {
			t.Errorf("durationBucket(%v) = %q, want %q", d, got, want)
		}
	}
	counts := map[int]string{0: "0", 1: "1-5", 5: "1-5", 6: "6-20", 20: "6-20", 21: "21+", 5000: "21+"}
	for n, want := range counts {
		if got := countBucket(n); got != want {
			t.Errorf("countBucket(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]Level{"off": LevelOff, "minimal": LevelMinimal, "full": LevelFull, "": LevelOff, "garbage": LevelOff} {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestClient_should_replace_odd_values_with_safe_ones(t *testing.T) {
	c := Client{App: "cli", Version: "v1 /home/me", OS: "plan9", Arch: "mips"}.Sanitised()

	if c.Version != "unknown" || c.OS != "other" || c.Arch != "other" {
		t.Fatalf("client %+v", c)
	}
}

func TestSample_should_show_every_event_of_the_level_with_example_values(t *testing.T) {
	minimal, full := Sample(LevelMinimal, testClient()), Sample(LevelFull, testClient())

	if len(minimal.Events) != 1 || len(full.Events) != len(Catalogue()) {
		t.Fatalf("minimal %d full %d events", len(minimal.Events), len(full.Events))
	}
	if err := full.Validate(); err != nil {
		t.Fatalf("sample is not valid: %v", err)
	}
}

func TestSender_should_post_the_payload_as_json(t *testing.T) {
	var gotBody []byte
	var gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	c := newCollector(LevelMinimal)
	c.Started()
	p, _ := c.Payload()

	err := NewSender(srv.URL, nil).Send(context.Background(), p)

	if err != nil || gotType != "application/json" || !strings.Contains(string(gotBody), `"app_started"`) {
		t.Fatalf("err %v type %q body %s", err, gotType, gotBody)
	}
}

func TestSender_Given_no_endpoint_Then_ErrNoEndpoint_and_nothing_sent(t *testing.T) {
	err := NewSender("", nil).Send(context.Background(), Payload{})

	if !errors.Is(err, ErrNoEndpoint) {
		t.Fatalf("err %v", err)
	}
}

func TestSender_Given_server_error_Then_error_mentions_status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer srv.Close()

	err := NewSender(srv.URL, nil).Send(context.Background(), Sample(LevelMinimal, testClient()))

	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("err %v", err)
	}
}

func TestSender_Given_an_invalid_payload_Then_refused_before_sending(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer srv.Close()
	bad := Payload{Level: "full", Client: testClient(), Events: []Event{{Name: EventCommandRun, Fields: map[string]string{"command": "https://x"}}}}

	err := NewSender(srv.URL, nil).Send(context.Background(), bad)

	if !errors.Is(err, ErrNotInCatalogue) || hit {
		t.Fatalf("err %v hit %v", err, hit)
	}
}

func TestReference_should_match_docs_reference_telemetry_md(t *testing.T) {
	const file = "../docs/reference/telemetry.md"
	got := Reference()
	if *updateReference {
		if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(file)

	if err != nil || string(want) != got {
		t.Fatalf("%s is out of date; run go test ./telemetry -run Reference -update", file)
	}
}

func TestCollector_Lower_should_drop_what_the_new_level_does_not_allow(t *testing.T) {
	c := newCollector(LevelFull)
	c.Started()
	_ = c.Record(EventCommandRun, map[string]string{"command": "send", "exit": "ok"})

	c.Lower(LevelMinimal)
	minimal, _ := c.Payload()
	c.Lower(LevelFull) // a level can only go down
	stillMinimal, _ := c.Payload()
	c.Lower(LevelOff)
	_, sendable := c.Payload()

	if len(minimal.Events) != 1 || minimal.Level != "minimal" || len(stillMinimal.Events) != 1 || sendable {
		t.Fatalf("minimal %+v still %+v sendable %v", minimal, stillMinimal, sendable)
	}
}

func TestExplanation_should_name_a_setting_that_exists(t *testing.T) {
	for _, key := range []string{"telemetry_endpoint"} {
		if !strings.Contains(Explanation(), key) {
			t.Errorf("the explanation does not mention %s", key)
		}
		if _, ok := settings.Lookup(settings.Schema(), key); !ok {
			t.Errorf("%s is not a setting", key)
		}
	}
}
