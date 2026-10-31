// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"

	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
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
