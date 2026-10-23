// SPDX-License-Identifier: AGPL-3.0-or-later

package canvas

import (
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func sampleDiagram() Diagram {
	f := model.Flow{Nodes: []model.Node{
		{ID: "a", Type: "request"},
		{ID: "p", Type: "parallel", Branches: []model.Branch{{ID: "b1", Nodes: []model.Node{{ID: "x", Type: "fetch"}}}}},
		{ID: "z", Type: "request"},
	}}
	return Layout(f, nil, nil)
}

func TestUnitAt_should_prefer_the_innermost_unit(t *testing.T) {
	d := sampleDiagram()
	x, _ := d.Unit("x")
	p, _ := d.Unit("p")

	inner, okInner := UnitAt(d, Point{X: x.X + 5, Y: x.Y + 5})
	box, okBox := UnitAt(d, Point{X: p.X + 2, Y: p.Y + 2})
	_, none := UnitAt(d, Point{X: -500, Y: -500})

	if !okInner || inner.ID != "x" || !okBox || box.ID != "p" || none {
		t.Fatalf("inner %v %v box %v %v none %v", inner.ID, okInner, box.ID, okBox, none)
	}
}

func TestNearestEdge_and_InsertAt(t *testing.T) {
	d := sampleDiagram()
	a, _ := d.Unit("a")
	p, _ := d.Unit("p")
	mid := Point{X: (a.X + a.W + p.X) / 2, Y: a.PortY()}

	e, ok := NearestEdge(d, mid, 20, "")
	list, index, insertOK := InsertAt(d, e)
	_, far := NearestEdge(d, Point{X: mid.X, Y: mid.Y + 500}, 20, "")

	if !ok || e.From != "a" || e.To != "p" || far {
		t.Fatalf("edge %+v ok %v far %v", e, ok, far)
	}
	if !insertOK || list != "$.nodes" || index != 1 {
		t.Fatalf("insert at %q %d %v", list, index, insertOK)
	}
}

func TestNearestEdge_skips_edges_of_the_excluded_unit(t *testing.T) {
	d := sampleDiagram()
	a, _ := d.Unit("a")
	p, _ := d.Unit("p")
	mid := Point{X: (a.X + a.W + p.X) / 2, Y: a.PortY()}

	_, ok := NearestEdge(d, mid, 20, "a")

	if ok {
		t.Fatal("an edge of the dragged unit was offered")
	}
}

func TestSplitPath(t *testing.T) {
	list, i, ok := SplitPath("$.nodes[2].branches[0].nodes[3]")

	if !ok || list != "$.nodes[2].branches[0].nodes" || i != 3 {
		t.Fatalf("%q %d %v", list, i, ok)
	}
}

func TestEdgePath_into_a_lane_starts_inside_the_container(t *testing.T) {
	d := sampleDiagram()
	p, _ := d.Unit("p")

	path := EdgePath(d, Edge{From: "p", To: "x"})

	if path[0].X >= p.X+p.W/2 {
		t.Fatalf("starts at %+v, container spans %d..%d", path[0], p.X, p.X+p.W)
	}
}

// Before: a drop only snapped to the thin connector line (within 24 points),
// so dropping between two nodes, or on one, appended at the end. The flow
// runs left to right.
func TestInsertNear_should_insert_before_or_after_the_node_under_the_pointer_or_between_two_nodes(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "a", Type: "request"}, {ID: "b", Type: "request"}, {ID: "c", Type: "request"}}}
	d := Layout(f, nil, nil)
	a, _ := d.Unit("a")
	b, _ := d.Unit("b")
	tests := []struct {
		name  string
		at    Point
		index int
		ok    bool
	}{
		{"first half of b goes before b", Point{X: b.X + 2, Y: b.Y + b.H/2}, 1, true},
		{"second half of b goes after b", Point{X: b.X + b.W - 2, Y: b.Y + b.H/2}, 2, true},
		{"in the gap, far above the connector", Point{X: (a.X + a.W + b.X) / 2, Y: b.Y - 300}, 1, true},
		{"far beyond everything", Point{X: b.X + 5000, Y: b.Y}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, index, ok := InsertNear(d, tt.at, "")

			if ok != tt.ok || (ok && (list != "$.nodes" || index != tt.index)) {
				t.Fatalf("got %q %d %v, want $.nodes %d %v", list, index, ok, tt.index, tt.ok)
			}
		})
	}
}

// Behaviour change: dropping on the right half of a unit of several nodes (a
// strip, or a request with attached actions) counted as "after its first
// node", a place inside it that has no marker; it is after the whole unit.
func TestInsertNear_should_put_a_drop_on_a_multi_node_unit_before_or_after_the_whole_unit(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "pre", Type: "set", Attach: "r"}, {ID: "r", Type: "request"}, {ID: "post", Type: "assert", Attach: "r"},
		{ID: "next", Type: "fetch"},
	}}
	d := Layout(f, nil, nil, WithAttached())
	block, _ := d.UnitOf("r")
	left := Point{X: block.X + 10, Y: block.Y + block.H/2}
	right := Point{X: block.X + block.W - 10, Y: block.Y + block.H/2}

	_, before, okBefore := InsertNear(d, left, "")
	list, after, okAfter := InsertNear(d, right, "")
	x, _, _, marked := SlotMarker(d, list, after)

	if !okBefore || before != 0 || !okAfter || after != 3 || !marked || x <= block.X+block.W {
		t.Fatalf("before %d (%v), after %d (%v), marker x %d (%v), block %+v", before, okBefore, after, okAfter, x, marked, block)
	}
}
