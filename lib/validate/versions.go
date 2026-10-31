// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/version"
)

func checkDefaultVersions(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			if r.Default != "" && !hasVersion(r, r.Default) {
				loc := diag.Location{File: r.File, Path: "$.default"}
				out = append(out, diag.New(diag.CodeVersionDefaultAbsent, loc, map[string]string{"label": r.Default}))
			}
		}
	}
	return out
}

// hasVersion compares labels numerically, so "2" matches a "2.0" version.
// Unparseable labels fall back to exact text, and the codec has already
// reported them.
func hasVersion(r model.Request, label string) bool {
	want, err := version.Parse(label)
	return slices.ContainsFunc(r.Versions, func(v model.RequestVersion) bool {
		got, gotErr := version.Parse(v.Label)
		if err != nil || gotErr != nil {
			return v.Label == label
		}
		return got.Compare(want) == 0
	})
}
