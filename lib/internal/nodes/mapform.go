// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// mapForm renders every parameter not in reserved as "<var>: template".
// It returns the rendered values in file order of evaluation; a failed
// key is reported and left out.
func mapForm(n model.Node, s *State, reserved ...string) (map[string]any, diag.List) {
	out := map[string]any{}
	var diags diag.List
	for _, f := range n.Params {
		if slices.Contains(reserved, f.Key) {
			continue
		}
		v, err := eval(f.Value, s)
		if err != nil {
			diags = append(diags, templateDiags(n, f.Key, err)...)
			continue
		}
		out[f.Key] = v
	}
	return out, diags
}

// writeVars stores values as variables and returns them as outputs.
func writeVars(s *State, values map[string]any, diags diag.List) Outcome {
	for k, v := range values {
		s.Vars.Set(k, v)
	}
	return Outcome{Outputs: values, Diags: diags}
}
