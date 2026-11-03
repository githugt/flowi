// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"strconv"
	"strings"
)

// cronAlias is the five-field form of an @alias, "" when it is not one.
func cronAlias(name string) string {
	switch name {
	case "@hourly":
		return "0 * * * *"
	case "@daily", "@midnight":
		return "0 0 * * *"
	case "@weekly":
		return "0 0 * * 0"
	case "@monthly":
		return "0 0 1 * *"
	}
	return ""
}

// weekdayName names a day of the week, 0 being Sunday.
func weekdayName(d int) string {
	return [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}[d%7]
}

// DescribeCron says a cron expression in words ("Every day at 03:00"), for the patterns people write
// most. It returns "" for anything else, so the screen shows the cron text alone rather than a wrong sentence.
func DescribeCron(expr string) string {
	if alias := cronAlias(strings.TrimSpace(expr)); alias != "" {
		expr = alias
	}
	f := strings.Fields(expr)
	if len(f) != 5 {
		return ""
	}
	minute, hour, dom, month, dow := f[0], f[1], f[2], f[3], f[4]
	if month != "*" {
		return ""
	}
	switch {
	case isAll(minute, hour, dom, dow):
		return "Every minute"
	case isStep(minute) && isAll(hour, dom, dow):
		return describeStep(minute)
	case isAll(hour, dom, dow):
		return describeHourly(minute)
	}
	at, ok := clockTime(hour, minute)
	if !ok {
		return ""
	}
	return describeDay(dom, dow, at)
}

func isAll(fields ...string) bool {
	for _, f := range fields {
		if f != "*" {
			return false
		}
	}
	return true
}

func isStep(field string) bool { return strings.HasPrefix(field, "*/") }

// describeStep reads "*/15" as "Every 15 minutes"; only steps that divide the hour evenly are said.
func describeStep(field string) string {
	n, err := strconv.Atoi(strings.TrimPrefix(field, "*/"))
	if err != nil || n < 2 || n > 30 || 60%n != 0 {
		return ""
	}
	return fmt.Sprintf("Every %d minutes", n)
}

// describeHourly reads "30 * * * *" as "Every hour at :30"; "0" is plain "Every hour".
func describeHourly(minute string) string {
	m, ok := inRange(minute, 0, 59)
	switch {
	case !ok:
		return ""
	case m == 0:
		return "Every hour"
	}
	return fmt.Sprintf("Every hour at :%02d", m)
}

func clockTime(hour, minute string) (string, bool) {
	h, hok := inRange(hour, 0, 23)
	m, mok := inRange(minute, 0, 59)
	if !hok || !mok {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", h, m), true
}

// describeDay words the day part: every day, weekdays, one weekday, or one day of the month.
func describeDay(dom, dow, at string) string {
	switch {
	case dom == "*" && dow == "*":
		return "Every day at " + at
	case dom == "*" && dow == "1-5":
		return "Weekdays at " + at
	case dom == "*":
		d, ok := inRange(dow, 0, 7)
		if !ok {
			return ""
		}
		return fmt.Sprintf("Every %s at %s", weekdayName(d), at)
	case dow == "*":
		d, ok := inRange(dom, 1, 31)
		if !ok {
			return ""
		}
		return fmt.Sprintf("On day %d of every month at %s", d, at)
	}
	return ""
}

// inRange reads a plain number (no lists, ranges or steps) that lies within lo..hi.
func inRange(field string, lo, hi int) (int, bool) {
	n, err := strconv.Atoi(field)
	return n, err == nil && n >= lo && n <= hi
}
