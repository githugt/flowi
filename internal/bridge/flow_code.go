// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

// FlowSourceView is the text of a flow: the flow body (editable) and the
// Arazzo file saving would write (read-only).
type FlowSourceView struct {
	Flow string `json:"flow"`
	File string `json:"file"`
}

// Source is the flow's text as it stands now, unsaved edits included.
func (s *FlowService) Source(node string) (FlowSourceView, error) {
	type result struct {
		v   FlowSourceView
		err error
	}
	ch := make(chan result, 1)
	err := s.edit(node, func(o *openFlow) error {
		o.doc.Source(
			func(flow, file string) { ch <- result{v: FlowSourceView{Flow: flow, File: file}} },
			func(err error) { ch <- result{err: err} },
		)
		return nil
	})
	if err != nil {
		return FlowSourceView{}, err
	}
	r := <-ch // waits off the actor: the worker posts the answer to it
	return r.v, r.err
}

// ApplySource replaces the flow with the edited text, as one undoable edit.
// It returns the problems found (the flow is then left as it was); an empty
// list means the text was applied.
func (s *FlowService) ApplySource(node, text string) ([]string, error) {
	type result struct {
		problems []string
		err      error
	}
	ch := make(chan result, 1)
	err := s.edit(node, func(o *openFlow) error {
		o.doc.ApplySource(text,
			func(problems []string) { ch <- result{problems: problems} },
			func(err error) { ch <- result{err: err} },
		)
		return nil
	})
	if err != nil {
		return nil, err
	}
	r := <-ch
	if r.problems == nil {
		r.problems = []string{}
	}
	return r.problems, r.err
}
