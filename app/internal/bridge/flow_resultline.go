// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strings"
	"time"

	"github.com/githugt/flowi/app/internal/vm"
)

// resultLine is the short line on a step's card after a run. It says more than a status word and a duration:
// the HTTP status of a request, the size of what came back, how many times a step ran, and how long it took.
// A passed step with a status code drops the word "passed" (the mark and the colour already say it).
// Before: every card said only "passed · 120ms", whatever the step was.
func resultLine(d vm.NodeDetail) string {
	var parts []string
	code, size := 0, int64(0)
	if d.Record != nil {
		code, size = d.Record.Status, d.Record.Body.Size
	}
	if d.Status != "passed" || code == 0 {
		parts = append(parts, d.Status)
	}
	if code > 0 {
		parts = append(parts, fmt.Sprint(code))
	}
	if size > 0 {
		parts = append(parts, vm.SizeText(size))
	}
	if d.Instances > 1 {
		parts = append(parts, fmt.Sprintf("%d runs", d.Instances))
	}
	if d.Duration > 0 {
		parts = append(parts, d.Duration.Round(time.Millisecond).String())
	}
	return strings.Join(parts, " · ")
}
