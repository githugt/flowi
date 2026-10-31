// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/diag"
)

// SkipProblem is a step that reads something a skipped step would have made.
// Required is false when the read has a default, so a missing value is nil
// rather than an error.
type SkipProblem struct {
	Reader   string
	Skipped  string
	Reads    string
	Required bool
	Text     string // the library's sentence for it, with a hint
}

// The skip list belongs to the open flow, not to the file: it is how a person
// runs "everything but the notification" without editing the flow. A run takes
// it as RunOptions.Skip.

// ToggleSkip skips step in the next runs, or stops skipping it.
func (r *FlowRun) ToggleSkip(step string) {
	if i := slices.Index(r.skip, step); i >= 0 {
		r.skip = slices.Delete(r.skip, i, i+1)
	} else {
		r.skip = append(r.skip, step)
	}
	r.changed()
}

// Skipped lists the steps skipped, in flow order. A step deleted since is not
// listed.
func (r *FlowRun) Skipped() []string {
	out := []string{}
	for _, id := range r.doc.NodeIDs() {
		if slices.Contains(r.skip, id) {
			out = append(out, id)
		}
	}
	return out
}

// SetHold says whether skipped steps keep what the last run gave them, so the
// steps that read them still have a value.
func (r *FlowRun) SetHold(on bool) {
	r.hold = on
	r.changed()
}

// Hold reports whether skipped steps keep their last values.
func (r *FlowRun) Hold() bool { return r.hold }

// SkipProblems lists what skipping would break in the flow as drafted. With
// hold on, a read the last run can still answer is not a problem.
func (r *FlowRun) SkipProblems() []SkipProblem {
	skipped := r.Skipped()
	out := []SkipProblem{}
	if len(skipped) == 0 {
		return out
	}
	for _, s := range analysis.SkipReaders(r.doc.Draft(), skipped) {
		if r.hold && r.holds(s) {
			continue
		}
		out = append(out, SkipProblem{Reader: s.Reader, Skipped: s.Skipped, Reads: s.Reads, Required: s.Required, Text: skipText(s)})
	}
	return out
}

func skipText(s analysis.SkipReader) string {
	d := diag.New(diag.CodeRunSkipRead, diag.Location{}, map[string]string{"reader": s.Reader, "reads": s.Reads, "skipped": s.Skipped})
	msg, hint := diag.English().Message(d)
	if hint == "" {
		return msg
	}
	return msg + ". " + strings.ToUpper(hint[:1]) + hint[1:]
}

// holds says whether the last run can answer what s reads: a step's outputs
// for nodes.<id>…, the variable as the run ended for vars.<name>.
func (r *FlowRun) holds(s analysis.SkipReader) bool {
	if name, ok := strings.CutPrefix(s.Reads, "vars."); ok {
		_, has := r.vars[strings.SplitN(name, ".", 2)[0]]
		return has
	}
	d, ran := r.details[s.Skipped]
	return ran && d.Outputs != nil
}

// applySkips adds the skip list to opts, and with hold on the last values of
// the skipped steps, which the engine reads in their place.
func (r *FlowRun) applySkips(opts *flowi.RunOptions) {
	opts.Skip = r.Skipped()
	if !r.hold || len(opts.Skip) == 0 {
		return
	}
	if opts.Prior == nil {
		opts.Prior = map[string]map[string]any{}
	}
	for _, id := range opts.Skip {
		if d, ran := r.details[id]; ran && d.Outputs != nil {
			opts.Prior[id] = d.Outputs
		}
	}
	opts.PriorVars = r.vars
}
