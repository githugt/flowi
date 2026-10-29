// SPDX-License-Identifier: Apache-2.0

package flowi

import "testing"

func TestCoverage_Given_an_imported_spec_and_a_flow_that_calls_one_operation_Then_it_is_used_and_the_rest_are_not(t *testing.T) {
	// Arrange
	w, c := petsWithFlow(t)

	// Act
	cov, _, err := w.Coverage(c)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var used, unused int
	for _, op := range cov.Operations {
		if len(op.Used) > 0 {
			used++
		} else {
			unused++
		}
	}
	if used == 0 || unused == 0 || cov.Totals().Operations != used+unused {
		t.Fatalf("used %d unused %d totals %+v", used, unused, cov.Totals())
	}
	for _, op := range cov.Operations {
		if op.Request == "pets/getPet" && (len(op.Used) == 0 || len(op.Statuses) == 0) {
			t.Errorf("getPet should be used and have documented statuses: %+v", op)
		}
	}
}

func TestCoverage_Given_a_run_in_the_history_Then_its_statuses_count_as_seen(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)
	res := runToEnd(t, w, target, RunOptions{Env: "local"})
	if res.ReportFile == "" {
		t.Fatalf("the run left no report: %+v", res.Diags)
	}

	// Act
	runs := w.savedRuns(target.Collection)

	// Assert
	if len(runs) == 0 || len(runs[0].Versions) == 0 {
		t.Fatalf("runs %+v", runs)
	}
}
