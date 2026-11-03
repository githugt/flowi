// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/serve/scheduler"
)

// outlookSpan is how far ahead the scheduler screen looks.
const outlookSpan = 24 * time.Hour

// ScheduleOutlook is when a schedule fires next and the minutes from now of every firing in the next 24 hours.
// A disabled or invalid schedule is never due: it returns a zero time and no firings.
func ScheduleOutlook(s model.Schedule, now time.Time) (next time.Time, minutesAhead []int) {
	if s.Disabled {
		return time.Time{}, nil
	}
	job, err := scheduler.NewJob(s)
	if err != nil {
		return time.Time{}, nil
	}
	next = job.Next(now)
	for at := next; !at.IsZero() && at.Sub(now) <= outlookSpan; at = job.Next(at) {
		minutesAhead = append(minutesAhead, int(at.Sub(now)/time.Minute))
	}
	return next, minutesAhead
}
