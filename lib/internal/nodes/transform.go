// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi-lib/model"
)

func transformSpec() Spec {
	return Spec{
		Type: "transform", Group: "Data", Summary: "Reshape data with an expression.",
		Params: []Param{
			{Name: "value", Kind: ParamText, Required: true, Doc: `The result, e.g. "{{map(nodes.list.body.items, .id)}}".`},
			{Name: "into", Kind: ParamString, Doc: "Also store the result in `vars.<into>`."},
		},
		Run: runTransform,
	}
}

// runTransform outputs value, and stores it in vars.<into> when set.
func runTransform(_ context.Context, n model.Node, s *State) Outcome {
	raw, _ := param(n, "value")
	v, err := eval(raw, s)
	if err != nil {
		return Outcome{Diags: templateDiags(n, "value", err)}
	}
	if into := stringParam(n, "into"); into != "" {
		s.Vars.Set(into, v)
	}
	return Outcome{Outputs: map[string]any{"value": v}}
}
