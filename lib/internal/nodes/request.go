// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

func requestSpec() Spec {
	return Spec{
		Type: "request", Group: "Call", Summary: "Run a saved request, with its version resolution and API hooks.",
		Typed: []string{"request", "version", "hooks", "with", "headers"},
		Run:   runRequest,
	}
}

// runRequest renders with (values keep their types) and headers in the
// node's scope; the runtime does the rest.
func runRequest(ctx context.Context, n model.Node, s *State) Outcome {
	call := n.Request
	if call == nil {
		return failed(n, diag.CodeNodeMissingField, map[string]string{"type": "request", "field": "request"})
	}
	with, diags := renderValues(n, "with ", call.With, s)
	headers, headerDiags := renderTexts(n, "header ", call.Headers, s)
	if diags = append(diags, headerDiags...); diags.HasErrors() {
		return Outcome{Diags: diags}
	}
	return s.Runtime.Request(ctx, RequestCall{Ref: call.Request, Version: call.Version, Hooks: call.Hooks, With: with, Headers: headers})
}

// renderValues renders templates keeping the type of single expressions.
func renderValues(n model.Node, field string, kvs []model.KeyValue, s *State) (map[string]any, diag.List) {
	out := make(map[string]any, len(kvs))
	var diags diag.List
	for _, kv := range kvs {
		v, err := eval(kv.Value, s)
		if err != nil {
			diags = append(diags, templateDiags(n, field+kv.Key, err)...)
			continue
		}
		out[kv.Key] = v
	}
	return out, diags
}

// renderTexts renders templates to strings.
func renderTexts(n model.Node, field string, kvs []model.KeyValue, s *State) (map[string]string, diag.List) {
	values, diags := renderValues(n, field, kvs, s)
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = template.String(v)
	}
	return out, diags
}
