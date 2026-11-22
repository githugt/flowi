// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// skipFixture is a flow where step b reads the variable step a writes.
func skipFixture(t *testing.T, events []engine.Event) (*vm.FlowRun, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b, f := flowFixture()
	f.Nodes[1].Params = []model.Field{{Key: "value", Value: "{{vars.x}}"}}
	b.Cols[0].Flows = []model.Flow{f}
	b.NextRun = &vmtest.Running{EventList: events, Result: engine.RunResult{Status: engine.StatusPassed, Report: report.Run{Vars: map[string]any{"x": "1"}}}}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()
	return vm.NewFlowRun(b, w, d), b, w
}

func toggleSkip(t *testing.T, r *vm.FlowRun, step string) {
	t.Helper()
	if err := r.ToggleSkip(step); err != nil {
		t.Fatal(err)
	}
}

func TestToggleSkip_Given_a_step_read_later_Then_the_problem_list_names_the_reader(t *testing.T) {
	// Arrange
	r, _, _ := skipFixture(t, nil)

	// Act
	toggleSkip(t, r, "a")
	problems := r.SkipProblems()

	// Assert
	if !slices.Equal(r.Skipped(), []string{"a"}) {
		t.Fatalf("skipped %v", r.Skipped())
	}
	if len(problems) != 1 || problems[0].Reader != "b" || problems[0].Skipped != "a" || !problems[0].Required {
		t.Fatalf("problems %+v", problems)
	}
	if !strings.Contains(problems[0].Text, `"b" reads vars.x`) {
		t.Errorf("text %q should say who reads what", problems[0].Text)
	}
}

func TestToggleSkip_Given_a_step_nothing_reads_Then_there_is_no_problem_and_a_second_toggle_unskips(t *testing.T) {
	// Arrange
	r, _, _ := skipFixture(t, nil)

	// Act
	toggleSkip(t, r, "b")
	problems := r.SkipProblems()
	skippedOnce := r.Skipped()
	toggleSkip(t, r, "b")

	// Assert
	if len(problems) != 0 || len(skippedOnce) != 1 || len(r.Skipped()) != 0 {
		t.Fatalf("problems %+v skipped %v then %v", problems, skippedOnce, r.Skipped())
	}
}

func TestRun_Given_a_skip_Then_the_run_input_carries_it(t *testing.T) {
	// Arrange
	r, b, w := skipFixture(t, nil)
	toggleSkip(t, r, "b")

	// Act
	r.Run("staging", nil)
	w.Wait()

	// Assert
	if !slices.Equal(b.LastRunOptions.Skip, []string{"b"}) {
		t.Fatalf("skip %v", b.LastRunOptions.Skip)
	}
}

func TestSetHold_Given_a_last_run_with_the_skipped_steps_values_Then_the_problem_goes_and_the_run_holds_them(t *testing.T) {
	// Arrange
	events := []engine.Event{engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusPassed, Outputs: map[string]any{"x": "1"}}}
	r, b, w := skipFixture(t, events)
	r.Run("staging", nil)
	w.Wait()
	toggleSkip(t, r, "a")
	before := len(r.SkipProblems())

	// Act
	r.SetHold(true)
	after := len(r.SkipProblems())
	r.Run("staging", nil)
	w.Wait()

	// Assert
	if before != 1 || after != 0 {
		t.Fatalf("problems before hold %d, after %d", before, after)
	}
	if b.LastRunOptions.Prior["a"]["x"] != "1" || b.LastRunOptions.PriorVars["x"] != "1" {
		t.Fatalf("prior %v vars %v", b.LastRunOptions.Prior, b.LastRunOptions.PriorVars)
	}
}

func TestSetHold_Given_no_last_run_Then_the_problem_stays(t *testing.T) {
	// Arrange
	r, _, _ := skipFixture(t, nil)
	toggleSkip(t, r, "a")

	// Act
	r.SetHold(true)

	// Assert
	if len(r.SkipProblems()) != 1 {
		t.Fatalf("a hold with nothing to hold should not hide the problem: %+v", r.SkipProblems())
	}
}

func TestToggleSkip_Given_a_skip_When_the_flow_is_saved_Then_the_saved_flow_marks_the_step(t *testing.T) {
	// Before: the skip list lived only in the open flow, so it was gone when the flow was opened again.
	// Arrange
	r, b, w := skipFixture(t, nil)

	// Act
	if err := r.ToggleSkip("b"); err != nil {
		t.Fatal(err)
	}
	r.Doc().Save(false)
	w.Wait()

	// Assert
	saved := b.SavedFlows[len(b.SavedFlows)-1]
	if saved.Nodes[0].Skipped || !saved.Nodes[1].Skipped {
		t.Fatalf("saved skipped marks: %v %v", saved.Nodes[0].Skipped, saved.Nodes[1].Skipped)
	}
}

func TestToggleSkip_Given_a_step_that_is_not_in_the_flow_Then_it_says_so(t *testing.T) {
	// Arrange
	r, _, _ := skipFixture(t, nil)

	// Act
	err := r.ToggleSkip("nope")

	// Assert
	if err == nil || len(r.Skipped()) != 0 {
		t.Fatalf("err %v skipped %v", err, r.Skipped())
	}
}
