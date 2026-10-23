// SPDX-License-Identifier: AGPL-3.0-or-later

package canvas

import "math"

// Where a dragged card or unit lands. These rules are pure functions of the
// diagram so every front end drops things in the same places.

// StripReach is how far above or below a strip (canvas units) the pointer
// still counts as over it: dragging sideways is not exact about the height.
const StripReach = 30

// Locator says where a node is now: its list's path and its index. A new
// node (from a palette) is not found.
type Locator func(node string) (list string, index int, found bool)

// Slot is where a dropped card would land: the node list and index, and
// where the marker is drawn (canvas units).
type Slot struct {
	List  string `json:"list"`
	Index int    `json:"index"`
	X     int64  `json:"x"`
	Y     int64  `json:"y"`
	H     int64  `json:"h"`
	OK    bool   `json:"ok"`
	// Attach is, for a slot inside a request block, the request the dropped
	// step is drawn with from then on; "" for a plain place in a list.
	Attach string `json:"attach,omitempty"`
	// Side is where around that request it is drawn: "above", "below" or "" beside it.
	Side string `json:"side,omitempty"`
	// Horizontal marks a slot in a stack above or below a request: its marker is
	// a line W wide at (X, Y), not a bar H high.
	Horizontal bool  `json:"horizontal,omitempty"`
	W          int64 `json:"w,omitempty"`
}

// SlotIn is the gap of strip u nearest canvas x: 0 before its first card, one
// more after each card, so len(u.Nodes) after the last.
func SlotIn(u Unit, x int64) int {
	gap := math.Round(float64(x-u.X-StripPad/2) / ActionW)
	return int(max(0, min(float64(len(u.Nodes)), gap)))
}

// CardTarget is the slot a card of node dropped at p takes: a gap of the
// strip p is over, else the place between or on the nodes p is at; !OK in
// empty space and where the card already is. snap is how close (canvas
// units) a drop must be to an edge.
func CardTarget(d Diagram, p Point, node string, snap float64, at Locator) Slot {
	list, index, ok := cardPlace(d, p, node, snap, at)
	if !ok {
		return Slot{}
	}
	return slotAt(d, list, index)
}

// UnitTarget is the slot a whole unit dragged with its centre at p would
// take: on or between other nodes, or onto a connector.
func UnitTarget(d Diagram, p Point, unit, node string, snap float64, at Locator) Slot {
	list, index, ok := ReorderTarget(d, p, unit, node, snap, at)
	if !ok {
		return Slot{}
	}
	return slotAt(d, list, index)
}

func slotAt(d Diagram, list string, index int) Slot {
	x, y, h, ok := SlotMarker(d, list, index)
	return Slot{List: list, Index: index, X: x, Y: y, H: h, OK: ok}
}

func cardPlace(d Diagram, p Point, node string, snap float64, at Locator) (list string, index int, ok bool) {
	for _, u := range d.Units {
		if u.Kind != KindStrip || u.Collapsed || p.X < u.X || p.X > u.X+u.W || p.Y < u.Y-StripReach || p.Y > u.Y+u.H+StripReach {
			continue
		}
		first, i, split := SplitPath(u.Path)
		if !split {
			continue
		}
		list, index = first, i+SlotIn(u, p.X)
		return list, index, !alreadyThere(node, list, index, at)
	}
	return ReorderTarget(d, p, "", node, snap, at)
}

// alreadyThere says whether putting node at index of list leaves it where it is.
func alreadyThere(node, list string, index int, at Locator) bool {
	from, i, found := at(node)
	return found && from == list && (index == i || index == i+1)
}

// SlotMarker is where to draw the slot of (list, index): at the gap before
// the unit that holds that index (a strip's own gap), or after the last unit.
func SlotMarker(d Diagram, list string, index int) (x, y, h int64, ok bool) {
	const gap = 30 // half the distance between two units
	for _, u := range d.Units {
		l, i, split := SplitPath(u.Path)
		if !split || l != list {
			continue
		}
		switch {
		case index >= i && index < i+len(u.Nodes) && u.Kind == KindStrip && !u.Collapsed:
			return u.X + StripPad/2 + int64(index-i)*ActionW, u.Y, u.H, true
		case index == i:
			return u.X - gap, u.Y, u.H, true
		case index == i+len(u.Nodes):
			x, y, h, ok = u.X+u.W+gap, u.Y, u.H, true // after the last: kept unless a later unit takes it
		}
	}
	if !ok { // an empty lane has no unit to be beside: mark the lane itself
		for _, l := range d.Lanes {
			if l.Path == list && index == LaneLength(d, l) {
				return l.X + 8, l.Y, max(l.H, 40), true
			}
		}
	}
	return x, y, h, ok
}

// laneEnd is the end of the lane p is in, unless the lane belongs to the unit
// being dragged.
func laneEnd(d Diagram, p Point, unit string) (list string, index int, ok bool) {
	lane, found := LaneAt(d, p)
	if !found || lane.Container == unit {
		return "", 0, false
	}
	return lane.Path, LaneLength(d, lane), true
}

// inLaneRoom reports whether p is inside a lane (not that of the unit being
// dragged) and not on one of that lane's steps.
func inLaneRoom(d Diagram, p Point, unit string) bool {
	lane, found := LaneAt(d, p)
	if !found || lane.Container == unit {
		return false
	}
	on, over := UnitAt(d, p)
	return !over || on.ID == unit || on.Parent != lane.Container || on.Lane != lane.Index
}

// ReorderTarget is the place in the node lists a unit dropped with its
// centre at p takes, when that is a place other than the one it has: on or
// between other nodes, or onto a connector. ok is false in empty space, and
// when the place is where the node already is.
func ReorderTarget(d Diagram, p Point, unit, node string, snap float64, at Locator) (list string, index int, ok bool) {
	if e, found := NearestEdge(d, p, snap, unit); found {
		list, index, ok = InsertAt(d, e)
	}
	if !ok && inLaneRoom(d, p, unit) {
		// Inside a lane, away from its steps: the end of that lane. The gap
		// matching below also reaches into a container from the connector
		// that leaves it, and would turn a drop here into "already there".
		list, index, ok = laneEnd(d, p, unit)
	}
	if !ok {
		list, index, ok = InsertNear(d, p, unit)
	}
	if !ok {
		list, index, ok = EndNear(d, p, unit) // before the first or after the last
	}
	if !ok {
		list, index, ok = laneEnd(d, p, unit) // an empty lane, or the room in a lane
	}
	if !ok {
		return "", 0, false
	}
	if alreadyThere(node, list, index, at) {
		return "", 0, false // already there
	}
	return list, index, true
}

// RequestAt is the request block (a request or subflow unit, with or without
// attached actions) containing p, other than exclude.
func RequestAt(d Diagram, p Point, exclude string) (Unit, bool) {
	for _, u := range d.Units {
		if u.Kind != KindRequest || u.ID == exclude || p.X < u.X || p.X > u.X+u.W || p.Y < u.Y || p.Y > u.Y+u.H {
			continue
		}
		return u, true
	}
	return Unit{}, false
}

// CellsOf are where the nodes of a request block are drawn, relative to its
// top-left corner: the unit's own cells, or the one cell of a request with
// nothing drawn with it.
func CellsOf(u Unit) []Cell {
	if len(u.Cells) == len(u.Nodes) && len(u.Cells) > 0 {
		return u.Cells
	}
	return []Cell{{W: requestW, H: requestH}}
}

// CellAt is the index in u.Nodes of the cell of a request block that p (relative
// to the block's top-left corner) is in, or the nearest one when p is in a gap.
func CellAt(u Unit, p Point) int {
	best, bestDist := 0, int64(-1)
	for i, c := range CellsOf(u) {
		dx := max(c.X-p.X, 0, p.X-(c.X+c.W))
		dy := max(c.Y-p.Y, 0, p.Y-(c.Y+c.H))
		if d := dx*dx + dy*dy; bestDist < 0 || d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}
