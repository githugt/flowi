// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// FlowSuggestionView is a flow Flowi proposes for a collection, for the review dialog.
type FlowSuggestionView struct {
	Name  string                   `json:"name"`
	Path  string                   `json:"path"` // where it would be saved; also what Accept names it by
	Steps []FlowSuggestionStepView `json:"steps"`
}

// FlowSuggestionStepView is one step of a suggestion and the values it takes from earlier steps.
type FlowSuggestionStepView struct {
	ID      string                   `json:"id"`
	Request string                   `json:"request"`
	Method  string                   `json:"method"`
	Takes   []FlowSuggestionTakeView `json:"takes"`
}

// FlowSuggestionTakeView says which field of which earlier step fills a path parameter, and how sure the match is.
type FlowSuggestionTakeView struct {
	Param      string  `json:"param"`
	From       string  `json:"from"`
	Field      string  `json:"field"`
	Confidence float64 `json:"confidence"` // 0 to 1
}

// Suggestions lists the flows proposed for a collection from what its requests return. Nothing is saved.
func (s *BlueprintsService) Suggestions(collection string) []FlowSuggestionView {
	out := []FlowSuggestionView{}
	s.app.actor.Do(func() {
		list, err := s.app.workspace.SuggestFlows(model.ID(collection))
		if err != nil {
			s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)})
			return
		}
		for _, f := range list {
			v := FlowSuggestionView{Name: f.Name, Path: f.Path, Steps: []FlowSuggestionStepView{}}
			for _, step := range f.Steps {
				sv := FlowSuggestionStepView{ID: step.ID, Request: step.Request, Method: step.Method, Takes: []FlowSuggestionTakeView{}}
				for _, t := range step.Takes {
					sv.Takes = append(sv.Takes, FlowSuggestionTakeView{Param: t.Param, From: t.From, Field: t.Field, Confidence: t.Confidence})
				}
				v.Steps = append(v.Steps, sv)
			}
			out = append(out, v)
		}
	})
	return out
}

// AcceptSuggestion saves the suggestion with this path as a flow of the collection and asks the window to open it
// ("open" event).
func (s *BlueprintsService) AcceptSuggestion(collection, path string) {
	s.app.actor.Do(func() {
		s.app.workspace.AcceptSuggestion(model.ID(collection), path,
			func(node string) { s.app.emit("open", node) },
			func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
	})
}

// NegativeVariantView is one way to send a request wrong, for the checklist.
type NegativeVariantView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// NegativesView is the variants of a request, or why there are none to offer.
type NegativesView struct {
	Variants []NegativeVariantView `json:"variants"`
	Error    string                `json:"error"`
}

// AcceptedNegativesView says what accepting made: the overlays saved, and the names left alone because they exist.
type AcceptedNegativesView struct {
	Saved   []string `json:"saved"`
	Skipped []string `json:"skipped"`
	Error   string   `json:"error"`
}

// Negatives lists the ways the request on a tree row can be sent wrong: each required field dropped, each field of
// the wrong type, an expired token. Nothing is saved.
func (s *BlueprintsService) Negatives(node string) NegativesView {
	out := NegativesView{Variants: []NegativeVariantView{}}
	s.app.actor.Do(func() {
		list, err := s.app.workspace.SuggestNegatives(node)
		if err != nil {
			out.Error = vm.ErrorText(err)
			return
		}
		for _, o := range list {
			out.Variants = append(out.Variants, NegativeVariantView{Name: o.Name, Description: o.Description})
		}
	})
	return out
}

// AcceptNegatives saves the ticked variants as flowi overlays of the request's collection.
func (s *BlueprintsService) AcceptNegatives(node string, names []string) AcceptedNegativesView {
	out := AcceptedNegativesView{Saved: []string{}, Skipped: []string{}}
	s.app.actor.Do(func() {
		saved, skipped, err := s.app.workspace.AcceptNegatives(node, names)
		out.Saved, out.Skipped = append(out.Saved, saved...), append(out.Skipped, skipped...)
		if err != nil {
			out.Error = vm.ErrorText(err)
			return
		}
		// Open requests list their overlays as they were when opened. Already on the actor here, so no actor.Do.
		for _, o := range s.app.requests.reqs {
			o.req.LoadOverlays()
		}
	})
	return out
}
