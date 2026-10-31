// SPDX-License-Identifier: AGPL-3.0-or-later

package card_test

import (
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"

	"github.com/githugt/flowi/internal/card"
)

func TestStartCard_Given_inputs_Then_rows_are_in_declared_order_with_types(t *testing.T) {
	// Arrange
	inputs := []model.Input{
		{Name: "email", Type: "string", Default: "a@b.c"},
		{Name: "count", Type: "number"},
		{Name: "debug"},
	}

	// Act
	got := card.StartCard(inputs)

	// Assert
	if got.Badge != "MANUAL" || got.Subline != "3 inputs" {
		t.Fatalf("badge %q subline %q", got.Badge, got.Subline)
	}
	want := []string{"email · string", "count · number", "debug"}
	if len(got.Rows) != len(want) {
		t.Fatalf("rows %+v", got.Rows)
	}
	for i, r := range got.Rows {
		if r.Summary != want[i] || r.Key != "input:"+inputs[i].Name || r.Editor != "input" {
			t.Errorf("row %d: %+v, want summary %q", i, r, want[i])
		}
	}
}

func TestStartCard_Given_no_inputs_Then_it_says_so_and_has_no_rows(t *testing.T) {
	got := card.StartCard(nil)

	if got.Subline != "no inputs" || len(got.Rows) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestStartCard_Given_a_single_input_Then_the_subline_is_singular(t *testing.T) {
	got := card.StartCard([]model.Input{{Name: "email"}})

	if got.Subline != "1 input" {
		t.Fatalf("subline %q", got.Subline)
	}
}

func TestEndCard_Given_no_outputs_Then_no_card_is_drawn(t *testing.T) {
	// Act
	_, drawn := card.EndCard(nil, nil)

	// Assert
	if drawn {
		t.Fatal("a flow with no outputs has no End card")
	}
}

func TestEndCard_Given_outputs_and_no_run_Then_rows_show_the_value_template(t *testing.T) {
	// Arrange
	outputs := []model.Output{{Name: "token", Type: "string", Value: "{{vars.token}}"}}

	// Act
	got, drawn := card.EndCard(outputs, nil)

	// Assert
	if !drawn || got.Badge != "RETURNS" || got.Subline != "1 value" {
		t.Fatalf("drawn %v %+v", drawn, got)
	}
	if got.Rows[0].Key != "output:token" || got.Rows[0].Summary != "token = {{vars.token}}" {
		t.Errorf("row %+v", got.Rows[0])
	}
}

func TestEndCard_Given_a_finished_run_Then_rows_carry_the_values(t *testing.T) {
	// Arrange
	outputs := []model.Output{{Name: "userId", Value: "{{vars.id}}"}, {Name: "name", Value: "{{vars.name}}"}}
	run := &report.Run{
		Status: "passed", Duration: 2100 * time.Millisecond,
		Nodes:   []report.Node{{ID: "a", Status: "passed"}, {ID: "b", Status: "passed"}, {ID: "c", Status: "failed"}},
		Outputs: []report.OutputValue{{Name: "userId", Value: "u-42"}},
	}

	// Act
	got, drawn := card.EndCard(outputs, run)

	// Assert
	if !drawn || !strings.Contains(got.Subline, "passed") || !strings.Contains(got.Subline, "2.1s") || !strings.Contains(got.Subline, "2 of 3 steps") {
		t.Fatalf("drawn %v subline %q", drawn, got.Subline)
	}
	if got.Rows[0].Summary != "userId = u-42" {
		t.Errorf("first row %q should carry the value", got.Rows[0].Summary)
	}
	if !strings.Contains(got.Rows[1].Summary, "no value") {
		t.Errorf("an output the run could not work out should say so, got %q", got.Rows[1].Summary)
	}
}
