// SPDX-License-Identifier: AGPL-3.0-or-later

// Package bridge connects the Svelte front end to the view-models in
// flowi/internal/vm. The view-models expect one "UI goroutine"; here it is
// the Actor. Services expose their methods to the front end, run them on
// the actor, and publish a fresh view (a plain struct) whenever one changes.
package bridge

import (
	"context"
	"sync"
)

// Actor runs functions one at a time, in order, on a single goroutine.
//
// Goroutine ownership: NewActor starts the goroutine; it ends when ctx is
// done. Functions still queued then are dropped, and Do returns without
// having run its function.
type Actor struct {
	inline bool
	mu     sync.Mutex
	queue  []func()
	wake   chan struct{}
	stop   <-chan struct{}
}

// NewActor starts an actor that runs until ctx is done.
func NewActor(ctx context.Context) *Actor {
	a := &Actor{wake: make(chan struct{}, 1), stop: ctx.Done()}
	go a.loop()
	return a
}

// NewInlineActor runs every function at once on the caller, in the order
// called. Tests use it, as vm.NewInlineWorker does.
func NewInlineActor() *Actor { return &Actor{inline: true} }

func (a *Actor) loop() {
	for {
		select {
		case <-a.stop:
			return
		case <-a.wake:
		}
		for f := a.next(); f != nil; f = a.next() {
			f()
		}
	}
}

func (a *Actor) next() func() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.queue) == 0 {
		return nil
	}
	f := a.queue[0]
	a.queue = a.queue[1:]
	return f
}

// Post queues f to run on the actor. It is the vm.Poster of the app.
func (a *Actor) Post(f func()) {
	if a.inline {
		f()
		return
	}
	a.mu.Lock()
	a.queue = append(a.queue, f)
	a.mu.Unlock()
	select {
	case a.wake <- struct{}{}:
	default: // a wake-up is already pending; the loop drains the whole queue
	}
}

// Do runs f on the actor and waits for it. It must not be called from a
// function the actor is running: that would wait on itself.
func (a *Actor) Do(f func()) {
	if a.inline {
		f()
		return
	}
	done := make(chan struct{})
	a.Post(func() {
		defer close(done)
		f()
	})
	select {
	case <-done:
	case <-a.stop:
	}
}
