// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

func TestVars_should_layer_and_snapshot(t *testing.T) {
	parent := NewVars(nil)
	parent.Set("a", 1)
	parent.Set("b", 2)
	child := NewVars(parent)

	child.Set("b", 20)
	child.Set("c", 30)

	a, okA := child.Get("a")
	if !okA || a != 1 {
		t.Fatalf("a = %v %v", a, okA)
	}
	if got := child.Snapshot(); got["a"] != 1 || got["b"] != 20 || got["c"] != 30 {
		t.Fatalf("snapshot %v", got)
	}
	if got := child.Own(); len(got) != 2 || got["b"] != 20 {
		t.Fatalf("own %v", got)
	}
	if b, _ := parent.Get("b"); b != 2 {
		t.Fatalf("parent changed: %v", b)
	}
}

func TestVars_should_be_safe_for_concurrent_use(t *testing.T) {
	v := NewVars(NewVars(nil))
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v.Set(fmt.Sprint("k", i%10), i)
			_ = v.Snapshot()
			_, _ = v.Get("k1")
		}()
	}
	wg.Wait()

	if len(v.Own()) != 10 {
		t.Fatalf("own %v", v.Own())
	}
}

func flowState() (*State, *recorder, *fakeRuntime) {
	s, rec := newState(PhaseFlow)
	rt := newFakeRuntime()
	s.Runtime = rt
	gen := template.NewGeneration(1, rt.Now).Func()
	inner := s.Scope
	s.Scope = func() template.Scope {
		sc := inner()
		sc.Gen = gen
		return sc
	}
	return s, rec, rt
}

func runOutcome(t *testing.T, n model.Node, s *State) Outcome {
	t.Helper()
	runner, ok := Lookup(n.Type)
	if !ok {
		t.Fatalf("no runner for %s", n.Type)
	}
	return runner(context.Background(), n, s)
}

func TestSet_Given_generate_values_When_run_Then_vars_and_outputs_are_written(t *testing.T) {
	s, _, _ := flowState()

	out := runOutcome(t, setNode(
		model.SetValue{Name: "id", From: "generate", Value: "{{$uuid-v4}}"},
		model.SetValue{Name: "n", From: "generate", Value: "{{$int(1, 1)}}"},
	), s)

	if len(out.Diags) != 0 || out.Outputs["n"] != varOf(s, "n") || out.Outputs["id"] == nil || varOf(s, "id") != out.Outputs["id"] {
		t.Fatalf("outcome %+v vars %v", out, s.Vars.Snapshot())
	}
}

func TestSet_Given_a_seed_on_a_generated_value_Then_it_does_not_change_with_the_run_seed(t *testing.T) {
	first, _, _ := flowState()
	second, _, rt := flowState()
	other := template.NewGeneration(999, rt.Now).Func()
	inner := second.Scope
	second.Scope = func() template.Scope { sc := inner(); sc.Gen = other; return sc }
	n := setNode(model.SetValue{Name: "name", From: "generate", Value: "{{$first-name}}", Seed: 7})

	a := runOutcome(t, n, first)
	b := runOutcome(t, n, second)

	if a.Outputs["name"] == nil || a.Outputs["name"] != b.Outputs["name"] {
		t.Fatalf("got %v and %v", a.Outputs, b.Outputs)
	}
}

func TestSet_Given_an_expression_over_a_list_Then_the_reshaped_value_is_written(t *testing.T) {
	s, _, _ := flowState()
	s.Vars.Set("items", []any{map[string]any{"id": 1}, map[string]any{"id": 2}})

	out := runOutcome(t, setNode(model.SetValue{Name: "ids", Value: "{{map(vars.items, .id)}}"}), s)

	if len(out.Diags) != 0 || fmt.Sprint(out.Outputs["ids"]) != "[1 2]" || fmt.Sprint(varOf(s, "ids")) != "[1 2]" {
		t.Fatalf("outcome %+v vars %v", out, s.Vars.Snapshot())
	}
}

func TestSet_Given_arithmetic_Then_the_result_is_written(t *testing.T) {
	tests := []struct {
		expr string
		want any
	}{
		{"{{vars.a + vars.b}}", 7},
		{"{{vars.a * vars.b - 1}}", 11},
		{"{{vars.b / vars.a}}", 4.0 / 3.0},
		{"{{vars.b % vars.a}}", 1},
		{"{{max(vars.a, vars.b)}}", 4},
		{"{{min(vars.a, vars.b)}}", 3},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			s, _, _ := flowState()
			s.Vars.Set("a", 3)
			s.Vars.Set("b", 4)

			out := runOutcome(t, setNode(model.SetValue{Name: "x", Value: tt.expr}), s)

			if len(out.Diags) != 0 || out.Outputs["x"] != tt.want || varOf(s, "x") != tt.want {
				t.Fatalf("outcome %+v", out)
			}
		})
	}
}

func TestSet_Given_a_division_by_zero_Then_a_diagnostic_is_reported_and_the_variable_is_not_written(t *testing.T) {
	for _, expr := range []string{"{{1 / 0}}", "{{5 % 0}}"} {
		t.Run(expr, func(t *testing.T) {
			s, _, _ := flowState()

			out := runOutcome(t, setNode(model.SetValue{Name: "x", Value: expr}), s)

			if len(out.Diags) != 1 {
				t.Fatalf("diags %v", out.Diags)
			}
			if _, set := s.Vars.Get("x"); set {
				t.Fatal("a failed value must not write the variable")
			}
		})
	}
}

func TestSet_Given_a_float_division_by_zero_Then_FLW_RUN_011(t *testing.T) {
	s, _, _ := flowState()

	out := runOutcome(t, setNode(model.SetValue{Name: "x", Value: "{{1 / 0}}"}), s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunCalcNumber {
		t.Fatalf("diags %v", out.Diags)
	}
}

func TestNote_should_output_rendered_text(t *testing.T) {
	s, _, _ := flowState()
	s.Vars.Set("who", "Ada")

	out := runOutcome(t, node("note", "text", "Signed up {{vars.who}}"), s)

	if len(out.Diags) != 0 || out.Outputs["text"] != "Signed up Ada" {
		t.Fatalf("outcome %+v", out)
	}
}

func TestSet_Given_a_response_value_in_PhaseFlow_Then_FLW_RUN_013(t *testing.T) {
	s, _, _ := flowState()

	out := runOutcome(t, setNode(model.SetValue{Name: "token", From: "response", Where: "body", Path: "$.token"}), s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunExtractNoResp {
		t.Fatalf("diags %v", out.Diags)
	}
}

func TestSet_Given_request_target_in_PhaseFlow_Then_FLW_RUN_004(t *testing.T) {
	s, _, _ := flowState()

	out := runOutcome(t, node("set", "target", "request.headers.X", "value", "v"), s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunSetTarget {
		t.Fatalf("diags %v", out.Diags)
	}
}

func TestSpecs_should_cover_every_node_type_in_catalogue_order(t *testing.T) {
	var got []model.NodeType
	for _, sp := range Specs() {
		got = append(got, sp.Type)
	}
	if fmt.Sprint(got) != fmt.Sprint(model.NodeTypes()) {
		t.Fatalf("specs %v", got)
	}
}
