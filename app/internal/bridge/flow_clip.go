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
	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	err := s.edit(node, func(o *openFlow) error {
		o.doc.CopyText(ids,
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

// PasteSteps adds the steps in text, as CopySteps made it, after the last selected step (at the end when none is
// selected), and selects them. It is one undoable edit. Text that is not steps changes nothing and fails.
func (s *FlowService) PasteSteps(node, text string) ([]string, error) {
	type result struct {
		ids []string
		err error
	}
	ch := make(chan result, 1)
	err := s.edit(node, func(o *openFlow) error {
		o.doc.PasteText(text, o.selected,
			func(ids []string) { ch <- result{ids: ids} },
			func(err error) { ch <- result{err: err} },
		)
		return nil
	})
	if err != nil {
		return nil, err
	}
	r := <-ch
	if r.err != nil {
		return nil, r.err
	}
	_ = s.edit(node, func(o *openFlow) error {
		o.selected, o.also = r.ids[len(r.ids)-1], r.ids[:len(r.ids)-1]
		s.app.emit("flow", s.view(node, o))
		return nil
	})
	return nonNil(r.ids), nil
}
