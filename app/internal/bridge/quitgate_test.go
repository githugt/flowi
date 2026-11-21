// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"time"
)

func TestQuitGate_Request(t *testing.T) {
	// Before: the close was cancelled until the page answered, so a hung page made the window impossible to close.
	clock := time.Unix(1000, 0)
	g := NewQuitGate(3 * time.Second)
	g.now = func() time.Time { return clock }

	if got := g.Request(); got != QuitAsk {
		t.Fatalf("first request: %v, want ask", got)
	}
	clock = clock.Add(time.Second)
	if got := g.Request(); got != QuitWait {
		t.Fatalf("a quick second request: %v, want wait (a double click must not force the quit)", got)
	}
	clock = clock.Add(3 * time.Second)
	if got := g.Request(); got != QuitForce {
		t.Fatalf("a second request after the grace time with no answer: %v, want force", got)
	}
}

func TestQuitGate_Request_Given_a_page_that_answered_Then_it_is_asked_again_and_never_forced(t *testing.T) {
	clock := time.Unix(1000, 0)
	g := NewQuitGate(3 * time.Second)
	g.now = func() time.Time { return clock }

	g.Request()
	g.Ack()
	clock = clock.Add(time.Minute)

	if got := g.Request(); got != QuitAsk {
		t.Fatalf("got %v, want ask: a page that answers is not hung", got)
	}
}
