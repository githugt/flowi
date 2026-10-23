// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	flowi "github.com/githugt/flowi-lib"
)

// maxCompletions bounds the suggestions shown at once.
const maxCompletions = 20

// Complete finds the template reference being typed at cursor (after an
// unclosed "{{") and the completions that contain it, sorted. start is
// where the typed reference begins; no matches means nothing to offer.
func Complete(text string, cursor int, all []flowi.Completion) ([]flowi.Completion, int) {
	cursor = min(max(cursor, 0), len(text))
	before := text[:cursor]
	open := strings.LastIndex(before, "{{")
	if open < 0 || strings.Contains(before[open:], "}}") {
		return nil, 0
	}
	start := open + 2
	typed := strings.ToLower(strings.TrimSpace(before[start:]))
	var out []flowi.Completion
	for _, c := range all {
		if strings.Contains(strings.ToLower(c.Text), typed) {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b flowi.Completion) int { return strings.Compare(a.Text, b.Text) })
	if len(out) > maxCompletions {
		out = out[:maxCompletions]
	}
	return out, start
}
