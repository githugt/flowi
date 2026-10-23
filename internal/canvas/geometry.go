// SPDX-License-Identifier: AGPL-3.0-or-later

package canvas

import (
	"math"
	"strconv"
	"strings"
)

// curveSteps is how finely S-curves are flattened.
const curveSteps = 24

// EdgePath is an edge's path in canvas units: a straight segment between
// ports on the same row, otherwise a cubic S-curve flattened to a
// polyline.
func EdgePath(d Diagram, e Edge) []Point {
	from, _ := d.Unit(e.From)
	to, _ := d.Unit(e.To)
	p0 := Point{X: from.X + from.W, Y: from.PortY()}
	if to.Parent == from.ID {
		p0.X = from.X + lanePad/2 // into a lane: from the container's left, not back across it
	}
	p3 := Point{X: to.X, Y: to.PortY()}
	if p0.Y == p3.Y {
		return []Point{p0, p3}
	}
	k := max(lanePad/2, (p3.X-p0.X)/2)
	out := make([]Point, 0, curveSteps+1)
	for i := 0; i <= curveSteps; i++ {
		t := float64(i) / curveSteps
		u := 1 - t
		x := u*u*u*float64(p0.X) + 3*u*u*t*float64(p0.X+k) + 3*u*t*t*float64(p3.X-k) + t*t*t*float64(p3.X)
		y := u*u*u*float64(p0.Y) + 3*u*u*t*float64(p0.Y) + 3*u*t*t*float64(p3.Y) + t*t*t*float64(p3.Y)
		out = append(out, Point{X: int64(math.Round(x)), Y: int64(math.Round(y))})
	}
	return out
}

// UnitAt is the innermost unit containing p.
func UnitAt(d Diagram, p Point) (Unit, bool) {
	var best Unit
	found := false
	for _, u := range d.Units {
		if p.X < u.X || p.Y < u.Y || p.X > u.X+u.W || p.Y > u.Y+u.H {
			continue
		}
		if !found || u.W*u.H < best.W*best.H {
			best, found = u, true
		}
	}
	return best, found
}

// NearestEdge is the edge closest to p within maxDist, skipping edges
// that touch the unit exclude (the one being dragged).
func NearestEdge(d Diagram, p Point, maxDist float64, exclude string) (Edge, bool) {
	var best Edge
	bestDist := maxDist
	found := false
	for _, e := range d.Edges {
		if e.Fail || e.From == exclude || e.To == exclude { // a fail route is not a place to drop on
			continue
		}
		if dist := distToPath(p, EdgePath(d, e)); dist <= bestDist {
			best, bestDist, found = e, dist, true
		}
	}
	return best, found
}

func distToPath(p Point, path []Point) float64 {
	best := math.MaxFloat64
	for i := 1; i < len(path); i++ {
		best = min(best, distToSegment(p, path[i-1], path[i]))
	}
	return best
}

func distToSegment(p, a, b Point) float64 {
	px, py := float64(p.X), float64(p.Y)
	ax, ay, bx, by := float64(a.X), float64(a.Y), float64(b.X), float64(b.Y)
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l2 := dx*dx + dy*dy; l2 > 0 {
		t = max(0, min(1, ((px-ax)*dx+(py-ay)*dy)/l2))
	}
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// InsertAt is where a node dropped on edge e goes: just before the edge's
// target unit, in that unit's list.
func InsertAt(d Diagram, e Edge) (string, int, bool) {
	to, ok := d.Unit(e.To)
	if !ok {
		return "", 0, false
	}
	return SplitPath(to.Path)
}

// SplitPath splits a node's YAML path into its list's path and its index:
// "$.nodes[2].body[1]" → "$.nodes[2].body", 1.
func SplitPath(path string) (string, int, bool) {
	open := strings.LastIndex(path, "[")
	if open < 0 || !strings.HasSuffix(path, "]") {
		return "", 0, false
	}
	i, err := strconv.Atoi(path[open+1 : len(path)-1])
	if err != nil {
		return "", 0, false
	}
	return path[:open], i, true
}

// LaneAt is the innermost lane containing p.
func LaneAt(d Diagram, p Point) (Lane, bool) {
	var best Lane
	found := false
	for _, l := range d.Lanes {
		if p.X < l.X || p.Y < l.Y || p.X > l.X+l.W || p.Y > l.Y+l.H {
			continue
		}
		if !found || l.W*l.H < best.W*best.H {
			best, found = l, true
		}
	}
	return best, found
}

// LaneLength is how many nodes the lane's list has (its units' nodes).
func LaneLength(d Diagram, l Lane) int {
	n := 0
	for _, u := range d.Units {
		if u.Parent == l.Container && u.Lane == l.Index {
			n += len(u.Nodes)
		}
	}
	return n
}

// endBand is how far (canvas units) above or below the row of top-level units
// a drop still counts as at the start or the end of it.
const endBand = 80

// EndNear is the start or the end of the top-level node list when p is beyond
// the first unit or past the last one (along the flow) and near their row: the
// places InsertNear cannot name, since there is no unit or connector there.
// exclude is a dragged unit, left out of what counts as first and last.
func EndNear(d Diagram, p Point, exclude string) (list string, index int, ok bool) {
	horizontal := runsHorizontally(d)
	along := func(pt Point) int64 {
		if horizontal {
			return pt.X
		}
		return pt.Y
	}
	across := func(pt Point) int64 {
		if horizontal {
			return pt.Y
		}
		return pt.X
	}
	var first, last Unit
	var rowLo, rowHi int64
	found := false
	for _, u := range d.Units {
		if u.Parent != "" || u.ID == exclude {
			continue
		}
		lo, hi := Point{X: u.X, Y: u.Y}, Point{X: u.X + u.W, Y: u.Y + u.H}
		if !found {
			first, last, rowLo, rowHi, found = u, u, across(lo), across(hi), true
			continue
		}
		if along(lo) < along(Point{X: first.X, Y: first.Y}) {
			first = u
		}
		if along(hi) > along(Point{X: last.X + last.W, Y: last.Y + last.H}) {
			last = u
		}
		rowLo, rowHi = min(rowLo, across(lo)), max(rowHi, across(hi))
	}
	if !found || across(p) < rowLo-endBand || across(p) > rowHi+endBand {
		return "", 0, false
	}
	if along(p) < along(Point{X: first.X, Y: first.Y}) {
		l, i, split := SplitPath(first.Path)
		return l, i, split
	}
	if along(p) > along(Point{X: last.X + last.W, Y: last.Y + last.H}) {
		l, i, split := SplitPath(last.Path)
		return l, i + len(last.Nodes), split
	}
	return "", 0, false
}

// gapMargin is how far beyond the gap between two nodes (along the flow) a
// drop still counts as "between them".
const gapMargin = 8

// InsertNear is where something dropped at p goes in the node lists, judged
// by position and not only by closeness to a connector line: on a node,
// before it in its first half and after it in its second half (along the
// direction the flow runs); else in the gap between two nodes, whatever the
// distance to the side. ok is false when p is neither (the caller appends or
// uses the lane). Containers are left to their lanes. exclude is a dragged
// unit.
func InsertNear(d Diagram, p Point, exclude string) (list string, index int, ok bool) {
	horizontal := runsHorizontally(d)
	along := func(pt Point) int64 {
		if horizontal {
			return pt.X
		}
		return pt.Y
	}
	if u, found := UnitAt(d, p); found && u.ID != exclude && u.Kind != KindContainer && !isLaneOwner(d, u.ID) {
		if l, i, split := SplitPath(u.Path); split {
			mid := Point{X: u.X + u.W/2, Y: u.Y + u.H/2}
			if along(p) >= along(mid) {
				i += len(u.Nodes) // after the whole unit: a strip or a request block is not split by a drop
			}
			return l, i, true
		}
	}
	var best Edge
	bestDist, found := math.MaxFloat64, false
	for _, e := range d.Edges {
		if e.From == exclude || e.To == exclude {
			continue
		}
		path := EdgePath(d, e)
		lo, hi := along(path[0]), along(path[0])
		for _, pt := range path {
			lo, hi = min(lo, along(pt)), max(hi, along(pt))
		}
		if along(p) < lo-gapMargin || along(p) > hi+gapMargin {
			continue
		}
		// The middle of the gap decides: how far to the side the pointer is
		// says nothing about which two nodes it is between.
		if dist := math.Abs(float64(along(p)) - float64(lo+hi)/2); dist < bestDist {
			best, bestDist, found = e, dist, true
		}
	}
	if !found {
		return "", 0, false
	}
	return InsertAt(d, best)
}

// runsHorizontally reports whether the flow runs left to right (as laid out
// today) rather than top to bottom: the first edge is wider than it is tall.
func runsHorizontally(d Diagram) bool {
	for _, e := range d.Edges {
		path := EdgePath(d, e)
		first, last := path[0], path[len(path)-1]
		return math.Abs(float64(last.X-first.X)) >= math.Abs(float64(last.Y-first.Y))
	}
	return true
}

func isLaneOwner(d Diagram, unit string) bool {
	for _, l := range d.Lanes {
		if l.Container == unit {
			return true
		}
	}
	return false
}
