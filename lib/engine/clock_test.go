// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"sync"
	"testing"
	"time"
)

// fakeClock is a controllable clock: After registers a timer that fires
// only when Advance passes it, so waits in the engine never need sleeping.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  []fakeTimer
	changed chan struct{} // closed and replaced whenever a timer is added
}

type fakeTimer struct {
	at time.Time
	ch chan time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC), changed: make(chan struct{})}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.now
		return ch
	}
	c.timers = append(c.timers, fakeTimer{at: c.now.Add(d), ch: ch})
	close(c.changed)
	c.changed = make(chan struct{})
	return ch
}

// Advance moves time forward and fires every timer that is due.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	pending := c.timers[:0]
	for _, t := range c.timers {
		if t.at.After(c.now) {
			pending = append(pending, t)
			continue
		}
		t.ch <- c.now
	}
	c.timers = pending
}

// WaitForTimers blocks until at least n timers are pending (the engine is
// waiting on them), failing the test after a real-time deadline.
func (c *fakeClock) WaitForTimers(t *testing.T, n int) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		c.mu.Lock()
		count, changed := len(c.timers), c.changed
		c.mu.Unlock()
		if count >= n {
			return
		}
		select {
		case <-changed:
		case <-deadline:
			t.Fatalf("waited for %d timers, have %d", n, count)
		}
	}
}
