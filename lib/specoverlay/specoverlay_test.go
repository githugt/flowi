// SPDX-License-Identifier: Apache-2.0

package specoverlay

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
)

const spec = `openapi: 3.0.3
info: {title: Examples, version: "1.0"}
paths:
  /examples:
    get:
      responses: {"200": {description: ok}}
  /internal:
    get:
      responses: {"200": {description: ok}}
`

func TestApply_should_add_an_operationId_and_remove_an_operation(t *testing.T) {
	ov := `overlay: 1.0.0
info: {title: fixes, version: 1.0.0}
actions:
  - target: $.paths['/examples'].get
    update: {operationId: listExamples}
  - target: $.paths['/internal']
    remove: true
`

	out, diags := Apply([]byte(spec), [][]byte{[]byte(ov)})

	if len(diags) != 0 || !strings.Contains(string(out), "operationId: listExamples") || strings.Contains(string(out), "/internal") {
		t.Fatalf("diags %v, out:\n%s", diags, out)
	}
}

func TestApply_Given_a_target_that_matches_nothing_Then_a_warning_and_Given_a_bad_overlay_Then_an_error(t *testing.T) {
	stale := `overlay: 1.0.0
info: {title: stale, version: 1.0.0}
actions:
  - target: $.paths['/gone'].get
    update: {operationId: x}
`

	out, warnings := Apply([]byte(spec), [][]byte{[]byte(stale)})
	_, bad := Apply([]byte(spec), [][]byte{[]byte("not: an overlay")})

	if out == nil || len(warnings) != 1 || warnings[0].Code != diag.CodeImportUnmapped {
		t.Fatalf("warnings %v", warnings)
	}
	if len(bad) != 1 || bad[0].Code != diag.CodeImportInvalid {
		t.Fatalf("bad %v", bad)
	}
}
