// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"
)

// FailMode is what a step does when it fails: its on_error.
type FailMode string

const (
	// FailStop fails the run (the default; on_error empty or "fail").
	FailStop FailMode = "fail"
	// FailContinue goes on with the next step.
	FailContinue FailMode = "continue"
	// FailGoto jumps to another step.
	FailGoto FailMode = "goto"
)

const gotoPrefix = "goto:"

// ParseFailRoute reads an on_error value. Anything it does not know reads as
// FailStop, which is what the engine does with it too.
func ParseFailRoute(onError string) (mode FailMode, target string) {
	switch {
	case onError == string(FailContinue):
		return FailContinue, ""
	case strings.HasPrefix(onError, gotoPrefix):
		return FailGoto, strings.TrimSpace(strings.TrimPrefix(onError, gotoPrefix))
	}
	return FailStop, ""
}

// FailRoute is the on_error value for mode and target; the default (stop) is
// the empty value, so a flow that never sets it stays clean.
func FailRoute(mode FailMode, target string) string {
	switch mode {
	case FailContinue:
		return string(FailContinue)
	case FailGoto:
		return gotoPrefix + target
	case FailStop:
	}
	return ""
}

// NodeIDs are the ids of every step in the flow, sorted.
func (d *FlowDoc) NodeIDs() []string {
	ids := map[string]bool{}
	allIDs(d.draft.Nodes, ids)
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}
