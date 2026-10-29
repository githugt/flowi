// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"testing"

	"github.com/githugt/flowi-lib/diag"
)

// schemaState is a state whose runtime answers CheckSchema for step "login" with out.
func schemaState(out SchemaCheck) *State {
	s, _ := newState(PhasePost)
	rt := newFakeRuntime()
	rt.schemaOut = map[string]SchemaCheck{"login": out}
	s.Runtime = rt
	return s
}

func TestAssert_Given_a_body_matching_the_schema_Then_it_passes(t *testing.T) {
	// Arrange
	s := schemaState(SchemaCheck{Found: true})

	// Act
	diags := run(t, node("assert", "schema", "login"), s)

	// Assert
	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
}

func TestAssert_Given_a_body_not_matching_the_schema_Then_it_fails_naming_the_path(t *testing.T) {
	// Arrange
	s := schemaState(SchemaCheck{Found: true, Request: "auth/login", Status: 200, Violations: []SchemaViolation{{Path: "$.data.token", Message: "got number, want string"}}})

	// Act
	diags := run(t, node("assert", "schema", "login"), s)

	// Assert
	if len(diags) != 1 || diags[0].Code != diag.CodeSchemaResponse || diags[0].Args["path"] != "$.data.token" || diags[0].Args["request"] != "auth/login" {
		t.Fatalf("diags %v", diags)
	}
}

func TestAssert_Given_a_mismatch_and_on_fail_warn_Then_it_is_a_warning_only(t *testing.T) {
	// Arrange
	s := schemaState(SchemaCheck{Found: true, Violations: []SchemaViolation{{Path: "$.a", Message: "m"}}})
	rec := s.Emit.(*recorder)

	// Act
	diags := run(t, node("assert", "schema", "login", "on_fail", "warn"), s)

	// Assert
	if len(diags) != 0 || len(rec.warns) != 1 {
		t.Fatalf("diags %v warns %v", diags, rec.warns)
	}
}

func TestAssert_Given_no_schema_kept_for_the_step_Then_it_fails_saying_so(t *testing.T) {
	// Arrange
	s := schemaState(SchemaCheck{})

	// Act
	diags := run(t, node("assert", "schema", "login"), s)

	// Assert
	if len(diags) != 1 || diags[0].Code != diag.CodeRunAssertNoSchema || diags[0].Args["step"] != "login" {
		t.Fatalf("diags %v", diags)
	}
}

func TestAssert_Given_a_check_and_a_schema_Then_both_must_hold(t *testing.T) {
	// Arrange
	s := schemaState(SchemaCheck{Found: true, Violations: []SchemaViolation{{Path: "$.a", Message: "m"}}})
	s.Response = sampleResponse()

	// Act
	diags := run(t, node("assert", "check", "status == 201", "schema", "login"), s)

	// Assert
	if len(diags) != 1 || diags[0].Code != diag.CodeSchemaResponse {
		t.Fatalf("the check holds, the schema does not: %v", diags)
	}
}

func TestAssert_Given_neither_check_nor_schema_Then_it_fails_saying_what_is_needed(t *testing.T) {
	s, _ := newState(PhasePost)

	diags := run(t, node("assert"), s)

	if len(diags) != 1 {
		t.Fatalf("diags %v", diags)
	}
}
