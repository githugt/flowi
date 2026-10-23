// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/canvas"
	"github.com/githugt/flowi/internal/vm"
)

// Select chooses the step shown in the inspector ("" for the flow itself).
func (s *FlowService) Select(node, step string) {
	s.app.actor.Do(func() {
		if o, ok := s.docs[node]; ok {
			o.selected, o.also = step, nil
			s.app.emit("flow", s.view(node, o))
		}
	})
}

// ToggleSelect adds a step to the selection, or takes it out (shift-click).
func (s *FlowService) ToggleSelect(node, step string) {
	s.app.actor.Do(func() {
		o, ok := s.docs[node]
		if !ok {
			return
		}
		sel := o.selection()
		if i := slices.Index(sel, step); i >= 0 {
			sel = slices.Delete(sel, i, i+1)
		} else {
			sel = append(sel, step)
		}
		o.selected, o.also = "", nil
		if n := len(sel); n > 0 {
			o.selected, o.also = sel[n-1], sel[:n-1]
		}
		s.app.emit("flow", s.view(node, o))
	})
}

// DeleteSelected removes every selected step.
func (s *FlowService) DeleteSelected(node string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.doc.Delete(o.selection()...)
		return nil
	})
}

// Connect makes the unit `to` wait for the unit `from`: its first step gets
// the last step of `from` in its needs (dragging from a port to a step).
func (s *FlowService) Connect(node, from, to string) {
	_ = s.edit(node, func(o *openFlow) error {
		g := o.doc.Diagram()
		src, ok1 := g.Unit(from)
		dst, ok2 := g.Unit(to)
		if !ok1 || !ok2 || from == to {
			return nil
		}
		first, need := dst.Nodes[0], src.Nodes[len(src.Nodes)-1]
		if _, ok := o.doc.Node(first); ok && !o.doc.AddNeed(first, need) {
			s.app.emit("notice", Notice{Level: "info", Text: redundantNeedText(first, need, o.doc.Previous(first) == need)})
		}
		return nil
	})
}

// redundantNeedText says why dragging a link from need to step did nothing.
func redundantNeedText(step, need string, follows bool) string {
	if follows {
		return step + " follows " + need + " in the list already, so it needs no link."
	}
	return step + " already waits for " + need + "."
}

// AddNeed makes the selected step wait for step id, in addition to the step
// before it. Naming the step before it, or one it waits for already, does nothing.
func (s *FlowService) AddNeed(node, id string) {
	_ = s.edit(node, func(o *openFlow) error { o.doc.AddNeed(o.selected, id); return nil })
}

// RemoveNeed stops the selected step waiting for step id.
func (s *FlowService) RemoveNeed(node, id string) {
	_ = s.edit(node, func(o *openFlow) error { o.doc.RemoveNeed(o.selected, id); return nil })
}

// RepairNeeds fixes steps that wait for a step after them.
func (s *FlowService) RepairNeeds(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.doc.RepairNeeds(); return nil })
}

// edit runs f on the open flow and returns what it reports. The document
// publishes the new view itself when f changed it.
func (s *FlowService) edit(node string, f func(o *openFlow) error) error {
	var err error
	s.app.actor.Do(func() {
		if o, ok := s.docs[node]; ok {
			err = f(o)
		}
	})
	return err
}

// SetText sets a text field of the selected step, or of the flow when none
// is selected. Fields: name, description, when, timeout, version,
// preferVersion, maxConcurrency, rateLimit.
func (s *FlowService) SetText(node, field, value string) error {
	return s.edit(node, func(o *openFlow) error {
		d, id := o.doc, o.selected
		merge := id + "/" + field
		switch field {
		case "name", "description", "when", "version":
			d.EditNodeMerged(merge, id, func(n *model.Node) { setNodeText(n, field, value) })
		case "timeout":
			dur, err := time.ParseDuration(value)
			if value != "" && err != nil {
				return errors.New("timeout must be a duration like 5s or 1m")
			}
			d.EditNodeMerged(merge, id, func(n *model.Node) { n.Timeout = dur })
		case "flowName", "flowDescription", "preferVersion", "rateLimit":
			d.EditFlowMerged("flow/"+field, func(f *model.Flow) { setFlowText(f, field, value) })
		case "maxConcurrency":
			n, err := strconv.Atoi(value)
			if value != "" && (err != nil || n < 0) {
				return errors.New("max concurrency must be a whole number")
			}
			d.EditFlowMerged("flow/"+field, func(f *model.Flow) { f.Execution.MaxConcurrency = n })
		default:
			return fmt.Errorf("unknown field %q", field)
		}
		return nil
	})
}

func setNodeText(n *model.Node, field, value string) {
	switch field {
	case "name":
		n.Name = value
	case "description":
		n.Description = value
	case "when":
		n.When = value
	case "version":
		if n.Request != nil {
			n.Request.Version = value
		}
	}
}

func setFlowText(f *model.Flow, field, value string) {
	switch field {
	case "flowName":
		f.Name = value
	case "flowDescription":
		f.Description = value
	case "preferVersion":
		f.PreferVersion = value
	case "rateLimit":
		f.Execution.RateLimit = value
	}
}

// SetConcurrent says whether the selected step may run alongside earlier ones.
func (s *FlowService) SetConcurrent(node string, on bool) {
	_ = s.edit(node, func(o *openFlow) error {
		o.doc.EditNode(o.selected, func(n *model.Node) { n.Concurrent = on })
		return nil
	})
}

// SetBranchPolicy sets the flow's on-branch-error policy.
func (s *FlowService) SetBranchPolicy(node, policy string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.doc.EditFlow(func(f *model.Flow) { f.Execution.OnBranchError = policy })
		return nil
	})
}

// SetParam sets parameter name of the selected step from the text the form
// holds; an empty text removes it. Typed parameters are converted by their
// kind, and a text that does not convert is ignored (the user is mid-typing).
func (s *FlowService) SetParam(node, name, value string) {
	_ = s.edit(node, func(o *openFlow) error {
		n, ok := o.doc.Node(o.selected)
		if !ok {
			return nil
		}
		v, keep := paramFrom(catalog().byType[string(n.Type)].Param(name))(value)
		if !keep && value != "" {
			return nil
		}
		o.doc.EditNodeMerged(o.selected+"/"+name, o.selected, func(n *model.Node) {
			if value == "" {
				n.Params = vm.DropParam(n.Params, name)
				return
			}
			n.Params = vm.SetParam(n.Params, name, v)
		})
		return nil
	})
}

// SetPairs replaces the rows of a key/value editor. Fields: with, headers,
// variables (steps); inputs, outputs (the flow).
func (s *FlowService) SetPairs(node, field string, rows []Pair) error {
	return s.edit(node, func(o *openFlow) error {
		d, id := o.doc, o.selected
		kv := keyValuesOf(rows)
		switch field {
		case "with", "headers":
			d.EditNodeMerged(id+"/"+field, id, func(n *model.Node) { setCallPairs(n, field, kv) })
		case "variables":
			spec := catalog().byType[nodeType(d, id)]
			d.EditNodeMerged(id+"/vars", id, func(n *model.Node) { n.Params = withVariables(n.Params, spec, kv) })
		case "inputs":
			d.EditFlowMerged("flow/inputs", func(f *model.Flow) {
				f.Inputs = nil
				for _, x := range kv {
					f.Inputs = append(f.Inputs, model.Input{Name: x.Key, Default: x.Value})
				}
			})
		case "outputs":
			d.EditFlowMerged("flow/outputs", func(f *model.Flow) { f.Outputs = kv })
		default:
			return fmt.Errorf("unknown field %q", field)
		}
		return nil
	})
}

func nodeType(d *vm.FlowDoc, id string) string {
	n, _ := d.Node(id)
	return string(n.Type)
}

func setCallPairs(n *model.Node, field string, kv []model.KeyValue) {
	switch {
	case n.Request != nil && field == "with":
		n.Request.With = kv
	case n.Request != nil && field == "headers":
		n.Request.Headers = kv
	case n.Subflow != nil && field == "with":
		n.Subflow.With = kv
	}
}

// SetCall chooses the request or flow the selected step calls.
func (s *FlowService) SetCall(node, ref string) {
	_ = s.edit(node, func(o *openFlow) error {
		n, ok := o.doc.Node(o.selected)
		if !ok {
			return nil
		}
		var choice *Choice
		choices := s.requestChoices(o.doc.Collection())
		if n.Subflow != nil {
			choices = flowChoices(o.doc.Collection())
		}
		for i := range choices {
			if choices[i].ID == ref {
				choice = &choices[i]
			}
		}
		if choice == nil {
			return nil
		}
		r := model.Ref{Collection: choice.Dep, ID: model.ID(choice.ID), Path: choice.Path}
		o.doc.EditNode(o.selected, func(n *model.Node) {
			switch {
			case n.Request != nil:
				n.Request.Request = r
			case n.Subflow != nil:
				n.Subflow.Flow = r
			}
		})
		return nil
	})
}

// SetHooks sets which API hooks the selected request step runs.
func (s *FlowService) SetHooks(node, mode string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.doc.EditNode(o.selected, func(n *model.Node) {
			if n.Request != nil {
				n.Request.Hooks = vm.HookControlOf(mode)
			}
		})
		return nil
	})
}

// SetFailRoute sets what the selected step does when it fails.
func (s *FlowService) SetFailRoute(node, mode, target string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.doc.EditNodeMerged(o.selected+"/on_error", o.selected, func(n *model.Node) {
			n.OnError = vm.FailRoute(vm.FailMode(mode), target)
		})
		return nil
	})
}

// SetAttach draws the selected step with the request or subflow step target
// ("" detaches it). Only the picture changes; the run order is the list order.
func (s *FlowService) SetAttach(node, target string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.doc.EditNodeMerged(o.selected+"/attach", o.selected, func(n *model.Node) { n.Attach = target })
		return nil
	})
}

// SetAttachAt says where around its request the selected attached step is drawn:
// "above" or "below" it in a stack, "" beside it. Only the picture changes.
func (s *FlowService) SetAttachAt(node, side string) {
	_ = s.edit(node, func(o *openFlow) error { o.doc.SetAttachAt(o.selected, side); return nil })
}

// AddAttached adds a step of type typ right before or after the selected request
// or subflow step and draws it with that step beside it. It returns the new
// step's id.
func (s *FlowService) AddAttached(node, typ string, after bool) (string, error) {
	return s.addAttached(node, typ, after, "")
}

// AddAttachedAt is AddAttached for a step drawn in the stack "above" the request
// (it runs right before it) or "below" it (right after).
func (s *FlowService) AddAttachedAt(node, typ, side string) (string, error) {
	return s.addAttached(node, typ, side == "below", side)
}

func (s *FlowService) addAttached(node, typ string, after bool, side string) (string, error) {
	var id string
	err := s.edit(node, func(o *openFlow) error {
		req, ok := o.doc.Node(o.selected)
		if !ok || (req.Type != "request" && req.Type != "subflow") {
			return fmt.Errorf("select a request or subflow step first")
		}
		list, at, _ := o.doc.Locate(req.ID)
		if after {
			at++
		}
		id = o.doc.Insert(model.NodeType(typ), list, at, func(n *model.Node) {
			n.Attach = req.ID
			if side == "above" || side == "below" {
				n.AttachAt = side
			}
		})
		o.selected = id
		s.app.emit("flow", s.view(node, o))
		return nil
	})
	return id, err
}

// MoveAttachedBy swaps the action id with its neighbour in the block it is
// drawn in: delta -1 moves it one place up (left), +1 one place down. The
// list is the run order, so this changes when the action runs, too. At the
// edge of the block it does nothing.
func (s *FlowService) MoveAttachedBy(node, id string, delta int) {
	_ = s.edit(node, func(o *openFlow) error {
		block, ok := o.doc.Diagram().UnitOf(id)
		if !ok || block.Kind != canvas.KindRequest || block.Nodes[block.Core] == id { // the request itself is not an action of its block
			return nil
		}
		pos := slices.Index(block.Nodes, id)
		next := pos + delta
		if pos < 0 || next < 0 || next >= len(block.Nodes) {
			return nil
		}
		list, at, _ := o.doc.Locate(block.Nodes[next])
		if delta > 0 {
			at++
		}
		moving, _ := o.doc.Node(id)
		return o.doc.MoveAttached(id, list, at, block.Nodes[block.Core], moving.AttachAt)
	})
}

// Rename gives the selected step a new id and returns the id it has after.
func (s *FlowService) Rename(node, to string) (string, error) {
	var id string
	err := s.edit(node, func(o *openFlow) error {
		id = o.selected
		if err := o.doc.Rename(o.selected, to); err != nil {
			return err
		}
		o.selected, id = to, to
		return nil
	})
	return id, err
}

// AddCaseTo adds an "else if" to condition step id (the "+ else if" on the canvas).
func (s *FlowService) AddCaseTo(node, id string) {
	_ = s.edit(node, func(o *openFlow) error {
		if n, ok := o.doc.Node(id); ok && n.Type == "condition" {
			o.doc.AddCase(id)
		}
		return nil
	})
}

// AddLane adds a case to the selected condition or a branch to the selected parallel.
func (s *FlowService) AddLane(node string) {
	_ = s.edit(node, func(o *openFlow) error {
		n, _ := o.doc.Node(o.selected)
		if n.Type == "condition" {
			o.doc.AddCase(o.selected)
		} else {
			o.doc.AddBranch(o.selected)
		}
		return nil
	})
}

// RemoveLane removes an empty case or branch of the selected step.
func (s *FlowService) RemoveLane(node string, i int) {
	_ = s.edit(node, func(o *openFlow) error {
		n, _ := o.doc.Node(o.selected)
		if n.Type == "condition" {
			o.doc.RemoveCase(o.selected, i)
		} else {
			o.doc.RemoveBranch(o.selected, i)
		}
		return nil
	})
}

// SetCaseWhen sets the condition of case i of the selected condition.
func (s *FlowService) SetCaseWhen(node string, i int, when string) {
	_ = s.edit(node, func(o *openFlow) error {
		id := o.selected
		o.doc.EditNodeMerged(fmt.Sprintf("%s/case%d", id, i), id, func(n *model.Node) {
			if i < len(n.Cases) {
				n.Cases[i].When = when
			}
		})
		return nil
	})
}

// Preview is the slot a drag would drop into at canvas point (x, y), for the
// marker drawn while dragging. step is the dragged step ("" for a new one
// from the palette), unit the dragged unit ("" for none), lift says the
// step is lifted out of its strip; snap is how close (canvas units) a drop
// must be to a connector.
func (s *FlowService) Preview(node, unit, step string, lift bool, x, y int64, snap float64) canvas.Slot {
	var slot canvas.Slot
	_ = s.edit(node, func(o *openFlow) error {
		slot = dropSlot(o.doc, unit, step, lift, model.Point{X: x, Y: y}, snap)
		return nil
	})
	return slot
}

func dropSlot(d *vm.FlowDoc, unit, step string, lift bool, p model.Point, snap float64) canvas.Slot {
	g := d.Diagram()
	if slot, ok := attachSlot(d, unit, step, lift, p); ok {
		return slot
	}
	if unit != "" && !lift {
		return canvas.UnitTarget(g, p, unit, firstNodeOf(g, unit, step), snap, d.Locate)
	}
	return canvas.CardTarget(g, p, step, snap, d.Locate)
}

// firstNodeOf is the node a whole dragged unit is placed by: its first one (a
// request block moves from its first action, not from the cell that was pressed).
func firstNodeOf(g canvas.Diagram, unit, step string) string {
	if u, ok := g.Unit(unit); ok && len(u.Nodes) > 0 {
		return u.Nodes[0]
	}
	return step
}

// attachable says whether a step of this type can be drawn with a request.
func attachable(t model.NodeType) bool {
	switch t {
	case "request", "subflow", "parallel", "condition", "batch", "run_till":
		return false
	}
	return true
}

// attachSlot is the slot of a single action dropped inside a request block: on
// the left half of the request itself it goes right before it and is drawn
// with it, on the right half right after. Between blocks, a drop is only a
// place in the list.
func attachSlot(d *vm.FlowDoc, unit, step string, lift bool, p model.Point) (canvas.Slot, bool) {
	n, ok := d.Node(step)
	if !ok || !attachable(n.Type) {
		return canvas.Slot{}, false
	}
	g := d.Diagram()
	if dragged, found := g.Unit(unit); found && !lift && len(dragged.Nodes) != 1 {
		return canvas.Slot{}, false // a whole block or strip is moved as it is, not attached
	}
	exclude := unit
	if lift {
		exclude = "" // a card lifted out of its own block can be put in another place of it
	}
	block, found := canvas.RequestAt(g, p, exclude)
	if !found {
		return canvas.Slot{}, false
	}
	rel := model.Point{X: p.X - block.X, Y: p.Y - block.Y}
	cell := canvas.CellAt(block, rel)
	list, at, located := d.Locate(block.Nodes[cell])
	if !located {
		return canvas.Slot{}, false
	}
	slot := attachmentAt(block, cell, rel, d)
	slot.List, slot.Index, slot.OK, slot.Attach = list, at+slot.Index, true, block.Nodes[block.Core]
	return slot, true
}

// attachmentAt says where in the cell of a request block a drop at rel (relative
// to the block) goes: the side of the request it is drawn on, whether it takes
// the place before the cell's node (Index 0) or after it (1), and the marker.
// The request's own cell has its top third for "above" it, its bottom third for
// "below" it, and its left and right halves for before and after it; the cell
// of an action takes its side from where that action is drawn.
func attachmentAt(block canvas.Unit, cell int, rel model.Point, d *vm.FlowDoc) canvas.Slot {
	c := canvas.CellsOf(block)[cell]
	at := func(x, y int64) (int64, int64) { return block.X + x, block.Y + y }
	vertical := func(after bool, side string) canvas.Slot {
		x := c.X
		if after {
			x += c.W
		}
		mx, my := at(x, c.Y)
		return canvas.Slot{X: mx, Y: my, H: c.H, Side: side, Index: boolIndex(after)}
	}
	horizontal := func(after bool, side string) canvas.Slot {
		y := c.Y
		if after {
			y += c.H
		}
		mx, my := at(c.X, y)
		return canvas.Slot{X: mx, Y: my, W: c.W, Horizontal: true, Side: side, Index: boolIndex(after)}
	}
	if cell == block.Core {
		switch third := c.H / 3; {
		case rel.Y < c.Y+third:
			return horizontal(false, "above")
		case rel.Y >= c.Y+c.H-third:
			return horizontal(true, "below")
		}
		return vertical(rel.X >= c.X+c.W/2, "")
	}
	n, _ := d.Node(block.Nodes[cell])
	if n.AttachAt == "above" || n.AttachAt == "below" {
		return horizontal(rel.Y >= c.Y+c.H/2, n.AttachAt)
	}
	return vertical(rel.X >= c.X+c.W/2, "")
}

func boolIndex(after bool) int {
	if after {
		return 1
	}
	return 0
}

// Drop ends a drag of a unit or a lifted step at (x, y): it moves the step
// (or, for a request block, all its steps) into the slot, or, for a whole unit
// dropped in empty space, leaves the unit where it was put (at, its top-left
// corner).
func (s *FlowService) Drop(node, unit, step string, lift bool, x, y int64, snap float64, atX, atY int64) {
	_ = s.edit(node, func(o *openFlow) error {
		d := o.doc
		slot := dropSlot(d, unit, step, lift, model.Point{X: x, Y: y}, snap)
		if slot.OK && moveTo(d, unit, step, lift, slot) == nil {
			if u, found := d.Diagram().UnitOf(step); found {
				d.Relayout([]string{u.ID})
			}
			return nil
		}
		if unit != "" && !lift {
			d.SetPosition(unit, model.Point{X: atX, Y: atY})
		}
		return nil
	})
}

func moveTo(d *vm.FlowDoc, unit, step string, lift bool, slot canvas.Slot) error {
	if slot.Attach != "" {
		return d.MoveAttached(step, slot.List, slot.Index, slot.Attach, slot.Side)
	}
	if u, ok := d.Diagram().Unit(unit); ok && !lift && len(u.Nodes) > 1 {
		return d.MoveRun(u.Nodes[0], len(u.Nodes), slot.List, slot.Index)
	}
	return d.Move(step, slot.List, slot.Index)
}

// AddStep adds a new step of type typ: at the slot under (x, y) when it has
// one, else at the end of the flow. It selects the new step.
func (s *FlowService) AddStep(node, typ string, x, y int64, snap float64, placed bool) string {
	var id string
	_ = s.edit(node, func(o *openFlow) error {
		d := o.doc
		list, index := "$.nodes", len(d.Draft().Nodes)
		// Added from the palette or the menu, a step goes after the selected one, else at the end.
		if l, i, ok := d.Locate(o.selected); !placed && o.selected != "" && ok {
			list, index = l, i+1
		}
		if placed {
			slot := canvas.CardTarget(d.Diagram(), model.Point{X: x, Y: y}, "", snap, d.Locate)
			if slot.OK {
				list, index = slot.List, slot.Index
			}
		}
		id = d.Insert(model.NodeType(typ), list, index, initialStep(model.NodeType(typ), d.NodesAt(list), index))
		o.selected = id
		s.app.emit("flow", s.view(node, o))
		return nil
	})
	return id
}

// AddStepNear adds a step of type typ right before or right after step, in
// the same list, selects it and returns its id. The canvas has a + on each
// side of a card: AddStep put every new step after the selected one, so a
// step could not be added in front of another. An unknown step adds at the
// end, as AddStep does.
func (s *FlowService) AddStepNear(node, typ, step string, before bool) string {
	var id string
	_ = s.edit(node, func(o *openFlow) error {
		d := o.doc
		list, index := "$.nodes", len(d.Draft().Nodes)
		if l, i, ok := d.Locate(step); ok {
			list, index = l, i
			if !before {
				index++
			}
		}
		id = d.Insert(model.NodeType(typ), list, index, initialStep(model.NodeType(typ), d.NodesAt(list), index))
		o.selected = id
		s.app.emit("flow", s.view(node, o))
		return nil
	})
	return id
}

// initialStep fills what a new step of type typ should not start without: a
// condition starts with a test about the request before it, which is what
// people test, so the field is not an empty box.
func initialStep(typ model.NodeType, siblings []model.Node, index int) func(n *model.Node) {
	if typ != "condition" || index < 1 || index > len(siblings) || siblings[index-1].Type != "request" {
		return nil
	}
	check := "nodes." + siblings[index-1].ID + ".status == 200"
	return func(n *model.Node) { n.Params = vm.SetParam(n.Params, "check", check) }
}

// paramFrom converts the text of a parameter of the given spec by its kind.
func paramFrom(spec flowi.NodeParam, known bool) func(string) (any, bool) {
	return func(text string) (any, bool) {
		if !known {
			return text, true
		}
		switch spec.Kind {
		case "bool":
			return text == "true", true
		case "int":
			n, err := strconv.ParseInt(text, 10, 64)
			return n, err == nil
		}
		return text, true
	}
}

// withVariables keeps the parameters the spec names and replaces the rest
// with the variable rows.
func withVariables(params []model.Field, spec flowi.NodeSpec, rows []model.KeyValue) []model.Field {
	var kept []model.Field
	for _, f := range params {
		if _, isSpec := spec.Param(f.Key); isSpec {
			kept = append(kept, f)
		}
	}
	for _, x := range rows {
		kept = append(kept, model.Field{Key: x.Key, Value: x.Value})
	}
	return kept
}

// Collapse folds a strip of actions into one pill, or opens it again.
func (s *FlowService) Collapse(node, unit string, collapsed bool) {
	_ = s.edit(node, func(o *openFlow) error { o.doc.SetCollapsed(unit, collapsed); return nil })
}

// Relayout puts the selected steps (all when none are selected) back where the
// layout would place them, dropping manual positions.
func (s *FlowService) Relayout(node string) {
	_ = s.edit(node, func(o *openFlow) error {
		var units []string
		for _, id := range o.selection() {
			if u, ok := o.doc.Diagram().UnitOf(id); ok {
				units = append(units, u.ID)
			}
		}
		o.doc.Relayout(units) // nil: everything
		return nil
	})
}

// Mermaid is the flow, unsaved edits included, as a Mermaid flowchart.
func (s *FlowService) Mermaid(node string) string {
	text := ""
	_ = s.edit(node, func(o *openFlow) error { text = o.doc.Mermaid(); return nil })
	return text
}

// SaveAsBlueprint saves the saved flow as a blueprint of its collection.
func (s *FlowService) SaveAsBlueprint(node, path, name string) {
	_ = s.edit(node, func(o *openFlow) error {
		s.app.workspace.SaveFlowAsBlueprint(o.doc.Target(), path, name,
			func() { s.app.emit("notice", Notice{Level: "info", Text: "Saved as a blueprint."}) },
			func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
		return nil
	})
}
