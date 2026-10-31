// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func chipKinds(chips []vm.Chip) []vm.ChipKind {
	var out []vm.ChipKind
	for _, c := range chips {
		out = append(out, c.Kind)
	}
	return out
}

func TestChipsFor_should_be_empty_when_nothing_is_wrong(t *testing.T) {
	got := vm.ChipsFor("", nil, nil, false, false)

	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestChipsFor_should_list_error_warning_disk_unsaved_in_that_order(t *testing.T) {
	got := vm.ChipsFor("boom", []string{"c needs b"}, []string{"a (log)"}, true, true)

	want := []vm.ChipKind{vm.ChipError, vm.ChipNeeds, vm.ChipWarning, vm.ChipDisk, vm.ChipUnsaved}
	if !slices.Equal(chipKinds(got), want) {
		t.Fatalf("got %v, want %v", chipKinds(got), want)
	}
}

func TestChipsFor_should_carry_the_error_text_and_the_flow_I_only_steps(t *testing.T) {
	got := vm.ChipsFor("boom", nil, []string{"a (log)", "b (set)"}, false, false)

	if got[0].Text != "boom" || !slices.Equal(got[1].Steps, []string{"a (log)", "b (set)"}) {
		t.Fatalf("got %+v", got)
	}
}

func TestFlowChips_should_show_unsaved_after_an_edit(t *testing.T) {
	d, _, _ := newFlowDoc(t)
	clean := vm.FlowChips(d)

	d.Insert("log", "$.nodes", 0, nil)
	edited := vm.FlowChips(d)

	if len(clean) != 0 || !slices.Contains(chipKinds(edited), vm.ChipUnsaved) {
		t.Fatalf("clean %v, edited %v", clean, edited)
	}
}

func TestNodeSummary_should_describe_request_subflow_and_parameter_nodes(t *testing.T) {
	tests := []struct {
		name string
		node model.Node
		want string
	}{
		{"request", model.Node{Type: "request", Request: &model.RequestCall{Request: model.Ref{Path: "auth/login"}}}, "auth/login"},
		{"request version", model.Node{Type: "request", Request: &model.RequestCall{Request: model.Ref{Path: "auth/login"}, Version: "2"}}, "auth/login v2"},
		{"subflow", model.Node{Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{Path: "login"}}}, "subflow login"},
		{"first parameter", model.Node{Type: "delay", Params: []model.Field{{Key: "duration", Value: "2s"}}}, "duration: 2s"},
		{"type only", model.Node{Type: "parallel"}, "parallel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vm.NodeSummary(tt.node)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestForwardNeeds_should_list_needs_on_a_later_node_and_RepairNeeds_should_drop_them(t *testing.T) {
	b, f := flowFixture()
	f.Nodes = []model.Node{
		{ID: "a", Type: "log"},
		{ID: "c", Type: "log", Needs: []string{"b"}},
		{ID: "b", Type: "log", Needs: []string{"a"}},
	}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()

	before := d.ForwardNeeds()
	d.RepairNeeds()

	if !slices.Equal(before, []string{"c needs b"}) || len(d.ForwardNeeds()) != 0 || !slices.Equal(d.Draft().Nodes[2].Needs, []string{"a"}) || !d.CanUndo() {
		t.Fatalf("before %v, after %v, needs of b %v", before, d.ForwardNeeds(), d.Draft().Nodes[2].Needs)
	}
}
