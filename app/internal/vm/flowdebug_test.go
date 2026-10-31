// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/report"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// debugFixture is the skip fixture's flow (steps a, b, p) whose runs pass and leave x = 1.
func debugFixture(t *testing.T) (*vm.FlowRun, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	r, b, w := skipFixture(t, []engine.Event{engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusPassed, Outputs: map[string]any{"x": "1"}}})
	return r, b, w
}

func onlyOf(b *vmtest.Backend, call int) []string { return b.RunOptionsLog[call].Only }

func TestDebug_Given_a_breakpoint_Then_the_run_goes_up_to_it_and_pauses_before_it(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	r.ToggleBreak("b")

	// Act
	r.Debug("staging", nil)
	w.Wait()

	// Assert
	if !slices.Equal(onlyOf(b, 0), []string{"a"}) || r.Paused() != "b" || r.Running() {
		t.Fatalf("only %v paused %q running %v", onlyOf(b, 0), r.Paused(), r.Running())
	}
}

func TestDebug_Given_a_pause_When_continued_Then_the_rest_runs_from_the_paused_step_and_the_session_ends(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	r.ToggleBreak("b")
	r.Debug("staging", nil)
	w.Wait()

	// Act
	r.Continue()
	w.Wait()

	// Assert
	if len(b.RunOptionsLog) != 2 || !slices.Equal(onlyOf(b, 1), []string{"b", "p"}) || r.Paused() != "" {
		t.Fatalf("runs %d only %v paused %q", len(b.RunOptionsLog), onlyOf(b, 1), r.Paused())
	}
	if b.RunOptionsLog[1].Prior["a"]["x"] != "1" {
		t.Errorf("the second part stands on what the first left: %v", b.RunOptionsLog[1].Prior)
	}
}

func TestDebug_Given_two_breakpoints_Then_each_segment_stops_at_the_next(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	r.ToggleBreak("a")
	r.ToggleBreak("p")

	// Act
	r.Debug("staging", nil)
	w.Wait()
	atStart := r.Paused()
	callsAtStart := len(b.RunOptionsLog)
	r.Continue()
	w.Wait()
	secondStop := r.Paused()
	r.Continue()
	w.Wait()

	// Assert
	if atStart != "a" || callsAtStart != 0 {
		t.Errorf("a breakpoint on the first step pauses before anything runs: paused %q after %d runs", atStart, callsAtStart)
	}
	if secondStop != "p" || !slices.Equal(onlyOf(b, 0), []string{"a", "b"}) || !slices.Equal(onlyOf(b, 1), []string{"p"}) || r.Paused() != "" {
		t.Fatalf("second stop %q, runs %v / %v, paused now %q", secondStop, onlyOf(b, 0), onlyOf(b, 1), r.Paused())
	}
}

func TestStep_Given_a_pause_Then_one_step_runs_and_it_pauses_before_the_next(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	r.ToggleBreak("a")
	r.Debug("staging", nil)
	w.Wait()

	// Act
	r.Step()
	w.Wait()

	// Assert
	if !slices.Equal(onlyOf(b, 0), []string{"a"}) || r.Paused() != "b" {
		t.Fatalf("only %v paused %q", onlyOf(b, 0), r.Paused())
	}
}

func TestDebug_Given_no_breakpoint_Then_it_is_an_ordinary_run(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)

	// Act
	r.Debug("staging", nil)
	w.Wait()

	// Assert
	if len(b.RunOptionsLog) != 1 || b.RunOptionsLog[0].Only != nil || r.Paused() != "" {
		t.Fatalf("runs %+v paused %q", b.RunOptionsLog, r.Paused())
	}
}

func TestDebug_Given_a_segment_that_fails_Then_it_does_not_pause_for_the_next_breakpoint(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	b.NextRun.Result = engine.RunResult{Status: engine.StatusFailed}
	r.ToggleBreak("b")

	// Act
	r.Debug("staging", nil)
	w.Wait()

	// Assert
	if r.Paused() != "" {
		t.Fatalf("a failed run must stop, not pause: %q", r.Paused())
	}
}

func TestSetVar_Given_a_pause_When_a_variable_is_changed_Then_the_next_part_runs_with_the_new_value(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	r.ToggleBreak("b")
	r.Debug("staging", nil)
	w.Wait()

	// Act
	r.SetVar("x", "99")
	r.SetVar("flag", "true")
	r.Continue()
	w.Wait()

	// Assert
	got := b.RunOptionsLog[1].PriorVars
	if got["x"] != float64(99) || got["flag"] != true {
		t.Fatalf("vars %v: a typed value reads as JSON", got)
	}
}

func TestSetOutput_Given_a_pause_When_a_steps_output_is_changed_Then_later_steps_read_the_new_value(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	r.ToggleBreak("b")
	r.Debug("staging", nil)
	w.Wait()

	// Act
	r.SetOutput("a", "x", `"patched"`)
	r.Continue()
	w.Wait()

	// Assert
	if b.RunOptionsLog[1].Prior["a"]["x"] != "patched" {
		t.Fatalf("prior %v", b.RunOptionsLog[1].Prior)
	}
}

func TestStopDebugging_Given_a_pause_Then_the_session_ends_and_continue_does_nothing(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	r.ToggleBreak("b")
	r.Debug("staging", nil)
	w.Wait()

	// Act
	r.StopDebugging()
	r.Continue()
	w.Wait()

	// Assert
	if r.Paused() != "" || len(b.RunOptionsLog) != 1 {
		t.Fatalf("paused %q runs %d", r.Paused(), len(b.RunOptionsLog))
	}
}

func TestToggleBreak_Given_steps_Then_they_are_listed_in_flow_order_and_a_second_toggle_removes(t *testing.T) {
	// Arrange
	r, _, _ := debugFixture(t)

	// Act
	r.ToggleBreak("p")
	r.ToggleBreak("a")
	r.ToggleBreak("ghost")
	order := r.Breaks()
	r.ToggleBreak("p")

	// Assert
	if !slices.Equal(order, []string{"a", "p"}) || !slices.Equal(r.Breaks(), []string{"a"}) {
		t.Fatalf("breaks %v then %v", order, r.Breaks())
	}
}

var _ = report.Run{}

func TestDebug_Given_several_parts_Then_they_all_use_the_seed_of_the_first_so_generated_values_agree(t *testing.T) {
	// Arrange
	r, b, w := debugFixture(t)
	b.NextRun.Result.Seed = 777
	r.ToggleBreak("b")
	r.ToggleBreak("p")

	// Act
	r.Debug("staging", nil)
	w.Wait()
	r.Continue()
	w.Wait()
	r.Continue()
	w.Wait()

	// Assert
	if len(b.RunOptionsLog) != 3 {
		t.Fatalf("runs %d", len(b.RunOptionsLog))
	}
	if b.RunOptionsLog[0].Seed != 0 || b.RunOptionsLog[1].Seed != 777 || b.RunOptionsLog[2].Seed != 777 {
		t.Fatalf("seeds %d %d %d: the first part picks one, the rest repeat it", b.RunOptionsLog[0].Seed, b.RunOptionsLog[1].Seed, b.RunOptionsLog[2].Seed)
	}
}
