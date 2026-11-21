// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func graphqlRequest(query string, mutate func(*model.RequestVersion)) model.RequestVersion {
	v := model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/graphql", Body: model.Body{JSON: `{"query": "` + query + `", "variables": {}}`}}
	if mutate != nil {
		mutate(&v)
	}
	return v
}

func runGraphQL(t *testing.T, v model.RequestVersion) (Status, []diag.Diagnostic) {
	t.Helper()
	f := newFixture(t)
	r := f.addRequest("gql", v)
	fl := f.addFlow("gql", requestNode("call", r))
	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))
	call, _ := finishedNode(events, "call")
	return res.Status, call.Diags
}

// Before: a GraphQL server answers 200 with the failure in the body, so a step with no assert on it passed.
func TestRequestNode_Given_a_graphql_answer_with_errors_Then_it_warns_by_default_and_the_step_still_passes(t *testing.T) {
	status, diags := runGraphQL(t, graphqlRequest("{ broken }", nil))

	var found diag.Diagnostic
	for _, d := range diags {
		if d.Code == diag.CodeRunGraphQLWarning {
			found = d
		}
	}
	if status != StatusPassed || found.Code == "" || !strings.Contains(found.Args["detail"], `Cannot query field "broken"; second`) || found.Args["count"] != "2 error(s)" {
		t.Fatalf("status %v diags %v", status, diags)
	}
}

func TestRequestNode_Given_graphql_errors_fail_Then_the_step_fails(t *testing.T) {
	status, diags := runGraphQL(t, graphqlRequest("{ broken }", func(v *model.RequestVersion) {
		v.Settings = model.Settings{"requests.graphql_errors": "fail"}
	}))

	if status != StatusFailed || !hasCode(diags, diag.CodeRunGraphQLError) {
		t.Fatalf("status %v diags %v", status, diags)
	}
}

func TestRequestNode_Given_ignore_or_an_assert_on_body_errors_or_a_clean_answer_Then_nothing_is_said(t *testing.T) {
	ignore, d1 := runGraphQL(t, graphqlRequest("{ broken }", func(v *model.RequestVersion) {
		v.Settings = model.Settings{"requests.graphql_errors": "ignore"}
	}))
	expected, d2 := runGraphQL(t, graphqlRequest("{ broken }", func(v *model.RequestVersion) {
		v.Assert = []string{"response.body.errors != empty"}
	}))
	clean, d3 := runGraphQL(t, graphqlRequest("{ ok }", nil))

	for name, diags := range map[string][]diag.Diagnostic{"ignore": d1, "assert": d2, "clean": d3} {
		if hasCode(diags, diag.CodeRunGraphQLWarning) || hasCode(diags, diag.CodeRunGraphQLError) {
			t.Errorf("%s: %v", name, diags)
		}
	}
	if ignore != StatusPassed || expected != StatusPassed || clean != StatusPassed {
		t.Errorf("statuses %v %v %v: %v", ignore, expected, clean, d2)
	}
}

func TestIsGraphQLRequest_should_need_a_json_object_with_a_text_query(t *testing.T) {
	for body, want := range map[string]bool{
		`{"query":"{ a }"}`: true, `{"query":"","variables":{}}`: false, `{"query":5}`: false,
		`{"name":"x"}`: false, `[{"query":"{a}"}]`: false, `not json`: false, ``: false,
	} {
		if got := isGraphQLRequest(body); got != want {
			t.Errorf("%q: %v", body, got)
		}
	}
}
