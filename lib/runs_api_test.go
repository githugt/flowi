// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/report"
)

func saveReport(t *testing.T, dir, id string, status string, nodes ...report.Node) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := report.WriteJSON(f, report.Run{ID: id, Kind: "flow", Env: "local", Status: status, Duration: 3 * time.Second, Nodes: nodes}); err != nil {
		t.Fatal(err)
	}
}

func TestFlowRuns_should_list_saved_runs_newest_first_and_skip_unreadable_files(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.DataDir(target.Collection), "runs", string(target.Flow))
	saveReport(t, dir, "01A", "passed", report.Node{ID: "login", Type: "request", Status: "passed", Duration: 120 * time.Millisecond})
	saveReport(t, dir, "01C", "failed", report.Node{ID: "login", Type: "request", Status: "failed"})
	if err := os.WriteFile(filepath.Join(dir, "01B.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := w.FlowRuns(target, 0)

	if len(got) != 2 || got[0].ID != "01C" || got[1].ID != "01A" {
		t.Fatalf("runs %+v", got)
	}
	if n := got[1].Nodes[0]; got[1].Duration != 3*time.Second || n.ID != "login" || n.Duration != 120*time.Millisecond {
		t.Fatalf("summary %+v", got[1])
	}
	if one := w.FlowRuns(target, 1); len(one) != 1 || one[0].ID != "01C" {
		t.Fatalf("limit: %+v", one)
	}
}

func TestFlowRuns_Given_no_runs_Then_empty(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindFlow("user-creation", "")

	if got := w.FlowRuns(target, 0); len(got) != 0 {
		t.Fatalf("runs %+v", got)
	}
}
