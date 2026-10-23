// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "time"

// Scheduler runs f once, on the UI goroutine, after d. The returned stop
// cancels it if it has not run.
type Scheduler interface {
	After(d time.Duration, f func()) (stop func())
}

// NewTimerScheduler schedules with real timers and posts each callback to
// the UI goroutine with post. The timer goroutine only posts; it ends when
// the timer fires or is stopped.
func NewTimerScheduler(post Poster) Scheduler { return timerScheduler{post: post} }

type timerScheduler struct{ post Poster }

func (s timerScheduler) After(d time.Duration, f func()) func() {
	t := time.AfterFunc(d, func() { s.post(f) })
	return func() { t.Stop() }
}
