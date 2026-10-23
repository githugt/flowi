// SPDX-License-Identifier: Apache-2.0

// Package version parses request version labels ("1", "1.1", "2.0.1") and
// resolves which version a request node runs.
package version

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ErrInvalidLabel is returned for labels that are not dotted non-negative
// integers without leading zeros.
var ErrInvalidLabel = errors.New("invalid version label")

// Label is a parsed version label. The zero value is not a valid label.
type Label struct {
	raw   string
	parts []uint64
}

// Parse parses a label such as "1", "1.10" or "2.0.1".
func Parse(s string) (Label, error) {
	segments := strings.Split(s, ".")
	parts := make([]uint64, 0, len(segments))
	for _, seg := range segments {
		n, ok := parseSegment(seg)
		if !ok {
			return Label{}, fmt.Errorf("%w: %q", ErrInvalidLabel, s)
		}
		parts = append(parts, n)
	}
	return Label{raw: s, parts: parts}, nil
}

// parseSegment accepts "0" or a digit string without a leading zero that
// fits in uint64.
func parseSegment(seg string) (uint64, bool) {
	if seg == "" || (len(seg) > 1 && seg[0] == '0') {
		return 0, false
	}
	for _, r := range seg {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(seg, 10, 64)
	return n, err == nil
}

// String returns the label exactly as written.
func (l Label) String() string { return l.raw }

// Compare orders labels numerically per segment. Missing trailing segments
// count as zero, so "1" and "1.0" are equal.
func (l Label) Compare(o Label) int {
	for i := range max(len(l.parts), len(o.parts)) {
		a, b := segmentAt(l.parts, i), segmentAt(o.parts, i)
		if a != b {
			if a < b {
				return -1
			}
			return 1
		}
	}
	return 0
}

func segmentAt(parts []uint64, i int) uint64 {
	if i < len(parts) {
		return parts[i]
	}
	return 0
}

// Highest returns the greatest label, or false when ls is empty.
func Highest(ls []Label) (Label, bool) {
	if len(ls) == 0 {
		return Label{}, false
	}
	return slices.MaxFunc(ls, Label.Compare), true
}

// Normalize trims a typed label and strips a leading "v", "ver", "ver." or
// "version" (any case, with dots, colons or spaces after it) when a number
// follows: "Version 1.2" is "1.2". Anything else is returned trimmed and as
// typed, for Parse to accept or refuse. Every place that takes a label from a
// person (the GUI, the CLI, the library's own save) uses it, so they agree.
func Normalize(raw string) string {
	s := strings.TrimSpace(raw)
	lower := strings.ToLower(s)
	for _, word := range []string{"version", "ver", "v"} {
		rest, found := strings.CutPrefix(lower, word)
		if !found {
			continue
		}
		rest = strings.TrimLeft(rest, ".: ")
		if rest != "" && rest[0] >= '0' && rest[0] <= '9' {
			return s[len(s)-len(rest):]
		}
	}
	return s
}
