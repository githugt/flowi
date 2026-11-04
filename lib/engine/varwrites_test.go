// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// The run says which variables each step wrote and what it wrote, so a later trace can show the value a reader saw.
func TestRunFlow_should_record_the_variables_each_step_wrote(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("writes",
		flowNode("a", "set", "target", "vars.x", "value", "{{1}}"),
		setValues("b", model.SetValue{Name: "y", Value: "{{vars.x + 1}}"}, model.SetValue{Name: "x", Value: "{{vars.x + 10}}"}),
		flowNode("c", "note", "text", "done"))

	events, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	a, _ := finishedNode(events, "a")
	b, _ := finishedNode(events, "b")
	c, _ := finishedNode(events, "c")
	if len(a.Wrote) != 1 || a.Wrote[0].Name != "x" || a.Wrote[0].Value != int64(1) && a.Wrote[0].Value != 1 {
		t.Errorf("a wrote %+v", a.Wrote)
	}
	if len(b.Wrote) != 2 || b.Wrote[0].Name != "y" || b.Wrote[1].Name != "x" {
		t.Errorf("b wrote %+v", b.Wrote)
	}
	if len(c.Wrote) != 0 {
		t.Errorf("c wrote %+v, want nothing", c.Wrote)
	}
	var reported []string
	for _, n := range res.Report.Nodes {
		if n.ID == "b" {
			for _, w := range n.Wrote {
				reported = append(reported, w.Name)
			}
		}
	}
	if len(reported) != 2 {
		t.Errorf("the report lists b's writes as %v", reported)
	}
}
