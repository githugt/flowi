// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/model"
)

// SaveImpact is the check before a save: what the edited request would break for the flows, hooks and checks that use
// it, that is fine today. It judges the consumers against the request as saved and as edited, and keeps the ones
// that are only broken by the edit.
type SaveImpact struct {
	b        Backend
	w        *Worker
	target   flowi.RequestTarget
	draft    model.Request
	lines    []string
	err      string
	onChange []func()
}

// NewSaveImpact returns the check for draft, an edited copy of the request at t; call Load.
func NewSaveImpact(b Backend, w *Worker, t flowi.RequestTarget, draft model.Request) *SaveImpact {
	return &SaveImpact{b: b, w: w, target: t, draft: cloneRequest(draft)}
}

// OnChange registers f to run once the check is done.
func (s *SaveImpact) OnChange(f func()) { s.onChange = append(s.onChange, f) }

// Lines say what the edit would break, one consumer each; none when it breaks nothing.
func (s *SaveImpact) Lines() []string { return s.lines }

// Err is why the check could not be made, or "". A save is not held up by it.
func (s *SaveImpact) Err() string { return s.err }

// Load makes the check in the background.
func (s *SaveImpact) Load() {
	t, draft := s.target, s.draft
	type loaded struct {
		saved, edited analysis.Report
		err           error
	}
	Run(s.w, func() loaded {
		saved, _, err := s.b.Impact(t, "")
		if err != nil {
			return loaded{err: err}
		}
		edited, _, err := s.b.ImpactOfDraft(t, draft)
		return loaded{saved, edited, err}
	}, func(l loaded) {
		s.lines, s.err = nil, ""
		if l.err != nil {
			s.err = ErrorText(l.err)
		} else {
			s.lines = newlyBroken(l.saved, l.edited)
		}
		for _, f := range s.onChange {
			f()
		}
	})
}

// newlyBroken lists the consumers that are broken in edited and were not in saved, each with its first failing read.
func newlyBroken(saved, edited analysis.Report) []string {
	type key struct{ kind, file, flow, node, hook, version string }
	keyOf := func(c analysis.Consumer) key { return key{c.Kind, c.File, c.Flow, c.Node, c.Hook, c.Version} }
	was := map[key]bool{}
	for _, v := range saved.Versions {
		for _, c := range v.Consumers {
			if c.Status == analysis.Broken {
				was[keyOf(c)] = true
			}
		}
	}
	var out []string
	for _, v := range edited.Versions {
		for _, c := range v.Consumers {
			if c.Status != analysis.Broken || was[keyOf(c)] {
				continue
			}
			row := consumerRow(c)
			line := consumerText(c) + " (version " + v.Version + ")"
			switch {
			case row.Detail != "":
				line += ": " + row.Detail
			case c.Reason != "":
				line += ": " + c.Reason
			}
			out = append(out, line)
		}
	}
	return out
}
