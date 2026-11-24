// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func checksApp(t *testing.T) (*bridge.App, *vmtest.Backend) {
	t.Helper()
	app, b, _ := checksAppPublishing(t)
	return app, b
}

func checksAppPublishing(t *testing.T) (*bridge.App, *vmtest.Backend, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Pair"}, Path: "pair", File: "/c/flows/pair.arazzo.yaml", Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1", Path: "auth/login"}}},
		{ID: "verify", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R2", Path: "auth/verify"}}},
	}}}
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	return app, b, pub
}

func TestChecks_Coverage_Then_the_view_has_a_row_for_each_operation_with_its_statuses_and_totals(t *testing.T) {
	// Arrange
	app, b := checksApp(t)
	b.Check.Coverage = analysis.Coverage{Operations: []analysis.OperationCoverage{
		{Request: "pets/getPet", Version: "1", Used: []string{"fetchName"}, Statuses: []analysis.StatusSeen{{Status: 200, Seen: true}, {Status: 404}}, Read: []string{"$.name"}, Unread: []string{"$.age"}},
		{Request: "pets/list", Version: "1"},
	}}

	// Act
	v, err := app.Checks().Coverage("")

	// Assert
	if err != nil || len(v.Operations) != 2 || v.Operations[0].Used[0] != "fetchName" || v.Operations[0].Statuses[1].Seen {
		t.Fatalf("view %+v err %v", v, err)
	}
	if v.OperationsUsed != 1 || v.OperationsTotal != 2 || v.StatusesSeen != 1 || v.StatusesTotal != 2 || v.FieldsRead != 1 || v.FieldsTotal != 2 {
		t.Fatalf("totals %+v", v)
	}
}

func TestChecks_SpecChange_Given_a_file_Then_it_is_read_and_the_breakages_name_the_flow_step_and_read(t *testing.T) {
	// Arrange
	app, b := checksApp(t)
	file := filepath.Join(t.TempDir(), "new.yaml")
	if err := os.WriteFile(file, []byte("openapi: 3.0.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b.Check.Impact = flowi.SpecImpact{Spec: "pets", Breakages: []flowi.Breakage{{
		Request: "pets/getPet", Version: "1",
		Consumer: analysis.Consumer{Flow: "arazzo/fetchName", Node: "fetch", Kind: analysis.KindNode, Reads: []analysis.Read{{Expr: "nodes.fetch.body.name", Reason: "not in the new schema"}}},
	}}}

	// Act
	v, err := app.Checks().SpecChange("C1", "pets", file)

	// Assert
	if err != nil || !v.Broken || len(v.Breakages) != 1 {
		t.Fatalf("view %+v err %v", v, err)
	}
	got := v.Breakages[0]
	if got.Where != "flow arazzo/fetchName › step fetch" || got.Reads[0].Expr != "nodes.fetch.body.name" || got.Reads[0].Why != "not in the new schema" {
		t.Fatalf("breakage %+v", got)
	}
	if !b.Called("ImpactAgainst C1 pets openapi: 3.0.3\n") {
		t.Fatalf("calls %v: the file's text should reach the backend", b.CallLog())
	}
}

func TestChecks_SpecChange_Given_a_file_that_is_missing_Then_it_is_an_error_that_names_it(t *testing.T) {
	app, _ := checksApp(t)

	_, err := app.Checks().SpecChange("C1", "", "/nope/new.yaml")

	if err == nil || !strings.Contains(err.Error(), "/nope/new.yaml") {
		t.Fatalf("err %v", err)
	}
}

// openChecksFlow opens a flow of two request steps over a backend the test can script.
func openChecksFlow(t *testing.T) (*bridge.FlowService, bridge.FlowView, *vmtest.Backend) {
	t.Helper()
	app, b := checksApp(t)
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	return app.Flows(), v, b
}

func TestFlowChecks_Choices_Then_environments_overlays_inputs_and_request_steps_are_listed_for_the_dialogs(t *testing.T) {
	// Arrange
	flows, v, _ := openChecksFlow(t)

	// Act
	got, err := flows.CheckChoices(v.Node)

	// Assert
	if err != nil || len(got.Envs) == 0 || len(got.Steps) != 2 || got.Steps[0] != "login" {
		t.Fatalf("choices %+v err %v", got, err)
	}
}

func TestFlowChecks_Matrix_Then_a_cell_for_each_combination_comes_back_with_its_outcome(t *testing.T) {
	// Arrange
	flows, v, b := openChecksFlow(t)
	b.NextRun = &vmtest.Running{Result: engine.RunResult{Status: engine.StatusPassed}}

	// Act
	got, err := flows.RunMatrix(v.Node, []bridge.MatrixDimView{{Name: "env", Values: []string{"local", "staging"}}}, nil)

	// Assert
	if err != nil || len(got.Cells) != 2 || got.Cells[1].Labels[0] != "staging" || got.Cells[1].Status != "passed" || got.Passed != 2 {
		t.Fatalf("matrix %+v err %v", got, err)
	}
}

func TestFlowChecks_Matrix_Given_a_bad_matrix_Then_it_is_refused_before_anything_runs(t *testing.T) {
	flows, v, b := openChecksFlow(t)

	_, err := flows.RunMatrix(v.Node, []bridge.MatrixDimView{{Name: "colour", Values: []string{"red"}}}, nil)

	if err == nil || len(b.RunOptionsLog) != 0 {
		t.Fatalf("err %v runs %d", err, len(b.RunOptionsLog))
	}
}

func TestFlowChecks_Access_Then_the_table_has_a_row_for_each_step_with_each_identity_and_whether_any_was_exposed(t *testing.T) {
	// Arrange
	flows, v, b := openChecksFlow(t)
	b.Check.Access = flowi.AccessReport{Flow: "pair", Owner: "alice", Steps: []flowi.AccessStep{{ID: "verify", Owner: 200, Attempts: []flowi.AccessAttempt{
		{Identity: "bob", Status: 200, Verdict: flowi.AccessAllowed}, {Identity: "carol", Status: 403, Verdict: flowi.AccessDenied},
	}}}}

	// Act
	got, err := flows.CheckAccess(v.Node, "alice", []string{"bob", "carol"}, nil, nil)

	// Assert
	if err != nil || !got.Exposed || len(got.Steps) != 1 || got.Steps[0].Attempts[0].Verdict != "allowed" || got.Steps[0].Attempts[1].Status != 403 {
		t.Fatalf("access %+v err %v", got, err)
	}
	if !b.Called("CheckAccess alice") {
		t.Fatalf("calls %v", b.CallLog())
	}
}

func TestFlowChecks_Access_Given_no_owner_or_no_other_identity_Then_it_asks_for_them(t *testing.T) {
	flows, v, _ := openChecksFlow(t)

	_, noOwner := flows.CheckAccess(v.Node, "", []string{"bob"}, nil, nil)
	_, noOther := flows.CheckAccess(v.Node, "alice", nil, nil, nil)

	if noOwner == nil || noOther == nil {
		t.Fatalf("owner %v other %v", noOwner, noOther)
	}
}

func TestFlowChecks_Drift_Then_every_difference_is_listed_with_whether_it_fails_the_check(t *testing.T) {
	// Arrange
	flows, v, b := openChecksFlow(t)
	b.Check.Drift = flowi.DriftReport{Flow: "pair", RunStatus: engine.StatusPassed, Requests: []flowi.RequestDrift{
		{Request: "auth/login", Version: "1", Status: 200, Differences: []schemainfer.Difference{
			{Path: "$.id", Kind: schemainfer.DriftChanged, Detail: "was integer, now string"}, {Path: "$.region", Kind: schemainfer.DriftNew},
		}},
		{Request: "auth/verify", Version: "1", Status: 204, NoSchema: true},
	}}

	// Act
	got, err := flows.CheckDrift(v.Node, "local", []string{"missing", "changed"})

	// Assert
	if err != nil || !got.Failing || len(got.Requests) != 2 || !got.Requests[1].NoSchema {
		t.Fatalf("drift %+v err %v", got, err)
	}
	d := got.Requests[0].Differences
	if !d[0].Fails || d[1].Fails || d[0].Detail != "was integer, now string" {
		t.Fatalf("differences %+v", d)
	}
}

func TestFlowChecks_Drift_Given_a_bad_fail_on_Then_it_names_the_value(t *testing.T) {
	flows, v, _ := openChecksFlow(t)

	_, err := flows.CheckDrift(v.Node, "local", []string{"everything"})

	if err == nil || !strings.Contains(err.Error(), "everything") {
		t.Fatalf("err %v", err)
	}
}

func TestFlowChecks_Sequence_Given_no_run_yet_Then_it_says_to_run_the_flow_first(t *testing.T) {
	flows, v, _ := openChecksFlow(t)

	_, err := flows.Sequence(v.Node, false)

	if err == nil || !strings.Contains(err.Error(), "run") {
		t.Fatalf("err %v", err)
	}
}

func openChecksRequest(t *testing.T) (*bridge.RequestService, string, *vmtest.Backend) {
	t.Helper()
	app, b := checksApp(t)
	node, ok := findNode(app.Workspace().Tree().Roots, "request")
	if !ok {
		t.Fatal("no request in the sample collection")
	}
	if _, err := app.Requests().Open(node.ID); err != nil {
		t.Fatal(err)
	}
	return app.Requests(), node.ID, b
}

func TestRequestMatrix_Then_a_cell_for_each_combination_comes_back_with_its_status_and_nothing_is_saved(t *testing.T) {
	// Arrange
	reqs, node, b := openChecksRequest(t)
	b.NextSend = &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusPassed, Record: &engine.Record{Status: 200}}}

	// Act
	got, err := reqs.RunMatrix(node, []bridge.MatrixDimView{{Name: "env", Values: []string{"local", "staging"}}, {Name: "with.id", Values: []string{"1"}}})

	// Assert
	if err != nil || len(got.Cells) != 2 || got.Cells[1].Labels[0] != "staging" || got.Cells[1].Detail != "200" || got.Passed != 2 {
		t.Fatalf("matrix %+v err %v", got, err)
	}
	if len(b.SendOpts) != 2 || b.SendOpts[1].With["id"] != "1" || b.SendOpts[1].Save != "none" {
		t.Fatalf("sends %+v", b.SendOpts)
	}
}

func TestRequestMatrix_Given_a_flow_style_dimension_Then_it_is_refused_before_anything_is_sent(t *testing.T) {
	reqs, node, b := openChecksRequest(t)

	_, err := reqs.RunMatrix(node, []bridge.MatrixDimView{{Name: "input.id", Values: []string{"1"}}})

	if err == nil || len(b.SendOpts) != 0 {
		t.Fatalf("err %v sends %d", err, len(b.SendOpts))
	}
}

func TestRequestMatrix_Choices_Then_environments_and_overlays_are_listed(t *testing.T) {
	reqs, node, _ := openChecksRequest(t)

	got, err := reqs.MatrixChoices(node)

	if err != nil || len(got.Envs) == 0 || got.Names == nil {
		t.Fatalf("choices %+v err %v", got, err)
	}
}

func TestRequestMatrix_Given_it_runs_Then_progress_is_published_for_the_node_and_the_total_is_known(t *testing.T) {
	// Arrange
	app, b, pub := checksAppPublishing(t)
	node, ok := findNode(app.Workspace().Tree().Roots, "request")
	if !ok {
		t.Fatal("no request in the sample collection")
	}
	if _, err := app.Requests().Open(node.ID); err != nil {
		t.Fatal(err)
	}
	b.NextSend = &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusPassed, Record: &engine.Record{Status: 200}}}

	// Act
	got, err := app.Requests().RunMatrix(node.ID, []bridge.MatrixDimView{{Name: "env", Values: []string{"local", "staging", "prod"}}})

	// Assert
	last, _ := pub.last["matrix"].(bridge.MatrixProgressView)
	if err != nil || got.Total != 3 || got.Cancelled || last.Node != node.ID || last.Done != 3 || last.Total != 3 {
		t.Fatalf("matrix %+v err %v last progress %+v", got, err, last)
	}
}

func TestFlowChecks_Matrix_Given_it_is_cancelled_midway_Then_the_cells_that_ran_come_back_marked_cancelled(t *testing.T) {
	// Arrange
	flows, v, b := openChecksFlow(t)
	runs := 0
	b.RunFor = func(flowi.RunOptions) (*vmtest.Running, error) {
		runs++
		if runs == 2 {
			flows.CancelMatrix(v.Node) // the person pressed Cancel while the second cell was running
		}
		return &vmtest.Running{Result: engine.RunResult{Status: engine.StatusPassed}}, nil
	}

	// Act
	got, err := flows.RunMatrix(v.Node, []bridge.MatrixDimView{{Name: "env", Values: []string{"local", "staging", "prod", "dev"}}}, nil)

	// Assert
	if err != nil || !got.Cancelled || got.Total != 4 || len(got.Cells) != 2 || got.Passed != 2 || runs != 2 {
		t.Fatalf("matrix %+v err %v runs %d", got, err, runs)
	}
}

func TestFlowChecks_Matrix_Given_one_is_already_running_Then_a_second_for_the_same_flow_is_refused(t *testing.T) {
	// Arrange
	flows, v, b := openChecksFlow(t)
	var second error
	b.RunFor = func(flowi.RunOptions) (*vmtest.Running, error) {
		_, second = flows.RunMatrix(v.Node, []bridge.MatrixDimView{{Name: "env", Values: []string{"local"}}}, nil)
		return &vmtest.Running{Result: engine.RunResult{Status: engine.StatusPassed}}, nil
	}

	// Act
	_, first := flows.RunMatrix(v.Node, []bridge.MatrixDimView{{Name: "env", Values: []string{"local"}}}, nil)

	// Assert
	if first != nil || second == nil || !strings.Contains(second.Error(), "already running") {
		t.Fatalf("first %v second %v", first, second)
	}
}

func TestFlowChecks_CancelMatrix_Given_none_is_running_Then_nothing_happens(t *testing.T) {
	flows, v, _ := openChecksFlow(t)

	flows.CancelMatrix(v.Node)
}

func driftBackend(b *vmtest.Backend) {
	b.Check.Drift = flowi.DriftReport{Flow: "pair", RunStatus: engine.StatusPassed, Requests: []flowi.RequestDrift{
		{Request: "auth/login", Version: "1", Status: 200, Differences: []schemainfer.Difference{{Path: "$.region", Kind: schemainfer.DriftNew}, {Path: "$.id", Kind: schemainfer.DriftChanged}}},
		{Request: "auth/verify", Version: "1", Status: 200, Differences: []schemainfer.Difference{{Path: "$.x", Kind: schemainfer.DriftNew}}},
	}}
}

func TestFlowChecks_AcceptDrift_Given_one_request_Then_only_it_is_accepted_and_the_check_is_settled_for_it(t *testing.T) {
	// Arrange
	flows, v, b := openChecksFlow(t)
	driftBackend(b)
	b.Check.Accepted = []flowi.SchemaChange{{}}
	if _, err := flows.CheckDrift(v.Node, "local", []string{"missing", "changed"}); err != nil {
		t.Fatal(err)
	}

	// Act
	got, err := flows.AcceptDrift(v.Node, []string{"auth/login"})

	// Assert
	// Before: the drift check was a read-only report; accepting meant leaving it and sending with learning on.
	if err != nil || !b.Called("AcceptDrift auth/login") || b.Called("AcceptDrift auth/login,auth/verify") {
		t.Fatalf("err %v calls %v", err, b.CallLog())
	}
	login, verify := got.Requests[0], got.Requests[1]
	if !login.Accepted || len(login.Differences) != 0 || verify.Accepted || len(verify.Differences) != 1 {
		t.Fatalf("login %+v verify %+v", login, verify)
	}
	if got.Failing || got.Note != "" {
		t.Fatalf("the failing change was accepted, so the check passes: %+v", got)
	}
}

func TestFlowChecks_AcceptDrift_Given_none_listed_Then_all_are_accepted_and_nothing_changing_says_why(t *testing.T) {
	// Arrange
	flows, v, b := openChecksFlow(t)
	driftBackend(b)
	if _, err := flows.CheckDrift(v.Node, "local", nil); err != nil {
		t.Fatal(err)
	}

	// Act
	got, err := flows.AcceptDrift(v.Node, nil)

	// Assert
	if err != nil || !b.Called("AcceptDrift auth/login,auth/verify") || !got.Requests[0].Accepted || !got.Requests[1].Accepted {
		t.Fatalf("err %v calls %v view %+v", err, b.CallLog(), got)
	}
	if !strings.Contains(got.Note, "never rewritten") {
		t.Fatalf("note %q", got.Note)
	}
}

func TestFlowChecks_AcceptDrift_Given_no_check_yet_Then_it_says_to_run_it_first(t *testing.T) {
	flows, v, _ := openChecksFlow(t)

	_, err := flows.AcceptDrift(v.Node, nil)

	if err == nil || !strings.Contains(err.Error(), "run the check first") {
		t.Fatalf("err %v", err)
	}
}

func TestFlowChecks_Drift_Then_the_view_has_the_command_that_runs_the_same_check_in_ci(t *testing.T) {
	// Arrange
	flows, v, b := openChecksFlow(t)
	driftBackend(b)

	// Act
	got, err := flows.CheckDrift(v.Node, "local", []string{"missing", "changed"})

	// Assert
	if err != nil || !strings.HasPrefix(got.CI, "flowi drift pair ") || !strings.Contains(got.CI, "--env local") || !strings.HasSuffix(got.CI, "--fail-on missing,changed") {
		t.Fatalf("ci %q err %v", got.CI, err)
	}
}

func TestFlowChecks_LastAccess_Given_a_kept_check_When_the_dialog_opens_Then_the_table_and_its_time_come_back(t *testing.T) {
	// Before: the results were lost when the dialog closed, and nothing said when the check was last run.
	flows, v, b := openChecksFlow(t)
	at := time.Date(2026, 11, 3, 9, 30, 0, 0, time.Local)
	b.Check.Saved = &flowi.SavedAccess{At: at, Report: flowi.AccessReport{Owner: "alice", Steps: []flowi.AccessStep{{ID: "read", Owner: 200,
		Attempts: []flowi.AccessAttempt{{Identity: "bob", Status: 403, Verdict: flowi.AccessDenied}}}}}}

	got, err := flows.LastAccess(v.Node)

	if err != nil || got.Owner != "alice" || got.Exposed || len(got.Steps) != 1 || got.Checked != "2026-11-03 09:30" {
		t.Fatalf("access %+v err %v", got, err)
	}
}

func TestFlowChecks_LastAccess_Given_no_kept_check_When_asked_Then_the_owner_is_empty_and_the_lists_are_not_null(t *testing.T) {
	flows, v, _ := openChecksFlow(t)

	got, err := flows.LastAccess(v.Node)

	if err != nil || got.Owner != "" || got.Steps == nil || got.Checked != "" {
		t.Fatalf("access %+v err %v", got, err)
	}
}

func TestFlowChecks_Access_When_run_Then_the_view_says_when_it_ran(t *testing.T) {
	flows, v, b := openChecksFlow(t)
	b.Check.Access = flowi.AccessReport{Owner: "alice"}

	got, err := flows.CheckAccess(v.Node, "alice", []string{"bob"}, nil, nil)

	if err != nil || got.Checked == "" {
		t.Fatalf("access %+v err %v", got, err)
	}
}
