// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
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
			if a, err := s.app.backend.RequestActivity(o.req.Target()); err == nil {
				d.Rows = append(d.Rows, activityRows(a, "Last sent")...)
			}
		} else if o, ok := s.app.flows.docs[node]; ok {
			d = flowDetails(s.app.flows.view(node, o))
			if a, err := s.app.backend.FlowActivity(o.doc.Target()); err == nil {
				d.Rows = append(d.Rows, activityRows(a, "Last run")...)
			}
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

// activityRows turn what is known of an entity's life into rows. An entity that arrived with the files (cloned,
// pulled) says so, and dates itself from when it was first seen here.
func activityRows(a flowi.Activity, ran string) []DetailRow {
	when := func(t time.Time) string {
		if t.IsZero() {
			return "never"
		}
		return t.Local().Format("2006-01-02 15:04")
	}
	origin := map[string]string{
		flowi.OriginCreated: "made here", flowi.OriginDuplicated: "duplicated", flowi.OriginImported: "imported",
		flowi.OriginRecorded: "recorded by the replayer", flowi.OriginFound: "came with the files (cloned or copied in)",
	}[a.Origin]
	if origin == "" {
		origin = a.Origin
	}
	if a.OriginDetail != "" {
		origin += " – " + a.OriginDetail
	}
	created := "Created"
	if a.Origin == flowi.OriginFound {
		created = "First seen here"
	}
	rows := []DetailRow{{"Origin", origin}, {created, when(a.Created)}}
	if a.Runs == 0 {
		return append(rows, DetailRow{ran + " (this client)", "never"})
	}
	return append(rows,
		DetailRow{ran + " (this client)", when(a.LastRun) + " · " + a.LastStatus},
		DetailRow{"Last success", when(a.LastSuccess)},
		DetailRow{"Last failure", when(a.LastFailure)},
		DetailRow{"Runs (this client)", fmt.Sprint(a.Runs)},
	)
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
