// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"slices"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// outcomesOf are the outcomes of request step n: its own when it has some, else the version's it runs. own
// says which, and kind picks the help the rules get (http, message or dataset).
func (s *FlowService) outcomesOf(own model.Collection, n model.Node) (outcomes []model.Outcome, ownList bool, kind string) {
	kind = "http"
	_, req, ok := s.requestOf(own, n.Request.Request)
	if v, found := versionOf(req, n.Request.Version); ok && found {
		switch v.Family() {
		case model.FamilyDataset:
			kind = "dataset"
		case model.FamilyHTTP:
		default:
			kind = "message"
		}
		outcomes = v.EffectiveOutcomes()
	} else {
		outcomes = model.DefaultOutcomes()
	}
	if len(n.Request.Outcomes) > 0 {
		return slices.Clone(n.Request.Outcomes), true, kind
	}
	return slices.Clone(outcomes), false, kind
}

// SetStepOutcomes gives request step `step` its own outcomes, in place of its request's (the same request can
// end in different ways in different flows). An empty list takes them away again.
func (s *FlowService) SetStepOutcomes(node, step string, outcomes []model.Outcome) error {
	return s.edit(node, func(o *openFlow) error {
		n, ok := o.doc.Node(step)
		if !ok || n.Request == nil {
			return fmt.Errorf("%s is not a request step", step)
		}
		o.doc.EditNode(step, func(n *model.Node) { n.Request.Outcomes = slices.Clone(outcomes) })
		return nil
	})
}

// SaveStepOutcomesToRequest writes the outcomes request step `step` shows into the version of the request it
// runs, and drops the step's own list: the request and the step then say the same. The request's file is
// rewritten, so any other flow using it sees the new outcomes.
func (s *FlowService) SaveStepOutcomesToRequest(node, step string) error {
	return s.edit(node, func(o *openFlow) error {
		n, ok := o.doc.Node(step)
		if !ok || n.Request == nil {
			return fmt.Errorf("%s is not a request step", step)
		}
		coll, req, found := s.requestOf(o.doc.Collection(), n.Request.Request)
		if !found {
			return errors.New("the request of this step cannot be found")
		}
		v, _ := versionOf(req, n.Request.Version)
		list, _, _ := s.outcomesOf(o.doc.Collection(), n)
		saved := slices.Clone(req.Versions)
		for i := range saved {
			if saved[i].Label == v.Label {
				saved[i].Outcomes = list
			}
		}
		req.Versions = saved
		if err := s.app.backend.SaveRequest(flowi.RequestTarget{Collection: coll.Manifest.ID, Request: req.ID}, req, false); err != nil {
			return fmt.Errorf("saving the request: %w", err)
		}
		o.doc.EditNode(step, func(n *model.Node) { n.Request.Outcomes = nil })
		return nil
	})
}
