// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestNodes(t *testing.T) {
	log := func(id string) model.Node { return model.Node{ID: id, Type: "note", Params: params("text", "x")} }
	tests := []struct {
		name     string
		nodes    []model.Node
		wantCode diag.Code
		wantPath string
	}{
		{name: "unknown_type", nodes: []model.Node{{ID: "a", Type: "teleport"}}, wantCode: diag.CodeNodeUnknownType, wantPath: "$.nodes[0].type"},
		{name: "duplicate_id_nested", nodes: []model.Node{log("a"), {ID: "b", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(1)}}, Body: []model.Node{log("a")}}}, wantCode: diag.CodeNodeDuplicateID, wantPath: "$.nodes[1].body[0].id"},
		{name: "needs_non_sibling", nodes: []model.Node{log("a"), {ID: "b", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(1)}}, Body: []model.Node{{ID: "c", Type: "note", Needs: []string{"a"}, Params: params("text", "x")}}}}, wantCode: diag.CodeNodeNeedsSibling, wantPath: "$.nodes[1].body[0].needs[0]"},
		{name: "after_failure_of_non_sibling", nodes: []model.Node{log("a"), {ID: "b", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(1)}}, Body: []model.Node{{ID: "c", Type: "note", AfterFailureOf: []string{"a"}, Params: params("text", "x")}}}}, wantCode: diag.CodeNodeNeedsSibling, wantPath: "$.nodes[1].body[0].after_failure_of[0]"},
		{name: "request_without_call", nodes: []model.Node{{ID: "a", Type: "request"}}, wantCode: diag.CodeNodeMissingField, wantPath: "$.nodes[0]"},
		{name: "batch_without_body", nodes: []model.Node{{ID: "a", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(1)}}}}, wantCode: diag.CodeNodeMissingField, wantPath: "$.nodes[0]"},
		{name: "parallel_with_one_branch", nodes: []model.Node{{ID: "a", Type: "parallel", Branches: []model.Branch{{ID: "x", Nodes: []model.Node{log("b")}}}}}, wantCode: diag.CodeNodeMissingField, wantPath: "$.nodes[0]"},
		{name: "condition_without_branches", nodes: []model.Node{{ID: "a", Type: "condition"}}, wantCode: diag.CodeNodeMissingField, wantPath: "$.nodes[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			f.flow(&c, "x", tt.nodes...)

			got := Validate(Target{Collections: []model.Collection{c}})

			d := only(t, got, tt.wantCode)
			if d.Path != tt.wantPath {
				t.Fatalf("path %q, want %q", d.Path, tt.wantPath)
			}
		})
	}
}

func TestNodes_should_accept_needs_on_sibling_inside_batch_body(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.flow(&c, "x", model.Node{ID: "b", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(1)}}, Body: []model.Node{
		{ID: "one", Type: "note", Params: params("text", "x")},
		{ID: "two", Type: "note", Needs: []string{"one"}, Params: params("text", "x")},
	}})

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}

// A node type has to be in model.NodeTypes to be known: stop was written and run in
// the engine before it was listed there, so a flow file with it was refused.
func TestNodes_should_know_the_stop_node(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.flow(&c, "x", model.Node{ID: "halt", Type: "stop", Params: params("status", "failed", "message", "no token")})

	got := Validate(Target{Collections: []model.Collection{c}})

	if got.HasErrors() {
		t.Fatalf("a stop node was refused: %+v", got)
	}
}

func TestLoopControl_should_be_valid_only_in_a_loop_body_at_any_depth(t *testing.T) {
	// Before: loop_control did not exist; a stop was the only way out and it ended the run.
	step := model.Node{ID: "out", Type: "loop_control", Params: params("action", "leave")}
	note := model.Node{ID: "n", Type: "note", Params: params("text", "x")}
	cond := func(id string, then ...model.Node) model.Node {
		return model.Node{ID: id, Type: "condition", Params: params("check", "true"), Then: then}
	}
	batch := func(body ...model.Node) model.Node {
		return model.Node{ID: "each", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(2)}}, Body: body}
	}
	tests := []struct {
		name     string
		nodes    []model.Node
		wantPath string // "" means valid
	}{
		{name: "in_a_batch_body", nodes: []model.Node{batch(step)}},
		{name: "in_a_run_till_body", nodes: []model.Node{{ID: "poll", Type: "run_till", Params: params("until", "false"), Body: []model.Node{step}}}},
		{name: "in_a_condition_in_the_body", nodes: []model.Node{batch(cond("c", step))}},
		{name: "in_a_parallel_branch_in_the_body", nodes: []model.Node{batch(model.Node{ID: "par", Type: "parallel", Branches: []model.Branch{
			{ID: "one", Nodes: []model.Node{step}}, {ID: "two", Nodes: []model.Node{note}}}})}},
		{name: "in_a_loop_in_a_condition_in_a_loop", nodes: []model.Node{batch(cond("c", model.Node{ID: "inner", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(1)}}, Body: []model.Node{step}}))}},
		{name: "at_the_top_level", nodes: []model.Node{step}, wantPath: "$.nodes[0]"},
		{name: "in_a_condition_outside_a_loop", nodes: []model.Node{cond("c", step)}, wantPath: "$.nodes[0].then[0]"},
		{name: "in_a_parallel_outside_a_loop", nodes: []model.Node{{ID: "par", Type: "parallel", Branches: []model.Branch{
			{ID: "one", Nodes: []model.Node{step}}, {ID: "two", Nodes: []model.Node{note}}}}}, wantPath: "$.nodes[0].branches[0].nodes[0]"},
		{name: "after_a_loop_not_in_it", nodes: []model.Node{batch(note), step}, wantPath: "$.nodes[1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			f.flow(&c, "x", tt.nodes...)

			got := Validate(Target{Collections: []model.Collection{c}})

			if tt.wantPath == "" {
				if len(got) != 0 {
					t.Fatalf("a loop control in a loop was refused: %v", codesOf(got))
				}
				return
			}
			if d := only(t, got, diag.CodeNodeLoopControl); d.Path != tt.wantPath {
				t.Fatalf("path %q, want %q", d.Path, tt.wantPath)
			}
		})
	}
}

func TestLoopControl_Given_a_bad_action_Then_FLW_NODE_005(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	step := model.Node{ID: "out", Type: "loop_control", Params: params("action", "break")}
	f.flow(&c, "x", model.Node{ID: "each", Type: "batch", Params: []model.Field{{Key: "times", Value: int64(2)}}, Body: []model.Node{step}})

	got := Validate(Target{Collections: []model.Collection{c}})

	if d := only(t, got, diag.CodeNodeParam); d.Path != "$.nodes[0].body[0].action" {
		t.Fatalf("path %q", d.Path)
	}
}
