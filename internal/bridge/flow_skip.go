// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"slices"
)

// SkipView is what the open flow skips this time, and what that would break.
type SkipView struct {
	Steps    []string      `json:"steps"`
	Hold     bool          `json:"hold"` // skipped steps keep what the last run gave them
	Problems []SkipProblem `json:"problems"`
}

// SkipProblem is a step that reads something a skipped step would have made.
type SkipProblem struct {
	Reader   string `json:"reader"`
	Skipped  string `json:"skipped"`
	Text     string `json:"text"`
	Required bool   `json:"required"` // false: the read has a default
}

// skipsOf is the open flow's skip list and its problems.
func skipsOf(o *openFlow) SkipView {
	v := SkipView{Steps: o.run.Skipped(), Hold: o.run.Hold(), Problems: []SkipProblem{}}
	for _, p := range o.run.SkipProblems() {
		v.Problems = append(v.Problems, SkipProblem{Reader: p.Reader, Skipped: p.Skipped, Text: p.Text, Required: p.Required})
	}
	return v
}

// skipMark says on a card whether its step is skipped, and, for a step that
// reads something a skipped step would have made, which one it waits on.
func skipMark(v SkipView, steps []string) (skipped bool, note string) {
	for _, id := range steps {
		skipped = skipped || slices.Contains(v.Steps, id)
		for _, p := range v.Problems {
			if p.Reader == id && note == "" {
				note = fmt.Sprintf("%s is skipped, so this has no value", p.Skipped)
			}
		}
	}
	return skipped, note
}

// ToggleSkip skips step in the next runs, or stops skipping it. The list is
// kept with the open flow, not in the file.
func (s *FlowService) ToggleSkip(node, step string) {
	_ = s.edit(node, func(o *openFlow) error {
		if _, ok := o.doc.Node(step); !ok {
			return fmt.Errorf("no step %s", step)
		}
		o.run.ToggleSkip(step)
		s.app.emit("flow", s.view(node, o))
		return nil
	})
}

// SetHold says whether skipped steps keep what the last run gave them.
func (s *FlowService) SetHold(node string, on bool) {
	_ = s.edit(node, func(o *openFlow) error {
		o.run.SetHold(on)
		s.app.emit("flow", s.view(node, o))
		return nil
	})
}
