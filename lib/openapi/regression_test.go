// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"strings"
	"testing"
)

// Variables and secrets inside an OpenAPI spec were
// interpolated at import. Template text in a spec is data and must reach
// the collection unchanged, never resolved.
func TestImport_Given_template_text_in_description_When_imported_Then_it_is_kept_verbatim(t *testing.T) {
	r, _ := importDoc(t, `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /pets:
    get:
      operationId: listPets
      description: "Use {{secret.api_key}} and {{vars.x}} as shown"
      responses:
        "200": {description: ok}
`)

	if len(r.Requests) != 1 {
		t.Fatalf("%d requests", len(r.Requests))
	}
	var docs strings.Builder
	for _, v := range r.Requests[0].Versions {
		docs.WriteString(v.Docs)
	}
	if !strings.Contains(docs.String(), "{{secret.api_key}}") {
		t.Fatalf("description changed: %q", docs.String())
	}
}
