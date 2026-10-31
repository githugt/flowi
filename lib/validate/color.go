// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkColors reports a step colour that is not one the canvas draws.
func checkColors(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, n := range sites.AllNodes(c) {
			if n.Value.Color == "" || slices.Contains(model.NodeColors(), n.Value.Color) {
				continue
			}
			loc := diag.Location{File: n.File, Path: yamlpath.Key(n.Path, "color")}
			out = append(out, diag.New(diag.CodeNodeParam, loc, map[string]string{
				"field": "color", "node": n.Value.ID,
				"detail": n.Value.Color + " is not a colour (use " + strings.Join(model.NodeColors(), ", ") + ")",
			}))
		}
	}
	return out
}
