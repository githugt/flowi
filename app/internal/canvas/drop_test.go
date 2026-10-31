// SPDX-License-Identifier: AGPL-3.0-or-later

package canvas

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// A step dragged by its left edge has its centre outside a narrow lane while
// the pointer is inside it. The place follows the pointer: in the room of a
// lane, away from its steps, that is the end of the lane, not the gap of the
// connector that leaves the container (which reaches into the container and
// used to turn this drop into "already there" or a place after the container).
func TestReorderTarget_Given_a_pointer_in_the_room_of_a_lane_When_dropping_Then_the_step_goes_to_the_lane_end(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "c", Type: "condition", Then: []model.Node{{ID: "x", Type: "log"}}},
		{ID: "z", Type: "request"},
		{ID: "m", Type: "request"},
	}}
	d := Layout(f, nil, nil)
	lane := d.Lanes[0]
	// the strip of the lane above its step: inside the lane, on no step
	inRoom := Point{X: lane.X + lane.W/2, Y: lane.Y + laneInset/4}
	if on, over := UnitAt(d, inRoom); over && on.ID == "x" {
		t.Fatalf("the point %+v is on the step: lane %+v", inRoom, lane)
	}

	list, index, ok := ReorderTarget(d, inRoom, "m", "m", 20, func(string) (string, int, bool) { return "", 0, false })

	if !ok || list != lane.Path || index != LaneLength(d, lane) {
		t.Fatalf("got list %q index %d ok %v, want the end of lane %q (%d)", list, index, ok, lane.Path, LaneLength(d, lane))
	}
}
