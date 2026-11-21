// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func logStep(id, value string) model.Node {
	return model.Node{ID: id, Type: "log", Params: []model.Field{{Key: "value", Value: value}}}
}

func TestDescribeChange_Given_two_flows_When_compared_Then_the_label_says_what_changed(t *testing.T) {
	// Before: Undo and Redo showed only on or off.
	base := model.Flow{Nodes: []model.Node{logStep("a", "1"), logStep("b", "2")}}
	tests := []struct {
		name string
		to   model.Flow
		want string
	}{
		{"one step added", model.Flow{Nodes: []model.Node{logStep("a", "1"), logStep("b", "2"), logStep("c", "3")}}, "add step c"},
		{"two added", model.Flow{Nodes: []model.Node{logStep("a", "1"), logStep("b", "2"), logStep("c", "3"), logStep("d", "4")}}, "add 2 steps"},
		{"one deleted", model.Flow{Nodes: []model.Node{logStep("a", "1")}}, "delete step b"},
		{"all deleted", model.Flow{}, "delete 2 steps"},
		{"one edited", model.Flow{Nodes: []model.Node{logStep("a", "changed"), logStep("b", "2")}}, "edit step a"},
		{"both edited", model.Flow{Nodes: []model.Node{logStep("a", "x"), logStep("b", "y")}}, "edit 2 steps"},
		{"settings", model.Flow{Nodes: base.Nodes, Inputs: []model.Input{{Name: "x"}}}, "edit the flow's settings"},
		{"mixed", model.Flow{Nodes: []model.Node{logStep("a", "x")}}, "change the flow"},
		{"nothing", base, "change"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DescribeChange(base, tt.to)

			if got != tt.want {
				t.Errorf("DescribeChange = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDescribeChange_Given_a_step_inside_a_container_When_edited_Then_the_inner_step_is_named(t *testing.T) {
	inner := func(v string) model.Node {
		return model.Node{ID: "loop", Type: "batch", Body: []model.Node{logStep("in", v)}}
	}

	got := DescribeChange(model.Flow{Nodes: []model.Node{inner("1")}}, model.Flow{Nodes: []model.Node{inner("2")}})

	if got != "edit step in" {
		t.Errorf("DescribeChange = %q, want the inner step, not the container", got)
	}
}

func TestFlowDoc_Given_edits_When_asked_for_the_labels_Then_undo_and_redo_say_what_they_do(t *testing.T) {
	d := &FlowDoc{draft: model.Flow{Nodes: []model.Node{logStep("a", "1")}}}
	if d.UndoLabel() != "Undo" || d.RedoLabel() != "Redo" {
		t.Fatalf("with nothing to undo the labels are plain, got %q and %q", d.UndoLabel(), d.RedoLabel())
	}

	_ = d.command(func(f *model.Flow) error { f.Nodes = append(f.Nodes, logStep("b", "2")); return nil })
	if got := d.UndoLabel(); got != "Undo: add step b" {
		t.Errorf("UndoLabel = %q", got)
	}

	d.Undo()
	if got := d.RedoLabel(); got != "Redo: add step b" {
		t.Errorf("RedoLabel = %q", got)
	}
}
