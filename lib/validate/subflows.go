// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkSubflowInputs: a subflow node passes only declared inputs, and
// every input without a default (FLW-NODE-008).
func checkSubflowInputs(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, n := range sites.AllNodes(c) {
			if n.Value.Subflow == nil {
				continue
			}
			target, code, _ := ix.targetCollection(c, n.Value.Subflow.Flow.Collection)
			fl, ok := ix.flows[entityKey{target, n.Value.Subflow.Flow.ID}]
			if code != "" || !ok {
				continue // reported by checkRefs
			}
			out = append(out, subflowInputProblems(n, fl)...)
		}
	}
	return out
}

func subflowInputProblems(n sites.Located[model.Node], fl model.Flow) diag.List {
	var out diag.List
	given := map[string]bool{}
	for _, kv := range n.Value.Subflow.With {
		given[kv.Key] = true
		if !slices.ContainsFunc(fl.Inputs, func(in model.Input) bool { return in.Name == kv.Key }) {
			at := diag.Location{File: n.File, Path: yamlpath.Key(yamlpath.Key(n.Path, "with"), kv.Key)}
			out = append(out, diag.New(diag.CodeNodeSubflowInput, at, map[string]string{"node": n.Value.ID, "detail": "no input", "name": kv.Key, "flow": fl.Path}))
		}
	}
	for _, in := range fl.Inputs {
		if in.Default == "" && !given[in.Name] {
			at := diag.Location{File: n.File, Path: n.Path}
			out = append(out, diag.New(diag.CodeNodeSubflowInput, at, map[string]string{"node": n.Value.ID, "detail": "missing input", "name": in.Name, "flow": fl.Path}))
		}
	}
	return out
}
