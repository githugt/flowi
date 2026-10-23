// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// intParam reads an integer parameter (YAML ints are int64), or def.
func intParam(n model.Node, key string, def int) int {
	v, ok := param(n, key)
	if !ok {
		return def
	}
	i, isInt := v.(int64)
	if !isInt {
		return def // validate reports the wrong kind
	}
	return int(i)
}

// durationParam reads a duration literal ("5s"); 0 when absent or invalid
// (validate reports invalid values).
func durationParam(n model.Node, key string) time.Duration {
	d, _ := time.ParseDuration(stringParam(n, key))
	return d
}

// combine is a container's status from its lanes: a failure or error
// decides (an error outranks a failure); otherwise cancelled lanes make it
// cancelled. Lanes cancelled because another failed (fail_fast) never
// hide that failure.
func combine(results []LaneResult) Status {
	worst := StatusPassed
	cancelled := false
	for _, r := range results {
		switch r.Status {
		case StatusError:
			worst = StatusError
		case StatusFailed:
			if worst != StatusError {
				worst = StatusFailed
			}
		case StatusCancelled:
			cancelled = true
		}
	}
	if worst == StatusPassed && cancelled {
		return StatusCancelled
	}
	return worst
}

// diagsOf collects every lane's diagnostics, in lane order.
func diagsOf(results []LaneResult) diag.List {
	var out diag.List
	for _, r := range results {
		out = append(out, r.Diags...)
	}
	return out
}
