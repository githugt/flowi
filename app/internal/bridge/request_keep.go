// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import "time"

// keepTimeout bounds Keep: storing an edit is one small file write, so
// anything near this long means the worker is stuck.
const keepTimeout = 10 * time.Second

// Keep stores the unsaved edits of the open requests among nodes in their
// edit slots, as the autosave would. Moving a request takes its edit slot
// along, but closing its tab first dropped what the autosave had not stored
// yet, so a request with edits could not be moved at all. It reports false
// when an edit could not be stored: changes only Save can keep (a new
// version, a new default), or a failed write.
func (s *RequestService) Keep(nodes []string) bool {
	var results []chan bool
	s.app.actor.Do(func() {
		for _, n := range nodes {
			o, ok := s.reqs[n]
			if !ok {
				continue
			}
			// Buffered: the answer may come on the actor after Do returns, and
			// must not block it if the wait below has given up.
			stored := make(chan bool, 1)
			results = append(results, stored)
			o.req.FlushResult(func(ok bool) { stored <- ok })
		}
	})
	deadline := time.After(keepTimeout)
	for _, stored := range results {
		select {
		case ok := <-stored:
			if !ok {
				return false
			}
		case <-deadline:
			return false
		}
	}
	return true
}
