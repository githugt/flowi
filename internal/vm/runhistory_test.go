// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
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
