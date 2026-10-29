// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"os"
)

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

// SourceApplied is what applying edited text did: the problems that stopped
// it (the flow is then as it was), or, when it was applied, what is wrong with
// the flow now (it is applied all the same).
type SourceApplied struct {
	Problems []string `json:"problems"`
	Notes    []string `json:"notes"`
}

// ApplySource replaces the flow with the edited text, as one undoable edit.
// An empty Problems means the text was applied.
func (s *FlowService) ApplySource(node, text string) (SourceApplied, error) {
	type result struct {
		problems, notes []string
		err             error
	}
	ch := make(chan result, 1)
	err := s.edit(node, func(o *openFlow) error {
		o.doc.ApplySource(text,
			func(problems, notes []string) { ch <- result{problems: problems, notes: notes} },
			func(err error) { ch <- result{err: err} },
		)
		return nil
	})
	if err != nil {
		return SourceApplied{}, err
	}
	r := <-ch
	return SourceApplied{Problems: nonNil(r.problems), Notes: nonNil(r.notes)}, r.err
}

// StandaloneView is the flow, and the flows of its collection it calls, as one
// Arazzo document other tools can read.
type StandaloneView struct {
	Text  string   `json:"text"`
	Notes []string `json:"notes"` // what had no Arazzo equivalent, one sentence each
	// Unsaved says the flow has edits the document does not show: it is made
	// from the saved flow.
	Unsaved bool `json:"unsaved"`
}

// StandaloneArazzo writes the saved flow as a standalone Arazzo document.
func (s *FlowService) StandaloneArazzo(node string) (StandaloneView, error) {
	type result struct {
		v   StandaloneView
		err error
	}
	ch := make(chan result, 1)
	err := s.edit(node, func(o *openFlow) error {
		unsaved := o.doc.Dirty()
		o.doc.Arazzo(
			func(text string, notes []string) {
				ch <- result{v: StandaloneView{Text: text, Notes: nonNil(notes), Unsaved: unsaved}}
			},
			func(err error) { ch <- result{err: err} },
		)
		return nil
	})
	if err != nil {
		return StandaloneView{}, err
	}
	r := <-ch
	return r.v, r.err
}

// SaveStandaloneArazzo asks where to save the standalone document and writes
// it there. It returns the file, "" when the user cancelled.
func (s *FlowService) SaveStandaloneArazzo(node string) (string, error) {
	view, err := s.StandaloneArazzo(node)
	if err != nil {
		return "", err
	}
	if s.app.pickers.Save == nil {
		return "", errNoDialogs
	}
	file, err := s.app.pickers.Save("Save the Arazzo document", "flow.arazzo.yaml")
	if err != nil || file == "" {
		return "", err
	}
	if err := os.WriteFile(file, []byte(view.Text), 0o600); err != nil {
		return "", fmt.Errorf("saving the document: %w", err)
	}
	return file, nil
}
