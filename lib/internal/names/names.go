// SPDX-License-Identifier: Apache-2.0

// Package names checks names that become file names inside Flowi's own
// directories (pinned examples, saved values, digests), so content from a
// collection can never choose a path outside them.
package names

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// maxRunes bounds a name so a file name stays well under OS limits; maxBytes does the same for names whose letters
// take several bytes each (a file name is limited in bytes, not characters).
const (
	maxRunes = 64
	maxBytes = 200
)

// Valid reports whether name is safe as a single file name: letters (of any script), marks, digits, '.', '_' and '-',
// not starting with '.' nor ending in one, at most 64 characters, in Unicode normal form C, and not a name Windows
// reserves (CON, NUL, COM1…). It rules out separators, "..", hidden files, absolute paths, control and bidi
// characters (they are not letters) and a name that macOS would store under another spelling.
//
// Before: only ASCII letters were accepted although the messages said "letters", so a request called "Café" or a
// flow in Japanese could not be saved under its own name.
func Valid(name string) bool {
	if name == "" || name[0] == '.' || name[len(name)-1] == '.' || len(name) > maxBytes || utf8.RuneCountInString(name) > maxRunes {
		return false
	}
	for _, r := range name {
		if !nameRune(r) {
			return false
		}
	}
	return norm.NFC.IsNormalString(name) && !reservedOnWindows(name)
}

// Normalize returns name in Unicode normal form C, the spelling Valid accepts. Callers that turn what a person typed
// into a name run it first, so "e" + a combining accent and "é" are one name.
func Normalize(name string) string { return norm.NFC.String(name) }

func nameRune(r rune) bool {
	if r < utf8.RuneSelf {
		return r == '.' || r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r)
}

// reservedOnWindows reports the device names Windows will not create a file under, with or without an extension.
func reservedOnWindows(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	switch s := strings.ToUpper(stem); s {
	case "CON", "PRN", "AUX", "NUL":
		return true
	default:
		return len(s) == 4 && (strings.HasPrefix(s, "COM") || strings.HasPrefix(s, "LPT")) && s[3] >= '1' && s[3] <= '9'
	}
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
