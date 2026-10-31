// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// What OpenAPI 3.2 adds beyond QUERY and additionalOperations (methods_test.go): nested tags, dataValue examples,
// the querystring parameter location and event-stream responses.
const doc32 = `openapi: 3.2.0
info: {title: Shop, version: "1.0"}
tags:
  - name: pets
  - name: dogs
    parent: pets
    kind: nav
    summary: Dogs
  - name: loop-a
    parent: loop-b
  - name: loop-b
    parent: loop-a
paths:
  /dogs:
    post:
      operationId: addDog
      tags: [dogs]
      requestBody:
        content:
          application/json:
            schema: {type: object, properties: {name: {type: string}}}
            examples:
              zzz: {dataValue: {name: Zed}}
              rex: {dataValue: {name: Rex}}
      responses: {"201": {description: ok}}
  /legacy:
    post:
      operationId: addLegacy
      requestBody:
        content:
          application/json:
            schema: {type: object}
            examples:
              one: {value: {kind: old}}
      responses: {"201": {description: ok}}
  /serialized:
    post:
      operationId: addSerialized
      requestBody:
        content:
          application/json:
            schema: {type: object, properties: {name: {type: string}}}
            examples:
              raw: {serializedValue: "{\"name\": \"Kept\"}"}
      responses: {"201": {description: ok}}
  /search:
    get:
      operationId: searchDogs
      parameters:
        - name: filter
          in: querystring
          content:
            application/x-www-form-urlencoded: {schema: {type: object}}
      responses: {"200": {description: ok}}
  /stream:
    get:
      operationId: streamDogs
      responses:
        "200":
          description: events
          content:
            text/event-stream:
              itemSchema: {type: object, properties: {id: {type: string}}}
  /looped:
    get:
      operationId: loopedThing
      tags: [loop-a]
      responses: {"200": {description: ok}}
`

func importDoc32(t *testing.T) map[string]model.Request {
	t.Helper()
	n := 0
	newID := func() model.ID { n++; return model.ID(fmt.Sprintf("ID%024d", n)) }

	got, diags := Import([]byte(doc32), Options{NewID: newID})

	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	out := map[string]model.Request{}
	for _, r := range got.Requests {
		out[r.Operation] = r
	}
	return out
}

func TestImport_Given_nested_tags_Then_the_request_goes_in_the_folder_of_its_parents(t *testing.T) {
	got := importDoc32(t)

	if p := got["addDog"].Path; p != "pets/dogs/add-dog" && !strings.HasPrefix(p, "pets/dogs/") {
		t.Fatalf("path %q, want it under pets/dogs/", p)
	}
}

func TestImport_Given_tags_that_are_each_others_parent_Then_the_import_still_finishes_with_a_folder(t *testing.T) {
	got := importDoc32(t)

	if p := got["loopedThing"].Path; !strings.HasSuffix(p, "/loopedThing") {
		t.Fatalf("path %q", p)
	}
}

func TestImport_Given_example_values_Then_the_body_is_the_first_by_name_and_dataValue_counts(t *testing.T) {
	tests := []struct{ operation, want string }{
		{"addDog", `"name": "Rex"`},         // dataValue, rex before zzz
		{"addLegacy", `"kind": "old"`},      // 3.1 value
		{"addSerialized", `"name": "Kept"`}, // serializedValue: a JSON text kept as written
	}
	got := importDoc32(t)
	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			body := got[tt.operation].Versions[0].Body.JSON

			if !strings.Contains(body, tt.want) {
				t.Fatalf("body %q, want it to contain %q", body, tt.want)
			}
		})
	}
}

func TestImport_Given_a_querystring_parameter_Then_the_whole_query_is_one_template(t *testing.T) {
	got := importDoc32(t)

	v := got["searchDogs"].Versions[0]

	if v.URL != "{{env.base_url}}/search?{{filter}}" || len(v.Query) != 0 {
		t.Fatalf("url %q query %+v", v.URL, v.Query)
	}
}

func TestImport_Given_an_event_stream_response_Then_the_operation_is_a_server_sent_events_request(t *testing.T) {
	got := importDoc32(t)

	v := got["streamDogs"].Versions[0]

	if v.Async == nil || v.Async.Protocol != "sse" || v.Async.Server != "{{env.base_url}}/stream" || v.Async.Receive == nil || v.Method != "" {
		t.Fatalf("version %+v async %+v", v, v.Async)
	}
}
