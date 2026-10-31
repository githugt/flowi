// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
)

func TestSetColor_Given_a_valid_name_Then_it_is_saved_and_undoable(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	err := d.SetColor("a", "amber")
	got, _ := d.Node("a")
	d.Undo()
	undone, _ := d.Node("a")

	// Assert
	if err != nil || got.Color != "amber" || undone.Color != "" {
		t.Fatalf("err %v colour %q after undo %q", err, got.Color, undone.Color)
	}
}

func TestSetColor_Given_an_unknown_name_Then_it_is_refused_and_nothing_changes(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	err := d.SetColor("a", "chartreuse")
	got, _ := d.Node("a")

	// Assert
	if err == nil || got.Color != "" || d.CanUndo() {
		t.Fatalf("err %v colour %q canUndo %v", err, got.Color, d.CanUndo())
	}
}

func TestSetColor_Given_an_empty_name_Then_the_category_colour_returns(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)
	_ = d.SetColor("a", "teal")

	// Act
	err := d.SetColor("a", "")
	got, _ := d.Node("a")

	// Assert
	if err != nil || got.Color != "" {
		t.Fatalf("err %v colour %q", err, got.Color)
	}
}

func TestSetColor_Given_an_unknown_step_Then_it_is_refused(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	if err := d.SetColor("nope", "teal"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDuplicate_Given_a_step_Then_a_copy_with_a_new_id_follows_it_and_undo_removes_it(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)
	_ = d.SetColor("b", "pink")

	// Act
	id, err := d.Duplicate("b")
	copyOf, _ := d.Node(id)
	order := nodeOrder(d)
	d.Undo()
	after := nodeOrder(d)

	// Assert
	if err != nil || id == "" || id == "b" {
		t.Fatalf("id %q err %v", id, err)
	}
	if copyOf.Type != "log" || copyOf.Color != "pink" {
		t.Errorf("the copy should be the same step: %+v", copyOf)
	}
	if order != "a,b,"+id+",p" || after != "a,b,p" {
		t.Errorf("order %q, after undo %q", order, after)
	}
}

func TestDuplicate_Given_a_container_Then_the_steps_inside_get_new_ids_too(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	id, err := d.Duplicate("p")
	ids := map[string]int{}
	for _, n := range d.NodeIDs() {
		ids[n]++
	}

	// Assert
	if err != nil || id == "" {
		t.Fatalf("id %q err %v", id, err)
	}
	for n, count := range ids {
		if count != 1 {
			t.Errorf("id %q is used %d times", n, count)
		}
	}
	if len(ids) != 12 { // a, b, p, its two lanes and x, y; and the copies of p, the lanes, x and y
		t.Errorf("ids %v", ids)
	}
}

// nodeOrder is the top-level step ids, comma separated.
func nodeOrder(d *vm.FlowDoc) string {
	ids := []string{}
	for _, n := range d.Draft().Nodes {
		ids = append(ids, n.ID)
	}
	return strings.Join(ids, ",")
}
