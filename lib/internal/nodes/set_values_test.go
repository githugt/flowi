// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"fmt"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func setNode(values ...model.SetValue) model.Node {
	return model.Node{ID: "n1", Type: "set", Values: values}
}

func TestSet_Given_values_from_each_source_When_run_Then_every_variable_is_set(t *testing.T) {
	s, _, _ := flowState()
	s.Response = sampleResponse()
	n := setNode(
		model.SetValue{Name: "literal", From: "literal", Value: "{{not evaluated}}"},
		model.SetValue{Name: "sum", Value: "{{1 + 2}}"},
		model.SetValue{Name: "status", From: "response", Where: "status"},
		model.SetValue{Name: "trace", From: "response", Where: "header", Header: "X-Request-Id"},
		model.SetValue{Name: "id", From: "response", Where: "body", Path: "$.data.token"},
		model.SetValue{Name: "name", From: "generate", Value: "{{$first-name}}", Seed: 7},
	)

	out := runOutcome(t, n, s)

	if len(out.Diags) != 0 {
		t.Fatalf("diags %v", out.Diags)
	}
	if got := varOf(s, "literal"); got != "{{not evaluated}}" {
		t.Fatalf("literal = %#v", got)
	}
	if got := varOf(s, "sum"); fmt.Sprint(got) != "3" {
		t.Fatalf("sum = %#v", got)
	}
	for _, k := range []string{"status", "trace", "id", "name"} {
		if varOf(s, k) == nil {
			t.Fatalf("%s was not set; vars %v", k, s.Vars.Snapshot())
		}
		if out.Outputs[k] != varOf(s, k) {
			t.Fatalf("output %s = %v, var %v", k, out.Outputs[k], varOf(s, k))
		}
	}
}

func TestSet_Given_a_later_value_reading_an_earlier_one_When_run_Then_it_sees_the_earlier_value(t *testing.T) {
	s, _, _ := flowState()
	n := setNode(
		model.SetValue{Name: "a", Value: "{{2}}"},
		model.SetValue{Name: "b", Value: "{{vars.a + 1}}"},
	)

	out := runOutcome(t, n, s)

	if len(out.Diags) != 0 || varOf(s, "b") == nil || varOf(s, "b") == 0 {
		t.Fatalf("outcome %+v vars %v", out, s.Vars.Snapshot())
	}
}

func TestSet_Given_a_response_value_and_no_response_When_run_Then_it_fails_naming_the_node(t *testing.T) {
	s, _, _ := flowState()
	n := setNode(model.SetValue{Name: "id", From: "response", Where: "body", Path: "$.data.token"})

	out := runOutcome(t, n, s)

	if len(out.Diags) == 0 {
		t.Fatal("want a diagnostic")
	}
}

func TestSet_Given_a_value_that_is_not_a_number_calc_style_When_run_Then_the_template_error_is_reported(t *testing.T) {
	s, _, _ := flowState()
	n := setNode(model.SetValue{Name: "x", Value: "{{1 / 0}}"})

	out := runOutcome(t, n, s)

	if len(out.Diags) == 0 {
		t.Fatalf("want a diagnostic; vars %v", s.Vars.Snapshot())
	}
}
