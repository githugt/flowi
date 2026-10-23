// SPDX-License-Identifier: AGPL-3.0-or-later

package canvas

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/examples"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

var update = flag.Bool("update", false, "rewrite golden files") //nolint:gochecknoglobals // test flag

// exampleFlows loads the example collection's flows by path.
func exampleFlows(t *testing.T) map[string]model.Flow {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "example"), examples.UserCreation()); err != nil {
		t.Fatal(err)
	}
	environ := []string{"FLOWI_CONFIG_DIR=" + filepath.Join(dir, "config"), "FLOWI_VAULT_KEY=test-only"}
	paths, err := settings.DiscoverPaths(environ)
	if err != nil {
		t.Fatal(err)
	}
	ws, _, err := flowi.OpenWorkspace(context.Background(), flowi.WorkspaceOptions{Paths: paths, Environ: environ})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.Mount(filepath.Join(dir, "example")); err != nil {
		t.Fatal(err)
	}
	out := map[string]model.Flow{}
	for _, f := range ws.Collections()[0].Flows {
		out[f.Path] = f
	}
	return out
}

func golden(t *testing.T, name string, d Diagram) {
	t.Helper()
	got, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join("testdata", name+".golden.json")
	if *update {
		if err := os.WriteFile(file, append(got, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(want) != string(got)+"\n" {
		t.Fatalf("%s differs from golden; got:\n%s", name, got)
	}
}

func TestLayout_golden_examples(t *testing.T) {
	flows := exampleFlows(t)
	for _, name := range []string{"user-creation", "onboarding-check", "shared/login"} {
		t.Run(name, func(t *testing.T) {
			golden(t, filepath.Base(name), Layout(flows[name], nil, nil))
		})
	}
}

func TestLayout_golden_needs_and_concurrent(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "a", Type: "set"}, {ID: "b", Type: "fetch"},
		{ID: "c", Type: "fetch", Concurrent: true, Needs: []string{"a"}},
		{ID: "d", Type: "log", Needs: []string{"b", "c"}},
	}}

	golden(t, "needs-concurrent", Layout(f, nil, nil))
}

func TestLayout_Given_empty_flow_Then_empty_diagram(t *testing.T) {
	d := Layout(model.Flow{}, nil, nil)

	if len(d.Units) != 0 || len(d.Edges) != 0 {
		t.Fatalf("got %+v", d)
	}
}

func TestLayout_manual_positions_move_containers_with_their_lanes(t *testing.T) {
	f := exampleFlows(t)["onboarding-check"]
	auto := Layout(f, nil, nil)
	both, _ := auto.Unit("both")
	notify, _ := auto.Unit("notify")

	moved := Layout(f, map[string]Point{"both": {X: both.X + 100, Y: both.Y + 50}}, nil)

	b2, _ := moved.Unit("both")
	n2, _ := moved.Unit("notify")
	if b2.X != both.X+100 || n2.X != notify.X+100 || n2.Y != notify.Y+50 {
		t.Fatalf("both %+v→%+v notify %+v→%+v", both, b2, notify, n2)
	}
}

func TestLayout_collapsed_strip_is_a_pill(t *testing.T) {
	f := exampleFlows(t)["onboarding-check"]

	d := Layout(f, nil, map[string]bool{"who": true})

	who, _ := d.Unit("who")
	if !who.Collapsed || who.W != pillW || len(who.Nodes) != 2 {
		t.Fatalf("who %+v", who)
	}
}

// A node whose needs name no node before it (the node was moved behind it)
// cannot start: the validator reports a cycle and the engine fails the run. The
// canvas draws no arrow into it rather than a chain that is not there.
func TestLayout_a_node_with_needs_that_point_forward_should_get_no_arrow(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "a", Type: "request", Request: &model.RequestCall{}},
		{ID: "b", Type: "request", Request: &model.RequestCall{}, Needs: []string{"c"}},
		{ID: "c", Type: "request", Request: &model.RequestCall{}},
	}}

	d := Layout(f, nil, nil)

	for _, e := range d.Edges {
		if e.To == "b" {
			t.Fatalf("an arrow into b from %s", e.From)
		}
	}
}

func requestFlow(ids ...string) model.Flow {
	var nodes []model.Node
	for _, id := range ids {
		nodes = append(nodes, model.Node{ID: id, Type: "request", Request: &model.RequestCall{}})
	}
	return model.Flow{Nodes: nodes}
}

func TestEndNear_should_find_the_start_and_the_end_of_the_top_level_list(t *testing.T) {
	d := Layout(requestFlow("a", "b", "c"), nil, nil)
	a, _ := d.Unit("a")
	c, _ := d.Unit("c")
	mid := a.Y + a.H/2

	before, beforeAt, beforeOK := EndNear(d, Point{X: a.X - 50, Y: mid}, "")
	after, afterAt, afterOK := EndNear(d, Point{X: c.X + c.W + 50, Y: mid}, "")
	_, _, between := EndNear(d, Point{X: a.X + a.W + 10, Y: mid}, "")
	_, _, farAbove := EndNear(d, Point{X: a.X - 50, Y: a.Y - 500}, "")

	if !beforeOK || before != "$.nodes" || beforeAt != 0 || !afterOK || after != "$.nodes" || afterAt != 3 || between || farAbove {
		t.Fatalf("before %v %d %v, after %v %d %v, between %v, far above %v", before, beforeAt, beforeOK, after, afterAt, afterOK, between, farAbove)
	}
}

func TestEndNear_should_leave_out_the_unit_being_dragged(t *testing.T) {
	d := Layout(requestFlow("a", "b", "c"), nil, nil)
	b, _ := d.Unit("b")
	c, _ := d.Unit("c")

	// b is dragged: c is the last of the others, so right of c is the end of the list
	_, at, ok := EndNear(d, Point{X: c.X + c.W + 50, Y: b.Y + b.H/2}, "b")

	if !ok || at != 3 {
		t.Fatalf("at %d, ok %v", at, ok)
	}
}

func TestLayout_Given_on_error_goto_Then_adds_a_fail_edge_that_is_not_a_drop_target(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "a", Type: "request", OnError: "goto:cleanup"},
		{ID: "b", Type: "request"},
		{ID: "cleanup", Type: "request"},
	}}

	d := Layout(f, nil, nil)

	var fails []Edge
	for _, e := range d.Edges {
		if e.Fail {
			fails = append(fails, e)
		}
	}
	if len(fails) != 1 || fails[0].From != "a" || fails[0].To != "cleanup" {
		t.Fatalf("fail edges %+v", fails)
	}
	mid := EdgePath(d, fails[0])[len(EdgePath(d, fails[0]))/2]
	if e, ok := NearestEdge(d, mid, 2, ""); ok && e.Fail {
		t.Fatalf("a fail edge was offered as a drop target: %+v", e)
	}
}

func TestLayout_Given_on_error_continue_or_unknown_target_Then_no_fail_edge(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "a", Type: "request", OnError: "continue"},
		{ID: "b", Type: "request", OnError: "goto:nowhere"},
	}}

	d := Layout(f, nil, nil)

	for _, e := range d.Edges {
		if e.Fail {
			t.Fatalf("unexpected fail edge %+v", e)
		}
	}
}

func TestLayout_should_mark_the_container_to_lane_edge_and_leave_room_around_a_laned_step(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "c", Type: "condition", Else: []model.Node{{ID: "x", Type: "log"}}}}}

	d := Layout(f, nil, nil)

	into := 0
	for _, e := range d.Edges {
		if e.Into && e.From == "c" && e.To == "x" {
			into++
		}
	}
	x, _ := d.Unit("x")
	lane := d.Lanes[1]
	if into != 1 || x.X-lane.X != laneInset || lane.X+lane.W-(x.X+x.W) < laneInset {
		t.Fatalf("into edges %d, step %+v in lane %+v", into, x, lane)
	}
}

func labelsOf(d Diagram) []string {
	var got []string
	for _, l := range d.Lanes {
		got = append(got, l.Label)
	}
	return got
}

func TestLayout_should_label_each_lane_with_what_takes_it(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "c", Type: "condition", Then: []model.Node{{ID: "a", Type: "log"}}, Cases: []model.Case{{When: "$status == 404"}}},
		{ID: "p", Type: "parallel", Branches: []model.Branch{{ID: "left"}, {}}},
	}}

	got := labelsOf(Layout(f, nil, nil))

	want := []string{"then", "else", "if $status == 404", "left", "branch 2"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("labels %v, want %v", got, want)
	}
}

// Behaviour change (graph view): a condition with cases still drew a "then" lane,
// which the engine ignores then, before "else" and the cases. It is now one chain:
// if, else if …, else.
func TestLayout_with_branches_should_draw_a_condition_as_one_chain(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "c", Type: "condition", Then: []model.Node{{ID: "a", Type: "log"}}, Cases: []model.Case{{When: "$status == 404"}, {When: "$status == 500"}, {}}},
		{ID: "d", Type: "condition", Params: []model.Field{{Key: "check", Value: "nodes.login.status == 200"}}},
		{ID: "e", Type: "condition"},
	}}

	got := labelsOf(Layout(f, nil, nil, WithBranches()))

	want := []string{"if $status == 404", "else if $status == 500", "else if …", "else", "if nodes.login.status == 200", "else", "if …", "else"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("labels %v, want %v", got, want)
	}
}

func TestLayout_WithBranches_should_put_the_lanes_right_of_the_container_card_on_the_ports_line(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "c", Type: "condition", Then: []model.Node{{ID: "a", Type: "log"}}, Else: []model.Node{{ID: "b", Type: "log"}}},
		{ID: "after", Type: "log"},
	}}

	d := Layout(f, nil, nil, WithBranches())

	c, _ := d.Unit("c")
	a, _ := d.Unit("a")
	b, _ := d.Unit("b")
	after, _ := d.Unit("after")
	thenLane, elseLane := d.Lanes[0], d.Lanes[1]
	if c.HeadW != decisionW || thenLane.X < c.X+decisionW+branchGapX {
		t.Fatalf("container %+v, then lane %+v", c, thenLane)
	}
	if a.PortY() != c.PortY() || thenLane.PortY != c.PortY() || elseLane.PortY != b.PortY() {
		t.Fatalf("ports: card %d, first step %d, lane %d, second step %d (lane %d)", c.PortY(), a.PortY(), thenLane.PortY, b.PortY(), elseLane.PortY)
	}
	joins := 0
	for _, e := range d.Edges {
		if e.To == "after" && !e.Into && (e.From == "a" || e.From == "b") {
			joins++
		}
	}
	if joins != 2 || after.X < thenLane.X+thenLane.W {
		t.Fatalf("join edges %d, next step %+v, lane %+v", joins, after, thenLane)
	}
}

func TestLayout_without_WithBranches_should_keep_the_box_with_the_header_on_top(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "c", Type: "condition", Then: []model.Node{{ID: "a", Type: "log"}}}}}

	d := Layout(f, nil, nil)

	c, _ := d.Unit("c")
	if c.HeadW != 0 || d.Lanes[0].X != c.X+lanePad {
		t.Fatalf("container %+v, lane %+v", c, d.Lanes[0])
	}
}

func TestLayout_WithAttached_should_draw_attached_actions_in_one_block_with_their_request(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "pick", Type: "set", Attach: "login"},
		{ID: "login", Type: "request"},
		{ID: "check", Type: "assert", Attach: "login"},
		{ID: "later", Type: "log"},
	}}

	d := Layout(f, nil, nil, WithAttached())

	block, _ := d.UnitOf("login")
	later, _ := d.UnitOf("later")
	if block.Kind != KindRequest || len(block.Nodes) != 3 || block.Core != 1 || block.Path != "$.nodes[0]" || block.W != requestW+2*attachedW {
		t.Fatalf("block %+v", block)
	}
	if len(d.Units) != 2 || later.Kind != KindStrip || len(later.Nodes) != 1 {
		t.Fatalf("units %+v", d.Units)
	}
}

func TestLayout_without_WithAttached_should_keep_attached_actions_in_strips(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "pick", Type: "set", Attach: "login"}, {ID: "login", Type: "request"}}}

	d := Layout(f, nil, nil)

	if len(d.Units) != 2 {
		t.Fatalf("units %+v", d.Units)
	}
}

func TestLayout_WithAttached_should_ignore_an_attach_that_is_not_next_to_its_request(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "pick", Type: "set", Attach: "login"}, {ID: "x", Type: "log"}, {ID: "login", Type: "request"}}}

	d := Layout(f, nil, nil, WithAttached())

	if len(d.Units) != 2 || len(d.Units[0].Nodes) != 2 {
		t.Fatalf("units %+v", d.Units)
	}
}

// A container dragged somewhere else (a manual position) takes its lanes with
// it, and the arrows to the lanes follow the lanes' port line.
func TestLayout_WithBranches_should_move_the_lane_port_lines_with_a_container_placed_by_hand(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "c", Type: "condition", Then: []model.Node{{ID: "a", Type: "log"}}}}}
	home := Layout(f, nil, nil, WithBranches())

	moved := Layout(f, map[string]Point{"c": {X: 100, Y: 300}}, nil, WithBranches())

	hc, _ := home.Unit("c")
	mc, _ := moved.Unit("c")
	dy := mc.Y - hc.Y
	if dy == 0 || moved.Lanes[0].PortY-home.Lanes[0].PortY != dy || moved.Lanes[0].Y-home.Lanes[0].Y != dy {
		t.Fatalf("container moved %d down, lane %+v (was %+v)", dy, moved.Lanes[0], home.Lanes[0])
	}
}

// Behaviour change: a needs that names the node right before was drawn as a
// dashed "waits for" arrow into a card of its own. It says nothing a list
// does not say already, so it is drawn as the plain sequence.
func TestLayout_Given_needs_naming_the_previous_node_Then_draws_the_plain_sequence(t *testing.T) {
	flow := model.Flow{Nodes: []model.Node{
		{ID: "a", Type: "log"},
		{ID: "b", Type: "log", Needs: []string{"a"}},
		{ID: "c", Type: "log", Needs: []string{"a"}}, // a real wait: a is not the node before c
	}}

	d := Layout(flow, nil, nil)

	if len(d.Units) != 2 || strings.Join(d.Units[0].Nodes, ",") != "a,b" {
		t.Fatalf("units %+v", d.Units)
	}
	for _, e := range d.Edges {
		if e.To == "b" && e.Needs {
			t.Fatalf("a redundant needs was drawn as a dependency: %+v", d.Edges)
		}
	}
}

func requestNode(id string) model.Node { return model.Node{ID: id, Type: "request"} }

func attachedNode(id, to, at string) model.Node {
	return model.Node{ID: id, Type: "log", Attach: to, AttachAt: at}
}

func TestLayout_should_stack_actions_that_say_above_or_below_over_and_under_their_request(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		attachedNode("left", "r", ""),
		attachedNode("up1", "r", "above"),
		attachedNode("up2", "r", "above"),
		requestNode("r"),
		attachedNode("right", "r", ""),
		attachedNode("down", "r", "below"),
		{ID: "after", Type: "log"},
	}}

	d := Layout(f, nil, nil, WithBranches(), WithAttached())

	block, ok := d.Unit("left")
	if !ok || block.Kind != KindRequest || len(block.Nodes) != 6 || block.Nodes[block.Core] != "r" {
		t.Fatalf("block %+v", block)
	}
	cells := map[string]Cell{}
	for i, id := range block.Nodes {
		cells[id] = block.Cells[i]
	}
	var wantAbove int64 = 2 * (stackH + stackGap)
	if block.Above != wantAbove || cells["r"] != (Cell{X: attachedW, Y: wantAbove, W: requestW, H: requestH}) {
		t.Fatalf("above %d, request cell %+v", block.Above, cells["r"])
	}
	if cells["up1"].Y != 0 || cells["up2"].Y != stackH+stackGap || cells["up1"].X != attachedW || cells["up1"].W != requestW {
		t.Fatalf("the stack above: %+v %+v", cells["up1"], cells["up2"])
	}
	if cells["left"] != (Cell{X: 0, Y: wantAbove, W: attachedW, H: requestH}) || cells["right"].X != attachedW+requestW || cells["right"].Y != wantAbove {
		t.Fatalf("beside it: %+v %+v", cells["left"], cells["right"])
	}
	if cells["down"].Y != wantAbove+requestH+stackGap || block.H != cells["down"].Y+stackH || block.W != attachedW+requestW+attachedW {
		t.Fatalf("below it %+v, block %dx%d", cells["down"], block.W, block.H)
	}
	if block.PortY() != block.Y+wantAbove+requestH/2 {
		t.Fatalf("the port line goes through the request: %d", block.PortY())
	}
	after, _ := d.Unit("after")
	if after.PortY() != block.PortY() {
		t.Fatalf("the next step is on the same line: %d vs %d", after.PortY(), block.PortY())
	}
	if block.Y < 0 {
		t.Fatalf("the stack above starts above the canvas: %+v", block)
	}
}

func TestCellAt_should_find_the_cell_under_a_point_and_the_nearest_in_a_gap(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{attachedNode("up", "r", "above"), requestNode("r"), attachedNode("side", "r", "")}}
	block, _ := Layout(f, nil, nil, WithBranches(), WithAttached()).Unit("up")
	idx := map[string]int{}
	for i, id := range block.Nodes {
		idx[id] = i
	}
	core := block.Cells[idx["r"]]

	cases := []struct {
		name string
		p    Point
		want string
	}{
		{"in the stack", Point{X: core.X + 5, Y: 5}, "up"},
		{"in the request", Point{X: core.X + 5, Y: core.Y + 5}, "r"},
		{"in the cell beside it", Point{X: core.X + core.W + 5, Y: core.Y + 5}, "side"},
		{"in the gap between the stack and the request", Point{X: core.X + 5, Y: core.Y - 1}, "r"},
		{"far below", Point{X: core.X, Y: 900}, "r"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := block.Nodes[CellAt(block, c.p)]; got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}
