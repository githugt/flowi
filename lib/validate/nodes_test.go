// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
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
