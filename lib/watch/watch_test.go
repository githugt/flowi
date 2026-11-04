// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"
)

func record(path string, status int, total time.Duration, body any) engine.Record {
	return engine.Record{RequestPath: path, Status: status, Timings: engine.Timings{Total: total}, Body: engine.Body{JSON: body}}
}

func kinds(changes []Change) []string {
	var out []string
	for _, c := range changes {
		out = append(out, c.Request+":"+c.Kind)
	}
	return out
}

func TestCompare_Given_the_first_snapshot_Then_nothing_changed(t *testing.T) {
	cur := Take([]engine.Record{record("a", 200, time.Second, map[string]any{"id": 1})})

	got := Compare(nil, cur)

	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestCompare_should_report_a_status_change_a_new_field_and_a_gone_field(t *testing.T) {
	before := Take([]engine.Record{
		record("a", 200, time.Second, map[string]any{"id": 1, "name": "x"}),
		record("b", 200, time.Second, map[string]any{"ok": true}),
	})
	after := Take([]engine.Record{
		record("a", 200, time.Second, map[string]any{"id": 1, "extra": 2}),
		record("b", 500, time.Second, map[string]any{"ok": true}),
	})

	got := Compare(&before, after)

	want := map[string]bool{"a:shape": true, "b:status": true}
	for _, k := range kinds(got) {
		delete(want, k)
	}
	if len(want) != 0 {
		t.Fatalf("missing %v in %+v", want, got)
	}
}

func TestCompare_should_flag_latency_only_when_it_more_than_triples(t *testing.T) {
	before := Take([]engine.Record{record("a", 200, 100*time.Millisecond, nil)})

	slower := Compare(&before, Take([]engine.Record{record("a", 200, 250*time.Millisecond, nil)}))
	slowest := Compare(&before, Take([]engine.Record{record("a", 200, 400*time.Millisecond, nil)}))

	if len(slower) != 0 || len(slowest) != 1 || slowest[0].Kind != "latency" {
		t.Fatalf("slower %+v, slowest %+v", slower, slowest)
	}
}

func TestCompare_Given_a_step_that_was_not_called_before_Then_it_is_not_a_change(t *testing.T) {
	before := Take([]engine.Record{record("a", 200, time.Second, nil)})
	after := Take([]engine.Record{record("a", 200, time.Second, nil), record("new", 200, time.Second, nil)})

	if got := Compare(&before, after); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestTake_should_keep_the_first_call_of_each_request_and_ignore_async_records(t *testing.T) {
	async := engine.Record{RequestPath: "m", Kind: "async"}

	got := Take([]engine.Record{record("a", 200, time.Second, nil), record("a", 404, time.Second, nil), async})

	if len(got.Steps) != 1 || got.Steps[0].Status != 200 {
		t.Fatalf("got %+v", got)
	}
}

func TestAppend_should_keep_only_the_latest_passes_in_order(t *testing.T) {
	var history []Pass
	for i := range 5 {
		history = Append(history, Pass{TotalMS: int64(i)}, 3)
	}

	if len(history) != 3 || history[0].TotalMS != 2 || history[2].TotalMS != 4 {
		t.Fatalf("history %+v: want the last three, oldest first", history)
	}
}

func TestPassOf_should_total_the_latency_and_count_the_changes(t *testing.T) {
	snap := Take([]engine.Record{record("a", 200, 120*time.Millisecond, nil), record("b", 200, 30*time.Millisecond, nil)})
	changes := []Change{{Request: "a", Kind: KindStatus}, {Request: "b", Kind: KindShape}}

	got := PassOf(snap, "passed", changes)

	if got.TotalMS != 150 || got.Changes != 2 || got.Status != "passed" || !got.At.Equal(snap.At) {
		t.Fatalf("pass %+v", got)
	}
}
