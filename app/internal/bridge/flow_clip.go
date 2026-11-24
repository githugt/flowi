// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

// SelectAll selects every step in the flow's own list (Ctrl+A on the canvas). The steps inside a container go with it.
func (s *FlowService) SelectAll(node string) {
	_ = s.edit(node, func(o *openFlow) error {
		all := o.doc.TopSteps()
		o.selected, o.also = "", nil
		if n := len(all); n > 0 {
			o.selected, o.also = all[n-1], all[:n-1]
		}
		s.app.emit("flow", s.view(node, o))
		return nil
	})
}

// CopySteps is the text to put on the clipboard for the steps ids: the form the flow's code view shows, holding only
// those steps. Arrows to steps that are not copied are left out of it.
func (s *FlowService) CopySteps(node string, ids []string) (string, error) {
	return s.copySteps(node, ids, false)
}

// CopyStepsPortable is CopySteps for putting the steps in another collection: each request and flow a step calls is
// written by its path, not its id, and each step is given a name and a description saying what it calls. Pasting it
// finds the same paths in the collection it lands in.
func (s *FlowService) CopyStepsPortable(node string, ids []string) (string, error) {
	return s.copySteps(node, ids, true)
}

func (s *FlowService) copySteps(node string, ids []string, portable bool) (string, error) {
	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	err := s.edit(node, func(o *openFlow) error {
		o.doc.CopyText(ids, portable,
			func(text string) { ch <- result{text: text} },
			func(err error) { ch <- result{err: err} },
		)
		return nil
	})
	if err != nil {
		return "", err
	}
	r := <-ch // waits off the actor: the worker posts the answer to it
	return r.text, r.err
}

// Pasted is what a paste did: the ids of the new steps, and the requests and flows the collection has no match for.
type Pasted struct {
	IDs []string `json:"ids"`
	// Unmatched are the paths (or ids, for a call that has no path) the pasted steps call that this collection does
	// not have. The steps are there all the same, still pointing at them, for the person to pick others.
	Unmatched []string `json:"unmatched"`
}

// PasteSteps adds the steps in text, as CopySteps or CopyStepsPortable made it, after the last selected step (at the
// end when none is selected), and selects them. Each request or flow a step calls is looked up in this flow's
// collection, by id and then by path. It is one undoable edit. Text that is not steps changes nothing and fails.
func (s *FlowService) PasteSteps(node, text string) (Pasted, error) {
	type result struct {
		ids       []string
		unmatched []string
		err       error
	}
	ch := make(chan result, 1)
	err := s.edit(node, func(o *openFlow) error {
		o.doc.PasteText(text, o.selected,
			func(ids, unmatched []string) { ch <- result{ids: ids, unmatched: unmatched} },
			func(err error) { ch <- result{err: err} },
		)
		return nil
	})
	if err != nil {
		return Pasted{}, err
	}
	r := <-ch
	if r.err != nil {
		return Pasted{}, r.err
	}
	_ = s.edit(node, func(o *openFlow) error {
		o.selected, o.also = r.ids[len(r.ids)-1], r.ids[:len(r.ids)-1]
		s.app.emit("flow", s.view(node, o))
		return nil
	})
	return Pasted{IDs: nonNil(r.ids), Unmatched: nonNil(r.unmatched)}, nil
}
