// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

func TestWrap_Given_two_steps_and_a_loop_Then_they_sit_inside_it_in_order_and_undo_puts_them_back(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	id, _, err := d.Wrap([]string{"b", "a"}, "loop")
	box, _ := d.Node(id)
	order := nodeOrder(d)
	d.Undo()

	// Assert
	// Before: selecting several steps offered only Delete; wrapping them meant dragging each one into a new container.
	if err != nil || box.Type != "batch" || order != id+",p" || !slices.Equal(ids(box.Body), []string{"a", "b"}) {
		t.Fatalf("id %q err %v type %q order %q body %v", id, err, box.Type, order, ids(box.Body))
	}
	if after := nodeOrder(d); after != "a,b,p" {
		t.Errorf("after undo %q", after)
	}
}

func TestWrap_Given_a_group_Then_the_steps_fill_its_first_lane_and_the_second_is_empty(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	id, _, err := d.Wrap([]string{"a", "b"}, "group")
	box, _ := d.Node(id)

	// Assert
	if err != nil || box.Type != "parallel" || len(box.Branches) != 2 || !slices.Equal(ids(box.Branches[0].Nodes), []string{"a", "b"}) || len(box.Branches[1].Nodes) != 0 {
		t.Fatalf("err %v box %+v", err, box)
	}
}

func TestWrap_Given_a_route_Then_the_steps_are_in_a_first_case_that_is_always_true(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	id, _, err := d.Wrap([]string{"b"}, "route")
	box, _ := d.Node(id)

	// Assert
	if err != nil || box.Type != "condition" || len(box.Cases) != 1 || box.Cases[0].When != "true" || !slices.Equal(ids(box.Cases[0].Nodes), []string{"b"}) {
		t.Fatalf("err %v box %+v", err, box)
	}
}

func TestWrap_Given_arrows_across_the_run_Then_incoming_ones_move_to_the_container_and_later_steps_wait_on_it(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)
	d.EditNode("b", func(n *model.Node) { n.Needs = []string{"a"} })
	d.EditNode("p", func(n *model.Node) { n.Needs = []string{"b"}; n.AfterFailureOf = []string{"b"} })

	// Act
	id, _, err := d.Wrap([]string{"b"}, "loop")
	box, _ := d.Node(id)
	inner, _ := d.Node("b")
	p, _ := d.Node("p")

	// Assert
	if err != nil || !slices.Equal(box.Needs, []string{"a"}) || len(inner.Needs) != 0 {
		t.Fatalf("err %v container needs %v inner needs %v", err, box.Needs, inner.Needs)
	}
	if !slices.Equal(p.Needs, []string{id}) || !slices.Equal(p.AfterFailureOf, []string{id}) {
		t.Errorf("the step after waits on %v / %v, want %s", p.Needs, p.AfterFailureOf, id)
	}
}

func TestWrap_Given_arrows_between_the_wrapped_steps_Then_they_stay(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)
	d.EditNode("b", func(n *model.Node) { n.Needs = []string{"a"} })

	// Act
	id, _, err := d.Wrap([]string{"a", "b"}, "loop")
	box, _ := d.Node(id)
	b, _ := d.Node("b")

	// Assert
	if err != nil || len(box.Needs) != 0 || !slices.Equal(b.Needs, []string{"a"}) {
		t.Fatalf("err %v container needs %v b needs %v", err, box.Needs, b.Needs)
	}
}

func TestWrap_Given_steps_that_are_not_next_to_each_other_Then_it_is_refused_and_nothing_changes(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	_, _, err := d.Wrap([]string{"a", "p"}, "loop")

	// Assert
	if !errors.Is(err, vm.ErrWrapApart) || nodeOrder(d) != "a,b,p" {
		t.Fatalf("err %v order %q", err, nodeOrder(d))
	}
}

func TestWrap_Given_steps_in_different_lists_Then_it_is_refused(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	_, _, err := d.Wrap([]string{"a", "x"}, "group")

	// Assert
	if !errors.Is(err, vm.ErrWrapApart) {
		t.Fatalf("err %v", err)
	}
}

func TestWrap_Given_a_step_outside_that_reads_a_wrapped_result_Then_it_is_named(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)
	d.EditNode("b", func(n *model.Node) { n.When = "nodes.a.status == 200" })

	// Act
	_, readBy, err := d.Wrap([]string{"a"}, "loop")

	// Assert
	if err != nil || !slices.Equal(readBy, []string{"b"}) {
		t.Fatalf("err %v readBy %v", err, readBy)
	}
}

func TestWrap_Given_a_kind_that_is_not_one_or_no_steps_Then_it_is_refused(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	_, _, badKind := d.Wrap([]string{"a"}, "pile")
	_, _, none := d.Wrap(nil, "loop")

	// Assert
	if badKind == nil || none == nil || d.CanUndo() {
		t.Fatalf("badKind %v none %v canUndo %v", badKind, none, d.CanUndo())
	}
}
