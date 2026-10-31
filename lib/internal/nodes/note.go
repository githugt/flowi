// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

func noteSpec() Spec {
	return Spec{
		Type: "note", Group: "Output", Summary: "An annotation in the run summary; no side effect.",
		Params: []Param{{Name: "text", Kind: ParamText, Required: true}},
		Run:    runNote,
	}
}

// runNote renders text for the report's notes.
func runNote(_ context.Context, n model.Node, s *State) Outcome {
	tpl, err := template.Parse(stringParam(n, "text"))
	if err == nil {
		var r template.Result
		r, err = tpl.Render(s.Scope())
		if err == nil {
			s.derived(r)
			return Outcome{Outputs: map[string]any{"text": r.Value}}
		}
	}
	return Outcome{Diags: templateDiags(n, "text", err)}
}
