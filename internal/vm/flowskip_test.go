// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
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

func TestToggleSkip_Given_a_step_read_later_Then_the_problem_list_names_the_reader(t *testing.T) {
	// Arrange
	r, _, _ := skipFixture(t, nil)

	// Act
	r.ToggleSkip("a")
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
	r.ToggleSkip("b")
	problems := r.SkipProblems()
	skippedOnce := r.Skipped()
	r.ToggleSkip("b")

	// Assert
	if len(problems) != 0 || len(skippedOnce) != 1 || len(r.Skipped()) != 0 {
		t.Fatalf("problems %+v skipped %v then %v", problems, skippedOnce, r.Skipped())
	}
}

func TestRun_Given_a_skip_Then_the_run_input_carries_it(t *testing.T) {
	// Arrange
	r, b, w := skipFixture(t, nil)
	r.ToggleSkip("b")

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
	r.ToggleSkip("a")
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
	r.ToggleSkip("a")

	// Act
	r.SetHold(true)

	// Assert
	if len(r.SkipProblems()) != 1 {
		t.Fatalf("a hold with nothing to hold should not hide the problem: %+v", r.SkipProblems())
	}
}
