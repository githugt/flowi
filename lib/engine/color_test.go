// SPDX-License-Identifier: Apache-2.0

package engine

import "testing"

func TestRun_Given_a_colour_Then_the_run_is_unchanged(t *testing.T) {
	f := newFixture(t)
	painted := flowNode("a", "note", "text", "hello")
	painted.Color = "amber"
	fl := f.addFlow("paint", painted)

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	a, _ := finishedNode(events, "a")
	if res.Status != StatusPassed || a.Outputs["text"] != "hello" {
		t.Fatalf("result %+v a %+v", res, a)
	}
}
