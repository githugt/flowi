// SPDX-License-Identifier: Apache-2.0

package analysis

import "testing"

// Inferred request-body schemas live next to the response ones as
// <version>-request.json; they must not be read as the response of a status.
func TestSchemaName_should_not_take_a_request_body_schema_for_a_response(t *testing.T) {
	if _, _, ok := schemaName("1-request.json"); ok {
		t.Fatal("1-request.json was read as a response schema")
	}
	if label, status, ok := schemaName("1.1-200.json"); !ok || label != "1.1" || status != 200 {
		t.Fatalf("got %q %d %v", label, status, ok)
	}
}
