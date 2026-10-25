// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"errors"
	"regexp"
	"slices"
	"testing"
)

func TestList_Sorted_should_order_by_file_line_column_code(t *testing.T) {
	l := List{
		New(CodeRefUnknownRequest, Location{File: "b.yaml", Pos: Position{Line: 1, Column: 1}}, nil),
		New(CodeIDDuplicate, Location{File: "a.yaml", Pos: Position{Line: 2, Column: 5}}, nil),
		New(CodeIDMissing, Location{File: "a.yaml", Pos: Position{Line: 2, Column: 5}}, nil),
		New(CodeYAMLSyntax, Location{File: "a.yaml", Pos: Position{Line: 1, Column: 9}}, nil),
	}

	got := l.Sorted()

	want := []Code{CodeYAMLSyntax, CodeIDDuplicate, CodeIDMissing, CodeRefUnknownRequest}
	var gotCodes []Code
	for _, d := range got {
		gotCodes = append(gotCodes, d.Code)
	}
	if !slices.Equal(gotCodes, want) {
		t.Fatalf("got %v, want %v", gotCodes, want)
	}
}

func TestList_Err_should_be_nil_with_only_warnings(t *testing.T) {
	l := List{New(CodeLintWhitespace, Location{File: "a.yaml"}, map[string]string{"field": "url"})}

	err := l.Err()

	if err != nil {
		t.Fatalf("got %v, want nil", err)
	}
	if l.HasErrors() {
		t.Fatal("HasErrors = true for warnings only")
	}
}

func TestList_Err_should_return_ListError_with_first_error_message(t *testing.T) {
	l := List{
		New(CodeRefUnknownRequest, Location{File: "a.yaml"}, map[string]string{"id": "X"}),
		New(CodeIDMissing, Location{File: "b.yaml"}, nil),
	}

	err := l.Err()

	var le *ListError
	if !errors.As(err, &le) {
		t.Fatalf("got %T, want *ListError", err)
	}
	if len(le.List) != 2 {
		t.Fatalf("ListError holds %d diagnostics, want 2", len(le.List))
	}
	if want := "unknown request id X (and 1 more)"; err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestNew_should_take_default_severity(t *testing.T) {
	tests := []struct {
		code Code
		want Severity
	}{
		{CodeRefUnknownRequest, SeverityError},
		{CodeRefStalePath, SeverityWarning},
		{CodeFileUnrecognised, SeverityWarning},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			d := New(tt.code, Location{}, nil)

			if d.Severity != tt.want {
				t.Fatalf("got %v, want %v", d.Severity, tt.want)
			}
		})
	}
}

// codeShape is how a code is written: FLW, the area, a three-digit number. Users search for
// codes and catalogues are keyed by them, so a misspelt one is a bug.
var codeShape = regexp.MustCompile(`^FLW-[A-Z]+-[0-9]{3}$`)

func TestCodes_should_follow_the_FLW_AREA_NNN_shape(t *testing.T) {
	for _, c := range Codes() {
		if !codeShape.MatchString(string(c)) {
			t.Errorf("code %q does not match %s", c, codeShape)
		}
	}
}

func TestCodes_should_each_have_an_english_message_and_a_known_severity(t *testing.T) {
	catalogue := English()

	for _, c := range Codes() {
		message, _ := catalogue.Message(New(c, Location{}, nil))

		if message == "" || message == string(c) {
			t.Errorf("code %s has no English message", c)
		}
		if sev := DefaultSeverity(c); sev != SeverityError && sev != SeverityWarning {
			t.Errorf("code %s has severity %v, want error or warning", c, sev)
		}
	}
}

func TestCodes_should_not_be_empty(t *testing.T) {
	if len(Codes()) == 0 {
		t.Fatal("no codes: the catalogue is empty")
	}
}
