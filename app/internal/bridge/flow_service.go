// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/canvas"
	"github.com/githugt/flowi/app/internal/card"
	"github.com/githugt/flowi/app/internal/vm"
)

// Card is what a canvas unit shows.
type Card struct {
	Title    string     `json:"title"`
	Subtitle string     `json:"subtitle"`
	Hooks    []HookTick `json:"hooks"`              // a request step: the hooks that run around it, and how each went
	Face     card.Card  `json:"face"`               // what the card shows: badge, sub-line, summary rows, ports
	Color    string     `json:"color,omitempty"`    // the colour the person gave the step
	Skipped  bool       `json:"skipped"`            // this run leaves the step out
	SkipNote string     `json:"skipNote,omitempty"` // "login is skipped, so this has no value", on a step that reads from one
	Break    bool       `json:"break"`              // a debug session pauses before this step
}

// FlowView is an open flow as the canvas shows it.
type FlowView struct {
	Rev       int             `json:"rev"`       // bumped by every change to the flow
	Selected  string          `json:"selected"`  // the selected step id, "" for none
	Selection []string        `json:"selection"` // every selected step, the primary last
	Chips     []ChipView      `json:"chips"`
	Form      Form            `json:"form"`
	Palette   []PaletteEntry  `json:"palette"`
	Steps     []StepChoice    `json:"steps"`   // what the step popup offers
	Start     card.Card       `json:"start"`   // the flow's inputs, drawn as the first card
	Inputs    []IOView        `json:"inputs"`  // what the Start card's rows open
	Outputs   []IOView        `json:"outputs"` // what the End card's rows open
	End       *card.Card      `json:"end"`     // its outputs, drawn as the last card; nil without outputs
	Run       RunView         `json:"run"`
	Skips     SkipView        `json:"skips"`
	Debug     DebugView       `json:"debug"`
	Vars      VarsView        `json:"vars"`
	Node      string          `json:"node"` // the workspace tree node it was opened from
	Name      string          `json:"name"`
	Dirty     bool            `json:"dirty"`
	OnDisk    bool            `json:"onDisk"`
	NeedsDrop bool            `json:"needsDrop"`
	CanUndo   bool            `json:"canUndo"`
	CanRedo   bool            `json:"canRedo"`
	Error     string          `json:"error,omitempty"`
	Diagram   canvas.Diagram  `json:"diagram"`
	Cards     map[string]Card `json:"cards"` // by unit id
	Data      []DataEdge      `json:"data"`  // values that pass between units
}

// DataEdge is a value one unit's step makes and another unit's step reads;
// the canvas draws it apart from the run order.
type DataEdge struct {
	From  string `json:"from"` // unit ids
	To    string `json:"to"`
	Label string `json:"label"`
}

type openFlow struct {
	doc      *vm.FlowDoc
	run      *vm.FlowRun
	history  *vm.RunHistory
	selected string   // the step the inspector shows
	also     []string // other selected steps (shift-click)
}

// FlowService is the set of flow documents open in the window.
type FlowService struct {
	app  *App
	docs map[string]*openFlow // by tree node id; used on the actor only
}

// Open opens the flow of a tree node, or returns the one already open.
// Every change to it publishes the "flow" event with the new FlowView.
func (s *FlowService) Open(node string) (FlowView, error) {
	var view FlowView
	var err error
	s.app.actor.Do(func() { view, err = s.open(node) })
	return view, err
}

func (s *FlowService) open(node string) (FlowView, error) {
	if o, ok := s.docs[node]; ok {
		return s.view(node, o), nil
	}
	n := s.app.workspace.Node(node)
	if n.Kind != vm.NodeFlow {
		return FlowView{}, fmt.Errorf("%q is not a flow", node)
	}
	c, ok := s.app.workspace.Collection(n.Collection)
	if !ok {
		return FlowView{}, fmt.Errorf("collection of %q is not loaded", node)
	}
	for _, f := range c.Flows {
		if f.ID != n.Flow {
			continue
		}
		doc := vm.NewFlowDoc(s.app.backend, s.app.worker, c, f)
		doc.UseGraphView()
		o := &openFlow{doc: doc, run: vm.NewFlowRun(s.app.backend, s.app.worker, doc), history: vm.NewRunHistory(s.app.backend, s.app.worker, doc)}
		doc.OnChange(func() { s.app.emit("flow", s.view(node, o)) })
		o.run.OnPrompt(s.app.promptPerson)
		o.run.OnStatus(func(string, string) { s.app.emit("flowrun", FlowRunEvent{Node: node, Run: runViewOf(o)}) })
		// The hook ticks on the cards are part of the flow view, so a run's change publishes both.
		o.run.OnChange(func() {
			s.app.emit("flowrun", FlowRunEvent{Node: node, Run: runViewOf(o)})
			s.app.emit("flow", s.view(node, o))
		})
		o.history.OnChange(func() { s.app.emit("flowhistory", HistoryEvent{Node: node, Runs: historyOf(o.history)}) })
		s.docs[node] = o
		doc.RestoreKept(func(bool) {}) // edits kept at the last quit come back as unsaved changes
		return s.view(node, o), nil
	}
	return FlowView{}, fmt.Errorf("flow of %q not found", node)
}

// adopt makes a document that has no file of its own (a request's hooks)
// editable under row, with the flow editor's commands.
func (s *FlowService) adopt(row string, doc *vm.FlowDoc) {
	doc.UseGraphView()
	o := &openFlow{doc: doc, run: vm.NewFlowRun(s.app.backend, s.app.worker, doc), history: vm.NewRunHistory(s.app.backend, s.app.worker, doc)}
	doc.OnChange(func() { s.app.emit("flow", s.view(row, o)) })
	s.docs[row] = o
}

// Close forgets the open flow of a tree node, discarding unsaved edits.
func (s *FlowService) Close(node string) {
	s.app.actor.Do(func() { delete(s.docs, node) })
}

// SetPosition records where the user dropped a unit.
func (s *FlowService) SetPosition(node, unit string, x, y int64) {
	s.with(node, func(d *vm.FlowDoc) { d.SetPosition(unit, model.Point{X: x, Y: y}) })
}

// Undo undoes the last structural edit.
func (s *FlowService) Undo(node string) { s.with(node, (*vm.FlowDoc).Undo) }

// Redo redoes what Undo undid.
func (s *FlowService) Redo(node string) { s.with(node, (*vm.FlowDoc).Redo) }

// Save writes the flow and its layout.
func (s *FlowService) Save(node string) { s.with(node, func(d *vm.FlowDoc) { d.Save(false) }) }

// Delete removes steps by node id.
func (s *FlowService) Delete(node string, ids []string) {
	s.with(node, func(d *vm.FlowDoc) { d.Delete(ids...) })
}

// Insert adds a step of type typ into the list at listPath, at index, and
// returns its id.
func (s *FlowService) Insert(node, typ, listPath string, index int) string {
	var id string
	s.with(node, func(d *vm.FlowDoc) { id = d.Insert(model.NodeType(typ), listPath, index, nil) })
	return id
}

func (s *FlowService) with(node string, f func(*vm.FlowDoc)) {
	s.app.actor.Do(func() {
		if o, ok := s.docs[node]; ok {
			f(o.doc)
		}
	})
}

// view builds the FlowView; call it on the actor.
func (s *FlowService) view(node string, o *openFlow) FlowView {
	d := o.doc
	o.prune()
	diagram := d.Diagram()
	_, rev := d.Snapshot()
	start, end := startEndOf(o)
	return FlowView{
		Rev:  rev,
		Node: node, Name: d.Draft().Name, Dirty: d.Dirty(), OnDisk: d.ChangedOnDisk(), NeedsDrop: d.NeedsDropConfirm(), CanUndo: d.CanUndo(), CanRedo: d.CanRedo(),
		Error: d.Err(), Diagram: diagram, Cards: s.cardsOf(o, diagram), Data: dataEdgesOf(d, diagram), Selection: o.selection(), Chips: chipsOf(d),
		Selected: o.selected, Form: s.formOf(d, o.run, o.selected), Palette: paletteOf(), Steps: stepsOf(),
		Run: runViewOf(o), Skips: skipsOf(o), Debug: debugOf(o), Vars: varsOf(o), Start: start, End: end, Inputs: inputViewsOf(d.Draft()), Outputs: outputViewsOf(d.Draft()),
	}
}

// dataEdgesOf maps the data links of the flow to the units that hold their
// steps. A link inside one unit (two steps of a strip) is not drawn.
func dataEdgesOf(d *vm.FlowDoc, g canvas.Diagram) []DataEdge {
	unitOf := map[string]string{}
	for _, u := range g.Units {
		for _, n := range u.Nodes {
			unitOf[n] = u.ID
		}
	}
	out := []DataEdge{}
	for _, l := range analysis.DataLinks(d.Draft()) {
		from, to := unitOf[l.From], unitOf[l.To]
		if from != "" && to != "" && from != to {
			out = append(out, DataEdge{From: from, To: to, Label: l.Via})
		}
	}
	return out
}

func (s *FlowService) cardsOf(o *openFlow, g canvas.Diagram) map[string]Card {
	cards := make(map[string]Card, len(g.Units))
	selected := o.selection()
	skips, debug := skipsOf(o), debugOf(o)
	for _, u := range g.Units {
		n, _ := o.doc.Node(u.Nodes[u.Core]) // the request of a block with attached actions, else the only or first node
		skipped, note := skipMark(skips, u.Nodes)
		cards[u.ID] = Card{Title: unitTitle(u, n, selected), Subtitle: unitSubtitle(u, n), Hooks: nonNilTicks(s.hookTicksOf(o, n)), Face: faceOf(o.doc, n), Color: n.Color, Skipped: skipped, SkipNote: note, Break: slices.ContainsFunc(u.Nodes, func(id string) bool { return slices.Contains(debug.Breaks, id) })}
	}
	return cards
}

// faceOf is what the step's card shows; empty when the document draws no cards.
func faceOf(d *vm.FlowDoc, n model.Node) card.Card {
	face, ok := d.Cards()[n.ID]
	if !ok {
		return card.Card{Rows: []card.Row{}, More: []card.Row{}, Ports: []card.Port{}}
	}
	return face
}

func nonNilTicks(t []HookTick) []HookTick {
	if t == nil {
		return []HookTick{}
	}
	return t
}

func unitTitle(u canvas.Unit, n model.Node, selected []string) string {
	switch {
	case u.Collapsed:
		return fmt.Sprintf("%d actions", len(u.Nodes))
	case u.Kind == canvas.KindStrip:
		return strings.Join(markSelected(u.Nodes, selected), " · ")
	}
	return n.ID
}

// markSelected brackets the selected action of a strip: "set · [log] · note".
func markSelected(nodes []string, selected []string) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n
		if slices.Contains(selected, n) {
			out[i] = "[" + n + "]"
		}
	}
	return out
}

func unitSubtitle(u canvas.Unit, n model.Node) string {
	switch {
	case u.Kind == canvas.KindStrip:
		return strings.Join(u.Types, " · ")
	case n.Request != nil, n.Subflow != nil:
		return vm.NodeSummary(n)
	}
	text := vm.NodeSummary(n)
	if u.Concurrent {
		text += " ∥"
	}
	return text
}

// Revert reloads the flow from disk, dropping edits.
func (s *FlowService) Revert(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.doc.Revert(); return nil })
}

// SaveDroppingComments saves although the file's comments are lost.
func (s *FlowService) SaveDroppingComments(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.doc.ConfirmDrop(); return nil })
}

// DeclineDrop gives up a save that would drop comments.
func (s *FlowService) DeclineDrop(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.doc.DeclineDrop(); return nil })
}

// selection is every selected step, the primary (the one the inspector shows) last.
func (o *openFlow) selection() []string {
	out := slices.Clone(o.also)
	if o.selected != "" {
		out = append(out, o.selected)
	}
	return nonNil(out)
}

// prune forgets selected steps that were deleted or renamed away.
func (o *openFlow) prune() {
	o.also = slices.DeleteFunc(o.also, func(id string) bool { _, ok := o.doc.Node(id); return !ok })
	if _, ok := o.doc.Node(o.selected); !ok {
		o.selected = ""
		if n := len(o.also); n > 0 {
			o.selected, o.also = o.also[n-1], o.also[:n-1]
		}
	}
}

// ChipView is one thing to tell about the flow: an error, steps that wait for
// a step after them, steps only Flowi can run, a change on disk, unsaved edits.
type ChipView struct {
	Kind  string   `json:"kind"` // error, needs, warning, disk, unsaved
	Text  string   `json:"text,omitempty"`
	Steps []string `json:"steps,omitempty"`
}

func chipsOf(d *vm.FlowDoc) []ChipView {
	out := []ChipView{}
	for _, c := range vm.FlowChips(d) {
		out = append(out, ChipView{Kind: string(c.Kind), Text: c.Text, Steps: nonNil(c.Steps)})
	}
	return out
}
