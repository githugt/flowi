// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// Before: a request could not be a GraphQL operation; the body picker offered none, text, form and file.
func TestRequestVM_SetBodyKind_Given_graphql_Then_the_body_is_a_graphql_one_and_the_method_is_post(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.Edit(func(v *model.RequestVersion) { v.Method, v.Body = "", model.Body{} })

	r.SetBodyKind(vm.BodyGraphQL)

	if r.BodyKind() != vm.BodyGraphQL || !r.IsGraphQL() || r.Current().Method != "POST" || !r.Dirty() {
		t.Fatalf("kind %v graphql %v method %q dirty %v", r.BodyKind(), r.IsGraphQL(), r.Current().Method, r.Dirty())
	}
	if vm.TypeOf(r.Current()) != vm.TypeGraphQL || vm.Verb(r.Current()) != vm.GraphQLVerb {
		t.Errorf("type %q verb %q", vm.TypeOf(r.Current()), vm.Verb(r.Current()))
	}
}

func TestRequestVM_SetBodyKind_Given_graphql_When_text_is_picked_again_Then_the_graphql_body_goes(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.SetBodyKind(vm.BodyGraphQL)

	r.SetBodyKind(vm.BodyText)

	if r.IsGraphQL() || r.BodyKind() != vm.BodyText {
		t.Fatalf("graphql %v kind %v", r.IsGraphQL(), r.BodyKind())
	}
}

// Before (the risk this guards): an edit through a shared pointer changed the saved copy too, so Discard did not undo it.
func TestRequestVM_SetGraphQL_Then_it_is_an_edit_that_discard_undoes(t *testing.T) {
	b := sampleBackend()
	b.Cols[0].Requests[0].Versions[0].Body = model.Body{GraphQL: &model.GraphQL{Query: "{ a }"}}
	r, _ := newRequestVM(t, b)

	r.SetGraphQL(vm.GraphQLQuery, "query Q { b }")
	r.SetGraphQL(vm.GraphQLVariables, `{"n": 1}`)
	r.SetGraphQL(vm.GraphQLOperation, " Q ")
	edited := r.GraphQL()
	dirty := r.Dirty()
	r.Revert()

	if edited.Query != "query Q { b }" || edited.Variables != `{"n": 1}` || edited.Operation != "Q" || !dirty {
		t.Errorf("edited %+v dirty %v", edited, dirty)
	}
	if got := r.GraphQL(); got.Query != "{ a }" || got.Variables != "" || r.Dirty() {
		t.Errorf("after discard %+v dirty %v", got, r.Dirty())
	}
}

func TestRequestVM_SetGraphQL_Given_another_body_Then_nothing_changes(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	r.SetGraphQL(vm.GraphQLQuery, "{ a }")

	if r.IsGraphQL() || r.Dirty() {
		t.Fatalf("graphql %v dirty %v", r.IsGraphQL(), r.Dirty())
	}
}

func TestRequestVM_GraphQLInfo_should_say_which_operation_runs_and_what_is_wrong(t *testing.T) {
	tests := []struct {
		name      string
		g         model.GraphQL
		kind      string
		problem   string // a part of the sentence; "" for none
		varsBad   bool
		wantGetOK bool
	}{
		{name: "a query", g: model.GraphQL{Query: "query Q { a }"}, kind: "query", wantGetOK: true},
		{name: "a mutation", g: model.GraphQL{Query: "mutation M { a }"}, kind: "mutation"},
		{name: "a subscription", g: model.GraphQL{Query: "subscription S { a }"}, kind: "subscription"},
		{name: "empty", g: model.GraphQL{}, problem: "Write a query"},
		{name: "two operations and none chosen", g: model.GraphQL{Query: "query A { a } mutation B { b }"}, problem: "several operations"},
		{name: "two operations, one chosen", g: model.GraphQL{Query: "query A { a } mutation B { b }", Operation: "B"}, kind: "mutation"},
		{name: "an operation that is not there", g: model.GraphQL{Query: "query A { a }", Operation: "Z"}, problem: `no operation called "Z"`},
		{name: "a template in the query", g: model.GraphQL{Query: `{ a(id: "{{x}}") }`}, problem: "sent as written"},
		{name: "variables that are not json", g: model.GraphQL{Query: "{ a }", Variables: `{"a": }`}, kind: "query", varsBad: true, wantGetOK: true},
		{name: "variables with a template", g: model.GraphQL{Query: "{ a }", Variables: `{"a": {{x}}}`}, kind: "query", wantGetOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := sampleBackend()
			b.Cols[0].Requests[0].Versions[0].Body = model.Body{GraphQL: &tt.g}
			r, _ := newRequestVM(t, b)

			info := r.GraphQLInfo()

			if info == nil || info.Kind != tt.kind || info.GetOK != tt.wantGetOK || (info.VariablesProblem != "") != tt.varsBad {
				t.Fatalf("info %+v", info)
			}
			if (tt.problem == "") != (info.Problem == "") || !strings.Contains(info.Problem, tt.problem) {
				t.Errorf("problem %q, want one with %q", info.Problem, tt.problem)
			}
		})
	}
}

func TestRequestVM_GraphQLInfo_Given_another_body_Then_there_is_none(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	if r.GraphQLInfo() != nil {
		t.Fatal("an HTTP request has no GraphQL info")
	}
}

// Before: the type menu had no GraphQL, so a request made as HTTP and wanted as GraphQL was made again from nothing.
func TestConvertVersion_Given_an_http_request_with_a_graphql_json_body_When_made_graphql_Then_the_body_becomes_the_operation(t *testing.T) {
	v := httpVersion()
	v.Body = model.Body{JSON: `{"query": "query Q($id: ID) { a }", "variables": {"id": 1}, "operationName": "Q"}`}
	v.Method = "PUT"

	got, lost, err := vm.ConvertVersion(v, vm.TypeGraphQL)

	g := got.Body.GraphQL
	if err != nil || g == nil || g.Query != "query Q($id: ID) { a }" || g.Operation != "Q" || g.Variables != `{"id": 1}` {
		t.Fatalf("err %v body %+v", err, got.Body)
	}
	if got.Method != "POST" || got.URL != v.URL || len(got.Headers) != 1 || len(got.Query) != 1 || got.Expect != "2xx" || len(got.Assert) != 1 {
		t.Errorf("what an HTTP request shares was lost: %+v", got)
	}
	if !slicesContain(lost, "the method PUT") {
		t.Errorf("lost %v", lost)
	}
}

func TestConvertVersion_Given_an_http_request_with_another_body_When_made_graphql_Then_the_body_is_left_behind(t *testing.T) {
	got, lost, err := vm.ConvertVersion(httpVersion(), vm.TypeGraphQL)

	if err != nil || got.Body.GraphQL == nil || !got.Body.GraphQL.IsZero() || !slicesContain(lost, "the body") {
		t.Fatalf("err %v body %+v lost %v", err, got.Body, lost)
	}
}

func TestConvertVersion_Given_a_graphql_request_When_made_http_Then_it_is_a_post_of_the_json_it_is_sent_as(t *testing.T) {
	v := model.RequestVersion{Label: "1", URL: "http://x/graphql", Body: model.Body{GraphQL: &model.GraphQL{Query: "{ a }", Variables: `{"n": 1}`, Operation: "A"}}}

	got, _, err := vm.ConvertVersion(v, vm.TypeHTTP)

	if err != nil || got.Body.GraphQL != nil || got.Method != "POST" || !strings.Contains(got.Body.JSON, `"query": "{ a }"`) || !strings.Contains(got.Body.JSON, `"operationName": "A"`) {
		t.Fatalf("err %v version %+v", err, got)
	}
}

func TestVersionFacts_Given_a_graphql_request_Then_the_card_is_a_graphql_one(t *testing.T) {
	v := model.RequestVersion{URL: "{{env.base_url}}/graphql", Body: model.Body{GraphQL: &model.GraphQL{Query: "query GetUser($id: ID) { a }", Variables: `{"id": 1, "x": 2}`}}}

	f := vm.VersionFacts(v)

	if f.Kind != "graphql" || f.Method != "query" || f.Target != "GetUser" || f.Variables != 2 || f.Verb != "GQL" {
		t.Fatalf("facts %+v", f)
	}
}

func slicesContain(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// Before: a subscription had no way to say how many events to wait for.
func TestRequestVM_SetGraphQLReceive_should_change_what_a_subscription_waits_for_and_revert(t *testing.T) {
	b := sampleBackend()
	b.Cols[0].Requests[0].Versions[0].Body = model.Body{GraphQL: &model.GraphQL{Query: "subscription S { a }"}}
	r, _ := newRequestVM(t, b)

	r.SetGraphQLReceive(&model.AsyncReceive{Count: 3, Match: "body.data.a > 1"})
	got := r.GraphQL().Receive
	dirty := r.Dirty()
	r.Revert()

	if got == nil || got.Count != 3 || got.Match != "body.data.a > 1" || !dirty {
		t.Fatalf("receive %+v dirty %v", got, dirty)
	}
	if r.GraphQL().Receive != nil || r.Dirty() {
		t.Errorf("Revert kept %+v (dirty %v)", r.GraphQL().Receive, r.Dirty())
	}
}

func TestRequestVM_SetGraphQLReceive_Given_another_body_Then_nothing_changes(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	r.SetGraphQLReceive(&model.AsyncReceive{Count: 2})

	if r.IsGraphQL() || r.Dirty() {
		t.Fatalf("graphql %v dirty %v", r.IsGraphQL(), r.Dirty())
	}
}
