// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/serve/replay"
)

func TestHandler_should_log_every_call_and_serve_the_log_and_routes(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a/{{id}}"), examples: map[string]string{"ok": ex(200, `{}`), "gone": ex(404, `{}`)}}).materialise()
	log := replay.NewLog(100)
	f.options.Log = log
	f.start()

	f.do("GET", "/a/1")
	f.do("GET", "/a/2", "X-Flowi-Status", "404", "X-Flowi-Latency", "5ms")
	f.do("GET", "/zzz")
	served := f.do("GET", "/__flowi/traffic")
	routes := f.do("GET", "/__flowi/routes")

	entries := log.Entries()
	if len(entries) != 3 {
		t.Fatalf("%d entries: %+v", len(entries), entries)
	}
	if e := entries[1]; e.Method != "GET" || e.Path != "/a/2" || e.Status != 404 || e.Request != "a" || e.Example != "gone" || e.Match != "ok" || e.Latency != 5*time.Millisecond {
		t.Errorf("entry %+v", e)
	}
	if e := entries[2]; e.Status != 404 || e.Match != "none" || e.Request != "" {
		t.Errorf("unmatched entry %+v", e)
	}
	var fromAPI []replay.Entry
	if err := json.Unmarshal([]byte(served.body), &fromAPI); err != nil || len(fromAPI) != 3 {
		t.Errorf("/__flowi/traffic: %v %s", err, served.body)
	}
	var listed []map[string]any
	if err := json.Unmarshal([]byte(routes.body), &listed); err != nil || len(listed) != 1 || listed[0]["pattern"] != "/a/{id}" || listed[0]["method"] != "GET" {
		t.Errorf("/__flowi/routes: %v %s", err, routes.body)
	}
}

func TestLog_should_keep_only_the_newest_entries(t *testing.T) {
	log := replay.NewLog(3)

	for i := range 5 {
		log.Add(replay.Entry{Path: itoa(i)})
	}

	got := log.Entries()
	if len(got) != 3 || got[0].Path != "2" || got[2].Path != "4" || got[2].Seq != 5 {
		t.Fatalf("%+v", got)
	}
}

func TestLog_Subscribe_should_stream_new_entries_and_stop_with_its_context(t *testing.T) {
	log := replay.NewLog(10)
	ctx, cancel := context.WithCancel(context.Background())
	ch := log.Subscribe(ctx)

	log.Add(replay.Entry{Path: "/x"})
	first := <-ch
	cancel()
	for range ch { // drained and closed after cancel
	}

	if first.Path != "/x" {
		t.Fatalf("%+v", first)
	}
}

func TestLog_Subscribe_should_not_block_the_replayer_for_a_slow_reader(t *testing.T) {
	log := replay.NewLog(10)
	_ = log.Subscribe(context.Background()) // never read

	done := make(chan struct{})
	go func() {
		for range 1000 {
			log.Add(replay.Entry{})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Add blocked on a slow subscriber")
	}
}

func TestHandler_Sequence_should_walk_responses_in_order_and_wrap(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"),
		examples: map[string]string{"1-pending": ex(200, `{"state": "pending"}`), "2-done": ex(200, `{"state": "done"}`)}}).materialise()
	f.options.Sequence = true
	f.start()

	var states []any
	for range 3 {
		states = append(states, f.do("GET", "/a").json(t)["state"])
	}
	pinned := f.do("GET", "/a", "X-Flowi-Example", "1-pending")
	afterPin := f.do("GET", "/a").json(t)["state"]
	reset := f.do("POST", "/__flowi/reset")
	afterReset := f.do("GET", "/a").json(t)["state"]

	if states[0] != "pending" || states[1] != "done" || states[2] != "pending" {
		t.Errorf("sequence %v", states)
	}
	if pinned.json(t)["state"] != "pending" || afterPin != "done" {
		t.Errorf("an explicit pick must not advance the sequence: afterPin %v", afterPin)
	}
	if reset.status != 204 || afterReset != "pending" {
		t.Errorf("reset: %d then %v", reset.status, afterReset)
	}
}

func TestHandler_Sequence_should_skip_last_when_per_status_files_exist(t *testing.T) {
	f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"),
		saved: map[string]map[string]string{"1": {
			"200":  `{"status": 200, "body": {"json": {"s": 200}}}`,
			"500":  `{"status": 500, "body": {"json": {"s": 500}}}`,
			"last": `{"status": 500, "body": {"json": {"s": 500}}}`,
		}}}).materialise()
	f.options.Sequence = true
	f.start()

	var statuses []int
	for range 4 {
		statuses = append(statuses, f.do("GET", "/a").status)
	}

	if statuses[0] != 200 || statuses[1] != 500 || statuses[2] != 200 || statuses[3] != 500 {
		t.Fatalf("%v", statuses)
	}
}

// Before: any web page could POST /__flowi/reset on a replayer it could reach
// at its loopback address, wiping the traffic log and sequence positions.
func TestHandler_Given_a_control_call_from_another_origin_Then_refuses_it(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		headers []string
		want    int
	}{
		{"foreign Origin reset", "POST", []string{"Origin", "https://evil.example"}, 403},
		{"own Origin", "GET", []string{"Origin", "http://example.com"}, 200},
		{"cross-site fetch", "GET", []string{"Sec-Fetch-Site", "cross-site"}, 403},
		{"same-origin page", "GET", []string{"Sec-Fetch-Site", "same-origin"}, 200},
		{"no browser headers", "GET", nil, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, spec{path: "a", versions: get("{{env.base_url}}/a"), examples: map[string]string{"ok": ex(200, `{}`)}}).materialise().start()

			got := f.do(tt.method, "/__flowi/traffic", tt.headers...)

			if got.status != tt.want {
				t.Fatalf("status %d, want %d", got.status, tt.want)
			}
		})
	}
}
