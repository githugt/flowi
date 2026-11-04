// SPDX-License-Identifier: Apache-2.0

package flowi

import "github.com/githugt/flowi/lib/analysis"

// ExplainFlow tells a saved flow in words and lists what looks wrong with it. It works from the flow as written,
// so it needs no run.
func (w *Workspace) ExplainFlow(t FlowTarget) (analysis.Explanation, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return analysis.Explanation{}, err
	}
	return analysis.Explain(f), nil
}
