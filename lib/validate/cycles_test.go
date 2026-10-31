// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestDependencyCycles(t *testing.T) {
	log := func(id string, needs ...string) model.Node {
		return model.Node{ID: id, Type: "note", Needs: needs, Params: params("text", "x")}
	}
	tests := []struct {
		name      string
		nodes     []model.Node
		wantCycle string
	}{
		{name: "explicit_two_node_cycle", nodes: []model.Node{log("a", "b"), log("b", "a")}, wantCycle: "a → b → a"},
		{name: "through_implicit_previous", nodes: []model.Node{log("a", "c"), log("b"), log("c")}, wantCycle: "a → c → b → a"},
		{name: "through_a_failure_arrow", nodes: []model.Node{{ID: "a", Type: "note", AfterFailureOf: []string{"b"}, Params: params("text", "x")}, log("b")}, wantCycle: "a → b → a"},
		{name: "self_dependency", nodes: []model.Node{log("a", "a")}, wantCycle: "a → a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			f.flow(&c, "x", tt.nodes...)

			got := Validate(Target{Collections: []model.Collection{c}})

			d := only(t, got, diag.CodeCycleNeeds)
			if d.Args["cycle"] != tt.wantCycle {
				t.Fatalf("cycle %q, want %q", d.Args["cycle"], tt.wantCycle)
			}
		})
	}
}

func TestDependencyCycles_should_not_flag_join_after_condition(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.flow(&c, "x",
		model.Node{ID: "start", Type: "note", Params: params("text", "x")},
		model.Node{ID: "branch", Type: "condition", Params: params("check", "true"), Then: []model.Node{{ID: "t", Type: "note", Params: params("text", "x")}}, Else: []model.Node{{ID: "e", Type: "note", Params: params("text", "x")}}},
		model.Node{ID: "side", Type: "note", Needs: []string{"start"}, Params: params("text", "x")},
		model.Node{ID: "join", Type: "note", Needs: []string{"branch", "side"}, Params: params("text", "x")},
	)

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}

func subflow(id string, target model.Flow, collection string) model.Node {
	return model.Node{ID: id, Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{Collection: collection, ID: target.ID, Path: target.Path}}}
}

func TestSubflowCycles(t *testing.T) {
	t.Run("self_loop", func(t *testing.T) {
		f := newFixture(t)
		c := f.collection("c")
		self := f.flow(&c, "loop")
		c.Flows[0].Nodes = []model.Node{subflow("again", self, "")}

		got := Validate(Target{Collections: []model.Collection{c}})

		d := only(t, got, diag.CodeCycleSubflow)
		if d.Args["cycle"] != "loop → loop" {
			t.Fatalf("cycle %q", d.Args["cycle"])
		}
	})
	t.Run("across_collections", func(t *testing.T) {
		f := newFixture(t)
		a := f.collection("a")
		b := f.collection("b")
		a.Manifest.DependsOn = []model.Dependency{{ID: b.Manifest.ID, Name: "b"}}
		b.Manifest.DependsOn = []model.Dependency{{ID: a.Manifest.ID, Name: "a"}}
		fa := f.flow(&a, "one")
		fb := f.flow(&b, "two")
		a.Flows[0].Nodes = []model.Node{subflow("call", fb, "b")}
		b.Flows[0].Nodes = []model.Node{subflow("back", fa, "a")}

		got := Validate(Target{Collections: []model.Collection{a, b}})

		d := only(t, got, diag.CodeCycleSubflow)
		if d.Args["cycle"] != "one → b:two → one" {
			t.Fatalf("cycle %q", d.Args["cycle"])
		}
	})
	t.Run("inside_nested_container", func(t *testing.T) {
		f := newFixture(t)
		c := f.collection("c")
		hook := f.flow(&c, "hook")
		r := f.request(&c, "r", "1")
		c.Requests[0].Hooks.Pre = []model.HookStep{{Inline: []model.Node{subflow("h", hook, "")}}}
		c.Flows[0].Nodes = []model.Node{{ID: "inner", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(1)}}, Body: []model.Node{subflow("again", hook, "")}}}
		_ = r

		got := Validate(Target{Collections: []model.Collection{c}})

		only(t, got, diag.CodeCycleSubflow)
	})
}
