// SPDX-License-Identifier: Apache-2.0

package jsonpos

import (
	"strings"
	"testing"
)

func TestScan_should_record_offsets_of_kept_values(t *testing.T) {
	src := `{"log": {"entries": [ {"a": 1},
  {"b": [1, 2]} ], "x": "y"}}`

	got, err := Scan([]byte(src), func(p []string) bool { return len(p) == 3 && p[1] == "entries" })

	if err != nil {
		t.Fatal(err)
	}
	if got["log/entries/0"] != strings.Index(src, `{"a"`) || got["log/entries/1"] != strings.Index(src, `{"b"`) || len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestScan_should_return_the_offset_of_a_syntax_error(t *testing.T) {
	_, err := Scan([]byte(`{"a": [1, 2,, 3]}`), func([]string) bool { return false })

	if off, ok := ErrorOffset(err, 17); !ok || off != 12 {
		t.Fatalf("offset %d %v (%v)", off, ok, err)
	}
	_, truncated := Scan([]byte(`{"a": [1`), func([]string) bool { return false })
	if off, ok := ErrorOffset(truncated, 8); !ok || off != 8 {
		t.Fatalf("truncated input points at the end: %d", off)
	}
}
