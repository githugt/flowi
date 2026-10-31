// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strings"
)

// DetailRow is one labelled value of the Details pane.
type DetailRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// DetailsView is what the Details pane of the footer shows for the tab in
// front: the values of what is open, stated plainly, where the headers only
// hint at them. Kind is request or flow, "" for a tab that has none.
type DetailsView struct {
	Kind  string      `json:"kind"`
	Title string      `json:"title"`
	Rows  []DetailRow `json:"rows"`
}

// Details is the Details pane for tree node `node`: empty when no request or
// flow is open under it.
func (s *PanelsService) Details(node string) DetailsView {
	d := DetailsView{Rows: []DetailRow{}}
	s.app.actor.Do(func() {
		if o, ok := s.app.requests.reqs[node]; ok {
			d = requestDetails(s.app.requests.view(node, o), s.app.workspace.SelectedEnv())
		} else if o, ok := s.app.flows.docs[node]; ok {
			d = flowDetails(s.app.flows.view(node, o))
		}
	})
	return d
}

func requestDetails(v RequestView, env string) DetailsView {
	rows := []DetailRow{
		{"Method", v.Verb},
		{"URL", v.FullURL},
		{"Version", v.Selected},
		{"Overlay", orWord(v.FlowiOverlay, "none")},
		{"Auth", orWord(v.Auth.Kind, "none")},
		{"Environment", orWord(env, "none selected")},
		{"Hooks", hooksText(v.HookChain)},
		{"Last result", lastResultText(v.Response)},
	}
	return DetailsView{Kind: "request", Title: v.Name, Rows: rows}
}

func flowDetails(v FlowView) DetailsView {
	names := make([]string, 0, len(v.Inputs))
	for _, in := range v.Inputs {
		names = append(names, in.Name)
	}
	steps := 0
	for _, u := range v.Diagram.Units {
		steps += len(u.Nodes)
	}
	rows := []DetailRow{
		{"Inputs", orWord(strings.Join(names, ", "), "none")},
		{"Steps", fmt.Sprint(steps)},
		{"Last run", orWord(v.Run.Summary, "not run yet")},
	}
	return DetailsView{Kind: "flow", Title: v.Name, Rows: rows}
}

func orWord(s, word string) string {
	if s == "" {
		return word
	}
	return s
}

func hooksText(chain []string) string {
	if len(chain) == 0 {
		return "none"
	}
	return fmt.Sprintf("%d: %s", len(chain), strings.Join(chain, ", "))
}

func lastResultText(r *ResponseView) string {
	if r == nil {
		return "not sent yet"
	}
	return strings.TrimSpace(r.Code + " · " + r.Took)
}
