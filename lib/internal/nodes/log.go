// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
	"github.com/githugt/flowi-lib/vault"
)

func logSpec() Spec {
	return Spec{
		Type: "log", Group: "Output", Summary: "Write a message or value to the run log.",
		Params: []Param{{Name: "value", Kind: ParamText, Required: true}},
		Run:    runLog,
	}
}

// runLog writes value to the run log, with secrets masked.
func runLog(_ context.Context, n model.Node, s *State) Outcome {
	tpl, err := template.Parse(stringParam(n, "value"))
	if err != nil {
		return Outcome{Diags: templateDiags(n, "value", err)}
	}
	r, err := tpl.Render(s.Scope())
	if err != nil {
		return Outcome{Diags: templateDiags(n, "value", err)}
	}
	s.derived(r)
	message := r.Value.(string)
	if r.Tainted {
		message = vault.Mask // built from a secret: nothing of it is shown
	}
	s.Emit.Log(n.ID, s.Masker.String(message))
	return Outcome{}
}
