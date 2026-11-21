// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"strconv"
	"strings"
)

// Expectation is the HTTP status a request is meant to answer with: the `expect:` of a version. A negative test
// ("a wrong password gives 401") says so here and passes on that status, where the seeded hook would fail on
// anything but a 2xx.
type Expectation struct {
	ranges []statusRange
}

type statusRange struct{ lo, hi int }

// ParseExpect reads an expect: value: a comma-separated list of a status (401), a range (400-499) or a class
// (4xx, which is 400-499). "" is no expectation.
func ParseExpect(s string) (Expectation, error) {
	var e Expectation
	s = strings.TrimSpace(s)
	if s == "" {
		return e, nil
	}
	for part := range strings.SplitSeq(s, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		r, ok := parseStatusRange(part)
		if !ok {
			return Expectation{}, fmt.Errorf("%q is not an HTTP status, a range like 400-499 or a class like 4xx", part)
		}
		e.ranges = append(e.ranges, r)
	}
	return e, nil
}

func parseStatusRange(s string) (statusRange, bool) {
	if len(s) == 3 && s[1:] == "xx" && s[0] >= '1' && s[0] <= '5' {
		lo := int(s[0]-'0') * 100
		return statusRange{lo, lo + 99}, true
	}
	if lo, hi, found := strings.Cut(s, "-"); found {
		a, okA := parseStatus(strings.TrimSpace(lo))
		b, okB := parseStatus(strings.TrimSpace(hi))
		return statusRange{a, b}, okA && okB && a <= b
	}
	n, ok := parseStatus(s)
	return statusRange{n, n}, ok
}

func parseStatus(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 100 && n <= 599
}

// Set reports whether there is an expectation at all.
func (e Expectation) Set() bool { return len(e.ranges) > 0 }

// Matches reports whether status is one the request is expected to answer with.
func (e Expectation) Matches(status int) bool {
	for _, r := range e.ranges {
		if status >= r.lo && status <= r.hi {
			return true
		}
	}
	return false
}

// String is the expectation as written back, in the order given: "401", "200-299", "404, 410".
func (e Expectation) String() string {
	parts := make([]string, len(e.ranges))
	for i, r := range e.ranges {
		if r.lo == r.hi {
			parts[i] = strconv.Itoa(r.lo)
		} else {
			parts[i] = fmt.Sprintf("%d-%d", r.lo, r.hi)
		}
	}
	return strings.Join(parts, ", ")
}
