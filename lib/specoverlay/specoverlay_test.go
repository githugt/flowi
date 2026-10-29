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

func TestApplyTo_should_warn_when_extends_names_another_document(t *testing.T) {
	cases := map[string]struct {
		extends, name string
		warns         bool
	}{
		"same file, other place":  {"https://example.com/api/examples.yaml", "specs/examples.yaml", false},
		"pointer after the name":  {"examples.yaml#/paths", "examples.yaml", false},
		"another document":        {"https://example.com/other.yaml", "examples.yaml", true},
		"no extends":              {"", "examples.yaml", false},
		"document without a name": {"other.yaml", "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ov := "overlay: 1.0.0\ninfo: {title: t, version: 1.0.0}\n"
			if tc.extends != "" {
				ov += "extends: " + tc.extends + "\n"
			}
			ov += "actions:\n  - target: $.paths['/examples'].get\n    update: {operationId: listExamples}\n"

			out, diags := ApplyTo([]byte(spec), tc.name, [][]byte{[]byte(ov)})

			if got := len(diags) == 1 && diags[0].Code == diag.CodeImportUnmapped; got != tc.warns {
				t.Errorf("warned %v, want %v: %v", got, tc.warns, diags)
			}
			if !strings.Contains(string(out), "listExamples") {
				t.Errorf("the overlay was not applied:\n%s", out)
			}
		})
	}
}
