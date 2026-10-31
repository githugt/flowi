// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/mermaid"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/canvas"
	"github.com/githugt/flowi/internal/card"
)

// maxUndo bounds the undo history.
const maxUndo = 200

// FlowDoc is a flow being edited on the canvas (design §10.3): the draft,
// structural commands with undo/redo, manual positions, saving and
// reloading.
//
// Undo keeps whole copies of the flow: restoring one is exact by
// construction, which matters more here than memory (flows are small).
type FlowDoc struct {
	graphView bool // draw branches as arrows to lanes and attached actions with their request
	b         Backend
	w         *Worker
	coll      model.Collection
	target    flowi.FlowTarget
	disk      model.Flow
	draft     model.Flow
	undo      []model.Flow
	redo      []model.Flow
	positions map[string]model.Point // manual unit positions, by unit id
	collapsed map[string]bool
	layoutDue bool     // positions changed since the layout was last saved
	lastMerge string   // merge key of the last command; see EditNodeMerged
	rev       int      // bumped by every change to the draft
	onlyFlowI []string // steps only Flowi can run, as "id (type)"
	layoutRev int      // bumped by every change to positions or collapsed strips
	diagram   canvas.Diagram
	cards     map[string]card.Card // what each step shows, by node id; nil unless the graph view is on
	dirty     bool
	onDisk    bool
	confirm   bool
	err       string
	onChange  []func()
	onDrop    []func()
}

// NewFlowDoc opens flow f of collection c and loads its layout.
func NewFlowDoc(b Backend, w *Worker, c model.Collection, f model.Flow) *FlowDoc {
	d := &FlowDoc{b: b, w: w, coll: c, target: flowi.FlowTarget{Collection: c.Manifest.ID, Flow: f.ID},
		positions: map[string]model.Point{}, collapsed: map[string]bool{}}
	d.load(f)
	d.loadLayout()
	d.loadCompatibility()
	return d
}

// FlowIOnlySteps are the steps of the flow in its Arazzo file that only
// Flowi can run, as "id (type)": other Arazzo tools cannot run them.
func (d *FlowDoc) FlowIOnlySteps() []string { return d.onlyFlowI }

// loadCompatibility reads them in the background, and shows them when read.
func (d *FlowDoc) loadCompatibility() {
	target := d.target // read here: the worker must not touch the document
	Run(d.w, func() []string {
		var out []string
		for _, dg := range d.b.FlowCompatibility(target) {
			out = append(out, dg.Args["step"]+" ("+dg.Args["type"]+")")
		}
		return out
	}, func(steps []string) {
		d.onlyFlowI = steps
		d.changed()
	})
}

func (d *FlowDoc) load(f model.Flow) {
	d.disk, d.draft = f, cloneFlow(f)
	d.undo, d.redo = nil, nil // copies of another file version could resurrect what it removed
	d.dirty, d.onDisk, d.confirm = false, false, false
	d.relayout()
}

func (d *FlowDoc) loadLayout() {
	target := d.target
	type loaded struct {
		l     model.Layout
		diags diag.List
	}
	Run(d.w, func() loaded {
		l, diags, err := d.b.FlowLayout(target)
		if err != nil {
			return loaded{diags: diag.List{diag.New(diag.CodeFileUnreadable, diag.Location{}, map[string]string{"detail": err.Error()})}}
		}
		return loaded{l: l, diags: diags}
	}, func(r loaded) {
		maps.Copy(d.positions, r.l.Nodes)
		for _, id := range r.l.Collapsed {
			d.collapsed[id] = true
		}
		if len(r.diags) > 0 {
			d.err = "the layout file could not be read; positions start fresh: " + ErrorText(r.diags.Err())
		}
		d.relayout()
		d.changed()
	})
}

// OnChange registers f to run after any change.
func (d *FlowDoc) OnChange(f func()) { d.onChange = append(d.onChange, f) }

// OnDropConfirm registers f to run once each time a save stops because it
// would drop the flow file's comments.
func (d *FlowDoc) OnDropConfirm(f func()) { d.onDrop = append(d.onDrop, f) }

func (d *FlowDoc) changed() {
	for _, f := range d.onChange {
		f()
	}
}

// UseGraphView makes the diagram draw every step as a card with its lanes as
// branches (see canvas.WithCards) instead of as a box around the lanes and
// strips of actions.
func (d *FlowDoc) UseGraphView() {
	d.graphView = true
	d.relayout()
}

func (d *FlowDoc) relayout() {
	var opts []canvas.Option
	d.cards = nil
	if d.graphView {
		d.cards = CardsOf(d.coll, d.draft)
		opts = append(opts, canvas.WithCards(d.cards))
	}
	d.diagram = canvas.Layout(d.draft, d.positions, d.collapsed, opts...)
}

// Cards is what each step's card shows, by node id.
func (d *FlowDoc) Cards() map[string]card.Card { return d.cards }

// Target identifies the flow.
func (d *FlowDoc) Target() flowi.FlowTarget { return d.target }

// Collection is the flow's collection.
func (d *FlowDoc) Collection() model.Collection { return d.coll }

// Mermaid is the draft, unsaved edits included, as a Mermaid flowchart.
func (d *FlowDoc) Mermaid() string { return mermaid.Export(d.draft) }

// Arazzo exports the saved flow, and the flows it calls, as an Arazzo
// document: done gets its text and a note for each thing that has no Arazzo
// equivalent. Unsaved edits are not in it; the caller says so.
func (d *FlowDoc) Arazzo(done func(text string, notes []string), fail func(error)) {
	t := d.target
	type result struct {
		text  string
		diags diag.List
		err   error
	}
	Run(d.w, func() result {
		out, diags, err := d.b.ExportArazzo(t)
		return result{string(out), diags, err}
	}, func(r result) {
		switch {
		case r.err != nil && fail != nil:
			fail(r.err)
		case r.err == nil && done != nil:
			var notes []string
			for _, x := range r.diags {
				msg, _ := diag.English().Message(x)
				notes = append(notes, msg)
			}
			done(r.text, notes)
		}
	})
}

// Draft is the flow as edited.
func (d *FlowDoc) Draft() model.Flow { return d.draft }

// Diagram is the laid-out draft.
func (d *FlowDoc) Diagram() canvas.Diagram { return d.diagram }

// Preview lays out the draft with unit placed at p, without changing
// anything (the canvas shows it while a unit is dragged).
func (d *FlowDoc) Preview(unit string, p model.Point) canvas.Diagram {
	positions := maps.Clone(d.positions)
	positions[unit] = p
	return canvas.Layout(d.draft, positions, d.collapsed)
}

// Collapsed reports whether a strip is shown as a pill.
func (d *FlowDoc) Collapsed(unit string) bool { return d.collapsed[unit] }

// Dirty reports unsaved changes (flow or layout).
func (d *FlowDoc) Dirty() bool { return d.dirty || d.layoutDue }

// ChangedOnDisk reports a change on disk while there were edits.
func (d *FlowDoc) ChangedOnDisk() bool { return d.onDisk }

// NeedsDropConfirm reports that saving would drop the file's comments.
func (d *FlowDoc) NeedsDropConfirm() bool { return d.confirm }

// Err is the last problem, or "".
func (d *FlowDoc) Err() string { return d.err }

// CanUndo and CanRedo report whether there is something to undo or redo.
func (d *FlowDoc) CanUndo() bool { return len(d.undo) > 0 }

// CanRedo reports whether there is something to redo.
func (d *FlowDoc) CanRedo() bool { return len(d.redo) > 0 }

// command applies a structural change: it remembers the draft for undo,
// clears redo, and refreshes. A change that fails leaves the draft as it
// was and reports why.
func (d *FlowDoc) command(change func(f *model.Flow) error) error { return d.mergedCommand("", change) }

// mergedCommand is command where consecutive changes with the same
// non-empty merge key share one undo step (typing into one field).
func (d *FlowDoc) mergedCommand(merge string, change func(f *model.Flow) error) error {
	before := cloneFlow(d.draft)
	if err := change(&d.draft); err != nil {
		d.draft = before
		d.err = err.Error()
		d.changed()
		return err
	}
	if merge == "" || merge != d.lastMerge || len(d.undo) == 0 {
		d.undo = append(d.undo, before)
	}
	d.lastMerge = merge
	d.rev++
	if len(d.undo) > maxUndo {
		d.undo = d.undo[1:]
	}
	d.redo, d.dirty, d.err = nil, true, ""
	d.relayout()
	d.changed()
	return nil
}

// Undo restores the draft before the last command.
func (d *FlowDoc) Undo() {
	if len(d.undo) == 0 {
		return
	}
	d.redo = append(d.redo, d.draft)
	d.draft, d.undo = d.undo[len(d.undo)-1], d.undo[:len(d.undo)-1]
	d.lastMerge = "" // typing after an undo starts a new step
	d.rev++
	d.dirty = true
	d.relayout()
	d.changed()
}

// Redo re-applies the last undone command.
func (d *FlowDoc) Redo() {
	if len(d.redo) == 0 {
		return
	}
	d.undo = append(d.undo, d.draft)
	d.draft, d.redo = d.redo[len(d.redo)-1], d.redo[:len(d.redo)-1]
	d.rev++
	d.dirty = true
	d.relayout()
	d.changed()
}

// Insert adds a node of type typ at index of the list at listPath (an
// index past the end appends) and returns its generated id. init, if set,
// fills the node before it is added.
func (d *FlowDoc) Insert(typ model.NodeType, listPath string, index int, init func(*model.Node)) string {
	var id string
	_ = d.command(func(f *model.Flow) error {
		list, ok := listAt(f, listPath)
		if !ok {
			return fmt.Errorf("no node list at %s", listPath)
		}
		taken := map[string]bool{}
		allIDs(f.Nodes, taken)
		n := newNode(typ, taken)
		if init != nil {
			init(&n)
		}
		id = n.ID
		*list = slices.Insert(*list, min(max(index, 0), len(*list)), n)
		return nil
	})
	return id
}

// newNode is a node of typ with a free id ("log", "log-2", …) and the
// lists its type needs (two branches for a parallel).
func newNode(typ model.NodeType, taken map[string]bool) model.Node {
	n := model.Node{ID: freeID(strings.ReplaceAll(string(typ), "_", "-"), taken), Type: typ}
	taken[n.ID] = true
	if typ == "parallel" {
		for range 2 {
			b := model.Branch{ID: freeID("branch", taken)}
			taken[b.ID] = true
			n.Branches = append(n.Branches, b)
		}
	}
	return n
}

func freeID(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		if id := base + "-" + strconv.Itoa(i); !taken[id] {
			return id
		}
	}
}

// Delete removes nodes (with what they contain) and drops needs that
// named them.
func (d *FlowDoc) Delete(ids ...string) {
	_ = d.command(func(f *model.Flow) error {
		gone := map[string]bool{}
		for _, id := range ids {
			path, i, ok := locate(f.Nodes, "$.nodes", id)
			if !ok {
				continue
			}
			list, _ := listAt(f, path)
			allIDs((*list)[i:i+1], gone)
			*list = slices.Delete(*list, i, i+1)
		}
		dropNeeds(f.Nodes, gone)
		return nil
	})
}

// ErrMoveIntoItself refuses moving a container into one of its own lists.
var ErrMoveIntoItself = errors.New("a node cannot move into itself")

// Move puts node id at index of the list at listPath. A node that was drawn
// with a request (Attach) is not any more: it is somewhere else now.
func (d *FlowDoc) Move(id, listPath string, index int) error {
	return d.moveRun(id, 1, listPath, index, func(n *model.Node) { n.Attach, n.AttachAt = "", "" })
}

// MoveRun puts the count nodes that start at node id, as they are, at index of
// the list at listPath: a request with the actions drawn with it moves as one.
func (d *FlowDoc) MoveRun(id string, count int, listPath string, index int) error {
	return d.moveRun(id, count, listPath, index, nil)
}

// MoveAttached puts node id at index of the list at listPath and draws it with
// request (a step of that list, next to the index), beside it or, as side says,
// "above" or "below" it.
func (d *FlowDoc) MoveAttached(id, listPath string, index int, request, side string) error {
	return d.moveRun(id, 1, listPath, index, func(n *model.Node) { n.Attach, n.AttachAt = request, sideOrNone(side) })
}

// sideOrNone is side when it is a place a node can be drawn around its request.
func sideOrNone(side string) string {
	if side == "above" || side == "below" {
		return side
	}
	return ""
}

// SetAttachAt says where around its request the attached node id is drawn:
// "above", "below" or "" for beside it.
func (d *FlowDoc) SetAttachAt(id, side string) {
	d.EditNode(id, func(n *model.Node) {
		if n.Attach != "" {
			n.AttachAt = sideOrNone(side)
		}
	})
}

// moveRun moves count nodes from id on; change, if given, is applied to each.
func (d *FlowDoc) moveRun(id string, count int, listPath string, index int, change func(n *model.Node)) error {
	return d.command(func(f *model.Flow) error {
		from, i, ok := locate(f.Nodes, "$.nodes", id)
		if !ok {
			return fmt.Errorf("no node %s", id)
		}
		self := from + "[" + strconv.Itoa(i) + "]"
		if strings.HasPrefix(listPath, self+".") {
			return ErrMoveIntoItself
		}
		src, _ := listAt(f, from)
		if count < 1 || i+count > len(*src) {
			return fmt.Errorf("no %d nodes from %s", count, id)
		}
		run := slices.Clone((*src)[i : i+count])
		if i > 0 && len(run[0].Needs) == 1 && run[0].Needs[0] == (*src)[i-1].ID && len(run[0].AfterFailureOf) == 0 && len(run[0].AfterOutcome) == 0 {
			run[0].Needs = nil // "after the node before" said by hand: it now follows its new neighbour by itself
		}
		for k := range run {
			if change != nil {
				change(&run[k])
			}
		}
		*src = slices.Delete(*src, i, i+count)
		listPath = shiftAfterRemoval(listPath, from, i, count)
		dst, ok := listAt(f, listPath)
		if !ok {
			return fmt.Errorf("no node list at %s", listPath)
		}
		if from == listPath && index > i {
			index -= min(index-i, count) // the nodes left slots before the target
		}
		*dst = slices.Insert(*dst, min(max(index, 0), len(*dst)), run...)
		dropForwardNeeds(f.Nodes)
		return nil
	})
}

// ForwardNeeds lists the needs of the flow that name a node later in the same
// list, as "c needs b". Such a flow has a dependency cycle: the validator
// reports it and a run fails.
func ForwardNeeds(list []model.Node) []string {
	var out []string
	later := map[string]bool{}
	for _, n := range list {
		later[n.ID] = true
	}
	for i := range list {
		delete(later, list[i].ID)
		for _, id := range list[i].Needs {
			if later[id] {
				out = append(out, list[i].ID+" needs "+id)
			}
		}
		for _, id := range list[i].AfterFailureOf {
			if later[id] {
				out = append(out, list[i].ID+" runs after the failure of "+id)
			}
		}
		for _, entry := range list[i].AfterOutcome {
			if step, _, _ := strings.Cut(entry, ":"); later[step] {
				out = append(out, list[i].ID+" runs after "+entry)
			}
		}
		for _, child := range list[i].Children() {
			out = append(out, ForwardNeeds(child)...)
		}
	}
	return out
}

// ForwardNeeds are the draft's needs that name a later node.
func (d *FlowDoc) ForwardNeeds() []string { return ForwardNeeds(d.draft.Nodes) }

// RepairNeeds drops the needs that name a later node, so each such node follows
// the one before it again (one undoable edit).
func (d *FlowDoc) RepairNeeds() {
	_ = d.command(func(f *model.Flow) error { // the change cannot fail
		dropForwardNeeds(f.Nodes)
		return nil
	})
}

// dropForwardNeeds removes the needs that name a node later in the same list.
// A move can leave one (c needing b, then b moved behind c): nothing could be
// waited for there, and the canvas drew the chain broken at that node.
func dropForwardNeeds(list []model.Node) {
	later := map[string]bool{}
	for _, n := range list {
		later[n.ID] = true
	}
	for i := range list {
		delete(later, list[i].ID) // what is left in later is what comes after node i
		var kept []string
		for _, id := range list[i].Needs {
			if !later[id] {
				kept = append(kept, id)
			}
		}
		list[i].Needs = kept
		list[i].AfterFailureOf = slices.DeleteFunc(list[i].AfterFailureOf, func(id string) bool { return later[id] })
		if len(list[i].AfterFailureOf) == 0 {
			list[i].AfterFailureOf = nil
		}
		list[i].AfterOutcome = slices.DeleteFunc(list[i].AfterOutcome, func(entry string) bool { step, _, _ := strings.Cut(entry, ":"); return later[step] })
		if len(list[i].AfterOutcome) == 0 {
			list[i].AfterOutcome = nil
		}
		for _, child := range list[i].Children() {
			dropForwardNeeds(child)
		}
	}
}

// shiftAfterRemoval fixes a list path computed before the count nodes at index
// i of list from were removed: a path through a later sibling of that list
// now has that sibling count indexes lower.
func shiftAfterRemoval(path, from string, i, count int) string {
	prefix := from + "["
	rest, ok := strings.CutPrefix(path, prefix)
	if !ok {
		return path
	}
	end := strings.Index(rest, "]")
	k, err := strconv.Atoi(rest[:max(end, 0)])
	if end < 0 || err != nil || k < i+count {
		return path
	}
	return prefix + strconv.Itoa(k-count) + rest[end:]
}

// SetNeeds replaces node id's needs. A needs that only names the node right
// before id is dropped: a node follows its predecessor without being told to.
func (d *FlowDoc) SetNeeds(id string, needs []string) {
	needs = slices.Clone(needs)
	if n, ok := d.Node(id); ok && len(needs) == 1 && needs[0] == d.Previous(id) && len(n.AfterFailureOf)+len(n.AfterOutcome) == 0 {
		needs = nil // with no other arrow, success from the step before is what a step does without being told
	}
	d.EditNode(id, func(n *model.Node) { n.Needs = needs })
}

// SetArrow sets which outcomes of step `from` take the arrow into step `to`: success, failure, any named
// outcome of its request, or none (the arrow is removed). It rewrites to's needs, after_failure_of and
// after_outcome from the arrows to has now, so an arrow that was only implied (to follows the step before
// it) becomes written down when it changes. The arrow from the step before `to` cannot be removed, since to
// would follow that step again by itself, so it is marked unlinked: it has no arrow into it, and any arrow
// put in later links it again.
func (d *FlowDoc) SetArrow(from, to string, outcomes []string) error {
	n, ok := d.Node(to)
	if !ok {
		return fmt.Errorf("no step %s", to)
	}
	if !slices.Contains(d.EarlierIDs(to), from) {
		return fmt.Errorf("%s does not come before %s", from, to)
	}
	previous := d.Previous(to)
	arrows := slices.DeleteFunc(n.Arrows(previous), func(a model.Arrow) bool { return a.From == from })
	if arrow := arrowOf(from, outcomes); len(outcomes) > 0 {
		arrows = append(arrows, arrow)
	}
	d.setArrows(to, arrows, previous)
	return nil
}

// arrowOf is the arrow from step `from` taken on the given outcomes.
func arrowOf(from string, outcomes []string) model.Arrow {
	a := model.Arrow{From: from}
	for _, o := range outcomes {
		switch o {
		case model.OutcomeSuccess:
			a.OnSuccess = true
		case model.OutcomeFailure:
			a.OnFailure = true
		default:
			if !a.Takes(o) {
				a.On = append(a.On, o)
			}
		}
	}
	return a
}

// ConnectOutcome makes step `to` run when step `from` ends with the outcome (dragging from that outcome's
// port to a step). A step with no arrows written down only follows the step before it; that implied arrow
// is dropped, since every arrow into a step has to be taken and `to` could never run when `from` ends this
// way and the step before it is skipped. Arrows already written stay, and `from` keeps the outcomes it had.
func (d *FlowDoc) ConnectOutcome(from, to, outcome string) error {
	n, ok := d.Node(to)
	if !ok {
		return fmt.Errorf("no step %s", to)
	}
	if !slices.Contains(d.EarlierIDs(to), from) {
		return fmt.Errorf("%s does not come before %s", from, to)
	}
	previous := d.Previous(to)
	var arrows []model.Arrow
	// The arrow from the step before is dropped only for an arrow that replaces it (a failure or a named outcome):
	// one more success arrow is added to it.
	if len(n.Needs)+len(n.AfterFailureOf)+len(n.AfterOutcome) > 0 || outcome == model.OutcomeSuccess {
		arrows = n.Arrows(previous)
	}
	i := slices.IndexFunc(arrows, func(a model.Arrow) bool { return a.From == from })
	if i < 0 {
		arrows = append(arrows, arrowOf(from, []string{outcome}))
	} else {
		merged := arrowOf(from, []string{outcome})
		arrows[i].OnSuccess = arrows[i].OnSuccess || merged.OnSuccess
		arrows[i].OnFailure = arrows[i].OnFailure || merged.OnFailure
		for _, name := range merged.On {
			if !arrows[i].Takes(name) {
				arrows[i].On = append(arrows[i].On, name)
			}
		}
	}
	d.setArrows(to, arrows, previous)
	return nil
}

// setArrows writes arrows as to's needs, after_failure_of and after_outcome.
func (d *FlowDoc) setArrows(to string, arrows []model.Arrow, previous string) {
	var needs, failures, named []string
	for _, a := range arrows {
		if a.OnSuccess {
			needs = append(needs, a.From)
		}
		if a.OnFailure {
			failures = append(failures, a.From)
		}
		for _, name := range a.On {
			named = append(named, a.From+":"+name)
		}
	}
	if len(failures) == 0 && len(named) == 0 && len(needs) == 1 && needs[0] == previous {
		needs = nil // success from the step before is what a step does without being told
	}
	unlinked := len(arrows) == 0 && previous != ""
	d.EditNode(to, func(n *model.Node) {
		n.Needs, n.AfterFailureOf, n.AfterOutcome, n.Unlinked = needs, failures, named, unlinked
	})
}

// NodesAt is the nodes of the list at listPath ("$.nodes", or inside a lane); nil
// when there is no such list.
func (d *FlowDoc) NodesAt(listPath string) []model.Node {
	list, ok := listAt(&d.draft, listPath)
	if !ok {
		return nil
	}
	return slices.Clone(*list)
}

// Previous is the id of the node right before id in its list, or "".
func (d *FlowDoc) Previous(id string) string {
	siblings, i := d.siblings(id)
	if i < 1 {
		return ""
	}
	return siblings[i-1].ID
}

// EarlierIDs are the ids of the nodes before id in its list: what it could
// wait for.
func (d *FlowDoc) EarlierIDs(id string) []string {
	siblings, i := d.siblings(id)
	var out []string
	for k := 0; k < i; k++ {
		out = append(out, siblings[k].ID)
	}
	return out
}

// siblings is the list that holds node id and id's index in it (-1 when there
// is no such node).
func (d *FlowDoc) siblings(id string) ([]model.Node, int) {
	listPath, i, ok := d.Locate(id)
	if !ok {
		return nil, -1
	}
	list, ok := listAt(&d.draft, listPath)
	if !ok {
		return nil, -1
	}
	return *list, i
}

// AddNeed makes node id wait for node need as well. Naming the node right
// before id, or a node id already waits for, changes nothing and reports false.
func (d *FlowDoc) AddNeed(id, need string) bool {
	n, ok := d.Node(id)
	if ok && n.Unlinked && need != id && slices.Contains(d.EarlierIDs(id), need) {
		// No arrow into it: the step before it is not a redundant wait, it is the arrow that was taken away.
		return d.ConnectOutcome(need, id, model.OutcomeSuccess) == nil
	}
	if !ok || need == id || slices.Contains(n.Needs, need) || (len(n.Needs)+len(n.AfterFailureOf)+len(n.AfterOutcome) == 0 && need == d.Previous(id)) {
		return false
	}
	base := slices.Clone(n.Needs)
	if len(n.Needs)+len(n.AfterFailureOf)+len(n.AfterOutcome) == 0 && !n.Unlinked && d.Previous(id) != "" {
		base = []string{d.Previous(id)} // the arrow from the step before it was implied: written down, so it is kept
	}
	d.SetNeeds(id, append(base, need))
	return true
}

// RemoveNeed stops node id waiting for node need.
func (d *FlowDoc) RemoveNeed(id, need string) {
	n, ok := d.Node(id)
	if !ok {
		return
	}
	d.SetNeeds(id, slices.DeleteFunc(slices.Clone(n.Needs), func(x string) bool { return x == need }))
}

// EditNode changes node id.
func (d *FlowDoc) EditNode(id string, change func(n *model.Node)) {
	_ = d.command(func(f *model.Flow) error {
		path, i, ok := locate(f.Nodes, "$.nodes", id)
		if !ok {
			return fmt.Errorf("no node %s", id)
		}
		list, _ := listAt(f, path)
		change(&(*list)[i])
		return nil
	})
}

// EditNodeMerged is EditNode for typing: edits with the same merge key
// in a row (e.g. "login/url") are one undo step.
func (d *FlowDoc) EditNodeMerged(merge, id string, change func(n *model.Node)) {
	_ = d.mergedCommand(merge, func(f *model.Flow) error {
		path, i, ok := locate(f.Nodes, "$.nodes", id)
		if !ok {
			return fmt.Errorf("no node %s", id)
		}
		list, _ := listAt(f, path)
		change(&(*list)[i])
		return nil
	})
}

// EditFlowMerged is EditFlow for typing (see EditNodeMerged).
func (d *FlowDoc) EditFlowMerged(merge string, change func(f *model.Flow)) {
	_ = d.mergedCommand(merge, func(f *model.Flow) error { change(f); return nil })
}

// Rename changes node id's id to name and updates every needs that named
// it. Template references (nodes.<id>…) are not rewritten; validate
// reports them.
func (d *FlowDoc) Rename(id, name string) error {
	err := d.command(func(f *model.Flow) error {
		taken := map[string]bool{}
		allIDs(f.Nodes, taken)
		switch {
		case name == "":
			return errors.New("a node needs an id")
		case name != id && taken[name]:
			return fmt.Errorf("id %q is taken", name)
		}
		path, i, ok := locate(f.Nodes, "$.nodes", id)
		if !ok {
			return fmt.Errorf("no node %s", id)
		}
		list, _ := listAt(f, path)
		(*list)[i].ID = name
		renameNeeds(f.Nodes, id, name)
		return nil
	})
	if err == nil && id != name {
		d.moveLayoutKey(id, name)
	}
	return err
}

// moveLayoutKey carries a unit's manual position and collapsed state over
// to its new id.
func (d *FlowDoc) moveLayoutKey(from, to string) {
	changed := false
	if p, ok := d.positions[from]; ok {
		delete(d.positions, from)
		d.positions[to] = p
		changed = true
	}
	if d.collapsed[from] {
		delete(d.collapsed, from)
		d.collapsed[to] = true
		changed = true
	}
	if changed {
		d.layoutDue = true
		d.layoutRev++
		d.relayout()
		d.changed()
	}
}

func renameNeeds(list []model.Node, from, to string) {
	for i := range list {
		for _, ids := range [][]string{list[i].Needs, list[i].AfterFailureOf} {
			for j, n := range ids {
				if n == from {
					ids[j] = to
				}
			}
		}
		for j, entry := range list[i].AfterOutcome {
			if step, name, ok := strings.Cut(entry, ":"); ok && step == from {
				list[i].AfterOutcome[j] = to + ":" + name
			}
		}
		for _, child := range listsOf(&list[i]) {
			renameNeeds(*child, from, to)
		}
	}
}

// AddBranch adds an empty branch with a free id to parallel node id.
func (d *FlowDoc) AddBranch(id string) {
	d.EditNode(id, func(n *model.Node) {
		taken := map[string]bool{}
		allIDs(d.draft.Nodes, taken)
		n.Branches = append(n.Branches, model.Branch{ID: freeID("branch", taken)})
	})
}

// RemoveCase removes case i of condition id, if its lane is empty: removing a
// case with steps in it would delete the steps, so those are moved out first.
func (d *FlowDoc) RemoveCase(id string, i int) {
	d.EditNode(id, func(n *model.Node) {
		if i >= 0 && i < len(n.Cases) && len(n.Cases[i].Nodes) == 0 {
			n.Cases = slices.Delete(n.Cases, i, i+1)
		}
	})
}

// RemoveBranch removes branch i of parallel id, if its lane is empty.
func (d *FlowDoc) RemoveBranch(id string, i int) {
	d.EditNode(id, func(n *model.Node) {
		if i >= 0 && i < len(n.Branches) && len(n.Branches[i].Nodes) == 0 {
			n.Branches = slices.Delete(n.Branches, i, i+1)
		}
	})
}

// AddCase adds an empty case (to be given a condition) to node id: an "else
// if" after the cases it has. A condition that is still a plain if/else
// becomes a chain first, its check and its "then" steps being the first case,
// so what it does does not change.
func (d *FlowDoc) AddCase(id string) {
	d.EditNode(id, func(n *model.Node) {
		if len(n.Cases) == 0 {
			n.Cases = []model.Case{{When: ParamText(*n, "check"), Nodes: n.Then}}
			n.Then = nil
			n.Params = DropParam(n.Params, "check")
		}
		n.Cases = append(n.Cases, model.Case{})
	})
}

// EditFlow changes the flow's own properties.
func (d *FlowDoc) EditFlow(change func(f *model.Flow)) {
	_ = d.command(func(f *model.Flow) error { change(f); return nil })
}

// AttachChoices are the request or subflow steps node id can be shown with:
// the nearest one before it and the nearest one after it in its own list.
// Further away the canvas could not draw them as one block without moving
// steps in between.
func (d *FlowDoc) AttachChoices(id string) []string {
	list, at, ok := d.Locate(id)
	if !ok {
		return nil
	}
	var before, after string
	for _, u := range d.diagram.Units {
		l, first, split := canvas.SplitPath(u.Path)
		if !split || l != list {
			continue
		}
		for k, node := range u.Nodes {
			if u.Types[k] != "request" && u.Types[k] != "subflow" {
				continue
			}
			switch i := first + k; {
			case i < at:
				before = node
			case i > at && after == "":
				after = node
			}
		}
	}
	var out []string
	for _, c := range []string{before, after} {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

// Locate returns node id's list path and index.
func (d *FlowDoc) Locate(id string) (string, int, bool) { return locate(d.draft.Nodes, "$.nodes", id) }

// NodePath is node id's YAML path, or "".
func (d *FlowDoc) NodePath(id string) string {
	list, i, ok := d.Locate(id)
	if !ok {
		return ""
	}
	return list + "[" + strconv.Itoa(i) + "]"
}

// Node returns node id as edited.
func (d *FlowDoc) Node(id string) (model.Node, bool) {
	list, i, ok := d.Locate(id)
	if !ok {
		return model.Node{}, false
	}
	nodes, _ := listAt(&d.draft, list)
	return (*nodes)[i], true
}

// SetPosition places a unit by hand (free drag; not on the undo stack).
func (d *FlowDoc) SetPosition(unit string, p model.Point) {
	d.positions[unit] = p
	d.layoutDue = true
	d.layoutRev++
	d.relayout()
	d.changed()
}

// MakeRoom moves the units placed by hand at x or further right by dx, to make room for a step put in
// front of them. Units the layout places itself already move.
func (d *FlowDoc) MakeRoom(x, dx int64) {
	moved := false
	for id, p := range d.positions {
		if p.X >= x {
			d.positions[id] = model.Point{X: p.X + dx, Y: p.Y}
			moved = true
		}
	}
	if !moved {
		return
	}
	d.layoutDue = true
	d.layoutRev++
	d.relayout()
	d.changed()
}

// SetCollapsed shows a strip as a pill, or expands it.
func (d *FlowDoc) SetCollapsed(unit string, collapsed bool) {
	if collapsed {
		d.collapsed[unit] = true
	} else {
		delete(d.collapsed, unit)
	}
	d.layoutDue = true
	d.layoutRev++
	d.relayout()
	d.changed()
}

// Relayout drops manual positions of units (all when units is nil).
func (d *FlowDoc) Relayout(units []string) {
	if units == nil {
		clear(d.positions)
	}
	for _, u := range units {
		delete(d.positions, u)
	}
	d.layoutDue = true
	d.layoutRev++
	d.relayout()
	d.changed()
}

// Save writes the flow (if edited) and then the layout (if changed).
func (d *FlowDoc) Save(dropComments bool) {
	draft, flowDue, layoutDue := cloneFlow(d.draft), d.dirty, d.layoutDue
	rev, layoutRev := d.rev, d.layoutRev
	layout := model.Layout{Flow: d.target.Flow, Nodes: maps.Clone(d.positions), Collapsed: slices.Sorted(maps.Keys(d.collapsed))}
	target := d.target
	Run(d.w, func() error {
		if flowDue {
			if err := d.b.SaveFlow(target, draft, dropComments); err != nil {
				return err
			}
		}
		if layoutDue {
			return d.b.SaveFlowLayout(target, layout)
		}
		return nil
	}, func(err error) { d.saved(draft, flowDue, layoutDue, rev, layoutRev, err) })
}

// saved records a save's outcome. Edits made while it ran (a newer
// revision) stay unsaved.
func (d *FlowDoc) saved(draft model.Flow, flowDue, layoutDue bool, rev, layoutRev int, err error) {
	d.confirm, d.err = false, ""
	switch {
	case hasCode(err, diag.CodeWriteDropsComments):
		d.confirm = true
		d.changed()
		for _, f := range d.onDrop {
			f()
		}
		return
	case err != nil:
		d.err = ErrorText(err)
		d.changed()
		return
	}
	if flowDue {
		d.disk, d.dirty, d.onDisk = draft, d.rev != rev, false
	}
	if layoutDue {
		d.layoutDue = d.layoutRev != layoutRev
	}
	d.changed()
	if flowDue {
		d.loadCompatibility() // a saved edit may add or remove a Flowi-only step
	}
}

// Snapshot is a copy of the draft and its revision, for a write done
// elsewhere (a hook editor puts its nodes into the request); MarkSavedAt
// says it was written.
func (d *FlowDoc) Snapshot() (model.Flow, int) { return cloneFlow(d.draft), d.rev }

// MarkSavedAt records that the snapshot taken at revision rev is now what is
// on disk. Edits made since stay unsaved.
func (d *FlowDoc) MarkSavedAt(snapshot model.Flow, rev int) {
	d.disk, d.dirty, d.onDisk = snapshot, d.rev != rev, false
	d.changed()
}

// ConfirmDrop saves again, dropping the flow file's comments.
func (d *FlowDoc) ConfirmDrop() { d.Save(true) }

// DeclineDrop gives up the save that would drop comments.
func (d *FlowDoc) DeclineDrop() {
	d.confirm = false
	d.changed()
}

// Reloaded tells the document its file was reloaded; found is false when
// the flow no longer exists. Unsaved edits are kept and flagged.
func (d *FlowDoc) Reloaded(c model.Collection, f model.Flow, found bool) {
	d.coll = c
	switch {
	case !found:
		d.err = "this flow was removed or its file no longer loads (see Problems)"
	case d.dirty:
		d.disk, d.onDisk = f, true
	default:
		d.load(f)
	}
	d.changed()
}

// Revert drops the edits and shows the flow as on disk.
func (d *FlowDoc) Revert() {
	d.load(d.disk)
	d.err = ""
	d.changed()
}

// Source shows the draft, unsaved edits included, as text: flow is the
// flow's own data (what the code view edits), file the Arazzo document that
// saving would write.
func (d *FlowDoc) Source(done func(flow, file string), fail func(error)) {
	t, draft := d.target, cloneFlow(d.draft)
	type result struct {
		text flowi.FlowText
		err  error
	}
	Run(d.w, func() result {
		text, err := d.b.FlowSource(t, draft)
		return result{text, err}
	}, func(r result) {
		switch {
		case r.err != nil && fail != nil:
			fail(r.err)
		case r.err == nil && done != nil:
			done(r.text.Flow, r.text.File)
		}
	})
}

// ApplySource makes the flow what text says, as one undoable edit. When the
// text has problems, done gets one line for each ("line 3: …") and the draft
// stays as it was; no problems means it was applied (or already matched), and
// notes then say what is wrong with the flow as applied (a step that calls a
// request that does not exist): the flow is applied all the same, as saving
// one with problems is allowed too.
func (d *FlowDoc) ApplySource(text string, done func(problems, notes []string), fail func(error)) {
	t := d.target
	type result struct {
		flow  model.Flow
		diags diag.List
		err   error
	}
	Run(d.w, func() result {
		f, diags, err := d.b.ParseFlowSource(t, text)
		return result{f, diags, err}
	}, func(r result) {
		switch {
		case r.err != nil:
			if fail != nil {
				fail(r.err)
			}
		case len(r.diags) > 0:
			done(problemLines(r.diags), nil)
		default:
			d.replaceDraft(r.flow)
			d.noteDraftProblems(r.flow, func(notes []string) { done(nil, notes) })
		}
	})
}

// noteDraftProblems checks f against the rest of the workspace, off the UI
// goroutine, and hands done one line for each problem found.
func (d *FlowDoc) noteDraftProblems(f model.Flow, done func(notes []string)) {
	t := d.target
	Run(d.w, func() diag.List { return d.b.CheckFlowDraft(t, f) }, func(diags diag.List) {
		done(noteLines(f, diags))
	})
}

// noteLines words diagnostics about an applied flow, each with the step it is
// about when the path names a top-level step.
func noteLines(f model.Flow, diags diag.List) []string {
	var lines []string
	for _, x := range diags {
		msg, hint := diag.English().Message(x)
		line := string(x.Code) + ": " + msg
		if hint != "" {
			line += " (" + hint + ")"
		}
		if id := stepOfPath(f, x.Path); id != "" {
			line = "step " + id + ": " + line
		}
		lines = append(lines, line)
	}
	return lines
}

// stepOfPath is the id of the top-level step a path such as
// "$.nodes[2].request.id" is inside, or "".
func stepOfPath(f model.Flow, path string) string {
	rest, ok := strings.CutPrefix(path, "$.nodes[")
	if !ok {
		return ""
	}
	index, _, _ := strings.Cut(rest, "]")
	i, err := strconv.Atoi(index)
	if err != nil || i < 0 || i >= len(f.Nodes) {
		return ""
	}
	return f.Nodes[i].ID
}

// replaceDraft sets the draft to f, keeping the flow's identity; a flow equal
// to the draft is not an edit.
func (d *FlowDoc) replaceDraft(f model.Flow) {
	f.ID, f.File, f.Path = d.draft.ID, d.draft.File, d.draft.Path
	if reflect.DeepEqual(f, d.draft) {
		return
	}
	_ = d.command(func(draft *model.Flow) error { *draft = f; return nil })
}

// problemLines words diagnostics for the code view, each with its line.
func problemLines(diags diag.List) []string {
	var lines []string
	for _, x := range diags {
		msg, hint := diag.English().Message(x)
		line := string(x.Code) + ": " + msg
		if hint != "" {
			line += " (" + hint + ")"
		}
		if x.Pos.Line > 0 {
			line = fmt.Sprintf("line %d: %s", x.Pos.Line, line)
		}
		lines = append(lines, line)
	}
	return lines
}

// KeepEdits stores the unsaved edits, to come back with RestoreKept on the next
// start. It writes at once, not on the worker: a quit waits for it.
func (d *FlowDoc) KeepEdits() error {
	if !d.dirty {
		return nil
	}
	return d.b.KeepFlowEdits(d.target, d.draft)
}

// RestoreKept brings back the edits KeepEdits stored, as one edit that can be
// undone, and says whether there were any. Kept text that no longer reads is
// dropped: the file on disk is the flow then.
func (d *FlowDoc) RestoreKept(done func(restored bool)) {
	t := d.target
	type kept struct {
		text string
		ok   bool
	}
	Run(d.w, func() kept {
		text, ok, _ := d.b.KeptFlowEdits(t) // an unreadable store is as good as none
		return kept{text, ok}
	}, func(k kept) {
		if !k.ok {
			done(false)
			return
		}
		d.ApplySource(k.text, func(problems, _ []string) { done(len(problems) == 0) }, func(error) { done(false) })
	})
}
