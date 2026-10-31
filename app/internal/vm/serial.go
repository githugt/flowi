// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "slices"

// opQueue runs jobs one after another on the UI goroutine. A job starts the
// work and calls finish when its result has been applied. Without it, two
// quick actions on the same file (two cookie deletes) would read and write
// it at once and the second would undo the first, and results could be
// applied out of order.
type opQueue struct {
	jobs []func(finish func())
	busy bool
}

// add queues job, running it at once when nothing is in progress.
func (q *opQueue) add(job func(finish func())) {
	q.jobs = append(q.jobs, job)
	if !q.busy {
		q.next()
	}
}

func (q *opQueue) next() {
	if len(q.jobs) == 0 {
		q.busy = false
		return
	}
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	q.busy = true
	job(q.next)
}

// listeners are the callbacks of OnChange. A screen that goes away removes
// its own, so the view-model does not keep it alive.
type listeners struct {
	next int
	fs   map[int]func()
}

// add registers f and returns the function that removes it.
func (l *listeners) add(f func()) (remove func()) {
	if l.fs == nil {
		l.fs = map[int]func(){}
	}
	id := l.next
	l.next++
	l.fs[id] = f
	return func() { delete(l.fs, id) }
}

// fire calls every callback, in the order they were added.
func (l *listeners) fire() {
	ids := make([]int, 0, len(l.fs))
	for id := range l.fs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if f, ok := l.fs[id]; ok { // one may have removed another
			f()
		}
	}
}
