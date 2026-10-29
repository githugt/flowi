// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"maps"
	"slices"
)

// DebugView is the open flow's breakpoints, and while a session is paused, where, with the variables the next part
// will start with.
type DebugView struct {
	Breaks []string   `json:"breaks"` // steps a session pauses before, in flow order
	Paused string     `json:"paused"` // the step a session waits before, "" when none
	Vars   []VarValue `json:"vars"`   // what the next part starts with; empty unless paused
}

// debugOf is the open flow's debug state.
func debugOf(o *openFlow) DebugView {
	v := DebugView{Breaks: o.run.Breaks(), Paused: o.run.Paused(), Vars: []VarValue{}}
	if v.Paused != "" {
		vars := o.run.Vars()
		for _, name := range slices.Sorted(maps.Keys(vars)) {
			v.Vars = append(v.Vars, VarValue{Name: name, Value: oneLine(vars[name], varValueWidth)})
		}
	}
	return v
}

// ToggleBreak sets or clears a breakpoint before a step. The list is kept with the open flow, not in the file.
func (s *FlowService) ToggleBreak(node, step string) {
	_ = s.edit(node, func(o *openFlow) error {
		if _, ok := o.doc.Node(step); !ok {
			return fmt.Errorf("no step %s", step)
		}
		o.run.ToggleBreak(step)
		s.app.emit("flow", s.view(node, o))
		return nil
	})
}

// Debug runs the flow and pauses before the first breakpoint.
func (s *FlowService) Debug(node string, inputs map[string]string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.run.Debug(s.app.workspace.SelectedEnv(), inputs)
		return nil
	})
}

// Continue runs from the paused step to the next breakpoint, or to the end.
func (s *FlowService) Continue(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.run.Continue(); return nil })
}

// Step runs the paused step and pauses before the next.
func (s *FlowService) Step(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.run.Step(); return nil })
}

// StopDebugging ends the session.
func (s *FlowService) StopDebugging(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.run.StopDebugging(); return nil })
}

// SetDebugVar changes a variable the next part starts with. text is read as JSON, or kept as text.
func (s *FlowService) SetDebugVar(node, name, text string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.run.SetVar(name, text)
		s.app.emit("flow", s.view(node, o))
		return nil
	})
}

// SetDebugOutput changes one value of a finished step's outputs, which the steps after it read.
func (s *FlowService) SetDebugOutput(node, step, key, text string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.run.SetOutput(step, key, text)
		s.app.emit("flow", s.view(node, o))
		return nil
	})
}
