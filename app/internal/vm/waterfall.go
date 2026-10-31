// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"time"

	"github.com/githugt/flowi-lib/engine"
)

// Phases of a request, in the order they happen.
const (
	PhaseDNS      = "DNS"
	PhaseConnect  = "Connect"
	PhaseTLS      = "TLS"
	PhaseWait     = "Waiting"
	PhaseDownload = "Download"
)

// Phase is one bar of the waterfall: when it began, counted from the start
// of the request, and how long it lasted.
type Phase struct {
	Name            string
	Start, Duration time.Duration
}

// Waterfall lays the timings out as the phases of one request, end to end.
// DNS, Connect and TLS are durations; TTFB and Total are counted from the
// start, so waiting is what is left of TTFB after the set-up and the
// download what is left of Total after TTFB. A phase that did not happen (a
// reused connection has no DNS) is left out.
func Waterfall(t engine.Timings) []Phase {
	var out []Phase
	at := time.Duration(0)
	add := func(name string, d time.Duration) {
		if d > 0 {
			out = append(out, Phase{Name: name, Start: at, Duration: d})
			at += d
		}
	}
	add(PhaseDNS, t.DNS)
	add(PhaseConnect, t.Connect)
	add(PhaseTLS, t.TLS)
	if t.TTFB == 0 { // no response came: there was nothing to wait for or download
		return out
	}
	add(PhaseWait, t.TTFB-at)
	at = max(at, t.TTFB)
	add(PhaseDownload, t.Total-at)
	return out
}
