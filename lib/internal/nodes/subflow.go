// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func subflowSpec() Spec {
	return Spec{
		Type: "subflow", Group: "Call", Summary: "Run another flow with its declared inputs; its outputs become this node's.",
		Typed: []string{"flow", "with"},
		Run:   runSubflow,
	}
}

func runSubflow(ctx context.Context, n model.Node, s *State) Outcome {
	if n.Subflow == nil {
		return failed(n, diag.CodeNodeMissingField, map[string]string{"type": "subflow", "field": "flow"})
	}
	inputs, diags := renderValues(n, "with ", n.Subflow.With, s)
	if diags.HasErrors() {
		return Outcome{Diags: diags}
	}
	return s.Runtime.Subflow(ctx, n.Subflow.Flow, inputs)
}
