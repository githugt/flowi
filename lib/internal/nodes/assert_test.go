// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/template"
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

// problemState is a state whose scope holds what an earlier request step "fail" received.
func problemState(status int, contentType string, body any) *State {
	s, _ := newState(PhasePost)
	base := s.Scope
	s.Scope = func() template.Scope {
		sc := base()
		sc.Values["nodes"] = map[string]any{"fail": map[string]any{
			"status": status, "headers": map[string]any{"Content-Type": contentType}, "body": body,
		}}
		return sc
	}
	return s
}

func TestAssert_Given_a_problem_details_response_When_checking_problem_Then_it_passes(t *testing.T) {
	// Arrange
	s := problemState(404, "application/problem+json", map[string]any{"title": "Not found", "status": 404})

	// Act
	diags := run(t, node("assert", "problem", "fail"), s)

	// Assert
	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
}

func TestAssert_Given_a_plain_json_error_When_checking_problem_Then_it_fails_naming_what_is_wrong(t *testing.T) {
	// Arrange
	s := problemState(400, "application/json", map[string]any{"error": "bad", "status": 500})

	// Act
	diags := run(t, node("assert", "problem", "fail"), s)

	// Assert
	if len(diags) != 1 || diags[0].Code != diag.CodeRunNotProblem || !strings.Contains(diags[0].Args["problems"], "Content-Type") || !strings.Contains(diags[0].Args["problems"], "$.status") {
		t.Fatalf("diags %v", diags)
	}
}

func TestAssert_Given_on_fail_warn_When_the_response_is_not_a_problem_Then_only_a_warning(t *testing.T) {
	// Arrange
	s := problemState(400, "text/plain", "oops")
	rec := s.Emit.(*recorder)

	// Act
	diags := run(t, node("assert", "problem", "fail", "on_fail", "warn"), s)

	// Assert
	if len(diags) != 0 || len(rec.warns) != 1 {
		t.Fatalf("diags %v warns %v", diags, rec.warns)
	}
}

func TestAssert_Given_a_step_that_has_no_response_When_checking_problem_Then_it_says_so(t *testing.T) {
	// Arrange
	s, _ := newState(PhasePost)

	// Act
	diags := run(t, node("assert", "problem", "missing"), s)

	// Assert
	if len(diags) != 1 || diags[0].Code != diag.CodeRunNoResponse {
		t.Fatalf("diags %v", diags)
	}
}

func TestResponse_Values_Given_a_quota_header_Then_ratelimit_is_in_the_scope(t *testing.T) {
	// Arrange
	r := &Response{Status: 429, Header: http.Header{"Ratelimit": {`"default";r=0;t=12`}}}

	// Act
	got := r.Values()

	// Assert
	quota, _ := got["ratelimit"].(map[string]any)
	if quota["remaining"] != 0 || quota["wait"] != 12*time.Second {
		t.Fatalf("ratelimit %v", got["ratelimit"])
	}
	if _, has := (&Response{Status: 200, Header: http.Header{}}).Values()["ratelimit"]; has {
		t.Fatal("a response with no quota header must not carry one")
	}
}
