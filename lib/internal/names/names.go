// SPDX-License-Identifier: Apache-2.0

// Package names checks names that become file names inside Flowi's own
// directories (pinned examples, saved values, digests), so content from a
// collection can never choose a path outside them.
package names

import "unicode/utf8"

// maxRunes bounds a name so a file name stays well under OS limits.
const maxRunes = 64

// Valid reports whether name is safe as a single file name: letters,
// digits, '.', '_' and '-', not starting with '.', at most 64 runes. It
// rules out separators, "..", hidden files and absolute paths.
func Valid(name string) bool {
	if name == "" || name[0] == '.' || utf8.RuneCountInString(name) > maxRunes {
		return false
	}
	for _, r := range name {
		if !nameRune(r) {
			return false
		}
	}
	return true
}

func nameRune(r rune) bool {
	return r == '.' || r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// AllValid reports whether every name is Valid. Ids and version labels
// from a collection become directory names in the workspace data dir, so
// code that joins them into a path checks them first: the codec reports a
// bad id but still loads it.
func AllValid(list ...string) bool {
	for _, n := range list {
		if !Valid(n) {
			return false
		}
	}
	return true
}
