// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
)

func TestRun_Given_declared_outputs_Then_the_report_lists_them_in_declared_order_with_values(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("out",
		setValues("a", model.SetValue{Name: "id", Value: "{{42}}"}, model.SetValue{Name: "name", Value: "{{'Ada'}}"}))
	fl.Outputs = []model.Output{
		{Name: "name", Type: "string", Value: "{{vars.name}}"},
		{Name: "id", Type: "number", Value: "{{vars.id}}"},
	}

	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	want := []report.OutputValue{{Name: "name", Type: "string", Value: "Ada"}, {Name: "id", Type: "number", Value: 42}}
	got := res.Report.Outputs
	if res.Status != StatusPassed || len(got) != 2 || got[0] != want[0] || got[1].Name != "id" || got[1].Type != "number" || got[1].Value != 42 {
		t.Fatalf("status %s outputs %+v", res.Status, got)
	}
}

func TestRun_Given_a_failed_run_Then_outputs_that_could_not_be_evaluated_are_absent_not_wrong(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("out",
		setValues("a", model.SetValue{Name: "id", Value: "{{42}}"}),
		flowNode("boom", "assert", "check", "false"))
	fl.Outputs = []model.Output{
		{Name: "id", Value: "{{vars.id}}"},
		{Name: "never", Value: "{{nodes.nobody.x}}"},
	}

	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	got := res.Report.Outputs
	if res.Status != StatusFailed || len(got) != 1 || got[0].Name != "id" {
		t.Fatalf("status %s outputs %+v", res.Status, got)
	}
}

func TestRun_Given_an_output_built_from_a_secret_Then_the_report_masks_it(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("out", flowNode("n", "note", "text", "x"))
	fl.Outputs = []model.Output{{Name: "pw", Value: "pw={{secret.password}}"}}

	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	if len(res.Report.Outputs) != 1 || res.Report.Outputs[0].Value == "pw=hunter2" {
		t.Fatalf("outputs %+v", res.Report.Outputs)
	}
}
