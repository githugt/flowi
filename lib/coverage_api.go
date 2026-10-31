// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
)

// maxRunsPerFlow bounds how much history Coverage reads for one flow.
const maxRunsPerFlow = 200

// Coverage measures how much of the API descriptions kept in collection the flows use: which operations a flow
// calls, which of their documented statuses a saved run ever got, and which response fields a flow reads. An
// operation is a request that came from an imported description and has response schemas. Statuses come from
// the run history, so a flow that never ran leaves its statuses unseen. An empty collection measures all of them.
func (w *Workspace) Coverage(collection model.ID) (analysis.Coverage, diag.List, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cs := w.collections
	if collection != "" {
		c, ok := w.collection(collection)
		if !ok {
			return analysis.Coverage{}, nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
		}
		cs = []model.Collection{c}
	}
	var runs []report.Run
	for _, c := range cs {
		runs = append(runs, w.runsOf(c)...)
	}
	return analysis.Cover(cs, w.contracts(), runs), nil, nil
}

// savedRuns is the run history of a collection, read from the reports the engine saved.
func (w *Workspace) savedRuns(collection model.ID) []report.Run {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return nil
	}
	return w.runsOf(c)
}

// runsOf reads the newest saved reports of each flow of c. A report that cannot be read is left out: reports
// are a cache the engine wrote, and a damaged one is not a reason to refuse a measurement.
func (w *Workspace) runsOf(c model.Collection) []report.Run {
	var out []report.Run
	for _, f := range c.Flows {
		dir := filepath.Join(w.DataDir(c.Manifest.ID), "runs", string(f.ID))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				names = append(names, e.Name())
			}
		}
		slices.Sort(names)
		slices.Reverse(names) // run ids are time-ordered: newest first
		for _, name := range names[:min(len(names), maxRunsPerFlow)] {
			data, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // the engine's own report file
			if err != nil {
				continue
			}
			var r report.Run
			if json.Unmarshal(data, &r) == nil {
				out = append(out, r)
			}
		}
	}
	return out
}
