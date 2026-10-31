// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func node(id, status string, d time.Duration) flowi.RunNodeSummary {
	return flowi.RunNodeSummary{ID: id, Status: status, Duration: d}
}

func TestCompareRuns_should_line_up_nodes_with_status_changes_and_time_gaps(t *testing.T) {
	before := flowi.RunSummary{Nodes: []flowi.RunNodeSummary{
		node("a", "passed", 100*time.Millisecond), node("b", "passed", 50*time.Millisecond), node("gone", "passed", 0)}}
	after := flowi.RunSummary{Nodes: []flowi.RunNodeSummary{
		node("a", "passed", 100*time.Millisecond), node("b", "failed", 20*time.Millisecond), node("new", "passed", 0)}}

	rows := vm.CompareRuns(before, after)

	want := []vm.DiffRow{
		{Node: "a", Before: "passed", After: "passed"},
		{Node: "b", Before: "passed", After: "failed", DurationGap: "-30ms", Changed: true},
		{Node: "new", After: "passed", Changed: true},
		{Node: "gone", Before: "passed", Changed: true},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows %+v", rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("row %d: got %+v want %+v", i, rows[i], want[i])
		}
	}
}

func TestRunHistory_should_load_newest_first_and_compare_older_runs_with_the_newest(t *testing.T) {
	b, f := flowFixture()
	b.Cols[0].Flows = []model.Flow{f}
	b.RunList = []flowi.RunSummary{
		{ID: "new", Nodes: []flowi.RunNodeSummary{node("a", "failed", 0)}},
		{ID: "old", Nodes: []flowi.RunNodeSummary{node("a", "passed", 0)}},
	}
	w := vmtest.SyncWorker()
	doc := vm.NewFlowDoc(b, w, b.Cols[0], f)
	h := vm.NewRunHistory(b, w, doc)
	changed := 0
	h.OnChange(func() { changed++ })

	h.Load()

	rows := h.CompareWithLatest(1)
	if changed != 1 || len(h.Runs()) != 2 || len(rows) != 1 || !rows[0].Changed || rows[0].Before != "passed" {
		t.Fatalf("changed %d runs %d rows %+v", changed, len(h.Runs()), rows)
	}
	if h.CompareWithLatest(0) != nil || h.CompareWithLatest(5) != nil {
		t.Fatal("the newest run has nothing to compare with")
	}
}

func TestRunHistory_should_compare_any_two_runs_in_depth(t *testing.T) {
	// Arrange
	b, f := flowFixture()
	b.Cols[0].Flows = []model.Flow{f}
	b.RunList = []flowi.RunSummary{
		{ID: "c", Nodes: []flowi.RunNodeSummary{node("a", "passed", 0)}},
		{ID: "b", Nodes: []flowi.RunNodeSummary{node("a", "passed", 0)}},
		{ID: "a", Nodes: []flowi.RunNodeSummary{node("a", "failed", 0)}},
	}
	b.RunDetails = map[string]flowi.RunDetail{
		"a": {Outputs: map[string]map[string]any{"a": {"body": "one"}}, Vars: map[string]any{"n": 1}},
		"b": {Outputs: map[string]map[string]any{"a": {"body": "two"}}, Vars: map[string]any{"n": 2}},
	}
	w := vmtest.SyncWorker()
	h := vm.NewRunHistory(b, w, vm.NewFlowDoc(b, w, b.Cols[0], f))
	h.Load()

	// Act
	rows := h.Between(2, 1)
	var got vm.RunComparison
	var gotErr error
	h.CompareInDepth(2, 1, func(c vm.RunComparison, err error) { got, gotErr = c, err })

	// Assert
	if len(rows) != 1 || rows[0].Before != "failed" || rows[0].After != "passed" {
		t.Fatalf("rows %+v: the earlier run is the one compared from", rows)
	}
	if gotErr != nil || len(got.Nodes["a"]) == 0 || len(got.Vars) != 1 || got.Vars[0].Name != "n" {
		t.Fatalf("comparison %+v err %v", got, gotErr)
	}
}

func TestRunHistory_Given_a_report_that_cannot_be_read_Then_the_comparison_says_so(t *testing.T) {
	b, f := flowFixture()
	b.Cols[0].Flows = []model.Flow{f}
	b.RunList = []flowi.RunSummary{{ID: "x"}, {ID: "y"}}
	w := vmtest.SyncWorker()
	h := vm.NewRunHistory(b, w, vm.NewFlowDoc(b, w, b.Cols[0], f))
	h.Load()
	var gotErr error

	h.CompareInDepth(0, 1, func(_ vm.RunComparison, err error) { gotErr = err })

	if gotErr == nil {
		t.Fatal("a run whose report is gone must be reported, not shown as no difference")
	}
}
