// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"strings"
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

// CronPreview is what the schedule editor says about a cron line while it is typed: the words, the next firings
// (in the schedule's own time zone) or why the line does not work.
type CronPreview struct {
	Words string   // "Every day at 03:00", "" when it is not a common pattern
	Next  []string // the next few firings, "Tue 09:00"; empty when the line is invalid
	Error string   // why the cron line or the zone cannot be used; "" when it can
}

// previewFirings is how many firings the editor lists.
const previewFirings = 3

// PreviewCron explains a cron line and zone as they stand in the editor, with no schedule saved.
// Before: a mistake in the cron line or the zone showed only after Save.
func PreviewCron(cron, tz string, now time.Time) CronPreview {
	cron = strings.TrimSpace(cron)
	if cron == "" {
		return CronPreview{}
	}
	job, err := scheduler.NewJob(model.Schedule{Name: "preview", Flow: "preview", Cron: cron, TZ: strings.TrimSpace(tz)})
	if err != nil {
		return CronPreview{Error: strings.TrimPrefix(ErrorText(err), "invalid schedule \"preview\": ")}
	}
	out := CronPreview{Words: DescribeCron(cron), Next: []string{}}
	for at, i := job.Next(now), 0; !at.IsZero() && i < previewFirings; at, i = job.Next(at), i+1 {
		out.Next = append(out.Next, at.In(job.Loc).Format("Mon 15:04"))
	}
	return out
}
