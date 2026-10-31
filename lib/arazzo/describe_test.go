// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/pb33f/libopenapi"

	"github.com/githugt/flowi/lib/model"
)

type fakeSchemas struct {
	request  map[string]any
	response map[int]map[string]any
}

func (f fakeSchemas) RequestSchema(model.Request, string) (map[string]any, bool) {
	return f.request, f.request != nil
}

func (f fakeSchemas) ResponseSchemas(model.Request, string) map[int]map[string]any { return f.response }

func describedCollection() model.Collection {
	login := model.Request{Meta: model.Meta{ID: "R1", Name: "Login"}, Path: "auth/login", Default: "1", Versions: []model.RequestVersion{
		{Label: "1", Method: "POST", URL: "{{env.base_url}}/v1/login", Headers: []model.KeyValue{{Key: "Content-Type", Value: "application/json"}, {Key: "X-Trace", Value: "{{secret.trace}}"}},
			Body: model.Body{JSON: `{"email":"{{email}}","password":"{{secret.password}}"}`}},
		{Label: "2", Method: "POST", URL: "{{env.base_url}}/v2/login"},
	}}
	get := model.Request{Meta: model.Meta{ID: "R2", Name: "Get user"}, Path: "users/get", Versions: []model.RequestVersion{
		{Label: "1", Method: "GET", URL: "{{env.base_url}}/v1/users/{{id}}?expand=true", Query: []model.KeyValue{{Key: "fields", Value: "name"}}},
	}}
	clash := model.Request{Meta: model.Meta{ID: "R3", Name: "Auth login"}, Path: "auth_login", Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "{{env.base_url}}/x"}}}
	return model.Collection{Manifest: model.Manifest{Meta: model.Meta{Name: "Shop"}}, Requests: []model.Request{login, get, clash}}
}

func describe(t *testing.T, c model.Collection, s SchemaLookup) (string, map[string]any) {
	t.Helper()
	data, err := Describe(c, s)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return string(data), doc
}

func TestDescribe_should_declare_one_operation_per_request_version_with_stable_ids(t *testing.T) {
	c := describedCollection()

	text, _ := describe(t, c, nil)

	for _, want := range []string{"operationId: auth_login\n", "operationId: auth_login_v2\n", "operationId: users_get\n", "operationId: auth_login_2\n", "operationId: " + BuiltinOperation, "x-flowi-request: R1"} {
		if !strings.Contains(text, want) {
			t.Errorf("description lacks %q:\n%s", want, text)
		}
	}
	// Adding a request after the others must not rename theirs.
	c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "R4"}, Path: "zzz/last", Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "{{env.base_url}}/z"}}})
	again, _ := describe(t, c, nil)
	if !strings.Contains(again, "operationId: auth_login_2\n") || !strings.Contains(again, "operationId: users_get\n") {
		t.Errorf("an id changed when a request was added:\n%s", again)
	}
}

func TestDescribe_should_derive_paths_servers_and_parameters_without_values(t *testing.T) {
	text, doc := describe(t, describedCollection(), nil)

	paths, _ := doc["paths"].(map[string]any)
	for _, want := range []string{"/v1/login", "/v1/users/{id}", "/flowi/custom"} {
		if _, ok := paths[want]; !ok {
			t.Errorf("no path %s in %v", want, keysOf(paths))
		}
	}
	for _, want := range []string{"name: expand", "name: fields", "name: id", "in: path", "name: X-Trace", "base_url"} {
		if !strings.Contains(text, want) {
			t.Errorf("description lacks %q:\n%s", want, text)
		}
	}
	for _, leaked := range []string{"{{secret", "{{email", "password"} {
		if strings.Contains(text, leaked) {
			t.Errorf("description leaks %q:\n%s", leaked, text)
		}
	}
}

func TestDescribe_should_use_stored_schemas_for_bodies_and_responses(t *testing.T) {
	s := fakeSchemas{
		request:  map[string]any{"type": "object", "properties": map[string]any{"email": map[string]any{"type": "string"}}},
		response: map[int]map[string]any{200: {"type": "object", "properties": map[string]any{"token": map[string]any{"type": "string"}}}},
	}

	text, _ := describe(t, describedCollection(), s)

	for _, want := range []string{"requestBody:", "email:", "\"200\":", "token:"} {
		if !strings.Contains(text, want) {
			t.Errorf("description lacks %q:\n%s", want, text)
		}
	}
}

func TestDescribe_Given_no_requests_Then_only_the_builtin_operation(t *testing.T) {
	text, doc := describe(t, model.Collection{Manifest: model.Manifest{Meta: model.Meta{Name: "Empty"}}}, nil)

	paths, _ := doc["paths"].(map[string]any)
	if len(paths) != 1 || !strings.Contains(text, BuiltinOperation) {
		t.Fatalf("paths %v\n%s", keysOf(paths), text)
	}
}

func TestDescribe_should_be_a_document_an_OpenAPI_parser_accepts_and_byte_stable(t *testing.T) {
	c := describedCollection()
	first, _ := Describe(c, nil)
	second, _ := Describe(c, nil)

	if _, err := libopenapi.NewDocument(first); err != nil {
		t.Fatalf("not a valid OpenAPI document: %v\n%s", err, first)
	}
	if string(first) != string(second) {
		t.Fatal("two runs gave different bytes")
	}
}

func TestResolver_should_find_the_operation_of_a_request_call_and_the_request_of_an_operation(t *testing.T) {
	r := NewResolver(describedCollection())

	op, ok := r.OperationOf(model.RequestCall{Request: model.Ref{ID: "R2"}})
	ref, version, found := r.RequestFor("auth_login_v2")
	_, known := r.OperationOf(model.RequestCall{Request: model.Ref{ID: "nope"}})

	if !ok || op.ID != "users_get" || op.Params["id"].In != "path" || op.Params["fields"].In != "query" {
		t.Fatalf("op %+v ok %v", op, ok)
	}
	if !found || ref.ID != "R1" || version != "2" || known {
		t.Fatalf("ref %+v version %q found %v known %v", ref, version, found, known)
	}
}

func keysOf(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
