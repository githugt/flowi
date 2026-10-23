// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func body(ids ...string) []model.Node {
	out := make([]model.Node, len(ids))
	for i, id := range ids {
		out[i] = model.Node{ID: id, Type: "log"}
	}
	return out
}

func TestDelay_should_sleep_fixed_and_seeded_range_durations(t *testing.T) {
	tests := []struct {
		duration string
		random   time.Duration
		want     time.Duration
	}{
		{duration: "2s", want: 2 * time.Second},
		{duration: "{{'150ms'}}", want: 150 * time.Millisecond},
		{duration: "1s..3s", random: 1500 * time.Millisecond, want: 1500 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.duration, func(t *testing.T) {
			s, _, rt := flowState()
			rt.random = tt.random

			out := runOutcome(t, node("delay", "duration", tt.duration), s)

			if len(out.Diags) != 0 || len(rt.sleeps) != 1 || rt.sleeps[0] != tt.want {
				t.Fatalf("diags %v sleeps %v", out.Diags, rt.sleeps)
			}
		})
	}
}

func TestDelay_Given_bad_duration_Then_FLW_NODE_005(t *testing.T) {
	for _, d := range []string{"soon", "3s..1s", ""} {
		s, _, rt := flowState()

		out := runOutcome(t, node("delay", "duration", d), s)

		if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeNodeParam || len(rt.sleeps) != 0 {
			t.Fatalf("%q: diags %v", d, out.Diags)
		}
	}
}

func TestDelay_Given_cancelled_sleep_Then_cancelled(t *testing.T) {
	s, _, rt := flowState()
	rt.sleepErr = errors.New("context canceled")

	out := runOutcome(t, node("delay", "duration", "2s"), s)

	if out.Status != StatusCancelled {
		t.Fatalf("outcome %+v", out)
	}
}

func TestCondition_should_pick_then_else_first_matching_case_or_none(t *testing.T) {
	tests := []struct {
		name       string
		n          model.Node
		wantLane   string
		wantBranch string
	}{
		{name: "then", n: model.Node{ID: "c", Type: "condition", Params: []model.Field{{Key: "check", Value: "vars.n > 1"}}, Then: body("a"), Else: body("b")}, wantLane: "then", wantBranch: "then"},
		{name: "else", n: model.Node{ID: "c", Type: "condition", Params: []model.Field{{Key: "check", Value: "vars.n > 5"}}, Then: body("a"), Else: body("b")}, wantLane: "else", wantBranch: "else"},
		{name: "case", n: model.Node{ID: "c", Type: "condition", Cases: []model.Case{{When: "vars.n == 1", Nodes: body("x")}, {When: "vars.n == 2", Nodes: body("y")}}, Else: body("z")}, wantLane: "case:1", wantBranch: "case:1"},
		{name: "case_default", n: model.Node{ID: "c", Type: "condition", Cases: []model.Case{{When: "vars.n == 9", Nodes: body("x")}}, Else: body("z")}, wantLane: "else", wantBranch: "else"},
		{name: "none", n: model.Node{ID: "c", Type: "condition", Params: []model.Field{{Key: "check", Value: "vars.n > 5"}}, Then: body("a")}, wantBranch: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, rt := flowState()
			s.Vars.Set("n", 2)

			out := runOutcome(t, tt.n, s)

			if len(out.Diags) != 0 || out.Outputs["branch"] != tt.wantBranch {
				t.Fatalf("outcome %+v", out)
			}
			if tt.wantLane == "" {
				if len(rt.lanes) != 0 {
					t.Fatalf("ran lanes %v", rt.lanes)
				}
				return
			}
			l := rt.lanes[0][0]
			if len(rt.lanes) != 1 || l.Label != tt.wantLane || l.OwnVars || l.OwnNodes {
				t.Fatalf("lanes %+v", rt.lanes)
			}
		})
	}
}

func batchNode(params ...any) model.Node {
	n := node("batch", params...)
	n.ID = "users"
	n.Body = body("create")
	return n
}

func TestBatch_should_run_one_isolated_lane_per_item_with_concurrency(t *testing.T) {
	s, _, rt := flowState()
	s.Vars.Set("emails", []any{"a@x", "b@x", "c@x"})

	out := runOutcome(t, batchNode("over", "{{vars.emails}}", "concurrency", int64(2)), s)

	if len(out.Diags) != 0 || len(rt.lanes) != 1 || len(rt.lanes[0]) != 3 {
		t.Fatalf("outcome %+v lanes %v", out, rt.lanes)
	}
	l := rt.lanes[0][1]
	if l.Label != "[1]" || l.Item != "b@x" || l.Index != 1 || !l.HasItem || !l.OwnVars || !l.OwnNodes || len(l.Nodes) != 1 {
		t.Fatalf("lane %+v", l)
	}
	if o := rt.laneOpts[0]; o.Concurrency != 2 || !o.LaneJars {
		t.Fatalf("options %+v", o)
	}
}

func TestBatch_times_should_iterate_indices_with_default_concurrency_1(t *testing.T) {
	s, _, rt := flowState()

	out := runOutcome(t, batchNode("times", int64(3)), s)

	if len(out.Diags) != 0 || len(rt.lanes[0]) != 3 || rt.lanes[0][2].Item != 2 || rt.laneOpts[0].Concurrency != 1 {
		t.Fatalf("outcome %+v lanes %+v opts %+v", out, rt.lanes, rt.laneOpts)
	}
}

func TestBatch_should_assemble_results_in_input_order(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = func(_ int, lanes []Lane) []LaneResult {
		out := make([]LaneResult, len(lanes))
		for i := range lanes {
			out[i] = LaneResult{Status: StatusPassed, Vars: map[string]any{"i": i}, Nodes: map[string]any{"create": map[string]any{"status": 200 + i}}}
		}
		return out
	}

	out := runOutcome(t, batchNode("times", int64(3)), s)

	results, _ := out.Outputs["results"].([]any)
	if len(results) != 3 || out.Outputs["count"] != 3 || out.Outputs["passed"] != 3 || out.Outputs["failed"] != 0 {
		t.Fatalf("outputs %+v", out.Outputs)
	}
	second := results[1].(map[string]any)
	if second["index"] != 1 || second["item"] != 1 || second["status"] != "passed" ||
		fmt.Sprint(second["vars"]) != "map[i:1]" || fmt.Sprint(second["nodes"]) != "map[create:map[status:201]]" {
		t.Fatalf("result %+v", second)
	}
}

func TestBatch_Given_over_not_a_list_Then_FLW_RUN_012(t *testing.T) {
	s, _, rt := flowState()

	out := runOutcome(t, batchNode("over", "{{'nope'}}"), s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunBatchOver || len(rt.lanes) != 0 {
		t.Fatalf("diags %v", out.Diags)
	}
}

func TestBatch_Given_one_failed_iteration_Then_failed_with_counts(t *testing.T) {
	s, _, rt := flowState()
	fail := diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "x", "message": "", "node": "create"})
	rt.laneReply = func(_ int, lanes []Lane) []LaneResult {
		out := make([]LaneResult, len(lanes))
		for i := range out {
			out[i] = LaneResult{Status: StatusPassed}
		}
		out[1] = LaneResult{Status: StatusFailed, Diags: diag.List{fail}}
		out[2] = LaneResult{Status: StatusCancelled}
		return out
	}

	out := runOutcome(t, batchNode("times", int64(3)), s)

	if out.Status != StatusFailed || out.Outputs["passed"] != 1 || out.Outputs["failed"] != 1 || len(out.Diags) != 1 {
		t.Fatalf("outcome %+v", out)
	}
}

func runTillNode(params ...any) model.Node {
	n := node("run_till", params...)
	n.ID = "poll"
	n.Body = body("status")
	return n
}

func readyAfter(n int) func(int, []Lane) []LaneResult {
	return func(call int, _ []Lane) []LaneResult {
		state := "pending"
		if call+1 >= n {
			state = "ready"
		}
		return []LaneResult{{Status: StatusPassed, Nodes: map[string]any{"status": map[string]any{"body": map[string]any{"state": state}}}}}
	}
}

func TestRunTill_should_stop_when_until_holds_and_expose_last(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = readyAfter(3)

	out := runOutcome(t, runTillNode("until", "nodes.status.body.state == 'ready'", "interval", "5s"), s)

	if len(out.Diags) != 0 || out.Outputs["iterations"] != 3 || out.Outputs["met"] != true {
		t.Fatalf("outcome %+v", out)
	}
	if l := rt.lanes[2][0]; l.Label != "[2]" || !l.OwnNodes || l.OwnVars {
		t.Fatalf("lane %+v", l)
	}
	if len(rt.sleeps) != 2 || rt.sleeps[0] != 5*time.Second {
		t.Fatalf("sleeps %v", rt.sleeps)
	}
	if fmt.Sprint(out.Outputs["last"]) != "map[status:map[body:map[state:ready]]]" {
		t.Fatalf("last %v", out.Outputs["last"])
	}
}

func TestRunTill_Given_max_reached_Then_FLW_RUN_008(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = readyAfter(100)

	out := runOutcome(t, runTillNode("until", "nodes.status.body.state == 'ready'", "max", int64(4)), s)

	if len(rt.lanes) != 4 || len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunTillLimit || out.Outputs["met"] != false {
		t.Fatalf("lanes %d outcome %+v", len(rt.lanes), out)
	}
}

func TestRunTill_should_default_max_to_10(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = readyAfter(100)

	runOutcome(t, runTillNode("until", "nodes.status.body.state == 'ready'"), s)

	if len(rt.lanes) != 10 {
		t.Fatalf("lanes %d", len(rt.lanes))
	}
}

func TestRunTill_Given_max_duration_exceeded_Then_FLW_RUN_008(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = readyAfter(100)
	rt.laneStep = 4 * time.Second

	out := runOutcome(t, runTillNode("until", "nodes.status.body.state == 'ready'", "max", int64(50), "max_duration", "10s"), s)

	if len(rt.lanes) != 3 || len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunTillLimit {
		t.Fatalf("lanes %d outcome %+v", len(rt.lanes), out)
	}
}

func TestRunTill_Given_failed_iteration_Then_stops_and_fails(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = func(int, []Lane) []LaneResult { return []LaneResult{{Status: StatusError}} }

	out := runOutcome(t, runTillNode("until", "false"), s)

	if len(rt.lanes) != 1 || out.Status != StatusError {
		t.Fatalf("lanes %d outcome %+v", len(rt.lanes), out)
	}
}

func TestRunTill_Given_cancelled_interval_Then_cancelled(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = readyAfter(100)
	rt.sleepErr = errors.New("context canceled")

	out := runOutcome(t, runTillNode("until", "false", "interval", "1s"), s)

	if len(rt.lanes) != 1 || out.Status != StatusCancelled {
		t.Fatalf("lanes %d outcome %+v", len(rt.lanes), out)
	}
}

func TestParallel_should_pass_wait_and_report_branch_statuses_and_winner(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = func(_ int, _ []Lane) []LaneResult {
		return []LaneResult{{Status: StatusCancelled}, {Status: StatusPassed, First: true}}
	}
	n := model.Node{ID: "both", Type: "parallel", Params: []model.Field{{Key: "wait", Value: "first-success"}},
		Branches: []model.Branch{{ID: "slow", Nodes: body("a")}, {ID: "fast", Nodes: body("b")}}}

	out := runOutcome(t, n, s)

	if out.Status != StatusPassed || out.Outputs["winner"] != "fast" {
		t.Fatalf("outcome %+v", out)
	}
	branches := out.Outputs["branches"].(map[string]any)
	if fmt.Sprint(branches) != "map[fast:map[status:passed] slow:map[status:cancelled]]" {
		t.Fatalf("branches %v", branches)
	}
	if o := rt.laneOpts[0]; o.Wait != "first-success" || o.Concurrency != 2 || !o.LaneJars || rt.lanes[0][0].Label != "slow" {
		t.Fatalf("options %+v lanes %+v", o, rt.lanes[0])
	}
}

func TestParallel_wait_all_Given_failed_branch_Then_failed(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = func(int, []Lane) []LaneResult {
		return []LaneResult{{Status: StatusFailed}, {Status: StatusCancelled}}
	}
	n := model.Node{ID: "both", Type: "parallel", Branches: []model.Branch{{ID: "a", Nodes: body("a")}, {ID: "b", Nodes: body("b")}}}

	out := runOutcome(t, n, s)

	if out.Status != StatusFailed || out.Outputs["winner"] != nil {
		t.Fatalf("outcome %+v", out)
	}
}

func TestParallel_first_success_Given_no_branch_passed_Then_FLW_RUN_019(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = func(int, []Lane) []LaneResult {
		return []LaneResult{{Status: StatusFailed}, {Status: StatusError}}
	}
	n := model.Node{ID: "both", Type: "parallel", Params: []model.Field{{Key: "wait", Value: "first-success"}},
		Branches: []model.Branch{{ID: "a", Nodes: body("a")}, {ID: "b", Nodes: body("b")}}}

	out := runOutcome(t, n, s)

	if out.Status != StatusFailed || len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunNoBranchOK {
		t.Fatalf("outcome %+v", out)
	}
}

// Final review, Important 6: validate bounds times, but the engine is
// public; a bad value must fail the node, not allocate or panic.
func TestBatch_Given_times_out_of_range_at_run_time_Then_FLW_NODE_005(t *testing.T) {
	for _, times := range []int64{-1, 0, 100001} {
		s, _, rt := flowState()

		out := runOutcome(t, batchNode("times", times), s)

		if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeNodeParam || len(rt.lanes) != 0 {
			t.Fatalf("times %d: %+v", times, out)
		}
	}
}
