// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"sync"
	"time"
)

// firstLoadTimeout bounds how long State waits for a view-model's first load.
const firstLoadTimeout = 5 * time.Second

// firstLoad lets State wait for the view-model's first background load, so
// the first answer is complete. Events cannot stand in for it: one sent
// before the window has registered its handler is lost.
type firstLoad struct {
	once sync.Once
	done chan struct{}
}

func newFirstLoad() *firstLoad { return &firstLoad{done: make(chan struct{})} }

// mark records that the view-model has loaded (call it from its OnChange).
func (f *firstLoad) mark() { f.once.Do(func() { close(f.done) }) }

// wait blocks until the first load, or the timeout. Call it off the actor.
func (f *firstLoad) wait() {
	select {
	case <-f.done:
	case <-time.After(firstLoadTimeout):
	}
}
