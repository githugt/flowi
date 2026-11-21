// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"testing"
)

func TestReadDataRows_Given_a_csv_Then_each_row_is_a_set_of_inputs_named_by_its_label_column(t *testing.T) {
	// Before: flowi run had no way to run a flow over a data file, and a failing row was only a number.
	rows, err := readDataRows("users.csv", []byte("\xef\xbb\xbfemail,plan,name\nada@x.test,pro,Ada\nbob@x.test,free,Bob\n"))

	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %+v err %v", rows, err)
	}
	if rows[0].label != "Ada" || rows[0].inputs["email"] != "ada@x.test" || rows[1].inputs["plan"] != "free" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestReadDataRows_Given_json_Then_values_become_the_text_of_an_input_and_a_name_column_names_the_row(t *testing.T) {
	rows, err := readDataRows("users.json", []byte(`[{"email":"a@x.test","age":41,"admin":true,"note":null,"tags":["x"]},{"email":"a@x.test"}]`))

	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %+v err %v", rows, err)
	}
	in := rows[0].inputs
	if in["age"] != "41" || in["admin"] != "true" || in["note"] != "" || in["tags"] != `["x"]` {
		t.Fatalf("inputs %v", in)
	}
	if rows[0].label != "row 1" || rows[1].label != "row 2" {
		t.Fatalf("labels %q %q: a JSON object has no first key, so without a name column a row is its number", rows[0].label, rows[1].label)
	}
	named, _ := readDataRows("users.json", []byte(`[{"email":"a@x.test","name":"same"},{"email":"b@x.test","name":"same"}]`))
	if named[0].label != "same" || named[1].label != "same#2" {
		t.Fatalf("a repeated label is told apart: %q %q", named[0].label, named[1].label)
	}
}

func TestReadDataRows_Given_a_bad_file_Then_it_is_a_usage_error_that_says_why(t *testing.T) {
	tests := map[string]struct{ file, data string }{
		"unknown type":     {"users.txt", "a\n1\n"},
		"no rows":          {"users.csv", "email\n"},
		"empty":            {"users.csv", ""},
		"unnamed column":   {"users.csv", "email,\na,b\n"},
		"repeated column":  {"users.csv", "email,email\na,b\n"},
		"short line":       {"users.csv", "email,plan\na\n"},
		"not an array":     {"users.json", `{"email":"a"}`},
		"array of scalars": {"users.json", `[1,2]`},
		"too many rows":    {"users.csv", "email\n" + repeat("a\n", maxMatrixCells+1)},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := readDataRows(tt.file, []byte(tt.data))

			var usage *UsageError
			if !errors.As(err, &usage) {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func repeat(s string, n int) string {
	out := ""
	for range n {
		out += s
	}
	return out
}
