// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func importWithFiles(t *testing.T, doc string, files fstest.MapFS) (Result, diag.List) {
	t.Helper()
	n := 0
	return Import([]byte(doc), Options{NewID: func() model.ID { n++; return model.ID(strings.Repeat("a", n)) }, Files: files})
}

const splitDoc = `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /pets:
    post:
      operationId: addPet
      requestBody:
        content:
          application/json:
            schema:
              $ref: ./schemas/pet.yaml
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "./schemas/pet.yaml#/properties/owner"
`

func TestImport_should_follow_a_reference_to_a_file_beside_the_document(t *testing.T) {
	files := fstest.MapFS{"schemas/pet.yaml": {Data: []byte("type: object\nproperties:\n  name: {type: string}\n  owner: {type: object, properties: {id: {type: integer}}}\n")}}

	r, diags := importWithFiles(t, splitDoc, files)

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
	body := r.Requests[0].Versions[0].Body.JSON
	if !strings.Contains(body, `"name"`) {
		t.Errorf("the body should be generated from the file's schema, got %q", body)
	}
	schema, _ := r.Schemas[r.Requests[0].Path]["1"]["200"].(map[string]any)
	if props, _ := schema["properties"].(map[string]any); props["id"] == nil {
		t.Errorf("the response schema should be the part of the file the pointer names, got %v", schema)
	}
	if _, ok := r.Files["schemas/pet.yaml"]; !ok || len(r.Files) != 1 {
		t.Errorf("the file read should be handed back to be kept, got %v", r.Files)
	}
}

func TestImport_should_resolve_a_reference_inside_a_file_relative_to_that_file(t *testing.T) {
	files := fstest.MapFS{
		"schemas/pet.yaml":           {Data: []byte("type: object\nproperties:\n  owner: {$ref: ./people/person.yaml}\n  self: {$ref: '#/properties/owner'}\n")},
		"schemas/people/person.yaml": {Data: []byte("type: object\nproperties:\n  age: {type: integer}\n")},
	}

	r, diags := importWithFiles(t, splitDoc, files)

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
	if !strings.Contains(r.Requests[0].Versions[0].Body.JSON, `"age"`) {
		t.Errorf("the nested file's schema should be inlined, got %q", r.Requests[0].Versions[0].Body.JSON)
	}
}

func TestImport_should_cut_a_cycle_that_runs_through_files(t *testing.T) {
	files := fstest.MapFS{
		"schemas/pet.yaml":   {Data: []byte("type: object\nproperties:\n  friend: {$ref: ./other.yaml}\n")},
		"schemas/other.yaml": {Data: []byte("type: object\nproperties:\n  back: {$ref: ./pet.yaml}\n")},
	}

	r, diags := importWithFiles(t, splitDoc, files)

	if len(r.Requests) != 1 || diags.HasErrors() {
		t.Fatalf("requests %d, diags %v", len(r.Requests), diags)
	}
}

func TestImport_should_expand_a_path_item_that_is_a_reference_to_a_file(t *testing.T) {
	doc := `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /pets:
    $ref: ./paths/pets.yaml
`
	files := fstest.MapFS{"paths/pets.yaml": {Data: []byte("get:\n  operationId: listPets\n  responses:\n    '200': {description: ok}\n")}}

	r, diags := importWithFiles(t, doc, files)

	if len(diags) != 0 || len(r.Requests) != 1 || r.Requests[0].Operation != "listPets" {
		t.Fatalf("requests %+v, diags %v", r.Requests, diags)
	}
}

func TestImport_should_report_why_a_reference_to_a_file_was_not_followed(t *testing.T) {
	tests := map[string]struct {
		ref   string
		files fstest.MapFS
		want  string
	}{
		"a URL":            {"https://example.com/pet.yaml", fstest.MapFS{}, "URL"},
		"outside the root": {"../secret.yaml", fstest.MapFS{"../secret.yaml": {}}, "outside"},
		"missing":          {"./gone.yaml", fstest.MapFS{}, "cannot be read"},
		"not a document":   {"./text.txt", fstest.MapFS{"text.txt": {Data: []byte("just words\n")}}, "not a YAML or JSON object"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			doc := strings.Replace(splitDoc, "./schemas/pet.yaml", tc.ref, 1)

			r, diags := importWithFiles(t, doc, tc.files)

			if len(r.Requests) != 1 {
				t.Fatalf("the import should go on, got %d requests", len(r.Requests))
			}
			var details []string
			for _, d := range diags {
				details = append(details, d.Args["detail"])
			}
			if !strings.Contains(strings.Join(details, "|"), tc.want) {
				t.Errorf("want a reason that says %q, got %v", tc.want, details)
			}
		})
	}
}

func TestImport_should_not_follow_files_without_a_folder_to_read_from(t *testing.T) {
	r, diags := Import([]byte(splitDoc), Options{NewID: func() model.ID { return "a" }})

	if len(r.Requests) != 1 || len(diags) == 0 || !strings.Contains(diags[0].Args["detail"], "within the document") {
		t.Fatalf("requests %d, diags %v", len(r.Requests), diags)
	}
}
