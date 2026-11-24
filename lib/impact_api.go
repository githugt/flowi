// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"

	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Impact reports who uses request t across every mounted collection (flow
// request nodes, post-hooks, the request's own extract and assert) and
// whether each still fits the request's contract: its pinned examples, or
// else its last saved response (design §9). version limits the report to
// one version; "" reports all of them. An unknown version is a
// *diag.ListError (FLW-VER-004); diags are unreadable examples.
func (w *Workspace) Impact(t RequestTarget, version string) (analysis.Report, diag.List, error) {
	_, r, err := w.requestFor(t)
	if err != nil {
		return analysis.Report{}, nil, err
	}
	if version != "" && !slices.ContainsFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == version }) {
		return analysis.Report{}, nil, diag.List{diag.New(diag.CodeVersionPinnedAbsent, diag.Location{File: r.File},
			map[string]string{"label": version, "request": r.Path})}.Err()
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	rep, diags, err := analysis.Impact(w.collections, analysis.Target{Collection: t.Collection, Request: t.Request}, version, w.contracts())
	return rep, w.positioned(diags), err
}

// ImpactOfDraft is Impact for request t as it would be once draft, an edited copy of it, is saved: nothing is
// written. Compare it with Impact to see what the edit would break for the request's consumers (a removed extraction
// a flow reads, say).
func (w *Workspace) ImpactOfDraft(t RequestTarget, draft model.Request) (analysis.Report, diag.List, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cs := slices.Clone(w.collections)
	for i, c := range cs {
		if c.Manifest.ID != t.Collection {
			continue
		}
		j := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.ID == t.Request })
		if j < 0 {
			return analysis.Report{}, nil, diagErr(diag.CodeRequestNotFound, map[string]string{"ref": string(t.Request)})
		}
		draft.ID, draft.File, draft.Path = c.Requests[j].ID, c.Requests[j].File, c.Requests[j].Path
		c.Requests = slices.Clone(c.Requests)
		c.Requests[j] = draft
		cs[i] = c
	}
	rep, diags, err := analysis.Impact(cs, analysis.Target{Collection: t.Collection, Request: t.Request}, "", w.contracts())
	return rep, w.positioned(diags), err
}

func (w *Workspace) contracts() analysis.FileContracts {
	return analysis.FileContracts{DataDir: w.DataDir}
}

// contractWarnings are validate's contract phase (design §7.1): reads of
// response data that the request's contract does not contain, unfilled
// inputs, and unreadable examples. Callers hold mu.
func (w *Workspace) contractWarnings(cs []model.Collection) diag.List {
	reports, diags := analysis.Analyze(cs, w.contracts())
	return append(diags, analysis.Warnings(reports)...)
}

// CollectionImpact is the impact report of every request of the collection that something uses (a flow, a hook, a
// check), against the contract each has: what a page of the collection needs to say, per operation, whether what
// reads its responses still fits. A request nothing uses has no report.
func (w *Workspace) CollectionImpact(collection model.ID) ([]analysis.Report, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if _, ok := w.collection(collection); !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	reports, _ := analysis.Analyze(w.collections, w.contracts())
	return slices.DeleteFunc(reports, func(r analysis.Report) bool { return r.CollectionID != collection }), nil
}
