// SPDX-License-Identifier: AGPL-3.0-or-later

// Package canvas computes the flow canvas layout (design §10.3) as a pure
// function: a flow becomes units (request blocks, action strips,
// containers with lanes) placed left to right, and the edges between
// them. It knows nothing about widgets; the canvas widget draws a Diagram.
package canvas

import (
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/card"
)

// key and index build YAML paths the way flowi-lib writes them
// ($.nodes[2].branches[0].nodes); the keys used here never need quoting.
func key(p, k string) string       { return p + "." + k }
func index(p string, i int) string { return p + "[" + strconv.Itoa(i) + "]" }

// Unit kinds.
const (
	KindRequest   = "request"   // a request or subflow node: [pre ⟨call⟩ post]
	KindStrip     = "strip"     // consecutive actions, collapsible into a pill
	KindAction    = "action"    // one node on its own (fetch, webhook, needs, concurrent)
	KindContainer = "container" // parallel, condition, batch, run_till with lanes
)

// Sizes in canvas units.
const (
	requestW, requestH = 220, 64
	actionW, stripH    = 120, 48
	stripPad           = 20
	pillW              = 96
	gapX               = 60
	headerH            = 40
	lanePad            = 20
	laneInset          = 8 // room between a lane's box and the steps in it
	laneGap            = 30
	portRequest        = requestH / 2
	portStrip          = stripH / 2
	portContainer      = headerH / 2
	attachedW          = 100                              // an action shown with a request: its cell in the request's block
	stackH             = 40                               // an action in a stack above or below its request
	stackGap           = 4                                // between the cards of a stack, and between a stack and the request
	decisionW          = 170                              // a container's own card, when its lanes are drawn as branches
	branchGapX         = 70                               // between that card and its lanes: room for the labelled arrows
	minLaneW           = actionW + stripPad + 2*laneInset // an empty lane is still a target big enough to drop a step on
	baseline           = portRequest                      // every row's port line sits this far below its top
)

// Strip geometry: an action strip is StripPad wide plus ActionW per
// action (exported for hit-testing a click on one action).
const (
	ActionW  = 120
	StripPad = 20
)

// Point is a canvas position.
type Point = model.Point

// Unit is one draggable thing on the canvas.
type Unit struct {
	ID         string   `json:"id"`    // its first node's id
	Kind       string   `json:"kind"`  //
	Nodes      []string `json:"nodes"` // node ids, in order
	Types      []string `json:"types"` // node types, in order
	Path       string   `json:"path"`  // YAML path of the first node
	X          int64    `json:"x"`
	Y          int64    `json:"y"`
	W          int64    `json:"w"`
	H          int64    `json:"h"`
	Parent     string   `json:"parent,omitempty"` // the container unit it is inside
	Lane       int      `json:"lane,omitempty"`   // its lane in the parent
	Concurrent bool     `json:"concurrent,omitempty"`
	Collapsed  bool     `json:"collapsed,omitempty"`
	Core       int      `json:"core,omitempty"`  // attached layout: the index in Nodes of the request the block is about (the other cells are attached actions)
	Cells      []Cell   `json:"cells,omitempty"` // a request block with attached actions: where each of Nodes is drawn, relative to the block's top-left corner
	Above      int64    `json:"above,omitempty"` // a request block: how far its request sits below its top (the height of the stack above it)
	HeadW      int64    `json:"headW,omitempty"` // branch layout: the container is a card this wide with its lanes to the right, not a box around them
	Card       bool     `json:"card,omitempty"`  // drawn as a card (see WithCards): its ports are in Ports
	HeadH      int64    `json:"headH,omitempty"` // a container drawn as a card: the card's own height (H covers its lanes too)
	Ports      []string `json:"ports,omitempty"` // a card's control ports, top to bottom
}

// Cell is where one node of a request block is drawn, relative to the block.
type Cell struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
	W int64 `json:"w"`
	H int64 `json:"h"`
}

// PortY is the y of the unit's in and out ports.
func (u Unit) PortY() int64 {
	if u.Card {
		return u.Y + cardInPort
	}
	switch u.Kind {
	case KindContainer:
		if u.HeadW > 0 {
			return u.Y + baseline // the card is centred on the row's port line, as its first lane's steps are
		}
		return u.Y + portContainer
	case KindStrip, KindAction:
		return u.Y + portStrip
	}
	return u.Y + u.Above + portRequest
}

// Edge connects two units by id.
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Curve bool   `json:"curve,omitempty"` // the ports are on different rows
	Needs bool   `json:"needs,omitempty"` // an explicit needs, not list order
	Into  bool   `json:"into,omitempty"`  // from a container to the first step of one of its lanes: not drawn, the lane box says it
	Fail  bool   `json:"fail,omitempty"`  // taken when the step it leaves fails: an after_failure_of arrow, or the on_error goto route
	Goto  bool   `json:"goto,omitempty"`  // …the on_error goto route (a Fail edge that is not an after_failure_of arrow)
	// FromStep is the step an after_failure_of arrow names: the unit it leaves may hold several steps.
	FromStep string `json:"fromStep,omitempty"`
	// Outcome is the named outcome of the step's request that takes the arrow (after_outcome); "" for the
	// success and failure arrows.
	Outcome string `json:"outcome,omitempty"`
	// FromPort is the card port the edge leaves from (done, fail, case-i, else);
	// "" for units that are not cards.
	FromPort string `json:"fromPort,omitempty"`
}

// Lane is one list inside a container (a branch, then/else/case, a
// body): a drop target even when empty.
type Lane struct {
	Container string `json:"container"`
	Index     int    `json:"index"`
	Path      string `json:"path"` // YAML path of the lane's node list
	X         int64  `json:"x"`
	Y         int64  `json:"y"`
	W         int64  `json:"w"`
	H         int64  `json:"h"`
	PortY     int64  `json:"portY"` // the line the steps in the lane have their ports on
	Label     string `json:"label"` // what takes the lane: then, else, "if <when>", a branch's name, body
	// Port is the card port that feeds the lane (case-i or else for a route's
	// lanes), "" when the container has no port per lane.
	Port string `json:"port,omitempty"`
}

// Diagram is a laid-out flow.
type Diagram struct {
	Units []Unit `json:"units"`
	Edges []Edge `json:"edges"`
	Lanes []Lane `json:"lanes"`
}

// Unit returns the unit with id.
func (d Diagram) Unit(id string) (Unit, bool) {
	for _, u := range d.Units {
		if u.ID == id {
			return u, true
		}
	}
	return Unit{}, false
}

// UnitOf returns the unit holding node id.
func (d Diagram) UnitOf(node string) (Unit, bool) {
	for _, u := range d.Units {
		for _, n := range u.Nodes {
			if n == node {
				return u, true
			}
		}
	}
	return Unit{}, false
}

// Layout places a flow. manual holds positions the user dragged units to
// (by unit id); collapsed lists strips shown as pills (by unit id).
func Layout(f model.Flow, manual map[string]Point, collapsed map[string]bool, opts ...Option) Diagram {
	l := &layouter{collapsed: collapsed}
	for _, o := range opts {
		o(l)
	}
	l.list(f.Nodes, key("$", "nodes"), 0, 0, "", 0)
	l.failEdges(f.Nodes)
	// Lists, never null: the window reads them with .filter and .find, and Go writes a nil slice as null.
	d := Diagram{Units: orEmpty(l.units), Edges: orEmpty(l.edges), Lanes: orEmpty(l.lanes)}
	portEdges(&d)
	applyManual(&d, manual)
	markCurves(&d)
	return d
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// Option changes how Layout draws a flow.
type Option func(*layouter)

// WithBranches draws a container's lanes as branches: the container is a card
// at the left, its lanes are stacked to its right, and the canvas draws a
// labelled arrow from the card to each lane (the steps after the container
// already join every lane's last step). Without it the lanes sit in a box
// under the container's header.
func WithBranches() Option { return func(l *layouter) { l.branches = true } }

// WithAttached draws actions that name a request in their attach field in one
// block with it, in the order of the list: [before · ⟨request⟩ · after], when
// they sit right next to it. Without it they are strips like any others.
func WithAttached() Option { return func(l *layouter) { l.attached = true } }

type layouter struct {
	cards     map[string]card.Card // set by WithCards: every step is a card sized by its summary
	attached  bool
	branches  bool
	collapsed map[string]bool
	units     []Unit
	edges     []Edge
	lanes     []Lane
}

// failEdges adds an edge from each step whose on_error is goto:<id> to the unit
// of its target. They do not shape the layout: the flow still reads left to
// right by list order and needs.
func (l *layouter) failEdges(nodes []model.Node) {
	unitOf := map[string]string{}
	for _, u := range l.units {
		for _, n := range u.Nodes {
			unitOf[n] = u.ID
		}
	}
	var walk func(list []model.Node)
	walk = func(list []model.Node) {
		for _, n := range list {
			target, ok := strings.CutPrefix(n.OnError, "goto:")
			from, to := unitOf[n.ID], unitOf[strings.TrimSpace(target)]
			if ok && from != "" && to != "" && from != to {
				l.edges = append(l.edges, Edge{From: from, To: to, Fail: true, Goto: true})
			}
			for _, child := range n.Children() {
				walk(child)
			}
		}
	}
	walk(nodes)
}

// listResult is a laid-out node list: its size and its first and last
// units (lasts: several when the list ends in a container's lanes).
type listResult struct {
	w, h  int64
	first string
	lasts []string
}

// list lays out nodes left to right from (x, y), with ports on the line
// y+baseline.
func (l *layouter) list(nodes []model.Node, path string, x, y int64, parent string, lane int) listResult {
	nodes = withoutRedundantNeeds(nodes)
	groups := group(nodes, l.attached)
	if l.cards != nil {
		groups = cardGroups(nodes)
	}
	top := y
	for _, g := range groups {
		if g.kind == KindRequest && g.end-g.start > 1 {
			_, _, _, above := blockCells(nodes[g.start:g.end], g.core)
			top = max(top, y+above) // the stack above a request starts where the row would
		}
	}
	y0, y := y, top
	res := listResult{}
	unitOf := map[string]string{} // node id → unit id, for needs edges
	var prevLasts []string
	cursor := x
	var column handlerColumn
	for _, g := range groups {
		n := nodes[g.start]
		source, handler := handlerSource(groupNeeds(nodes[g.start:g.end], n), unitOf)
		at, row := cursor, y
		if handler {
			at, row = column.slot(source, cursor, l.bottomOf(source)+handlerDrop)
		}
		u := Unit{ID: n.ID, Kind: g.kind, Path: index(path, g.start), Parent: parent, Lane: lane, Concurrent: n.Concurrent, Core: g.core}
		for _, m := range nodes[g.start:g.end] {
			u.Nodes = append(u.Nodes, m.ID)
			u.Types = append(u.Types, string(m.Type))
			unitOf[m.ID] = u.ID
		}
		if g.kind == KindRequest && g.end-g.start > 1 {
			u.Cells, u.W, u.H, u.Above = blockCells(nodes[g.start:g.end], g.core)
		}
		var lasts []string
		u, lasts = l.place(u, n, at, row)
		from, explicit := deps(groupNeeds(nodes[g.start:g.end], n), prevLasts, unitOf)
		for _, dep := range from {
			l.edges = append(l.edges, Edge{From: dep, To: u.ID, Needs: explicit})
		}
		grouped := groupNeeds(nodes[g.start:g.end], n)
		for _, id := range grouped.AfterFailureOf {
			if from, ok := unitOf[id]; ok && from != u.ID {
				l.edges = append(l.edges, Edge{From: from, To: u.ID, Fail: true, FromStep: id})
			}
		}
		for _, entry := range grouped.AfterOutcome {
			id, name, _ := strings.Cut(entry, ":")
			if from, ok := unitOf[id]; ok && from != u.ID {
				l.edges = append(l.edges, Edge{From: from, To: u.ID, Outcome: name, FromStep: id})
			}
		}
		if res.first == "" {
			res.first = u.ID
		}
		prevLasts = lasts
		if handler {
			if column.advance(u) {
				cursor += u.W + gapX
			}
		} else {
			column = handlerColumn{}
			cursor += u.W + gapX
		}
		res.h = max(res.h, u.Y+u.H-y0)
	}
	res.w = max(0, cursor-x-gapX)
	res.lasts = prevLasts
	return res
}

// place sizes and positions one unit (laying out a container's lanes) and
// returns the units its successor connects from.
func (l *layouter) place(u Unit, n model.Node, x, y int64) (Unit, []string) {
	u.X = x
	switch u.Kind {
	case KindRequest:
		if u.W == 0 { // a request with nothing drawn with it
			u.W, u.H = requestW, requestH
		}
		u.Y = y + baseline - portRequest - u.Above
	case KindCard:
		c := l.cards[n.ID]
		u.W, u.H, u.Card, u.Ports = cardW, cardHeight(c), true, cardPorts(c)
		u.Y = y + baseline - cardInPort
	case KindStrip, KindAction:
		u.W, u.H = stripPad+int64(len(u.Nodes))*actionW, stripH
		if l.collapsed[u.ID] && u.Kind == KindStrip {
			u.W, u.Collapsed = pillW, true
		}
		u.Y = y + baseline - portStrip
	case KindContainer:
		return l.container(u, n, x, y)
	}
	l.units = append(l.units, u)
	return u, []string{u.ID}
}

// container lays out each lane under the header, inside the box.
func (l *layouter) container(u Unit, n model.Node, x, y int64) (Unit, []string) {
	if l.branches {
		return l.branchContainer(u, n, x, y)
	}
	u.Y = y + baseline - portContainer
	idx := len(l.units)
	l.units = append(l.units, u) // placeholder; size is known after the lanes
	laneY := u.Y + headerH + lanePad
	var w int64 = 200
	var lasts []string
	first := len(l.lanes)
	for i, lane := range lanes(n, u.Path, false) {
		res := l.list(lane.nodes, lane.path, x+lanePad+laneInset, laneY, u.ID, i)
		if res.first != "" {
			l.edges = append(l.edges, Edge{From: u.ID, To: res.first, Into: true})
			lasts = append(lasts, res.lasts...)
		}
		h := max(res.h+laneInset, stripH)
		l.lanes = append(l.lanes, Lane{Container: u.ID, Index: i, Path: lane.path, X: x + lanePad, Y: laneY, H: h, PortY: laneY + baseline, Label: lane.label})
		w = max(w, res.w+2*(lanePad+laneInset))
		laneY += h + laneGap
	}
	for i := first; i < len(l.lanes); i++ {
		l.lanes[i].W = w - 2*lanePad
	}
	u.W, u.H = w, laneY-u.Y-laneGap+lanePad
	l.units[idx] = u
	if len(lasts) == 0 {
		lasts = []string{u.ID}
	}
	return u, lasts
}

// branchContainer lays a container out as a card with its lanes to the right.
// The first lane's steps and the card share the row's port line, so the arrow
// into the card and the arrow to the first lane are straight.
func (l *layouter) branchContainer(u Unit, n model.Node, x, y int64) (Unit, []string) {
	u.Y, u.HeadW = y, decisionW
	head := int64(decisionW)
	var own card.Card
	if l.cards != nil {
		own = l.cards[n.ID]
		head, u.HeadW = cardW, cardW
		u.Y = y + baseline - cardInPort
		u.Card, u.Ports, u.HeadH = true, cardPorts(own), cardHeight(own)
	}
	idx := len(l.units)
	l.units = append(l.units, u) // placeholder; size is known after the lanes
	laneX := x + head + branchGapX
	laneY := y
	emptyW, emptyH := int64(minLaneW), int64(baseline-portStrip+stripH+laneInset) // an empty lane is as big as one with a step in it
	if l.cards != nil {
		emptyW, emptyH = cardW+2*laneInset, int64(baseline-cardInPort)+cardHeight(card.Card{})+laneInset
	}
	w := head + branchGapX + emptyW
	var lasts []string
	first := len(l.lanes)
	for i, lane := range lanes(n, u.Path, true) {
		res := l.list(lane.nodes, lane.path, laneX+laneInset, laneY, u.ID, i)
		if res.first != "" {
			l.edges = append(l.edges, Edge{From: u.ID, To: res.first, Into: true})
			lasts = append(lasts, res.lasts...)
		}
		h := max(res.h+laneInset, emptyH)
		l.lanes = append(l.lanes, Lane{Container: u.ID, Index: i, Path: lane.path, X: laneX, Y: laneY, H: h, PortY: laneY + baseline, Label: lane.label, Port: lane.port})
		w = max(w, head+branchGapX+res.w+2*laneInset)
		laneY += h + laneGap
	}
	for i := first; i < len(l.lanes); i++ {
		l.lanes[i].W = w - head - branchGapX
	}
	u.W, u.H = w, max(laneY-laneGap-u.Y, headerH+(baseline-portContainer))
	if l.cards != nil {
		u.H = max(u.H, cardHeight(own))
	}
	l.units[idx] = u
	if len(lasts) == 0 {
		lasts = []string{u.ID}
	}
	return u, lasts
}

type laneSpec struct {
	nodes []model.Node
	path  string
	label string
	port  string // the card port that takes the lane
}

// lanes are a container's child lists with their YAML paths. chain says a condition
// is drawn as one chain (if, else if …, else); without it the lanes are the lists
// in the order of the file (then, else, the cases), as the older window draws them.
func lanes(n model.Node, p string, chain bool) []laneSpec {
	var out []laneSpec
	add := func(path, label string, nodes []model.Node) {
		out = append(out, laneSpec{nodes: nodes, path: path, label: label, port: laneFeed(n, label, len(out))})
	}
	switch n.Type {
	case "parallel":
		for i, b := range n.Branches {
			label := b.ID
			if label == "" {
				label = "branch " + strconv.Itoa(i+1)
			}
			add(key(index(key(p, "branches"), i), "nodes"), label, b.Nodes)
		}
	case "condition":
		if !chain {
			add(key(p, "then"), "then", n.Then)
			add(key(p, "else"), "else", n.Else)
			for i, c := range n.Cases {
				add(key(index(key(p, "cases"), i), "nodes"), "if "+c.When, c.Nodes)
			}
			break
		}
		// One chain, read top to bottom: if, else if …, else. A condition with
		// cases has no "then" lane: its check is not used, and drawing the lane
		// next to the cases made it look as if it were.
		if len(n.Cases) == 0 {
			add(key(p, "then"), conditionLabel("if", checkOf(n)), n.Then)
		}
		for i, c := range n.Cases {
			word := "else if"
			if i == 0 {
				word = "if"
			}
			add(key(index(key(p, "cases"), i), "nodes"), conditionLabel(word, c.When), c.Nodes)
		}
		add(key(p, "else"), "else", n.Else)
	default:
		add(key(p, "body"), "body", n.Body)
	}
	return out
}

// deps are the units a unit's incoming edges start from: its needs, or the
// previous unit(s) when it has none. explicit says they are the needs. Needs
// that name no node before this one (a node moved behind it, a missing id) draw
// no arrow: the node cannot start, the validator reports a dependency cycle and
// the engine fails the run, so the canvas does not draw a chain that is not
// there (the header offers to repair it).
func deps(n model.Node, prev []string, unitOf map[string]string) (units []string, explicit bool) {
	if len(n.Needs) == 0 {
		if len(n.AfterFailureOf)+len(n.AfterOutcome) > 0 || n.Unlinked {
			return nil, true // it has arrows of its own, none on success, or none at all: it no longer follows the step before it
		}
		return prev, false
	}
	for _, id := range n.Needs {
		if u, ok := unitOf[id]; ok {
			units = append(units, u)
		}
	}
	return units, true
}

// withoutRedundantNeeds returns nodes with the needs that only say "after the
// node before" removed. A node waits for the one before it anyway, so such a
// needs changes nothing at run time; drawn as it is, it would make the node a
// card of its own with a dashed "waits for" arrow, as if it were a dependency.
func withoutRedundantNeeds(nodes []model.Node) []model.Node {
	var out []model.Node
	for i := 1; i < len(nodes); i++ {
		if len(nodes[i].Needs) != 1 || nodes[i].Needs[0] != nodes[i-1].ID || len(nodes[i].AfterFailureOf)+len(nodes[i].AfterOutcome) > 0 {
			continue
		}
		if out == nil {
			out = slices.Clone(nodes) // the caller's nodes are not changed
		}
		out[i].Needs = nil
	}
	if out == nil {
		return nodes
	}
	return out
}

type groupSpan struct {
	start, end int
	kind       string
	core       int // for a request block with attached actions: the request's index within the span
}

// groupNeeds is the node whose needs a unit's incoming edges follow: the first
// node's own, or for a block of several nodes the needs of all of them.
func groupNeeds(span []model.Node, first model.Node) model.Node {
	if len(span) < 2 {
		return first
	}
	var needs, failures, named []string
	for _, n := range span {
		needs = append(needs, n.Needs...)
		failures = append(failures, n.AfterFailureOf...)
		named = append(named, n.AfterOutcome...)
	}
	first.Needs, first.AfterFailureOf, first.AfterOutcome = needs, failures, named
	return first
}

// group splits a node list into units: consecutive plain actions form a
// strip; calls, containers and nodes with needs or concurrent stand alone.
// With attached, actions that name a neighbouring request join its block.
func group(nodes []model.Node, attached bool) []groupSpan {
	var blocks map[int][2]int // request index → the span of its block
	owned := map[int]bool{}
	if attached {
		blocks = attachedBlocks(nodes)
		for _, span := range blocks {
			for k := span[0]; k < span[1]; k++ {
				owned[k] = true
			}
		}
	}
	var out []groupSpan
	for i := 0; i < len(nodes); {
		if owned[i] {
			for r, span := range blocks {
				if span[0] == i {
					out = append(out, groupSpan{start: span[0], end: span[1], kind: KindRequest, core: r - span[0]})
					i = span[1]
					break
				}
			}
			continue
		}
		kind := unitKind(nodes[i])
		end := i + 1
		if kind == KindStrip {
			for end < len(nodes) && unitKind(nodes[end]) == KindStrip && !owned[end] {
				end++
			}
		}
		out = append(out, groupSpan{start: i, end: end, kind: kind})
		i = end
	}
	return out
}

// attachedBlocks finds, for each request or subflow node, the run of actions
// right before and after it that name it in attach: only those that touch the
// request or each other, so a block is always a stretch of the list and
// the order of the run is untouched.
func attachedBlocks(nodes []model.Node) map[int][2]int {
	out := map[int][2]int{}
	for r, n := range nodes {
		if n.Type != "request" && n.Type != "subflow" {
			continue
		}
		lo, hi := r, r+1
		for lo > 0 && attachesTo(nodes[lo-1], n) {
			lo--
		}
		for hi < len(nodes) && attachesTo(nodes[hi], n) {
			hi++
		}
		if hi-lo > 1 {
			out[r] = [2]int{lo, hi}
		}
	}
	return out
}

func attachesTo(n, request model.Node) bool {
	return n.Attach == request.ID && n.Type != "request" && n.Type != "subflow" && unitKind(n) != KindContainer
}

func unitKind(n model.Node) string {
	switch {
	case n.Type == "request" || n.Type == "subflow":
		return KindRequest
	case n.Type == "parallel" || n.Type == "condition" || n.Type == "batch" || n.Type == "run_till":
		return KindContainer
	case n.Type == "fetch" || n.Type == "webhook" || len(n.Needs) > 0 || len(n.AfterFailureOf)+len(n.AfterOutcome) > 0 || n.Unlinked || n.Concurrent:
		return KindAction
	}
	return KindStrip
}

// applyManual moves units the user dragged; a moved container takes its
// lanes with it.
func applyManual(d *Diagram, manual map[string]Point) {
	for i := range d.Units {
		p, ok := manual[d.Units[i].ID]
		if !ok {
			continue
		}
		dx, dy := p.X-d.Units[i].X, p.Y-d.Units[i].Y
		shift(d, d.Units[i].ID, dx, dy)
	}
}

func shift(d *Diagram, id string, dx, dy int64) {
	for i := range d.Units {
		if d.Units[i].ID == id {
			d.Units[i].X += dx
			d.Units[i].Y += dy
		}
	}
	for i := range d.Lanes {
		if d.Lanes[i].Container == id {
			d.Lanes[i].X += dx
			d.Lanes[i].Y += dy
			d.Lanes[i].PortY += dy
		}
	}
	for _, u := range d.Units {
		if u.Parent == id {
			shift(d, u.ID, dx, dy)
		}
	}
}

func markCurves(d *Diagram) {
	for i, e := range d.Edges {
		from, _ := d.Unit(e.From)
		to, _ := d.Unit(e.To)
		d.Edges[i].Curve = from.PortYOf(e.FromPort) != to.PortY()
	}
}

// checkOf is the check of a condition without cases.
func checkOf(n model.Node) string {
	for _, f := range n.Params {
		if f.Key == "check" {
			text, _ := f.Value.(string)
			return text
		}
	}
	return ""
}

// conditionLabel is "if <test>" for a lane, or just the word when the test is
// not written yet.
func conditionLabel(word, test string) string {
	if strings.TrimSpace(test) == "" {
		return word + " …"
	}
	return word + " " + test
}

// placement is where around its request a node of a block is drawn.
type placement int

const (
	placeLeft placement = iota
	placeRight
	placeAbove
	placeBelow
)

func placementOf(n model.Node, i, core int) placement {
	switch n.AttachAt {
	case "above":
		return placeAbove
	case "below":
		return placeBelow
	}
	if i < core {
		return placeLeft
	}
	return placeRight
}

// blockCells lays out a request with the actions drawn with it, as a row of
// cells with the request in the middle, and the actions that say above or
// below stacked over and under the request. The cells are in the order of
// span; w, h are the block's size and above is how far down the request sits.
func blockCells(span []model.Node, core int) (cells []Cell, w, h, above int64) {
	var left, right, up, down int64
	for i, n := range span {
		if i == core {
			continue
		}
		switch placementOf(n, i, core) {
		case placeLeft:
			left++
		case placeRight:
			right++
		case placeAbove:
			up++
		case placeBelow:
			down++
		}
	}
	above = up * (stackH + stackGap)
	coreX := left * attachedW
	cells = make([]Cell, len(span))
	var l, r, a, b int64
	for i, n := range span {
		if i == core {
			cells[i] = Cell{X: coreX, Y: above, W: requestW, H: requestH}
			continue
		}
		switch placementOf(n, i, core) {
		case placeLeft:
			cells[i] = Cell{X: l * attachedW, Y: above, W: attachedW, H: requestH}
			l++
		case placeRight:
			cells[i] = Cell{X: coreX + requestW + r*attachedW, Y: above, W: attachedW, H: requestH}
			r++
		case placeAbove:
			cells[i] = Cell{X: coreX, Y: a * (stackH + stackGap), W: requestW, H: stackH}
			a++
		case placeBelow:
			cells[i] = Cell{X: coreX, Y: above + requestH + stackGap + b*(stackH+stackGap), W: requestW, H: stackH}
			b++
		}
	}
	w = left*attachedW + requestW + right*attachedW
	h = above + requestH
	if down > 0 {
		h += stackGap + down*(stackH+stackGap) - stackGap
	}
	return cells, w, h, above
}

// The room between the main line and a step that hangs below it, and between two of those stacked.
const (
	handlerDrop = 44
	handlerGap  = 24
)

// handlerSource says whether a unit only runs on an outcome of another step (it has arrows after a failure or a
// named outcome and none on success) and, if so, the unit it hangs from.
func handlerSource(n model.Node, unitOf map[string]string) (string, bool) {
	if len(n.Needs) > 0 || len(n.AfterFailureOf)+len(n.AfterOutcome) == 0 {
		return "", false
	}
	first := ""
	if len(n.AfterFailureOf) > 0 {
		first = n.AfterFailureOf[0]
	} else if id, _, ok := strings.Cut(n.AfterOutcome[0], ":"); ok {
		first = id
	}
	unit, ok := unitOf[first]
	return unit, ok
}

// bottomOf is the bottom edge of an already placed unit.
func (l *layouter) bottomOf(id string) int64 {
	for _, u := range l.units {
		if u.ID == id {
			return u.Y + u.H
		}
	}
	return 0
}

// handlerColumn is where the steps that hang below the line from one source go: one column right after the
// source, the first one at the top and each next one under it.
type handlerColumn struct {
	source string
	x      int64
	nextY  int64
	placed bool
}

// slot is where the next step hanging from source goes: the cursor and the given top when it starts a column,
// else the column's x and the next free row.
func (c *handlerColumn) slot(source string, cursor, top int64) (x, y int64) {
	if c.placed && c.source == source {
		return c.x, c.nextY
	}
	c.source, c.placed = source, false // a new column, opened by the unit that goes here
	return cursor, top
}

// advance records the unit just placed and says whether it opened a column, which the steps after it make
// room for; a step stacked in an open column takes none.
func (c *handlerColumn) advance(u Unit) bool {
	opened := !c.placed || c.x != u.X
	*c = handlerColumn{source: c.source, x: u.X, nextY: u.Y + u.H + handlerGap, placed: true}
	return opened
}
