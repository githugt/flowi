// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func countFinished(events []Event, id string, status Status) int {
	n := 0
	for _, ev := range events {
		if nf, ok := ev.(NodeFinished); ok && nf.Node == id && nf.Status == status {
			n++
		}
	}
	return n
}

func TestRun_Given_a_skipped_step_Then_it_does_not_execute_and_is_reported_skipped(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("skip",
		setValues("a", model.SetValue{Name: "a", Value: "{{1}}"}),
		setValues("b", model.SetValue{Name: "b", Value: "{{2}}"}),
		flowNode("c", "note", "text", "b={{vars.b ?? 'none'}}"))
	in := f.runInput(fl)
	in.Skip = []string{"b"}

	events, res := runFlow(testCtx(t), t, f.deps(), in)

	b, _ := finishedNode(events, "b")
	c, _ := finishedNode(events, "c")
	if res.Status != StatusPassed || b.Status != StatusSkipped || b.Reason != "skipped by you" || c.Outputs["text"] != "b=none" {
		t.Fatalf("result %+v b %+v c %+v", res, b, c)
	}
	if got := res.Report.Nodes[1]; got.Status != "skipped" || got.Reason != "skipped by you" {
		t.Fatalf("report node %+v", got)
	}
}

func TestRun_Given_hold_mode_and_prior_vars_Then_the_later_step_reads_the_old_value(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("hold",
		setValues("a", model.SetValue{Name: "token", Value: "{{'fresh'}}"}),
		flowNode("c", "note", "text", "token={{vars.token}} out={{nodes.a.token}}"))
	in := f.runInput(fl)
	in.Skip = []string{"a"}
	in.PriorVars = map[string]any{"token": "old"}
	in.Prior = map[string]map[string]any{"a": {"token": "old"}}

	events, res := runFlow(testCtx(t), t, f.deps(), in)

	c, _ := finishedNode(events, "c")
	if res.Status != StatusPassed || c.Outputs["text"] != "token=old out=old" {
		t.Fatalf("result %+v c %+v", res, c)
	}
}

func TestRun_Given_a_skip_inside_a_batch_iteration_Then_only_that_step_is_skipped_in_each_iteration(t *testing.T) {
	f := newFixture(t)
	body := batchOf("each", []model.Field{{Key: "times", Value: int64(2)}},
		setValues("s1", model.SetValue{Name: "x", Value: "{{1}}"}),
		flowNode("n1", "note", "text", "x={{vars.x ?? 'none'}}"))
	fl := f.addFlow("loop", body)
	in := f.runInput(fl)
	in.Skip = []string{"s1"}

	events, res := runFlow(testCtx(t), t, f.deps(), in)

	if res.Status != StatusPassed || countFinished(events, "s1", StatusSkipped) != 2 || countFinished(events, "n1", StatusPassed) != 2 {
		t.Fatalf("result %+v; s1 skipped %d, n1 passed %d", res, countFinished(events, "s1", StatusSkipped), countFinished(events, "n1", StatusPassed))
	}
}
