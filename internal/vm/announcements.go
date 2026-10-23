// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/githugt/flowi-lib/announce"
)

// Announcements is the list of project notices on the welcome screen. It
// is quiet by design: no feed configured, announcements turned off, or a
// failed fetch all simply leave the list empty (or showing the last good
// feed), because a notice board must never get in the way.
type Announcements struct {
	b        Backend
	w        *Worker
	items    []announce.Item
	onChange []func()
	onError  func(error)
	mu       sync.Mutex // serialises the read-modify-write of the dismissed list
}

// NewAnnouncements returns the list's view-model; call Load.
func NewAnnouncements(b Backend, w *Worker) *Announcements { return &Announcements{b: b, w: w} }

// OnChange registers f to run when the items change.
func (a *Announcements) OnChange(f func()) { a.onChange = append(a.onChange, f) }

// OnError registers f for the errors of Dismiss and TurnOff, which would
// otherwise change nothing without a word.
func (a *Announcements) OnError(f func(error)) { a.onError = f }

func (a *Announcements) failed(err error) {
	if err != nil && a.onError != nil {
		a.onError(err)
	}
}

// Items are the announcements to show.
func (a *Announcements) Items() []announce.Item { return a.items }

// Load fetches in the background.
func (a *Announcements) Load() {
	Run(a.w, func() []announce.Item {
		// The error is not shown: the result already holds whatever the
		// cache could offer, and nothing when there is nothing.
		res, _ := a.b.Announcements(context.Background(), false)
		dismissed := dismissedIDs(a.b)
		// An id the setting could not store whole (it is a comma-separated
		// list) could never be remembered, so it would come back every time.
		return slices.DeleteFunc(slices.Clone(res.Items), func(it announce.Item) bool {
			return slices.Contains(dismissed, it.ID) || !storableID(it.ID)
		})
	}, a.show)
}

func (a *Announcements) show(items []announce.Item) {
	a.items = items
	for _, f := range a.onChange {
		f()
	}
}

// dismissedKey is the user setting that remembers the dismissed ids.
const dismissedKey = "announcements_dismissed"

// dismissedIDs reads the dismissed ids; it blocks, so call it from a worker.
func dismissedIDs(b Backend) []string {
	v, ok := b.Settings("").Get(dismissedKey)
	if !ok {
		return nil
	}
	switch list := v.Value.(type) {
	case []string:
		return list
	case []any:
		var out []string
		for _, e := range list {
			if s, isString := e.(string); isString {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// Dismiss remembers that the user saw announcement id ("Got it"): it is not
// shown again, other announcements still are.
func (a *Announcements) Dismiss(id string) {
	Run(a.w, func() error {
		a.mu.Lock()
		defer a.mu.Unlock()
		ids := dismissedIDs(a.b)
		if slices.Contains(ids, id) {
			return nil
		}
		return a.b.SetUserSetting(dismissedKey, strings.Join(append(ids, id), ","))
	}, func(err error) {
		a.failed(err)
		if err == nil {
			a.show(slices.DeleteFunc(slices.Clone(a.items), func(it announce.Item) bool { return it.ID == id }))
		}
	})
}

// storableID reports whether the dismissed-list setting keeps id as it is.
func storableID(id string) bool {
	return id != "" && !strings.Contains(id, ",") && id == strings.TrimSpace(id)
}

// TurnOff switches announcements off (the announcements setting) and
// empties the list.
func (a *Announcements) TurnOff() {
	Run(a.w, func() error { return a.b.SetUserSetting("announcements", "false") }, func(err error) {
		a.failed(err)
		if err == nil {
			a.show(nil)
		}
	})
}
