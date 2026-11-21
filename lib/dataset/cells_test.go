// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"testing"
	"time"
)

func TestScalar_Given_driver_values_When_flattened_Then_nested_values_become_text(t *testing.T) {
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name string
		in   any
		want any
	}{
		{"number kept", int64(7), int64(7)},
		{"bool kept", true, true},
		{"nil kept", nil, nil},
		{"time as RFC 3339", when, "2026-01-02T03:04:05Z"},
		{"bytes as text", []byte("abc"), "abc"},
		{"list as JSON", []any{1, "a"}, `[1,"a"]`},
		{"map as JSON", map[string]any{"k": 1}, `{"k":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scalar(tt.in); got != tt.want {
				t.Fatalf("scalar(%v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestPlainBytes_Given_database_bytes_When_typed_Then_numbers_parse_and_binary_is_base64(t *testing.T) {
	tests := []struct {
		name, typ string
		in        []byte
		want      any
	}{
		{"integer column", "BIGINT", []byte("42"), int64(42)},
		{"unsigned beyond int64", "UNSIGNED BIGINT", []byte("18446744073709551615"), uint64(18446744073709551615)},
		{"float column", "DOUBLE", []byte("1.5"), 1.5},
		{"integer column holding junk stays text", "INT", []byte("n/a"), "n/a"},
		{"binary column", "BLOB", []byte{0xff, 0x00}, "/wA="},
		{"invalid UTF-8 in a text column", "TEXT", []byte{0xc3, 0x28}, "wyg="},
		{"plain text", "TEXT", []byte("héllo"), "héllo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plainBytes(tt.in, tt.typ); got != tt.want {
				t.Fatalf("plainBytes(%q, %s) = %#v, want %#v", tt.in, tt.typ, got, tt.want)
			}
		})
	}
}

func TestNumberOf_Given_cells_When_read_as_numbers_Then_text_is_trimmed_and_junk_refused(t *testing.T) {
	if f, ok := numberOf(" 2.5 "); !ok || f != 2.5 {
		t.Fatalf("padded text: got %v, %v", f, ok)
	}
	if _, ok := numberOf("two"); ok {
		t.Fatal("words are not numbers")
	}
	for _, v := range []any{int8(1), int16(1), int32(1), uint(1), uint8(1), uint16(1), uint32(1), uint64(1), float32(1)} {
		if f, ok := asFloat(v); !ok || f != 1 {
			t.Errorf("asFloat(%T) = %v, %v", v, f, ok)
		}
	}
}
