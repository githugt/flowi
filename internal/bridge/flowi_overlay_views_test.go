// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"strings"
	"testing"
	"time"

	flowi "github.com/githugt/flowi-lib"
)

func TestSpecExportView_should_name_the_files_and_say_why_each_change_was_left_out(t *testing.T) {
	// Arrange
	res := flowi.OverlayExport{
		Documents:  []flowi.SpecOverlayDocument{{Spec: flowi.KeptSpec{Name: "pets"}, YAML: "overlay: 1.0.0\n", Requests: []string{"pets/listPets"}}},
		NotCarried: []flowi.ExportNote{{Request: "pets/listPets", What: "the checks", Why: "the spec has no place for it"}},
	}

	// Act
	got := specExportView("documented", res)

	// Assert
	if len(got.Documents) != 1 || got.Documents[0].File != "documented.pets.overlay.yaml" {
		t.Fatalf("documents %+v", got.Documents)
	}
	if len(got.NotCarried) != 1 || !strings.HasPrefix(got.NotCarried[0], "pets/listPets: the checks") {
		t.Fatalf("not carried %v", got.NotCarried)
	}
}

func TestSpecExportView_Given_nothing_to_export_Then_lists_are_empty_not_null(t *testing.T) {
	got := specExportView("x", flowi.OverlayExport{})

	if got.Documents == nil || got.NotCarried == nil {
		t.Fatal("the window iterates these lists; null would break it")
	}
}

func TestVarValuesOf_should_sort_names_and_show_each_value_on_one_line(t *testing.T) {
	// Arrange
	vars := map[string]any{"b": map[string]any{"n": 1}, "a": "line one\n  line two", "c": strings.Repeat("x", 300)}

	// Act
	got := varValuesOf(vars)

	// Assert
	if len(got) != 3 || got[0].Name != "a" || got[0].Value != "line one line two" || got[1].Value != `{"n":1}` {
		t.Fatalf("got %+v", got)
	}
	if n := len([]rune(got[2].Value)); n != varValueWidth {
		t.Fatalf("a long value is cut to %d runes, got %d", varValueWidth, n)
	}
}

func TestValueType_should_name_json_shaped_values_in_the_words_expressions_use(t *testing.T) {
	cases := map[string]any{"text": "a", "number": 3, "true/false": true, "list": []any{1}, "object": map[string]any{}, "null": nil, "duration": time.Second}
	for want, v := range cases {
		if got := valueType(v); got != want {
			t.Errorf("%T: got %q, want %q", v, got, want)
		}
	}
	if valueType(struct{}{}) != "" {
		t.Error("a value of no JSON type has no type hint")
	}
}

func TestOutputType_Given_no_run_Then_says_what_such_an_output_always_is(t *testing.T) {
	if got := outputType(nil, "login", "login.status"); got != "number" {
		t.Errorf("status: %q", got)
	}
	if got := outputType(nil, "login", "login.token"); got != "" {
		t.Errorf("an output nothing says the type of: %q", got)
	}
}
