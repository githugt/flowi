// SPDX-License-Identifier: Apache-2.0

// Package cron parses five-field cron expressions and finds when they next
// fire. It has no clock and starts no goroutines: callers pass the time.
//
// Fields are minute, hour, day of month, month and day of week. Each is a
// comma list of "*", "?", a value, a range a-b, optionally followed by
// "/step". Months and weekdays may be written as names (jan, mon). Day of
// week 7 means Sunday. When both day fields are restricted a day matches if
// either does (the traditional rule). The macros @yearly, @annually,
// @monthly, @weekly, @daily, @midnight and @hourly are accepted.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseError says which field of an expression is wrong and why.
type ParseError struct {
	Field  string // "" when the expression as a whole is wrong
	Detail string
}

func (e *ParseError) Error() string {
	if e.Field == "" {
		return e.Detail
	}
	return e.Field + ": " + e.Detail
}

// Schedule is a parsed expression.
type Schedule struct {
	minute, hour, dom, month, dow uint64 // bit n set: value n matches
	domStar, dowStar              bool
}

type fieldSpec struct {
	name   string
	lo, hi int
	names  []string // names[i] is the name of value lo+i; nil for plain numbers
	kind   string   // what a bad name is not: "a number", "a month", "a weekday"
}

func fieldSpecs() [5]fieldSpec {
	return [5]fieldSpec{
		{name: "minute", lo: 0, hi: 59, kind: "a number"},
		{name: "hour", lo: 0, hi: 23, kind: "a number"},
		{name: "day of month", lo: 1, hi: 31, kind: "a number"},
		{name: "month", lo: 1, hi: 12, kind: "a month", names: strings.Fields("jan feb mar apr may jun jul aug sep oct nov dec")},
		{name: "day of week", lo: 0, hi: 7, kind: "a weekday", names: strings.Fields("sun mon tue wed thu fri sat")},
	}
}

func macros() map[string]string {
	return map[string]string{
		"@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *", "@monthly": "0 0 1 * *",
		"@weekly": "0 0 * * 0", "@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@hourly": "0 * * * *",
	}
}

// Parse reads an expression. Errors are *ParseError.
func Parse(expr string) (Schedule, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "@") {
		full, ok := macros()[strings.ToLower(expr)]
		if !ok {
			return Schedule{}, &ParseError{Detail: "unknown macro " + expr}
		}
		expr = full
	}
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return Schedule{}, &ParseError{Detail: fmt.Sprintf("expected 5 fields (minute hour day-of-month month day-of-week), got %d", len(fields))}
	}
	specs := fieldSpecs()
	var bits [5]uint64
	for i, f := range fields {
		b, err := parseField(f, specs[i])
		if err != nil {
			return Schedule{}, &ParseError{Field: specs[i].name, Detail: err.Error()}
		}
		bits[i] = b
	}
	if bits[4]&(1<<7) != 0 { // 7 is Sunday too
		bits[4] = bits[4]&^(1<<7) | 1
	}
	return Schedule{minute: bits[0], hour: bits[1], dom: bits[2], month: bits[3], dow: bits[4],
		domStar: isStar(fields[2]), dowStar: isStar(fields[4])}, nil
}

func isStar(f string) bool { return strings.HasPrefix(f, "*") || strings.HasPrefix(f, "?") }

func parseField(field string, s fieldSpec) (uint64, error) {
	var bits uint64
	for _, item := range strings.Split(field, ",") {
		if item == "" {
			return 0, fmt.Errorf("empty list item")
		}
		b, err := parseItem(item, s)
		if err != nil {
			return 0, err
		}
		bits |= b
	}
	return bits, nil
}

func parseItem(item string, s fieldSpec) (uint64, error) {
	base, stepText, hasStep := strings.Cut(item, "/")
	step := 1
	if hasStep {
		n, err := strconv.Atoi(stepText)
		if err != nil {
			return 0, fmt.Errorf("%q is not a number", stepText)
		}
		if n < 1 {
			return 0, fmt.Errorf("step must be at least 1")
		}
		step = n
	}
	lo, hi, err := parseRange(base, s, hasStep)
	if err != nil {
		return 0, err
	}
	var bits uint64
	for v := lo; v <= hi; v += step {
		bits |= 1 << uint(v) //nolint:gosec // v is within the field's 0-59 bounds
	}
	return bits, nil
}

// parseRange returns the inclusive bounds a base selects. A lone value with
// a step ("10/20") runs to the end of the field.
func parseRange(base string, s fieldSpec, hasStep bool) (int, int, error) {
	if base == "*" || base == "?" {
		return s.lo, s.hi, nil
	}
	from, to, isRange := strings.Cut(base, "-")
	lo, err := parseValue(from, s)
	if err != nil {
		return 0, 0, err
	}
	switch {
	case isRange:
		hi, err := parseValue(to, s)
		if err != nil {
			return 0, 0, err
		}
		if hi < lo {
			return 0, 0, fmt.Errorf("range %s runs backwards", base)
		}
		return lo, hi, nil
	case hasStep:
		return lo, s.hi, nil
	}
	return lo, lo, nil
}

func parseValue(text string, s fieldSpec) (int, error) {
	v, ok := lookupName(text, s)
	if !ok {
		n, err := strconv.Atoi(text)
		if err != nil {
			return 0, fmt.Errorf("%q is not %s", text, s.kind)
		}
		v = n
	}
	if v < s.lo || v > s.hi {
		return 0, fmt.Errorf("%d is outside %d-%d", v, s.lo, s.hi)
	}
	return v, nil
}

func lookupName(text string, s fieldSpec) (int, bool) {
	for i, n := range s.names {
		if strings.EqualFold(text, n) {
			return s.lo + i, true
		}
	}
	return 0, false
}

// searchDays bounds Next: a leap day can be eight years away.
const searchDays = 366 * 9

// Next returns the first matching minute strictly after the given time, in
// its location, or the zero time if the expression never fires (Feb 30).
//
// A time inside a spring-forward gap fires at the first moment after the
// gap that day; a repeated autumn hour fires once.
func (s Schedule) Next(after time.Time) time.Time {
	loc := after.Location()
	y, m, d := after.Date()
	for day := 0; day < searchDays; day++ {
		date := time.Date(y, m, d+day, 12, 0, 0, 0, loc) // noon: stable across DST shifts
		if !s.matchesDay(date) {
			continue
		}
		if t, ok := s.firstOnDay(date, after); ok {
			return t
		}
	}
	return time.Time{}
}

func (s Schedule) matchesDay(date time.Time) bool {
	if s.month&(1<<uint(date.Month())) == 0 { //nolint:gosec // Month is 1-12
		return false
	}
	inMonth := s.dom&(1<<uint(date.Day())) != 0 //nolint:gosec // Day is 1-31
	inWeek := s.dow&(1<<uint(date.Weekday())) != 0
	if !s.domStar && !s.dowStar {
		return inMonth || inWeek
	}
	return inMonth && inWeek // a star field has every bit set, so it never excludes a day
}

func (s Schedule) firstOnDay(date, after time.Time) (time.Time, bool) {
	y, m, d := date.Date()
	for h := 0; h < 24; h++ {
		if s.hour&(1<<uint(h)) == 0 { //nolint:gosec // h is 0-23
			continue
		}
		for mi := 0; mi < 60; mi++ {
			if s.minute&(1<<uint(mi)) == 0 { //nolint:gosec // mi is 0-59
				continue
			}
			t := time.Date(y, m, d, h, mi, 0, 0, date.Location())
			if t.Hour() != h || t.Minute() != mi {
				// The wall time falls in a spring-forward gap, where Go
				// picks a neighbouring instant. Run at the end of the gap.
				_, t = t.ZoneBounds()
			}
			if !t.IsZero() && t.After(after) {
				return t, true
			}
		}
	}
	return time.Time{}, false
}
