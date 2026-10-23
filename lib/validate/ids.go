// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// checkDuplicateIDs reports ids used by more than one entity anywhere in
// the target. Each extra occurrence is reported once, pointing at the
// others through Related.
func checkDuplicateIDs(ix *index) diag.List {
	seen := map[model.ID][]diag.Location{}
	add := func(id model.ID, file string) {
		if id != "" {
			seen[id] = append(seen[id], diag.Location{File: file, Path: "$.id"})
		}
	}
	sharedEnvs := map[string]bool{} // global environments appear in every collection: count each file once
	for _, c := range ix.collections {
		add(c.Manifest.ID, c.Manifest.File)
		for _, e := range c.Environments {
			if e.Scope != "" && sharedEnvs[e.File] {
				continue
			}
			sharedEnvs[e.File] = true
			add(e.ID, e.File)
		}
		for _, r := range c.Requests {
			add(r.ID, r.File)
		}
		for _, f := range c.Flows {
			add(f.ID, f.File)
		}
		for _, b := range c.Blueprints {
			add(b.ID, b.File)
		}
	}
	var out diag.List
	for id, locs := range seen {
		if len(locs) < 2 {
			continue
		}
		slices.SortFunc(locs, func(a, b diag.Location) int { return strings.Compare(a.File, b.File) })
		for i, loc := range locs[1:] {
			others := slices.Delete(slices.Clone(locs), i+1, i+2)
			d := diag.New(diag.CodeIDDuplicate, loc, map[string]string{"id": string(id)})
			d.Related = others
			out = append(out, d)
		}
	}
	return out
}
