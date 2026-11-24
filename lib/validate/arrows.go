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

// checkArrows reports a step whose arrows are asked to be drawn in a way the canvas does not know.
func checkArrows(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, n := range sites.AllNodes(c) {
			if n.Value.ArrowStyle == "" || slices.Contains(model.NodeArrowStyles(), n.Value.ArrowStyle) {
				continue
			}
			loc := diag.Location{File: n.File, Path: yamlpath.Key(n.Path, "arrow_style")}
			out = append(out, diag.New(diag.CodeNodeParam, loc, map[string]string{
				"field": "arrow_style", "node": n.Value.ID,
				"detail": n.Value.ArrowStyle + " is not a way to draw arrows (use " + strings.Join(model.NodeArrowStyles(), " or ") + ")",
			}))
		}
	}
	return out
}
