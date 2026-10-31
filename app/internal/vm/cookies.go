// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"
)

// Cookies is the request screen's Cookies tab: the saved jar of the
// request's collection, which every standalone send of that collection
// shares.
type Cookies struct {
	b          Backend
	w          *Worker
	collection string
	rows       []flowi.CookieInfo
	err        string
	onChange   listeners
	queue      opQueue
}

// NewCookies returns the tab's view-model; call Load.
func NewCookies(b Backend, w *Worker, collection model.ID) *Cookies {
	return &Cookies{b: b, w: w, collection: string(collection)}
}

// OnChange registers f to run after the rows or the error change.
// It returns the function that removes f again.
func (c *Cookies) OnChange(f func()) (remove func()) { return c.onChange.add(f) }

// Rows are the cookies in the jar.
func (c *Cookies) Rows() []flowi.CookieInfo { return c.rows }

// Err is the last problem, or "".
func (c *Cookies) Err() string { return c.err }

// Collection is the id of the collection whose jar this is.
func (c *Cookies) Collection() string { return c.collection }

// Load reads the jar in the background.
func (c *Cookies) Load() { c.change(func() error { return nil }) }

// Delete removes one cookie (CookieInfo.Key), then reloads.
func (c *Cookies) Delete(key string) {
	c.change(func() error { return c.b.DeleteCookie(c.collection, key) })
}

// Clear empties the jar, then reloads.
func (c *Cookies) Clear() { c.change(func() error { return c.b.ClearCookies(c.collection) }) }

// change runs act and then reads the jar, both off the UI goroutine.
func (c *Cookies) change(act func() error) {
	c.queue.add(func(finish func()) { c.run(act, finish) })
}

func (c *Cookies) run(act func() error, finish func()) {
	type loaded struct {
		rows []flowi.CookieInfo
		err  error
	}
	Run(c.w, func() loaded {
		actErr := act()
		rows, err := c.b.Cookies(c.collection)
		if actErr != nil {
			err = actErr // the jar is shown as it is, with why the change failed
		}
		return loaded{rows, err}
	}, func(l loaded) {
		c.err, c.rows = "", l.rows
		if l.err != nil {
			c.err = ErrorText(l.err)
		}
		c.onChange.fire()
		finish()
	})
}
