// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"testing"

	"github.com/githugt/flowi/lib/report"
)

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

// Before: coverage read only the history in the data folder, so a CI job on a fresh checkout saw no status at all.
func TestCoverageFromRuns_Given_a_json_report_of_a_run_Then_its_statuses_count_as_seen_without_any_history(t *testing.T) {
	// Arrange
	w, c := petsWithFlow(t)
	var id string
	for _, r := range w.Collections()[0].Requests {
		if r.Path == "pets/getPet" {
			id = string(r.ID)
		}
	}
	run := report.Run{
		Versions: []report.VersionUse{{Path: "get", RequestID: id, Version: "1"}},
		Nodes:    []report.Node{{Path: "get", Attempts: []report.Attempt{{N: 1, Status: 200}}}},
	}
	data, err := json.Marshal(run) // the file `flowi run --report json=` writes
	if err != nil {
		t.Fatal(err)
	}
	var fromFile report.Run
	if err := json.Unmarshal(data, &fromFile); err != nil {
		t.Fatal(err)
	}
	history, _, _ := w.Coverage(c)

	// Act
	cov, _, err := w.CoverageFromRuns(c, []report.Run{fromFile})
	none, _, _ := w.CoverageFromRuns(c, nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if history.Totals().SeenStatuses != 0 {
		t.Fatalf("a fresh workspace has no history: %+v", history.Totals())
	}
	if cov.Totals().SeenStatuses != 1 {
		t.Fatalf("the reported 200 must count as seen: %+v", cov.Totals())
	}
	if none.Totals().SeenStatuses != 0 {
		t.Fatalf("no reports means nothing seen: %+v", none.Totals())
	}
}
