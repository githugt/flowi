// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"sync"
	"time"
)

// QuitStep is what to do with a request to close the window that has unsaved work.
type QuitStep int

const (
	// QuitAsk: cancel the close and ask the page what to do.
	QuitAsk QuitStep = iota
	// QuitWait: the page was asked a moment ago; cancel the close and do not ask again.
	QuitWait
	// QuitForce: the page did not answer the earlier question. Keep what can be kept and quit.
	QuitForce
)

// QuitGate stops a hung or crashed page from making the window impossible to close. Before, the close was cancelled
// until the page answered, so a page that never answered kept the window open for good. The page acknowledges the
// question at once; a second close request that finds the first one unacknowledged for the grace time quits.
type QuitGate struct {
	mu    sync.Mutex
	now   func() time.Time
	grace time.Duration
	asked time.Time
	acked bool
}

// NewQuitGate makes a gate that forces the quit when the first question stays unacknowledged for grace.
func NewQuitGate(grace time.Duration) *QuitGate {
	return &QuitGate{now: time.Now, grace: grace}
}

// Request is called for every request to close the window that has unsaved work.
func (g *QuitGate) Request() QuitStep {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.asked.IsZero() && !g.acked {
		if g.now().Sub(g.asked) >= g.grace {
			return QuitForce
		}
		return QuitWait
	}
	g.asked, g.acked = g.now(), false
	return QuitAsk
}

// Ack is called when the page has received the question.
func (g *QuitGate) Ack() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.acked = true
}
