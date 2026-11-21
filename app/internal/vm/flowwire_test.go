// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
)

// Before: a link from a step in a branch to one outside it was accepted and never drawn.
func TestFlowDoc_CanWait_Should_accept_only_an_earlier_step_of_the_same_list(t *testing.T) {
	d, _, _ := newFlowDoc(t) // a, b, p{b1: x, b2: y}

	cases := []struct {
		from, to string
		ok       bool
		reason   string
	}{
		{"a", "b", true, ""},
		{"a", "p", true, ""},
		{"b", "a", false, "b comes after a"},
		{"a", "a", false, "cannot wait for itself"},
		{"a", "x", false, "different lists"},
		{"x", "b", false, "different lists"},
		{"x", "y", false, "different lists"},
		{"a", "nope", false, "there is no step nope"},
	}
	for _, c := range cases {
		ok, why := d.CanWait(c.from, c.to)
		if ok != c.ok || (c.reason != "" && !strings.Contains(why, c.reason)) {
			t.Errorf("%s → %s: ok %v, %q", c.from, c.to, ok, why)
		}
	}
}

func TestFlowDoc_WireTargets_Should_list_every_step_including_those_in_branches(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	got := map[string]bool{}
	for _, tg := range d.WireTargets("a", vm.WireWait) {
		got[tg.ID] = tg.OK
	}
	data := map[string]bool{}
	for _, tg := range d.WireTargets("a", vm.WireData) {
		data[tg.ID] = tg.OK
	}

	if len(got) != 5 || !got["b"] || !got["p"] || got["x"] || got["y"] || got["a"] {
		t.Fatalf("wait targets %v", got)
	}
	if !data["b"] || !data["x"] || data["a"] { // a value, unlike an arrow, reaches into a branch that comes after
		t.Fatalf("data targets %v", data)
	}
}
