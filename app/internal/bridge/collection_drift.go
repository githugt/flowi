// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"
)

// maxImpactNotes and maxDriftNotes bound the lines an operation lists about what reads it or what moved; the counts
// say the rest.
const (
	maxImpactNotes = 5
	maxDriftNotes  = 6
)

// PageImpact says, for one operation, who reads its responses and whether they still fit its contract. It comes from
// the same analysis as the request's Impact view and costs no request.
type PageImpact struct {
	Consumers int      `json:"consumers"` // flows, hooks and checks that read it
	Broken    int      `json:"broken"`    // the ones that read something the contract no longer has
	Unknown   int      `json:"unknown"`   // the ones the contract cannot judge (no example or saved response)
	Contract  string   `json:"contract"`  // what they were judged against: example, response, or "" when nothing is known
	Notes     []string `json:"notes"`     // the broken ones, by where they are
}

// PageDrift says what the last drift check found for one operation. A check runs a flow, so the page keeps what the
// last one saw until the app closes, and says when it was and through which flows; nothing is stored on disk.
type PageDrift struct {
	Checked  bool     `json:"checked"`  // some flow that calls it was checked
	At       string   `json:"at"`       // the newest check, in local time as "2006-01-02 15:04"
	Via      []string `json:"via"`      // the flows checked that call it
	New      int      `json:"new"`      // fields the stored schema does not know
	Missing  int      `json:"missing"`  // required fields that are gone
	Changed  int      `json:"changed"`  // fields of another type
	NoSchema bool     `json:"noSchema"` // a response had no stored schema to compare with
	Notes    []string `json:"notes"`
}

// Moved is the number of differences found.
func (d PageDrift) Moved() int { return d.New + d.Missing + d.Changed }

// flowDrift is what one flow's drift check saw.
type flowDrift struct {
	at     time.Time
	flow   model.ID
	label  string
	env    string
	status string
	seen   []flowi.RequestDrift
}

// driftMemory holds the newest drift check of each flow, by collection. It lives as long as the app.
type driftMemory struct {
	mu   sync.Mutex
	runs map[string]map[model.ID]flowDrift
}

func (m *driftMemory) put(collection string, d flowDrift) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runs == nil {
		m.runs = map[string]map[model.ID]flowDrift{}
	}
	if m.runs[collection] == nil {
		m.runs[collection] = map[model.ID]flowDrift{}
	}
	m.runs[collection][d.flow] = d
}

func (m *driftMemory) of(collection string) []flowDrift {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []flowDrift
	for _, d := range m.runs[collection] {
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b flowDrift) int { return strings.Compare(a.label, b.label) })
	return out
}

// DriftRun is what a drift check from the page did.
type DriftRun struct {
	Checked  int      `json:"checked"`  // flows that ran
	Problems []string `json:"problems"` // a flow that could not be checked, with why
}

// CheckDrift runs a flow of the collection (flow is its tree row; "" runs every flow) without learning and keeps what
// moved against the stored response schemas, so the page can show it per operation. Each flow really runs, with the
// selected environment, as the flow's own drift check does; the stored schemas are not touched.
func (s *CollectionService) CheckDrift(collection, flow string) (DriftRun, error) {
	id := model.ID(collection)
	c, ok := s.collectionOf(id)
	if !ok {
		return DriftRun{}, fmt.Errorf("collection %q is not in the workspace", collection)
	}
	var flows []model.Flow
	for _, f := range c.Flows {
		if flow == "" || flow == "f:"+collection+":"+string(f.ID) {
			flows = append(flows, f)
		}
	}
	if len(flows) == 0 {
		return DriftRun{}, fmt.Errorf("no flow %q in %s", flow, c.Manifest.Name)
	}
	env := s.app.workspace.SelectedEnv()
	var run DriftRun
	for _, f := range flows {
		rep, err := s.app.backend.CheckDrift(context.Background(), flowi.FlowTarget{Collection: id, Flow: f.ID}, flowi.DriftOptions{Env: env})
		if err != nil {
			run.Problems = append(run.Problems, pageFlowLabel(f)+": "+err.Error())
			continue
		}
		s.drift.put(collection, flowDrift{at: time.Now(), flow: f.ID, label: pageFlowLabel(f), env: env, status: string(rep.RunStatus), seen: rep.Requests})
		run.Checked++
	}
	return run, nil
}

func (s *CollectionService) collectionOf(id model.ID) (model.Collection, bool) {
	for _, c := range s.app.backend.Collections() {
		if c.Manifest.ID == id {
			return c, true
		}
	}
	return model.Collection{}, false
}

// pageImpactOf sums a request's report: the consumers over all its versions, judged against the contract each has.
func pageImpactOf(rep analysis.Report) PageImpact {
	out := PageImpact{Notes: []string{}}
	for _, v := range rep.Versions {
		if out.Contract == "" {
			out.Contract = v.Contract.Source
		}
		for _, cons := range v.Consumers {
			out.Consumers++
			switch cons.Status {
			case analysis.Broken:
				out.Broken++
				if len(out.Notes) < maxImpactNotes {
					out.Notes = append(out.Notes, consumerNote(cons))
				}
			case analysis.Unknown:
				out.Unknown++
			}
		}
	}
	return out
}

// consumerNote says where a broken consumer is and what it cannot find.
func consumerNote(c analysis.Consumer) string {
	where := firstNonEmpty(c.Hook, joinNonEmpty(" · ", c.Flow, c.Node), c.File)
	if c.Reason != "" {
		return where + ": " + c.Reason
	}
	for _, r := range c.Reads {
		if r.Status == analysis.Broken {
			return where + " reads " + firstNonEmpty(r.Target, r.Expr)
		}
	}
	return where
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func joinNonEmpty(sep string, vals ...string) string {
	var parts []string
	for _, v := range vals {
		if v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, sep)
}

// pageDriftOf gathers, by request path, what the kept checks of the collection's flows saw. A flow that is gone, or
// that no longer calls a request, no longer counts for it.
func pageDriftOf(c model.Collection, runs []flowDrift) map[string]PageDrift {
	live := map[model.ID]bool{}
	for _, f := range c.Flows {
		live[f.ID] = true
	}
	out := map[string]PageDrift{}
	newest := map[string]time.Time{}
	for _, run := range runs {
		if !live[run.flow] {
			continue
		}
		for _, q := range run.seen {
			d := out[q.Request]
			d.Checked = true
			d.NoSchema = d.NoSchema || q.NoSchema
			if !slices.Contains(d.Via, run.label) {
				d.Via = append(d.Via, run.label)
			}
			for _, diff := range q.Differences {
				switch diff.Kind {
				case schemainfer.DriftNew:
					d.New++
				case schemainfer.DriftMissing:
					d.Missing++
				case schemainfer.DriftChanged:
					d.Changed++
				}
				if len(d.Notes) < maxDriftNotes {
					d.Notes = append(d.Notes, fmt.Sprintf("%d %s: %s", q.Status, diff.Path, firstNonEmpty(diff.Detail, string(diff.Kind))))
				}
			}
			if run.at.After(newest[q.Request]) {
				newest[q.Request] = run.at
				d.At = run.at.Local().Format("2006-01-02 15:04")
			}
			out[q.Request] = d
		}
	}
	return out
}
