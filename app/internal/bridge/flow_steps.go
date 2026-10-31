// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// StepChoice is one thing the step popup offers.
type StepChoice struct {
	Group       string `json:"group"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Mode        string `json:"mode,omitempty"`
}

// stepsOf lists what the popup offers, in order.
func stepsOf() []StepChoice {
	entries := vm.StepCatalog()
	out := make([]StepChoice, 0, len(entries))
	for _, e := range entries {
		out = append(out, StepChoice{Group: e.Group, Name: e.Name, Description: e.Description, Type: e.Type, Mode: e.Mode})
	}
	return out
}

// ConvertStep turns step into the kind typ (with mode, "" for the kind's own)
// and returns the fields that do not carry over, so the panel can say so
// before the person agrees. With apply false nothing changes; with apply
// true the change is one undo step.
func (s *FlowService) ConvertStep(node, step, typ, mode string, apply bool) ([]string, error) {
	lost := []string{}
	err := s.edit(node, func(o *openFlow) error {
		n, ok := o.doc.Node(step)
		if !ok {
			return fmt.Errorf("no step %s", step)
		}
		converted, gone := vm.ConvertStep(n, vm.StepEntry{Type: typ, Mode: mode})
		lost = append(lost, gone...)
		if !apply {
			return nil
		}
		o.doc.EditNode(step, func(target *model.Node) { *target = converted })
		s.app.emit("flow", s.view(node, o))
		return nil
	})
	return lost, err
}

// SetColor gives step the colour (one of the library's names, "" for its
// category's own). One undo step.
func (s *FlowService) SetColor(node, step, color string) error {
	return s.edit(node, func(o *openFlow) error {
		if err := o.doc.SetColor(step, color); err != nil {
			return err
		}
		s.app.emit("flow", s.view(node, o))
		return nil
	})
}

// Duplicate puts a copy of step right after it, selects the copy and returns
// its id.
func (s *FlowService) Duplicate(node, step string) (string, error) {
	var id string
	err := s.edit(node, func(o *openFlow) error {
		var err error
		if id, err = o.doc.Duplicate(step); err != nil {
			return err
		}
		o.selected = id
		s.app.emit("flow", s.view(node, o))
		return nil
	})
	return id, err
}

// AddRouteByStatus puts a Route that takes a path by the response status of
// step right after it (cases for 200, 404 and 5xx, and an else), selects it
// and returns its id.
func (s *FlowService) AddRouteByStatus(node, step string) (string, error) {
	var id string
	err := s.edit(node, func(o *openFlow) error {
		var err error
		if id, err = o.doc.AddRouteByStatus(step); err != nil {
			return err
		}
		o.selected = id
		s.app.emit("flow", s.view(node, o))
		return nil
	})
	return id, err
}
