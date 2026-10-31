// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// checkEnvironmentExtends reports an environment whose `extends:` names a missing base or leads back to
// itself. Global environments appear in every collection, so each file is checked once.
func checkEnvironmentExtends(ix *index) diag.List {
	var out diag.List
	checked := map[string]bool{}
	for _, c := range ix.collections {
		for _, e := range c.Environments {
			if e.Extends == "" || checked[e.File] {
				continue
			}
			checked[e.File] = true
			if _, err := model.EnvironmentChain(c.Environments, e.Name); err != nil {
				loc := diag.Location{File: e.File, Path: "$.extends"}
				out = append(out, diag.New(diag.CodeRequestEnvExtends, loc, map[string]string{"env": e.Name, "detail": err.Error()}))
			}
		}
	}
	return out
}
