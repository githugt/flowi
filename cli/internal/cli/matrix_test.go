// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
)

func TestParseMatrix_Given_dimensions_Then_names_and_values_are_read_in_order(t *testing.T) {
	// Act
	dims, err := parseMatrix([]string{"env=local,staging", "overlay=vip,guest", "input.email=a@x.test"})

	// Assert
	if err != nil || len(dims) != 3 || dims[0].name != "env" || dims[1].values[1] != "guest" || dims[2].name != "input.email" {
		t.Fatalf("dims %+v err %v", dims, err)
	}
}

func TestParseMatrix_Given_a_bad_spec_Then_it_is_a_usage_error_that_says_why(t *testing.T) {
	tests := map[string]string{
		"nothing":           "env",
		"no values":         "env=",
		"unknown dimension": "colour=red",
		"empty input name":  "input.=a",
		"repeated":          "env=a,a",
	}
	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseMatrix([]string{spec})

			var usage *UsageError
			if !errors.As(err, &usage) {
				t.Fatalf("err %v", err)
			}
		})
	}
	if _, err := parseMatrix([]string{"env=a", "env=b"}); err == nil {
		t.Error("the same dimension twice should be refused")
	}
}

func TestMatrixCells_Given_two_dimensions_Then_the_first_varies_slowest_and_each_cell_sets_its_options(t *testing.T) {
	// Arrange
	dims, _ := parseMatrix([]string{"env=local,staging", "overlay=vip,guest"})

	// Act
	cells := matrixCells(dims)

	// Assert
	if len(cells) != 4 || cells[0].label() != "local · vip" || cells[1].label() != "local · guest" || cells[3].label() != "staging · guest" {
		t.Fatalf("cells %+v", cells)
	}
	got := cells[3].options(flowi.RunOptions{Inputs: map[string]string{"keep": "1"}}, dims)
	if got.Env != "staging" || got.Overlay != "guest" || got.Inputs["keep"] != "1" {
		t.Fatalf("options %+v", got)
	}
}

func TestMatrixCells_Given_an_input_dimension_Then_it_is_set_without_touching_the_callers_inputs(t *testing.T) {
	// Arrange
	dims, _ := parseMatrix([]string{"input.email=a@x.test,b@x.test"})
	base := flowi.RunOptions{Inputs: map[string]string{"keep": "1"}}

	// Act
	got := matrixCells(dims)[1].options(base, dims)

	// Assert
	if got.Inputs["email"] != "b@x.test" || got.Inputs["keep"] != "1" || base.Inputs["email"] != "" {
		t.Fatalf("got %+v base %+v", got.Inputs, base.Inputs)
	}
}

func TestMatrixLimit_Given_too_many_cells_Then_it_is_refused_before_anything_runs(t *testing.T) {
	dims, err := parseMatrix([]string{"env=a,b,c,d,e,f,g,h,i,j", "overlay=a,b,c,d,e,f,g,h,i,j", "input.x=1,2,3"})
	if err != nil {
		t.Fatal(err)
	}

	err = checkMatrixSize(dims)

	if err == nil || !strings.Contains(err.Error(), "300") {
		t.Fatalf("err %v", err)
	}
}

func TestRenderGrid_Given_two_dimensions_Then_rows_are_the_first_and_columns_the_second(t *testing.T) {
	// Arrange
	dims, _ := parseMatrix([]string{"env=local,staging", "overlay=vip,guest"})
	cells := matrixCells(dims)
	results := []cellResult{
		{cell: cells[0], outcome: outcomePassed}, {cell: cells[1], outcome: outcomeFailed, why: "step verify: assertion failed"},
		{cell: cells[2], outcome: outcomePassed}, {cell: cells[3], outcome: outcomeError, why: "connection refused"},
	}
	var out bytes.Buffer

	// Act
	renderGrid(&out, dims, results)

	// Assert
	text := out.String()
	for _, want := range []string{"overlay", "vip", "guest", "local", "staging", "✓", "✗", "!", "staging · guest: connection refused", "local · guest: step verify: assertion failed"} {
		if !strings.Contains(text, want) {
			t.Errorf("the grid lacks %q:\n%s", want, text)
		}
	}
	lines := strings.Split(text, "\n")
	if !strings.Contains(lines[2], "local") || !strings.Contains(lines[2], "✓") || !strings.Contains(lines[2], "✗") {
		t.Errorf("the local row should show a pass then a fail:\n%s", text)
	}
}

func TestRenderGrid_Given_one_or_three_dimensions_Then_it_is_a_list_with_one_line_per_cell(t *testing.T) {
	dims, _ := parseMatrix([]string{"env=a,b"})
	cells := matrixCells(dims)
	var out bytes.Buffer

	renderGrid(&out, dims, []cellResult{{cell: cells[0], outcome: outcomePassed}, {cell: cells[1], outcome: outcomeFailed, why: "boom"}})

	text := out.String()
	if !strings.Contains(text, "✓ a") || !strings.Contains(text, "✗ b") || !strings.Contains(text, "boom") {
		t.Fatalf("%s", text)
	}
}
