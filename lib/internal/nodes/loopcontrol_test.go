// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/diag"
)

func TestLoopControl_should_end_the_item_by_default_and_leave_on_request(t *testing.T) {
	tests := []struct {
		name   string
		params []any
		want   string // what the runtime was asked, then what the log says
		log    string
	}{
		{name: "default", params: nil, want: "next", log: "n1: next"},
		{name: "next", params: []any{"action", "next", "message", "item {{1 + 1}}"}, want: "next", log: "n1: next — item 2"},
		{name: "leave", params: []any{"action", "leave", "message", "found it"}, want: "leave", log: "n1: left the loop — found it"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, rec, rt := flowState()

			out := runOutcome(t, node("loop_control", tt.params...), s)

			if len(out.Diags) != 0 || !slices.Equal(rt.ended, []string{tt.want}) || !slices.Equal(rec.logs, []string{tt.log}) {
				t.Fatalf("outcome %+v asked %v logs %v", out, rt.ended, rec.logs)
			}
			if out.Status != "" && out.Status != StatusPassed {
				t.Fatalf("the step itself must pass: %+v", out)
			}
		})
	}
}

func TestLoopControl_Given_no_loop_around_Then_FLW_NODE_013(t *testing.T) {
	// Before: there was no such step; the engine is public, so a flow that skipped validate still fails it.
	s, rec, rt := flowState()
	rt.outsideLoop = true

	out := runOutcome(t, node("loop_control", "action", "leave"), s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeNodeLoopControl || len(rec.logs) != 0 {
		t.Fatalf("outcome %+v logs %v", out, rec.logs)
	}
}

func TestLoopControl_Given_bad_message_template_Then_FLW_TMPL_and_the_loop_goes_on(t *testing.T) {
	s, _, rt := flowState()

	out := runOutcome(t, node("loop_control", "message", "{{ nope }}"), s)

	if !out.Diags.HasErrors() || len(rt.ended) != 0 {
		t.Fatalf("outcome %+v asked %v", out, rt.ended)
	}
}

func TestBatch_should_run_its_lanes_as_loop_turns_and_say_when_one_left(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = func(_ int, _ []Lane) []LaneResult {
		return []LaneResult{{Status: StatusPassed, Left: true}, {Status: StatusSkipped}, {Status: StatusSkipped}}
	}
	n := node("batch", "times", int64(3))
	n.Body = body("a")

	out := runOutcome(t, n, s)

	if !rt.laneOpts[0].Loop || out.Status != "" && out.Status != StatusPassed || out.Outputs["left"] != true || out.Outputs["passed"] != 1 || out.Outputs["count"] != 3 {
		t.Fatalf("opts %+v outcome %+v", rt.laneOpts, out)
	}
}

func TestBatch_Given_nothing_left_Then_left_is_false(t *testing.T) {
	s, _, _ := flowState()
	n := node("batch", "times", int64(2))
	n.Body = body("a")

	out := runOutcome(t, n, s)

	if out.Outputs["left"] != false {
		t.Fatalf("outputs %+v", out.Outputs)
	}
}

func TestRunTill_Given_a_round_that_left_Then_met_and_left_without_asking_until(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = func(call int, _ []Lane) []LaneResult {
		return []LaneResult{{Status: StatusPassed, Left: call == 1}}
	}

	out := runOutcome(t, runTillNode("until", "false", "max", int64(5)), s)

	if len(rt.lanes) != 2 || len(out.Diags) != 0 || out.Outputs["iterations"] != 2 || out.Outputs["met"] != true || out.Outputs["left"] != true || !rt.laneOpts[0].Loop {
		t.Fatalf("lanes %d outcome %+v", len(rt.lanes), out)
	}
}

func TestRunTill_Given_the_turn_around_it_ended_Then_it_starts_no_more_rounds(t *testing.T) {
	s, _, rt := flowState()
	rt.laneReply = readyAfter(100)
	rt.turnEnded = true

	out := runOutcome(t, runTillNode("until", "nodes.status.body.state == 'ready'", "max", int64(5)), s)

	if len(rt.lanes) != 1 || len(out.Diags) != 0 || out.Outputs["met"] != false {
		t.Fatalf("lanes %d outcome %+v", len(rt.lanes), out)
	}
}
