// SPDX-License-Identifier: AGPL-3.0-or-later

package canvas

import (
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/card"
)

func loopCards(ids ...string) map[string]card.Card {
	cards := map[string]card.Card{"loop": cardWithRows(1)}
	for _, id := range ids {
		cards[id] = cardWithRows(0)
	}
	return cards
}

// loopFlow is a loop of the given kind (batch or run_till) with the given body, then a step after it. layout is the
// loop's layout field: "" leaves it out.
func loopFlow(kind, layout string, body ...string) model.Flow {
	loop := model.Node{ID: "loop", Type: model.NodeType(kind)}
	if layout != "" {
		loop.Params = []model.Field{{Key: "layout", Value: layout}}
	}
	for _, id := range body {
		loop.Body = append(loop.Body, model.Node{ID: id, Type: "log"})
	}
	return model.Flow{Nodes: []model.Node{loop, {ID: "after", Type: "log"}}}
}

func layoutLoop(f model.Flow, ids ...string) Diagram {
	return Layout(f, nil, nil, WithCards(loopCards(append(ids, "after")...)))
}

func inside(step Unit, box Lane) bool {
	return step.X >= box.X && step.X+step.W <= box.X+box.W && step.Y >= box.Y && step.Y+step.H <= box.Y+box.H
}

func TestLayout_Given_a_loop_without_a_layout_Then_its_steps_are_stacked_in_a_box_under_its_card(t *testing.T) {
	// Before: the steps were laid out to the right of the loop's card, in line with the flow, so they read as the next steps.
	for _, kind := range []string{"batch", "run_till"} {
		for _, layout := range []string{"", "vertical"} {
			d := layoutLoop(loopFlow(kind, layout, "a", "b"), "a", "b")

			loop, _ := d.Unit("loop")
			a, _ := d.Unit("a")
			b, _ := d.Unit("b")
			if len(d.Lanes) != 1 {
				t.Fatalf("%s %q: %d lanes, want 1", kind, layout, len(d.Lanes))
			}
			box := d.Lanes[0]
			if !loop.Boxed || !loop.Down || loop.HeadW != cardW {
				t.Fatalf("%s %q: boxed %v, down %v, head width %d", kind, layout, loop.Boxed, loop.Down, loop.HeadW)
			}
			if box.Y < loop.Y+loop.HeadH || box.X != loop.X {
				t.Fatalf("%s %q: the box is at (%d,%d), want under the card (%d, below %d)", kind, layout, box.X, box.Y, loop.X, loop.Y+loop.HeadH)
			}
			if !inside(a, box) || !inside(b, box) {
				t.Fatalf("%s %q: the steps are not inside the box %+v: a=%+v b=%+v", kind, layout, box, a, b)
			}
			if a.X != b.X || b.Y < a.Y+a.H+columnGap {
				t.Fatalf("%s %q: the steps are not stacked: a=(%d,%d %d high) b=(%d,%d)", kind, layout, a.X, a.Y, a.H, b.X, b.Y)
			}
			if loop.Y+loop.H != box.Y+box.H {
				t.Fatalf("%s %q: the loop ends at %d, its box at %d", kind, layout, loop.Y+loop.H, box.Y+box.H)
			}
		}
	}
}

func TestLayout_Given_a_stacked_loop_Then_the_box_has_room_below_the_last_step_for_an_add_step_and_to_its_right_for_port_labels(t *testing.T) {
	// Before: the box ended at the last step, with nothing to say where the next step of the loop goes.
	d := layoutLoop(loopFlow("batch", "", "a", "b"), "a", "b")

	b, _ := d.Unit("b")
	box := d.Lanes[0]
	if room := box.Y + box.H - (b.Y + b.H); room < boxAddH {
		t.Fatalf("%d units of room below the last step, want at least %d", room, boxAddH)
	}
	if room := box.X + box.W - (b.X + b.W); room < boxPortRoom {
		t.Fatalf("%d units of room to the right of the steps, want at least %d", room, boxPortRoom)
	}
}

func TestLayout_Given_a_loop_with_a_horizontal_layout_Then_its_steps_are_in_one_row_in_the_box(t *testing.T) {
	// Before: there was one layout, a row, and no way to choose.
	for _, kind := range []string{"batch", "run_till"} {
		d := layoutLoop(loopFlow(kind, "horizontal", "a", "b"), "a", "b")

		loop, _ := d.Unit("loop")
		a, _ := d.Unit("a")
		b, _ := d.Unit("b")
		box := d.Lanes[0]
		if !loop.Boxed || loop.Down {
			t.Fatalf("%s: boxed %v, down %v", kind, loop.Boxed, loop.Down)
		}
		if !inside(a, box) || !inside(b, box) {
			t.Fatalf("%s: the steps are not inside the box %+v: a=%+v b=%+v", kind, box, a, b)
		}
		if a.Y != b.Y || b.X <= a.X {
			t.Fatalf("%s: the steps are not in one row, left to right: a=(%d,%d) b=(%d,%d)", kind, a.X, a.Y, b.X, b.Y)
		}
		if room := box.X + box.W - (b.X + b.W); room < boxAddW {
			t.Fatalf("%s: %d units of room after the last step, want at least %d", kind, room, boxAddW)
		}
	}
}

func TestLayout_Given_a_loop_Then_the_flow_goes_on_from_the_loops_own_port_to_the_step_after_it(t *testing.T) {
	// Before: the step after the loop was joined from the loop's last step, as if it ran after that step each time.
	for _, layout := range []string{"vertical", "horizontal"} {
		d := layoutLoop(loopFlow("batch", layout, "a", "b"), "a", "b")

		loop, _ := d.Unit("loop")
		after, _ := d.Unit("after")
		var leaves Edge
		for _, e := range d.Edges {
			if e.From == "b" && e.To == "after" {
				t.Fatalf("%s: an edge leaves the last step of the body: %+v", layout, e)
			}
			if e.From == "loop" && e.To == "after" {
				leaves = e
			}
		}
		if leaves.From == "" {
			t.Fatalf("%s: no edge leaves the loop: %+v", layout, d.Edges)
		}
		if start := EdgePath(d, leaves)[0]; start.X != loop.X+cardW || start.Y != loop.Y+cardInPort {
			t.Fatalf("%s: the edge leaves at (%d,%d), want the card's port (%d,%d)", layout, start.X, start.Y, loop.X+cardW, loop.Y+cardInPort)
		}
		if after.X < loop.X+loop.W+gapX {
			t.Fatalf("%s: the step after starts at %d, inside the loop's box (right edge %d)", layout, after.X, loop.X+loop.W)
		}
	}
}

func TestLayout_Given_an_empty_loop_Then_its_box_is_as_wide_as_its_card_and_the_flow_leaves_the_card(t *testing.T) {
	// Before: the empty body was a dashed box beside the card, and the arrow to the next step left from the far side of it.
	d := layoutLoop(loopFlow("batch", ""))

	loop, _ := d.Unit("loop")
	box := d.Lanes[0]
	if box.W != cardW || loop.W != cardW {
		t.Fatalf("the box is %d wide and the loop %d, want the card's %d", box.W, loop.W, cardW)
	}
	var leaves Edge
	for _, e := range d.Edges {
		if e.From == "loop" && e.To == "after" {
			leaves = e
		}
	}
	if start := EdgePath(d, leaves)[0]; start.X != loop.X+cardW || start.Y != loop.Y+cardInPort {
		t.Fatalf("the edge leaves at (%d,%d), want the card's port (%d,%d)", start.X, start.Y, loop.X+cardW, loop.Y+cardInPort)
	}
}

func TestEdgePath_Given_the_way_into_a_loops_body_Then_it_runs_straight_down_from_the_card(t *testing.T) {
	// Before: the way into the body started at the container's left, crossing the card, so a drop on the card landed in the body.
	for _, layout := range []string{"vertical", "horizontal"} {
		d := layoutLoop(loopFlow("batch", layout, "a"), "a")

		loop, _ := d.Unit("loop")
		a, _ := d.Unit("a")
		var into Edge
		for _, e := range d.Edges {
			if e.Into {
				into = e
			}
		}
		path := EdgePath(d, into)
		first, last := path[0], path[len(path)-1]
		if first.X != last.X || first.Y != loop.Y+loop.HeadH || last.X < a.X || last.X > a.X+a.W {
			t.Fatalf("%s: path from (%d,%d) to (%d,%d), want straight down from the card's foot %d onto the first step at x %d", layout, first.X, first.Y, last.X, last.Y, loop.Y+loop.HeadH, a.X)
		}
	}
}

func TestEdgePath_Given_two_stacked_steps_Then_the_arrow_runs_out_of_the_right_port_down_and_into_the_left_of_the_next(t *testing.T) {
	// Before: an arrow from the right edge of a card to the left edge of the one under it was an S that ran over both cards.
	d := layoutLoop(loopFlow("batch", "", "a", "b"), "a", "b")

	a, _ := d.Unit("a")
	b, _ := d.Unit("b")
	var down Edge
	for _, e := range d.Edges {
		if e.From == "a" && e.To == "b" {
			down = e
		}
	}
	if !down.Down {
		t.Fatalf("the edge between the stacked steps is not marked down: %+v", down)
	}
	path := EdgePath(d, down)
	first, last := path[0], path[len(path)-1]
	if first.X != a.X+cardW || first.Y != a.Y+cardInPort || last.X != b.X || last.Y != b.Y+cardInPort {
		t.Fatalf("the path runs from (%d,%d) to (%d,%d), want from a's port (%d,%d) to b's in port (%d,%d)", first.X, first.Y, last.X, last.Y, a.X+cardW, a.Y+cardInPort, b.X, b.Y+cardInPort)
	}
	for _, pt := range path {
		if pt.X < b.X-entryGutter || pt.X > a.X+cardW+exitGutter+gutterStep*int64(len(a.Ports)) {
			t.Fatalf("the point (%d,%d) is outside the gutters beside the cards", pt.X, pt.Y)
		}
		if inCard := pt.X > a.X && pt.X < a.X+cardW && pt.Y > a.Y && pt.Y < a.Y+a.H; inCard {
			t.Fatalf("the point (%d,%d) is over card a", pt.X, pt.Y)
		}
		if inCard := pt.X > b.X && pt.X < b.X+cardW && pt.Y > b.Y && pt.Y < b.Y+b.H; inCard {
			t.Fatalf("the point (%d,%d) is over card b", pt.X, pt.Y)
		}
	}
}

func TestLayout_Given_a_loop_in_a_row_Then_its_arrows_are_not_marked_down(t *testing.T) {
	// Before: nothing; a row keeps the arrows it had.
	d := layoutLoop(loopFlow("batch", "horizontal", "a", "b"), "a", "b")

	for _, e := range d.Edges {
		if e.Down {
			t.Fatalf("edge %+v is marked down in a row", e)
		}
	}
}

func TestInsertNear_Given_a_drop_between_two_stacked_steps_Then_it_goes_between_them_in_the_loops_body(t *testing.T) {
	// Before: the drop code judged the gap along x, as the flow runs at the top, so a drop between stacked steps went wrong.
	d := layoutLoop(loopFlow("batch", "", "a", "b"), "a", "b")
	a, _ := d.Unit("a")
	b, _ := d.Unit("b")
	gap := Point{X: a.X + a.W/2, Y: (a.Y + a.H + b.Y) / 2}

	list, i, ok := InsertNear(d, gap, "")

	if !ok || list != d.Lanes[0].Path || i != 1 {
		t.Fatalf("got %q %d %v, want %q 1 true", list, i, ok, d.Lanes[0].Path)
	}
}

func TestInsertNear_Given_a_drop_on_the_upper_or_lower_half_of_a_stacked_step_Then_it_goes_before_or_after_it(t *testing.T) {
	// Before: the half of a step was judged along x, so the upper half of a stacked step was "before" only by luck.
	d := layoutLoop(loopFlow("batch", "", "a", "b"), "a", "b")
	b, _ := d.Unit("b")
	upper := Point{X: b.X + b.W/2, Y: b.Y + 10}
	lower := Point{X: b.X + b.W/2, Y: b.Y + b.H - 10}

	_, before, _ := InsertNear(d, upper, "")
	_, after, _ := InsertNear(d, lower, "")

	if before != 1 || after != 2 {
		t.Fatalf("upper half gave index %d, lower half %d, want 1 and 2", before, after)
	}
}

func TestSlotAt_Given_a_stacked_loop_Then_the_marker_is_a_bar_across_the_box_in_the_gap(t *testing.T) {
	// Before: the marker of a slot was an upright bar beside the unit, which in a stack sat on the card's edge.
	d := layoutLoop(loopFlow("batch", "", "a", "b"), "a", "b")
	a, _ := d.Unit("a")
	b, _ := d.Unit("b")

	slot := slotAt(d, d.Lanes[0].Path, 1)

	if !slot.OK || !slot.Horizontal || slot.W <= 0 {
		t.Fatalf("slot %+v, want a horizontal marker", slot)
	}
	if slot.Y <= a.Y+a.H || slot.Y >= b.Y {
		t.Fatalf("the marker is at y=%d, want in the gap between %d and %d", slot.Y, a.Y+a.H, b.Y)
	}
}

func TestSlotAt_Given_an_empty_stacked_loop_Then_the_marker_is_in_the_middle_of_its_box(t *testing.T) {
	// Before: an empty box had an upright bar at its left edge.
	d := layoutLoop(loopFlow("batch", ""))
	box := d.Lanes[0]

	slot := slotAt(d, box.Path, 0)

	if !slot.OK || !slot.Horizontal || slot.Y != box.Y+box.H/2 {
		t.Fatalf("slot %+v, want a horizontal marker at y=%d", slot, box.Y+box.H/2)
	}
}

func TestRunsHorizontally_Given_a_stacked_loop_first_in_the_flow_Then_the_flow_still_runs_left_to_right(t *testing.T) {
	// Before: the first arrow decided the direction, and the first one made was inside the loop, running down.
	d := layoutLoop(loopFlow("batch", "", "a", "b"), "a", "b")

	if !runsHorizontally(d) {
		t.Fatal("the flow is taken to run top to bottom")
	}
}

func TestLayout_Given_a_condition_or_a_parallel_Then_it_is_not_boxed_and_its_lanes_stay_beside_its_card(t *testing.T) {
	// Before: nothing; the loop change must leave the containers that offer alternatives as they were.
	f := model.Flow{Nodes: []model.Node{{ID: "r", Type: "condition", Cases: []model.Case{{When: "x", Nodes: []model.Node{{ID: "a", Type: "log"}}}}}}}

	d := Layout(f, nil, nil, WithCards(map[string]card.Card{"r": routeCard(1), "a": cardWithRows(0)}))

	r, _ := d.Unit("r")
	if r.Boxed || r.Down {
		t.Fatalf("a condition is drawn as a box (boxed %v, down %v)", r.Boxed, r.Down)
	}
	for _, lane := range d.Lanes {
		if lane.X < r.X+cardW {
			t.Fatalf("lane at x=%d is not to the right of the card (right edge %d)", lane.X, r.X+cardW)
		}
	}
}

func TestLayout_Given_a_loop_inside_a_loop_Then_the_inner_box_is_inside_the_outer_one(t *testing.T) {
	// Before: nested loops were nested lanes to the right; the inner one had to read as a part of the outer.
	for _, layout := range []string{"vertical", "horizontal"} {
		inner := model.Node{ID: "inner", Type: "batch", Params: []model.Field{{Key: "layout", Value: layout}}, Body: []model.Node{{ID: "a", Type: "log"}}}
		outer := model.Node{ID: "outer", Type: "batch", Params: []model.Field{{Key: "layout", Value: layout}}, Body: []model.Node{inner}}
		cards := map[string]card.Card{"outer": cardWithRows(1), "inner": cardWithRows(1), "a": cardWithRows(0)}

		d := Layout(model.Flow{Nodes: []model.Node{outer}}, nil, nil, WithCards(cards))

		var outerBox, innerBox Lane
		for _, lane := range d.Lanes {
			switch lane.Container {
			case "outer":
				outerBox = lane
			case "inner":
				innerBox = lane
			}
		}
		if innerBox.X < outerBox.X || innerBox.Y < outerBox.Y || innerBox.X+innerBox.W > outerBox.X+outerBox.W || innerBox.Y+innerBox.H > outerBox.Y+outerBox.H {
			t.Fatalf("%s: inner box (%d,%d %dx%d) is not inside the outer (%d,%d %dx%d)", layout, innerBox.X, innerBox.Y, innerBox.W, innerBox.H, outerBox.X, outerBox.Y, outerBox.W, outerBox.H)
		}
	}
}
