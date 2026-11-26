// SPDX-License-Identifier: AGPL-3.0-or-later

package uidev

import (
	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
)

// Callers finds the flow steps of the made-up collections that run each request, as the real workspace does, so
// deleting a request the fixture's flows use shows the warning in the browser.
func (b *Backend) Callers(ts []flowi.RequestTarget) map[flowi.RequestTarget][]analysis.Caller {
	targets := make([]analysis.Target, len(ts))
	for i, t := range ts {
		targets[i] = analysis.Target{Collection: t.Collection, Request: t.Request}
	}
	found := analysis.Callers(b.Cols, targets)
	out := map[flowi.RequestTarget][]analysis.Caller{}
	for _, t := range ts {
		if c := found[analysis.Target{Collection: t.Collection, Request: t.Request}]; len(c) > 0 {
			out[t] = c
		}
	}
	return out
}
