// SPDX-License-Identifier: Apache-2.0

// Package textenc turns the text of an import source into plain UTF-8.
// Windows tools write a byte order mark, and PowerShell's > and Notepad's
// "Unicode" save write UTF-16, so a file that looks fine to its owner is not
// valid JSON or YAML to a parser.
package textenc

import (
	"bytes"
	"unicode/utf16"
)

// The byte order marks that start a text.
const (
	utf8Mark    = "\xef\xbb\xbf"
	utf16Little = "\xff\xfe"
	utf16Big    = "\xfe\xff"
)

// ToUTF8 drops a UTF-8 byte order mark and converts UTF-16 (marked by its
// byte order mark) to UTF-8. Anything else is returned unchanged. A trailing
// odd byte of UTF-16 text is dropped.
func ToUTF8(data []byte) []byte {
	switch {
	case bytes.HasPrefix(data, []byte(utf8Mark)):
		return data[len(utf8Mark):]
	case bytes.HasPrefix(data, []byte(utf16Little)):
		return decodeUTF16(data[2:], func(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 })
	case bytes.HasPrefix(data, []byte(utf16Big)):
		return decodeUTF16(data[2:], func(b []byte) uint16 { return uint16(b[0])<<8 | uint16(b[1]) })
	}
	return data
}

func decodeUTF16(data []byte, unit func([]byte) uint16) []byte {
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		units = append(units, unit(data[i:i+2]))
	}
	return []byte(string(utf16.Decode(units)))
}
