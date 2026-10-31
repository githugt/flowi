// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Pair"}, Path: "pair", File: "/c/flows/pair.arazzo.yaml", Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1", Path: "auth/login"}}},
		{ID: "verify", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R2", Path: "auth/verify"}}},
	}}}
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	return app, b
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
		t.Fatalf("calls %v: the file's text should reach the backend", b.Calls)
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
		t.Fatalf("calls %v", b.Calls)
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

	_, err := flows.Sequence(v.Node)

	if err == nil || !strings.Contains(err.Error(), "run") {
		t.Fatalf("err %v", err)
	}
}
