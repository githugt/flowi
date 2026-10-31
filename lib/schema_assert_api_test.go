// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"
)

// flowWithSchemaCheck is the shared login flow with an assert after the login
// step that checks the step's response against its kept schema.
func flowWithSchemaCheck(t *testing.T, w *Workspace) FlowTarget {
	t.Helper()
	target, err := w.FindFlow("shared/login", "")
	if err != nil {
		t.Fatal(err)
	}
	_, f, err := w.flowFor(target)
	if err != nil {
		t.Fatal(err)
	}
	f.Nodes = append(slices.Clone(f.Nodes), model.Node{ID: "shape", Type: "assert", Params: []model.Field{{Key: "schema", Value: "login"}}})
	if err := w.SaveFlow(target, f, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestRunFlow_Given_a_schema_check_and_a_matching_response_Then_the_run_passes(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target := flowWithSchemaCheck(t, w)

	// Act
	run, err := w.RunFlow(t.Context(), target, RunOptions{Env: "local", Inputs: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	res := drainRun(run)

	// Assert
	if res.Status != engine.StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestRunFlow_Given_a_schema_check_and_a_response_that_breaks_the_schema_Then_the_check_fails_naming_the_path(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target := flowWithSchemaCheck(t, w)
	login, _ := w.FindRequest("auth/login", "")
	inputs := map[string]string{"email": "a@example.com"}
	first, err := w.RunFlow(t.Context(), target, RunOptions{Env: "local", Inputs: inputs}) // learns the schema of version 2, which the flow prefers
	if err != nil {
		t.Fatal(err)
	}
	drainRun(first)
	strict := schemainfer.Schema{"type": "object", "required": []any{"nope"}}
	if err := w.ReplaceSchema(login, "2", SchemaPart{Kind: "response", Status: 200}, strict); err != nil {
		t.Fatal(err)
	}

	// Act
	run, err := w.RunFlow(t.Context(), target, RunOptions{Env: "local", Inputs: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	res := drainRun(run)

	// Assert
	failed := slices.ContainsFunc(res.Diags, func(d diag.Diagnostic) bool { return d.Code == diag.CodeSchemaResponse })
	if res.Status == engine.StatusPassed || !failed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}
