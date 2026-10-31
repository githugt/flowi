// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func importFile(t *testing.T, name string) (Result, diag.List) {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	return Import(data, Options{NewID: func() model.ID { n++; return model.ID(fmt.Sprintf("ID%024d", n)) }})
}

func request(t *testing.T, r Result, path string) model.Request {
	t.Helper()
	i := slices.IndexFunc(r.Requests, func(x model.Request) bool { return x.Path == path })
	if i < 0 {
		var paths []string
		for _, x := range r.Requests {
			paths = append(paths, x.Path)
		}
		t.Fatalf("no request %s in %v", path, paths)
	}
	return r.Requests[i]
}

func TestImport_should_map_operations_to_requests(t *testing.T) {
	r, diags := importFile(t, "petstore30.yaml")

	list := request(t, r, "pets/listPets")
	v := list.Versions[0]
	if diags.HasErrors() || list.Operation != "listPets" || v.Label != "1" || v.Method != "GET" || v.URL != "{{env.base_url}}/v1/pets" {
		t.Fatalf("diags %v request %+v", diags, list)
	}
	if !slices.Equal(v.Query, []model.KeyValue{{Key: "limit", Value: "{{limit}}"}}) || !slices.Equal(v.Headers, []model.KeyValue{{Key: "X-Trace", Value: "{{X_Trace}}"}}) {
		t.Fatalf("query %v headers %v", v.Query, v.Headers)
	}
	health := request(t, r, "get-health")
	if health.Versions[0].Label != "1" || health.Operation != "" {
		t.Fatalf("health %+v", health)
	}
}

func TestImport_should_group_path_versions_into_one_request(t *testing.T) {
	r, _ := importFile(t, "petstore30.yaml")

	get := request(t, r, "pets/getPet")

	if len(get.Versions) != 2 || get.Versions[0].URL != "{{env.base_url}}/v1/pets/{{petId}}" || get.Versions[1].Label != "2" ||
		get.Versions[1].URL != "{{env.base_url}}/v2/pets/{{petId}}" || get.Operation != "getPet" {
		t.Fatalf("request %+v", get)
	}
}

func TestImport_should_generate_bodies_from_schemas(t *testing.T) {
	r, _ := importFile(t, "petstore30.yaml")

	body := request(t, r, "pets/createPet").Versions[0].Body.JSON

	want := "{\n  \"name\": \"{{name}}\",\n  \"age\": 0,\n  \"tags\": [\n    \"{{tags}}\"\n  ],\n  \"vaccinated\": false\n}"
	if body != want {
		t.Fatalf("body:\n%s\nwant:\n%s", body, want)
	}
}

func TestImport_should_keep_response_schemas_with_refs_inlined_and_cycles_cut(t *testing.T) {
	r, _ := importFile(t, "petstore30.yaml")

	s := r.Schemas["pets/getPet"]["1"]["200"]
	data, _ := json.Marshal(s)

	if s == nil || len(r.Schemas["pets/getPet"]["1"]) != 2 || r.Schemas["pets/getPet"]["2"]["200"] == nil {
		t.Fatalf("schemas %v", r.Schemas["pets/getPet"])
	}
	if string(data) == "" || len(data) > 20000 {
		t.Fatalf("schema size %d", len(data))
	}
	var walk func(v any, depth int) int
	walk = func(v any, depth int) int {
		m, ok := v.(map[string]any)
		if !ok {
			if l, isList := v.([]any); isList {
				d := depth
				for _, x := range l {
					d = max(d, walk(x, depth+1))
				}
				return d
			}
			return depth
		}
		if _, ref := m["$ref"]; ref {
			t.Fatalf("a $ref was left: %v", m)
		}
		d := depth
		for _, x := range m {
			d = max(d, walk(x, depth+1))
		}
		return d
	}
	if depth := walk(s, 0); depth > 40 {
		t.Fatalf("schema depth %d: the cycle was not cut", depth)
	}
}

func TestImport_should_suggest_flows_from_links(t *testing.T) {
	r, _ := importFile(t, "petstore30.yaml")

	i := slices.IndexFunc(r.Flows, func(f model.Flow) bool { return f.Path == "suggested/createPet-to-getPet" })
	if i < 0 {
		t.Fatalf("flows %+v", r.Flows)
	}
	f := r.Flows[i]
	if len(f.Nodes) != 2 || f.Nodes[0].ID != "createPet" || f.Nodes[1].Request == nil ||
		!slices.Equal(f.Nodes[1].Request.With, []model.KeyValue{{Key: "petId", Value: "{{nodes.createPet.body.id}}"}}) {
		t.Fatalf("flow %+v", f.Nodes)
	}
	if f.Nodes[0].Request.Request.Path != "pets/createPet" || f.Nodes[1].Request.Request.Path != "pets/getPet" {
		t.Fatalf("refs %+v %+v", f.Nodes[0].Request.Request, f.Nodes[1].Request.Request)
	}
}

func TestImport_should_read_openapi_31(t *testing.T) {
	r, diags := importFile(t, "petstore31.yaml")

	get := request(t, r, "getOrder")
	if diags.HasErrors() || get.Versions[0].Label != "2" || r.Schemas["getOrder"]["2"]["200"] == nil {
		t.Fatalf("diags %v request %+v", diags, get)
	}
}

func TestImport_Given_invalid_document_Then_FLW_IMPORT_002(t *testing.T) {
	_, diags := Import([]byte("openapi: 3.0.0\npaths: [nope"), Options{NewID: func() model.ID { return "X" }})

	if !diags.HasErrors() || diags[0].Code != diag.CodeImportInvalid {
		t.Fatalf("diags %v", diags)
	}
}

func TestImport_Swagger_2_should_import_like_the_converted_OpenAPI_3_document(t *testing.T) {
	data, err := os.ReadFile("testdata/swagger2.yaml")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	res, diags := Import(data, Options{NewID: func() model.ID { n++; return model.ID(fmt.Sprintf("%026d", n)) }})

	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	byName := map[string]model.Request{}
	for _, r := range res.Requests {
		byName[r.Path] = r
	}
	create, get, put := byName["createExample"], byName["getExample"], byName["put-examples-id"]
	if len(res.Requests) != 3 || create.Versions[0].Method != "POST" || !strings.Contains(create.Versions[0].Body.JSON, `"name"`) {
		t.Fatalf("create %+v of %v", create, byName)
	}
	if get.Versions[0].URL != "{{env.base_url}}/examples/{{id}}" || len(get.Versions[0].Query) != 1 {
		t.Fatalf("get %+v", get.Versions[0])
	}
	if len(put.Versions[0].Body.Form) != 1 || put.Versions[0].Body.Form[0].Key != "name" {
		t.Fatalf("put body %+v", put.Versions[0].Body)
	}
	if res.Schemas["getExample"]["1"]["200"] == nil {
		t.Fatalf("schemas %v", res.Schemas)
	}
}

func TestImport_should_read_the_media_type_example_and_the_description(t *testing.T) {
	doc := `openapi: 3.0.3
info: {title: E, version: "1.0"}
paths:
  /pets:
    post:
      operationId: addPet
      description: Adds a pet.
      requestBody:
        content:
          application/json:
            schema: {type: object, properties: {name: {type: string}}}
            example: {name: Rex}
      responses: {"200": {description: ok}}
`

	r, diags := Import([]byte(doc), Options{NewID: func() model.ID { return "ID000000000000000000000001" }})

	v := request(t, r, "addPet").Versions[0]
	if diags.HasErrors() || v.Docs != "Adds a pet." || !strings.Contains(v.Body.JSON, `"name": "Rex"`) {
		t.Fatalf("diags %v docs %q body %s", diags, v.Docs, v.Body.JSON)
	}
}

func TestOperationsOf_should_name_operations_as_Import_names_requests(t *testing.T) {
	data, err := os.ReadFile("testdata/petstore30.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := importFile(t, "petstore30.yaml")

	ops, err := OperationsOf(data)

	if err != nil || len(ops) == 0 {
		t.Fatalf("ops %v err %v", ops, err)
	}
	for _, op := range ops {
		req := request(t, r, op.RequestPath)
		if !slices.ContainsFunc(req.Versions, func(v model.RequestVersion) bool { return v.Label == op.Label }) {
			t.Errorf("%s has no version %s", op.RequestPath, op.Label)
		}
	}
}
