// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "sync"

// Poster runs a function on the UI goroutine (the app posts it to its UI thread; tests
// may run it directly).
type Poster func(func())

// Worker runs backend calls off the UI goroutine and posts their results
// back.
//
// Goroutine ownership: Run starts one goroutine per call; it ends after
// posting the result. Wait blocks until every started call has posted.
type Worker struct {
	post   Poster
	inline bool // tests: run work and result on the caller, in order
	wg     sync.WaitGroup
}

// NewWorker returns a worker posting results with post.
func NewWorker(post Poster) *Worker { return &Worker{post: post} }

// NewInlineWorker runs every call's work and result on the calling
// goroutine, at once. Headless tests use it: they act as the UI goroutine
// themselves, so results must not arrive from another goroutine.
func NewInlineWorker() *Worker { return &Worker{post: func(f func()) { f() }, inline: true} }

// Run calls work on a new goroutine and then done with its result on the
// UI goroutine (inline workers: both at once on the caller).
func Run[R any](w *Worker, work func() R, done func(R)) {
	if w.inline {
		done(work())
		return
	}
	w.wg.Go(func() {
		r := work()
		w.post(func() { done(r) })
	})
}

// Post runs f on the UI goroutine.
func (w *Worker) Post(f func()) { w.post(f) }

// Wait blocks until every call started so far has posted its result.
func (w *Worker) Wait() { w.wg.Wait() }
