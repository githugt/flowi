// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// checkGotos checks on_error: fail, continue, or goto:<a later sibling>
// (plan Decision 5: loops are run_till's job, so gotos only jump forward).
func checkGotos(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, l := range sites.NodeLists(c) {
			for i, n := range l.Value {
				out = append(out, onErrorProblems(l, i, n)...)
			}
		}
	}
	return out
}

func onErrorProblems(l sites.NodeList, i int, n model.Node) diag.List {
	at := diag.Location{File: l.File, Path: yamlpath.Key(yamlpath.Index(l.Path, i), "on_error")}
	target, isGoto := strings.CutPrefix(n.OnError, "goto:")
	switch {
	case n.OnError == "" || n.OnError == "fail" || n.OnError == "continue":
		return nil
	case !isGoto:
		return diag.List{paramDiag(n, "on_error", "must be fail, continue or goto:<node>", at, diag.SeverityError)}
	}
	j := slices.IndexFunc(l.Value, func(s model.Node) bool { return s.ID == target })
	if j <= i {
		return diag.List{diag.New(diag.CodeNodeGoto, at, map[string]string{"node": n.ID, "target": target})}
	}
	return nil
}
