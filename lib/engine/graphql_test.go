// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"
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

func nativeGraphQL(g model.GraphQL, mutate func(*model.RequestVersion)) model.RequestVersion {
	v := model.RequestVersion{URL: "{{env.base_url}}/graphql", Body: model.Body{GraphQL: &g}}
	if mutate != nil {
		mutate(&v)
	}
	return v
}

func sendGraphQL(t *testing.T, v model.RequestVersion) SendResult {
	t.Helper()
	f := newFixture(t)
	res, _ := f.send(f.input(f.addRequest("gql", v)))
	return res
}

func echoOf(t *testing.T, res SendResult) map[string]any {
	t.Helper()
	if res.Record == nil {
		t.Fatalf("no record; diags %v", res.Diags)
	}
	data, _ := res.Record.Body.JSON.(map[string]any)["data"].(map[string]any)
	echo, _ := data["echo"].(map[string]any)
	return echo
}

func TestSend_Given_a_graphql_body_Then_it_posts_query_variables_and_operation_name_as_json(t *testing.T) {
	v := nativeGraphQL(model.GraphQL{
		Query: "query GetUser($id: ID!) { user(id: $id) { name } }", Variables: `{"id": "{{id}}"}`, Operation: "GetUser",
	}, nil)
	f := newFixture(t)
	in := f.input(f.addRequest("gql", v))
	in.With = map[string]string{"id": "7"}

	res, _ := f.send(in)

	echo := echoOf(t, res)
	vars, _ := echo["variables"].(map[string]any)
	if res.Status != StatusPassed || res.Record.Request.Method != "POST" || echo["operationName"] != "GetUser" || vars["id"] != "7" ||
		!strings.Contains(echo["query"].(string), "user(id: $id)") {
		t.Fatalf("status %v method %s echo %v diags %v", res.Status, res.Record.Request.Method, echo, res.Diags)
	}
	data := res.Record.Body.JSON.(map[string]any)["data"].(map[string]any)
	if data["contentType"] != "application/json" || !strings.Contains(data["accept"].(string), "application/json") {
		t.Fatalf("headers sent: %v", data)
	}
}

func TestSend_Given_a_graphql_body_with_get_Then_the_query_travels_in_the_url(t *testing.T) {
	v := nativeGraphQL(model.GraphQL{Query: "query Q($a: Int) { x(a: $a) }", Variables: `{"a": 1}`, Operation: "Q"}, func(v *model.RequestVersion) { v.Method = "GET" })

	res := sendGraphQL(t, v)

	echo := echoOf(t, res)
	vars, _ := echo["variables"].(map[string]any)
	if res.Status != StatusPassed || echo["operationName"] != "Q" || fmt.Sprint(vars["a"]) != "1" || echo["query"] != "query Q($a: Int) { x(a: $a) }" {
		t.Fatalf("status %v echo %v diags %v", res.Status, echo, res.Diags)
	}
	if res.Record.Request.Body != nil {
		t.Fatalf("a GET must have no body: %+v", res.Record.Request.Body)
	}
}

func TestSend_Given_a_mutation_with_get_Then_the_request_is_refused(t *testing.T) {
	v := nativeGraphQL(model.GraphQL{Query: "mutation { like(id: 1) }"}, func(v *model.RequestVersion) { v.Method = "GET" })

	res := sendGraphQL(t, v)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeGraphQLRequest) || res.Record != nil {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

// Before: a query with {{ in it was rendered as a template, so a value could change what the document meant.
func TestSend_Given_a_template_in_the_query_Then_it_is_refused_not_rendered(t *testing.T) {
	v := nativeGraphQL(model.GraphQL{Query: `{ user(id: "{{id}}") { name } }`}, nil)

	res := sendGraphQL(t, v)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeGraphQLRequest) {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_variables_that_are_not_an_object_Then_it_says_so_before_sending(t *testing.T) {
	for _, vars := range []string{`[1, 2]`, `{"a": }`, `7`} {
		res := sendGraphQL(t, nativeGraphQL(model.GraphQL{Query: "{ a }", Variables: vars}, nil))

		if res.Status != StatusError || !hasCode(res.Diags, diag.CodeGraphQLRequest) {
			t.Errorf("variables %q: status %v diags %v", vars, res.Status, res.Diags)
		}
	}
}

// Before: a GraphQL request that answered 200 with an errors list was a pass, so a broken query looked fine.
func TestSend_Given_a_graphql_body_and_an_errors_list_Then_the_step_fails_by_default(t *testing.T) {
	res := sendGraphQL(t, nativeGraphQL(model.GraphQL{Query: "{ broken }"}, nil))

	if res.Record == nil || res.Record.Status != 200 || res.Status != StatusFailed || !hasCode(res.Diags, diag.CodeRunGraphQLError) {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_a_graphql_get_and_an_errors_list_Then_the_step_fails_too(t *testing.T) {
	res := sendGraphQL(t, nativeGraphQL(model.GraphQL{Query: "{ broken }"}, func(v *model.RequestVersion) { v.Method = "GET" }))

	if res.Status != StatusFailed || !hasCode(res.Diags, diag.CodeRunGraphQLError) {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_a_graphql_body_and_a_check_on_errors_Then_expecting_them_passes(t *testing.T) {
	v := nativeGraphQL(model.GraphQL{Query: "{ broken }"}, func(v *model.RequestVersion) {
		v.Assert = []string{"response.body.errors[0].message != ''"}
	})

	res := sendGraphQL(t, v)

	if res.Status != StatusPassed || hasCode(res.Diags, diag.CodeRunGraphQLError) {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_graphql_errors_warn_Then_a_graphql_body_only_warns(t *testing.T) {
	v := nativeGraphQL(model.GraphQL{Query: "{ broken }"}, func(v *model.RequestVersion) {
		v.Settings = model.Settings{"requests.graphql_errors": "warn"}
	})

	res := sendGraphQL(t, v)

	if res.Status != StatusPassed || !hasCode(res.Diags, diag.CodeRunGraphQLWarning) {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_a_graphql_body_Then_extract_and_assert_read_the_data(t *testing.T) {
	v := nativeGraphQL(model.GraphQL{Query: "{ ok }", Variables: `{"n": 3}`}, func(v *model.RequestVersion) {
		v.Extract = []model.Extraction{{Name: "n", From: "body", Path: "$.data.echo.variables.n"}}
		v.Assert = []string{"status == 200", "response.body.data.ok == true"}
	})

	res := sendGraphQL(t, v)

	if res.Status != StatusPassed || fmt.Sprint(res.Record.Outputs["n"]) != "3" {
		t.Fatalf("status %v outputs %v diags %v", res.Status, res.Record.Outputs, res.Diags)
	}
}

func TestPreviewSend_Given_a_graphql_body_Then_it_shows_the_json_body_and_the_variables_with_secrets_masked(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("gql", nativeGraphQL(model.GraphQL{Query: "query { me }", Variables: `{"token": "{{secret.password}}"}`}, nil))

	got := PreviewSend(f.deps(), f.input(r), false)

	m := got.Masked
	if got.Diags.HasErrors() || m.Method != "POST" || m.BodyKind != "text" || strings.Contains(m.Body, secretV) || f.hits.Load() != 0 {
		t.Fatalf("preview %+v diags %v hits %d", m, got.Diags, f.hits.Load())
	}
}
