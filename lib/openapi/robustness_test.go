// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func importDoc(t *testing.T, doc string) (Result, diag.List) {
	t.Helper()
	n := 0
	return Import([]byte(doc), Options{NewID: func() model.ID { n++; return model.ID(fmt.Sprintf("ID%024d", n)) }})
}

func unmappedWhats(diags diag.List) []string {
	var out []string
	for _, d := range diags {
		if d.Code == diag.CodeImportUnmapped {
			out = append(out, d.Args["what"])
		}
	}
	return out
}

// Before: response keys went into schema file names unchecked, so
// "../../x" wrote outside the collection.
func TestImport_Given_a_status_key_that_is_not_a_status_code_Then_its_schema_is_left_out(t *testing.T) {
	r, diags := importDoc(t, `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "200": {description: ok, content: {application/json: {schema: {type: object}}}}
        "../../../evil": {description: x, content: {application/json: {schema: {type: object}}}}
        "2XX": {description: x, content: {application/json: {schema: {type: object}}}}
        default: {description: x, content: {application/json: {schema: {type: object}}}}
`)

	statuses := r.Schemas["listPets"]["1"]
	if len(statuses) != 1 || statuses["200"] == nil {
		t.Fatalf("schemas %v", statuses)
	}
	if whats := unmappedWhats(diags); len(whats) != 3 {
		t.Fatalf("each left-out response is reported: %v", whats)
	}
}

// Before: every acyclic $ref path was expanded in full, so densely
// referencing schemas took exponential time and memory.
func TestImport_Given_densely_referencing_schemas_Then_it_finishes_quickly(t *testing.T) {
	var schemas strings.Builder
	for i := range 12 {
		fmt.Fprintf(&schemas, "    S%d:\n      type: object\n      properties:\n", i)
		for j := range 12 {
			fmt.Fprintf(&schemas, "        p%d: {$ref: \"#/components/schemas/S%d\"}\n", j, j)
		}
	}
	doc := `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /a:
    post:
      operationId: a
      requestBody: {content: {application/json: {schema: {$ref: "#/components/schemas/S0"}}}}
      responses:
        "200": {description: ok, content: {application/json: {schema: {$ref: "#/components/schemas/S0"}}}}
components:
  schemas:
` + schemas.String()
	done := make(chan struct{})

	go func() {
		defer close(done)
		importDoc(t, doc)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("import did not finish within 10s")
	}
}

// Before: /pet-s and /pet_s both became get-pet-s (one was lost), and an
// operationId such as "Get A" made an invalid file name.
func TestImport_should_make_request_names_valid_and_unique(t *testing.T) {
	r, _ := importDoc(t, `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /pet-s: {get: {responses: {"200": {description: ok}}}}
  /pet_s: {get: {responses: {"200": {description: ok}}}}
  /a: {get: {operationId: "Get A", responses: {"200": {description: ok}}}}
  /b: {get: {operationId: "pets:list", tags: [pets], responses: {"200": {description: ok}}}}
`)

	var paths []string
	for _, req := range r.Requests {
		paths = append(paths, req.Path)
	}
	slices.Sort(paths)
	want := []string{"Get-A", "get-pet-s", "get-pet-s-2", "pets/pets-list"}
	if !slices.Equal(paths, want) {
		t.Fatalf("paths %v, want %v", paths, want)
	}
}

// Before: header expressions stayed as literal text, keys were not
// template-safe, and ~1 was not unescaped.
func TestImport_link_values_map_or_are_reported(t *testing.T) {
	r, diags := importDoc(t, `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /pets:
    post:
      operationId: createPet
      responses:
        "201":
          description: created
          links:
            Get:
              operationId: getPet
              parameters:
                X-Trace: $request.header.X-Trace
                petId: $response.body#/a~1b
                where: $response.header.Location
                fixed: literal
                odd: $url
  /pets/{petId}:
    get:
      operationId: getPet
      responses: {"200": {description: ok}}
`)

	with := r.Flows[0].Nodes[1].Request.With
	want := []model.KeyValue{
		{Key: "X_Trace", Value: "{{inputs.X_Trace}}"},
		{Key: "fixed", Value: "literal"},
		{Key: "petId", Value: `{{nodes.createPet.body["a/b"]}}`},
		{Key: "where", Value: `{{nodes.createPet.headers.location}}`},
	}
	if !slices.Equal(with, want) {
		t.Fatalf("with %v\nwant %v", with, want)
	}
	if whats := unmappedWhats(diags); len(whats) != 1 || !strings.Contains(whats[0], "odd") {
		t.Fatalf("diags %v", whats)
	}
}

// Before: only exact application/json was kept, so a vendor JSON 200
// left only the error schema and good reads were judged against it.
func TestImport_should_keep_json_suffix_media_types(t *testing.T) {
	r, _ := importDoc(t, `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /pets:
    get:
      operationId: listPets
      responses:
        "200": {description: ok, content: {application/vnd.pets+json: {schema: {type: object}}}}
`)

	if r.Schemas["listPets"]["1"]["200"] == nil {
		t.Fatalf("schemas %v", r.Schemas)
	}
}
