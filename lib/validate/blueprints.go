// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi-lib/blueprint"
	"github.com/githugt/flowi-lib/diag"
)

// checkBlueprints reports problems in each blueprint's parameters and in
// the parameter references of its nodes. The flow body is validated as an
// ordinary flow after the blueprint is used.
func checkBlueprints(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, b := range c.Blueprints {
			out = append(out, blueprint.Check(b)...)
		}
	}
	return out
}
