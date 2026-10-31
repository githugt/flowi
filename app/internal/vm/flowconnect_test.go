// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"
)

func TestFlowDoc_CanRead_Given_two_steps_Then_only_an_earlier_one_in_reach_may_be_read(t *testing.T) {
	d, _, _ := newFlowDoc(t) // a, b, then p with b1: x and b2: y
	tests := []struct {
		name     string
		from, to string
		ok       bool
		reason   string
	}{
		{"earlier_step_of_the_same_list", "a", "b", true, ""},
		{"earlier_step_of_an_enclosing_list", "a", "x", true, ""},
		{"later_step", "b", "a", false, "after"},
		{"the_same_step", "a", "a", false, "itself"},
		{"a_step_in_another_lane", "x", "y", false, "another lane"},
		{"a_container_it_is_inside", "p", "x", false, "inside"},
		{"a_step_inside_a_container_read_after_it", "x", "y", false, "another lane"},
		{"an_unknown_step", "ghost", "b", false, "no step"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := d.CanRead(tt.from, tt.to)

			if ok != tt.ok || !strings.Contains(reason, tt.reason) {
				t.Fatalf("CanRead(%s, %s) = %v, %q; want %v containing %q", tt.from, tt.to, ok, reason, tt.ok, tt.reason)
			}
		})
	}
}

func TestFlowDoc_CanRead_Given_a_step_after_a_container_Then_it_may_read_a_step_inside_it(t *testing.T) {
	d, _, _ := newFlowDoc(t)
	d.Insert("log", "$.nodes", 99, nil) // after p

	ok, reason := d.CanRead("x", "log")

	if !ok {
		t.Fatalf("a step after the container cannot read x: %q", reason)
	}
}

func TestFlowDoc_CanRead_Given_a_container_reading_a_step_inside_it_Then_it_is_refused_naming_both(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	ok, reason := d.CanRead("x", "p")

	if ok || !strings.Contains(reason, "x is inside p") {
		t.Fatalf("got %v %q", ok, reason)
	}
}
