// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/engine"
)

// A debug session runs a flow in parts. It stops before each step that has a breakpoint, and the person may read
// what the earlier steps left, change a variable or a step's output, then continue (to the next breakpoint), step
// (one step) or stop. Each part is an ordinary partial run that stands on the results of the parts before it, so
// nothing new is needed in the engine; breakpoints belong to the open flow and are not saved in the file, like
// skips. Only top-level steps can have one.

// debugSession is what a running session remembers between its parts.
type debugSession struct {
	env     string
	inputs  map[string]string
	pauseAt string // the step to pause before when the part that is running passes; "" when it is the last
	seed    uint64 // the seed the first part picked; the later parts repeat it, so generated values agree
}

// ToggleBreak sets or clears a breakpoint before step. A step that is not a top-level step of the flow is ignored.
func (r *FlowRun) ToggleBreak(step string) {
	if !slices.Contains(r.stepIDs(), step) {
		return
	}
	if i := slices.Index(r.breaks, step); i >= 0 {
		r.breaks = slices.Delete(r.breaks, i, i+1)
	} else {
		r.breaks = append(r.breaks, step)
	}
	r.changed()
}

// Breaks lists the steps with a breakpoint, in flow order. A step deleted since is not listed.
func (r *FlowRun) Breaks() []string {
	out := []string{}
	for _, id := range r.stepIDs() {
		if slices.Contains(r.breaks, id) {
			out = append(out, id)
		}
	}
	return out
}

// Paused is the step the session is waiting before, or "".
func (r *FlowRun) Paused() string { return r.paused }

// stepIDs are the ids of the flow's top-level steps, in order.
func (r *FlowRun) stepIDs() []string {
	nodes := r.doc.Draft().Nodes
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}

// Debug runs the saved flow and stops before the first step with a breakpoint. With none it is an ordinary run.
func (r *FlowRun) Debug(env string, inputs map[string]string) {
	if r.running {
		return
	}
	if len(r.Breaks()) == 0 {
		r.dbg, r.paused = nil, ""
		r.Run(env, inputs)
		return
	}
	r.ClearResults()
	r.dbg, r.paused = &debugSession{env: env, inputs: inputs}, ""
	r.runPart(0, false, true)
}

// Continue runs from the paused step to the next breakpoint, or to the end.
func (r *FlowRun) Continue() {
	if from, ok := r.pausedIndex(); ok {
		r.paused = ""
		r.runPart(from, false, false)
	}
}

// Step runs the paused step alone and pauses before the one after it.
func (r *FlowRun) Step() {
	if from, ok := r.pausedIndex(); ok {
		r.paused = ""
		r.runPart(from, true, false)
	}
}

// StopDebugging ends the session; what has run stays on the canvas.
func (r *FlowRun) StopDebugging() {
	r.dbg, r.paused = nil, ""
	r.changed()
}

func (r *FlowRun) pausedIndex() (int, bool) {
	if r.dbg == nil || r.paused == "" || r.running {
		return 0, false
	}
	i := slices.Index(r.stepIDs(), r.paused)
	return i, i >= 0
}

// runPart runs the steps from index from up to the next breakpoint (or just the one, when single). A session's
// first part looks for a breakpoint at from itself (withFirst); a part that continues from a pause does not, since
// that is the step being run.
func (r *FlowRun) runPart(from int, single, withFirst bool) {
	ids := r.stepIDs()
	end := len(ids)
	if single {
		end = min(from+1, len(ids))
	} else {
		start := from + 1
		if withFirst {
			start = from
		}
		for i := start; i < len(ids); i++ {
			if slices.Contains(r.breaks, ids[i]) {
				end = i
				break
			}
		}
	}
	r.dbg.pauseAt = ""
	if end < len(ids) {
		r.dbg.pauseAt = ids[end]
	}
	if end == from { // a breakpoint on the very first step: pause before anything runs
		r.paused = r.dbg.pauseAt
		r.changed()
		return
	}
	r.start(r.dbg.env, r.dbg.inputs, slices.Clone(ids[from:end]))
}

// debugAfter is called when a part ends: it pauses before the next breakpoint when the part passed, and ends the
// session when it failed or was the last.
func (r *FlowRun) debugAfter(o runOutcome) {
	if r.dbg == nil {
		return
	}
	if o.err == nil && o.result.Status == engine.StatusPassed && r.dbg.pauseAt != "" {
		r.paused = r.dbg.pauseAt
		if r.dbg.seed == 0 {
			r.dbg.seed = o.result.Seed
		}
		return
	}
	r.dbg, r.paused = nil, ""
}

// SetVar changes a variable the next part will start with. text is read as JSON (7, true, {"a":1}) and stays
// text when it is not. It works only while paused.
func (r *FlowRun) SetVar(name, text string) {
	if r.paused == "" || name == "" {
		return
	}
	vars := maps.Clone(r.vars)
	if vars == nil {
		vars = map[string]any{}
	}
	vars[name] = readValue(text)
	r.vars = vars
	r.changed()
}

// SetOutput changes one value of a finished step's outputs, which the steps after it will read. Like SetVar it
// reads JSON and works only while paused.
func (r *FlowRun) SetOutput(step, key, text string) {
	d, ran := r.details[step]
	if r.paused == "" || !ran || key == "" {
		return
	}
	out := maps.Clone(d.Outputs)
	if out == nil {
		out = map[string]any{}
	}
	out[key] = readValue(text)
	d.Outputs = out
	r.changed()
}

// readValue reads text as JSON, or keeps it as text.
func readValue(text string) any {
	var v any
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &v); err != nil {
		return text
	}
	return v
}
