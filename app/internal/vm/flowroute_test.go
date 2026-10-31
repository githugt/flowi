// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi/lib/template"
)

// matches is the index of the first case whose test is true when step b ends
// with status, and -1 when none is (the else lane).
func matches(t *testing.T, whens []string, status int) int {
	t.Helper()
	scope := template.Scope{Values: map[string]any{"nodes": map[string]any{"b": map[string]any{"status": status}}}}
	for i, when := range whens {
		ok, _, err := template.Check(when, scope)
		if err != nil {
			t.Fatalf("case %d %q does not run: %v", i, when, err)
		}
		if ok {
			return i
		}
	}
	return -1
}

func TestAddRouteByStatus_Then_four_ports_and_valid_expressions_are_made(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)
	d.UseGraphView()

	// Act
	id, err := d.AddRouteByStatus("b")
	n, _ := d.Node(id)
	var whens []string
	for _, c := range n.Cases {
		whens = append(whens, c.When)
	}

	// Assert
	if err != nil || n.Type != "condition" || len(n.Cases) != 3 {
		t.Fatalf("id %q err %v node %+v", id, err, n)
	}
	if ports := d.Cards()[id].Ports; len(ports) != 4 || ports[0].Label != "== 200" || ports[1].Label != "== 404" || ports[2].Label != "5xx" || ports[3].Label != "else" {
		t.Errorf("ports %+v", ports)
	}
	for status, want := range map[int]int{200: 0, 404: 1, 500: 2, 503: 2, 599: 2, 301: -1, 600: -1} {
		if got := matches(t, whens, status); got != want {
			t.Errorf("status %d took case %d, want %d", status, got, want)
		}
	}
}

func TestAddRouteByStatus_Then_it_follows_the_step_it_tests(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)

	// Act
	id, _ := d.AddRouteByStatus("a")

	// Assert
	if got := nodeOrder(d); got != "a,"+id+",b,p" {
		t.Fatalf("order %q", got)
	}
}

func TestAddRouteByStatus_Given_undo_Then_the_condition_is_removed(t *testing.T) {
	// Arrange
	d, _, _ := newFlowDoc(t)
	_, _ = d.AddRouteByStatus("b")

	// Act
	d.Undo()

	// Assert
	if got := nodeOrder(d); got != "a,b,p" {
		t.Fatalf("order %q", got)
	}
}

func TestAddRouteByStatus_Given_an_unknown_step_Then_it_is_refused(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	if _, err := d.AddRouteByStatus("nope"); err == nil {
		t.Fatal("expected an error")
	}
}
