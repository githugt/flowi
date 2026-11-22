// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/report"
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

func TestFlowRunDetail_should_return_each_nodes_outputs_and_the_final_variables(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.DataDir(target.Collection), "runs", string(target.Flow))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(filepath.Join(dir, "01A.json"))
	_ = report.WriteJSON(f, report.Run{ID: "01A", Kind: "flow", Status: "passed", Vars: map[string]any{"token": "abc"},
		Nodes: []report.Node{{ID: "login", Type: "request", Status: "passed", Outputs: map[string]any{"status": 200, "body": map[string]any{"ok": true}}}}})
	_ = f.Close()

	// Act
	got, err := w.FlowRunDetail(target, "01A")

	// Assert
	if err != nil || got.Vars["token"] != "abc" {
		t.Fatalf("detail %+v err %v", got, err)
	}
	if body, _ := got.Outputs["login"]["body"].(map[string]any); body["ok"] != true {
		t.Fatalf("outputs %+v", got.Outputs)
	}
}

func TestFlowRunDetail_Given_an_id_that_climbs_out_of_the_runs_folder_Then_refuses_it(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindFlow("user-creation", "")

	for _, id := range []string{"../../flowi", "a/b", "", ".."} {
		if _, err := w.FlowRunDetail(target, id); err == nil {
			t.Errorf("run id %q was accepted", id)
		}
	}
}

func TestDeleteFlowRun_Given_a_saved_run_When_deleted_Then_only_that_run_is_gone(t *testing.T) {
	// Before: saved runs could not be removed from the app; they piled up until the engine pruned the oldest.
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.DataDir(target.Collection), "runs", string(target.Flow))
	saveReport(t, dir, "01A", "passed")
	saveReport(t, dir, "01B", "failed")

	err = w.DeleteFlowRun(target, "01A")
	again := w.DeleteFlowRun(target, "01A")

	got := w.FlowRuns(target, 0)
	if err != nil || again != nil || len(got) != 1 || got[0].ID != "01B" {
		t.Fatalf("err %v again %v runs %+v", err, again, got)
	}
}

func TestDeleteFlowRun_Given_an_id_that_climbs_out_of_the_runs_folder_Then_refuses_it(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}

	err = w.DeleteFlowRun(target, "../../flows/x")

	if err == nil {
		t.Fatal("deleted a path outside the runs folder")
	}
}

func TestClearFlowRuns_Given_saved_runs_and_a_broken_one_When_cleared_Then_every_report_is_gone_and_other_files_stay(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.DataDir(target.Collection), "runs", string(target.Flow))
	saveReport(t, dir, "01A", "passed")
	saveReport(t, dir, "01B", "failed")
	if err := os.WriteFile(filepath.Join(dir, "01C.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := w.ClearFlowRuns(target)

	_, kept := os.Stat(filepath.Join(dir, "notes.txt"))
	if err != nil || n != 3 || len(w.FlowRuns(target, 0)) != 0 || kept != nil {
		t.Fatalf("n %d err %v runs %+v kept %v", n, err, w.FlowRuns(target, 0), kept)
	}
}

func TestClearFlowRuns_Given_a_flow_never_run_Then_nothing_is_removed(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}

	n, err := w.ClearFlowRuns(target)

	if err != nil || n != 0 {
		t.Fatalf("n %d err %v", n, err)
	}
}

func TestFlowRunSteps_Given_a_batch_with_a_step_inside_Then_the_instances_fold_into_one_step_on_the_runs_clock(t *testing.T) {
	// Before: a saved run could only be read as top-level outcomes, so there was no way to show its steps or a timeline.
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.DataDir(target.Collection), "runs", string(target.Flow))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	run := report.Run{ID: "01A", Kind: "flow", Status: "failed", Started: t0, Duration: 3 * time.Second, Nodes: []report.Node{
		{ID: "login", Type: "request", Status: "passed", Started: t0, Duration: 200 * time.Millisecond, Attempts: []report.Attempt{{N: 1, Status: 200}}},
		{ID: "each", Type: "batch", Status: "failed", Started: t0.Add(300 * time.Millisecond), Duration: 2 * time.Second, Lanes: []report.Lane{
			{Label: "1", Nodes: []report.Node{{ID: "pay", Type: "request", Status: "passed", Started: t0.Add(400 * time.Millisecond), Duration: 500 * time.Millisecond, Attempts: []report.Attempt{{N: 1, Status: 201}}}}},
			{Label: "2", Nodes: []report.Node{{ID: "pay", Type: "request", Status: "failed", Started: t0.Add(1 * time.Second), Duration: 700 * time.Millisecond, Attempts: []report.Attempt{{N: 1, Status: 500}}}}},
		}},
	}}
	f, err := os.Create(filepath.Join(dir, "01A.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := report.WriteJSON(f, run); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := w.FlowRunSteps(target, "01A")

	if err != nil || len(got.Steps) != 3 || got.Status != "failed" {
		t.Fatalf("err %v steps %+v", err, got)
	}
	login, each, pay := got.Steps[0], got.Steps[1], got.Steps[2]
	if login.ID != "login" || login.Code != 200 || login.Start != 0 || login.End != 200*time.Millisecond {
		t.Errorf("login %+v", login)
	}
	if each.ID != "each" || each.Start != 300*time.Millisecond || each.End != 2300*time.Millisecond {
		t.Errorf("each %+v", each)
	}
	if pay.ID != "pay" || pay.Instances != 2 || pay.Status != "failed" || pay.Code != 500 || pay.Start != 400*time.Millisecond || pay.End != 1700*time.Millisecond {
		t.Errorf("pay %+v", pay)
	}
}

func TestFlowRunSteps_Given_an_id_that_climbs_out_of_the_runs_folder_Then_refuses_it(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.FlowRunSteps(target, "../x")

	if err == nil {
		t.Fatal("read a path outside the runs folder")
	}
}
