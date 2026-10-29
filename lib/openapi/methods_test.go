// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"fmt"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

const queryDoc = `openapi: 3.2.0
info: {title: Search, version: "1.0"}
paths:
  /things:
    query:
      operationId: searchThings
      requestBody:
        content:
          application/json: {schema: {type: object}}
      responses:
        "200": {description: ok}
    additionalOperations:
      PROPFIND:
        responses:
          "207": {description: multi-status}
      Purge:
        responses:
          "200": {description: ok}
      get:
        responses:
          "200": {description: ignored, a fixed method has its own field}
`

func TestImport_Given_a_query_operation_and_additional_operations_Then_each_becomes_a_request_with_its_method(t *testing.T) {
	// Arrange
	n := 0
	newID := func() model.ID { n++; return model.ID(fmt.Sprintf("ID%024d", n)) }

	// Act
	got, diags := Import([]byte(queryDoc), Options{NewID: newID})

	// Assert
	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	methods := map[string]bool{}
	for _, r := range got.Requests {
		methods[r.Versions[0].Method] = true
	}
	for _, want := range []string{"QUERY", "PROPFIND", "Purge"} {
		if !methods[want] {
			t.Errorf("no request with method %q in %v", want, methods)
		}
	}
	if methods["GET"] || len(got.Requests) != 3 {
		t.Errorf("a name that is a fixed method must be left to its field: %v, %d requests", methods, len(got.Requests))
	}
}
