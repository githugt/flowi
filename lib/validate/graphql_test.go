// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestGraphQL_should_report_what_is_wrong_with_a_body(t *testing.T) {
	with := func(change func(*model.RequestVersion, *model.GraphQL)) model.RequestVersion {
		g := model.GraphQL{Query: "query Q($id: ID) { user(id: $id) { name } }", Variables: `{"id": "{{var.id}}"}`}
		v := model.RequestVersion{Label: "1", Method: "POST", URL: "http://x/graphql"}
		change(&v, &g)
		v.Body.GraphQL = &g
		return v
	}
	tests := []struct {
		name string
		v    model.RequestVersion
		want diag.Code // "" → no diagnostic
		path string
	}{
		{name: "valid", v: with(func(*model.RequestVersion, *model.GraphQL) {})},
		{name: "get_query", v: with(func(v *model.RequestVersion, _ *model.GraphQL) { v.Method = "GET" })},
		{name: "no_method", v: with(func(v *model.RequestVersion, _ *model.GraphQL) { v.Method = "" })},
		{name: "empty_query", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Query = " " }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.query"},
		{name: "no_operation", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Query = "fragment F on T { a }" }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.query"},
		{name: "two_operations_no_name", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Query = "query A { a } query B { b }" }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.query"},
		{name: "two_operations_named", v: with(func(_ *model.RequestVersion, g *model.GraphQL) {
			g.Query, g.Operation = "query A { a } query B { b }", "B"
		})},
		{name: "unknown_operation", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Operation = "Nope" }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.operation"},
		{name: "template_in_the_query", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Query = `{ user(id: "{{var.id}}") { name } }` }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.query"},
		{name: "variables_not_json", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Variables = `{"id": }` }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.variables"},
		{name: "variables_an_array", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Variables = `[1]` }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.variables"},
		{name: "unknown_template_in_variables", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Variables = `{"id": "{{"}` }), want: diag.CodeTemplateSyntax, path: "$.versions.1.body.graphql.variables"},
		// Before: every subscription was a problem, since nothing could send one.
		{name: "subscription", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Query = "subscription S { a }" })},
		{name: "subscription_with_receive", v: with(func(_ *model.RequestVersion, g *model.GraphQL) {
			g.Query, g.Receive = "subscription S { a }", &model.AsyncReceive{Count: 3, Timeout: 5_000_000_000}
		})},
		{name: "subscription_with_negative_count", v: with(func(_ *model.RequestVersion, g *model.GraphQL) {
			g.Query, g.Receive = "subscription S { a }", &model.AsyncReceive{Count: -1}
		}), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.receive.count"},
		{name: "receive_on_a_query", v: with(func(_ *model.RequestVersion, g *model.GraphQL) { g.Receive = &model.AsyncReceive{Count: 2} }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql.receive"},
		{name: "subscription_with_auth", v: with(func(v *model.RequestVersion, g *model.GraphQL) {
			g.Query, v.Auth = "subscription S { a }", &model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "name", Value: "X-Key"}, {Key: "value", Value: "{{secret.key}}"}}}
		}), want: diag.CodeGraphQLRequest, path: "$.versions.1.auth"},
		{name: "subscription_with_get", v: with(func(v *model.RequestVersion, g *model.GraphQL) { v.Method, g.Query = "GET", "subscription S { a }" }), want: diag.CodeGraphQLRequest, path: "$.versions.1.method"},
		{name: "mutation_with_get", v: with(func(v *model.RequestVersion, g *model.GraphQL) { v.Method, g.Query = "GET", "mutation M { m }" }), want: diag.CodeGraphQLRequest, path: "$.versions.1.method"},
		{name: "put", v: with(func(v *model.RequestVersion, _ *model.GraphQL) { v.Method = "PUT" }), want: diag.CodeGraphQLRequest, path: "$.versions.1.method"},
		{name: "also_grpc", v: with(func(v *model.RequestVersion, _ *model.GraphQL) { v.GRPC = &model.GRPC{} }), want: diag.CodeGraphQLRequest, path: "$.versions.1.body.graphql"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			c.Requests = append(c.Requests, model.Request{
				Meta: model.Meta{ID: "01J9ZA3M4N5P6Q7R8S9T0VAAAA", Name: "r"}, Path: "r", File: "r.req.yaml", Versions: []model.RequestVersion{tt.v},
			})

			got := Validate(Target{Collections: []model.Collection{c}})

			if tt.want == "" {
				for _, d := range got {
					t.Fatalf("unexpected %v", d)
				}
				return
			}
			if d := only(t, got, tt.want); d.Path != tt.path {
				t.Fatalf("path %q, want %q", d.Path, tt.path)
			}
		})
	}
}
