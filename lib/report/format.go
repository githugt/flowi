// SPDX-License-Identifier: Apache-2.0

package report

import (
	"time"
)

// statusMark is the symbol shown before a status in Markdown and text.
func statusMark(status string, handled bool) string {
	if handled {
		return "↷"
	}
	marks := map[string]string{
		"passed": "✓", "failed": "✗", "error": "!", "skipped": "○", "cancelled": "⊘", "aborted": "⊘", "running": "⋯",
	}
	if m, ok := marks[status]; ok {
		return m
	}
	return "?"
}

// statusLabel is the status as shown to people; handled failures say so.
func statusLabel(status string, handled bool) string {
	if handled {
		return status + " (continued)"
	}
	return status
}

// humanDuration keeps milliseconds below a second and two decimals above.
func humanDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(10 * time.Millisecond).String()
}

// seconds is a duration in seconds with millisecond precision (JUnit).
func seconds(d time.Duration) string {
	return formatFloat(d.Seconds())
}
