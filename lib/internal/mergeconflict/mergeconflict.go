// SPDX-License-Identifier: Apache-2.0

// Package mergeconflict finds the markers git leaves in a file it could not merge.
package mergeconflict

import "bytes"

// Find reports the 1-based line of the first conflict start marker ("<<<<<<< " at the start of a line) that is
// followed by its separator and end marker, which is what an unresolved merge leaves. A "<<<<<<<" with no
// "=======" and ">>>>>>>" after it is not one: it may be text in a document.
func Find(data []byte) (line int, ok bool) {
	start, sep := 0, false
	for i, l := range bytes.Split(data, []byte("\n")) {
		l = bytes.TrimSuffix(l, []byte("\r"))
		switch {
		case start == 0 && (bytes.HasPrefix(l, []byte("<<<<<<< ")) || bytes.Equal(l, []byte("<<<<<<<"))):
			start = i + 1
		case start != 0 && !sep && (bytes.Equal(l, []byte("=======")) || bytes.Equal(l, []byte("|||||||")) || bytes.HasPrefix(l, []byte("||||||| "))):
			sep = true
		case start != 0 && sep && (bytes.HasPrefix(l, []byte(">>>>>>> ")) || bytes.Equal(l, []byte(">>>>>>>"))):
			return start, true
		}
	}
	return 0, false
}
