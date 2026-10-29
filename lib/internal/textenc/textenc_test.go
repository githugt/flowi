// SPDX-License-Identifier: Apache-2.0

package textenc

import (
	"testing"
	"unicode/utf16"
)

func utf16Bytes(s string, bom []byte, order func(uint16) []byte) []byte {
	out := append([]byte{}, bom...)
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, order(u)...)
	}
	return out
}

func TestToUTF8(t *testing.T) {
	little := func(u uint16) []byte { return []byte{byte(u), byte(u >> 8)} }
	big := func(u uint16) []byte { return []byte{byte(u >> 8), byte(u)} }
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"plain", []byte(`{"a":1}`), `{"a":1}`},
		{"utf8_bom", []byte("\xef\xbb\xbf{\"a\":1}"), `{"a":1}`},
		{"utf16_little_endian", utf16Bytes(`{"a":"é😀"}`, []byte{0xff, 0xfe}, little), `{"a":"é😀"}`},
		{"utf16_big_endian", utf16Bytes(`{"a":"é"}`, []byte{0xfe, 0xff}, big), `{"a":"é"}`},
		{"odd_length_utf16_keeps_whole_units", append(utf16Bytes("ab", []byte{0xff, 0xfe}, little), 0x41), "ab"},
		{"empty", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(ToUTF8(tt.in))

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
