// SPDX-License-Identifier: Apache-2.0

package engine

import "github.com/githugt/flowi/lib/internal/nodes"

// keepVars remembers the flow's top-level variables for the report and gives
// them the values an earlier run ended with (see RunInput.PriorVars).
func (r *runner) keepVars(vars *nodes.Vars, prior map[string]any) {
	for name, value := range prior {
		vars.Set(name, value)
	}
	r.mu.Lock()
	r.rootVars = vars
	r.mu.Unlock()
}

// finalVars is the flow's variables now, with every secret the run read masked
// (a variable may hold a token a request returned).
func (r *runner) finalVars() map[string]any {
	r.mu.Lock()
	vars := r.rootVars
	r.mu.Unlock()
	if vars == nil {
		return nil
	}
	m := r.secrets.current()
	out := vars.Snapshot()
	for name, value := range out {
		out[name] = maskValue(value, m)
	}
	return out
}
