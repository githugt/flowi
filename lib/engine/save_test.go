// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"testing"
)

func TestPinWarning(t *testing.T) {
	// Before: a binary response pinned as an empty body and a huge JSON was committed, with no word on either.
	tests := map[string]struct {
		body Body
		want string
	}{
		"json":      {Body{JSON: map[string]any{"a": 1}, Size: 9}, ""},
		"text":      {Body{Text: "hi", Size: 2}, ""},
		"empty":     {Body{}, ""},
		"binary":    {Body{Base64: "AAEC", Size: 3}, "binary"},
		"file":      {Body{File: "/tmp/x", Size: 5 << 20}, "binary"},
		"large":     {Body{Text: "x", Size: 10 << 20}, "10240 KiB"},
		"just fits": {Body{Text: "x", Size: 256 << 10}, ""},
	}
	for name, tt := range tests {
		got := PinWarning(Record{Body: tt.body})
		if (tt.want == "") != (got == "") || !strings.Contains(got, tt.want) {
			t.Errorf("%s: got %q, want it to mention %q", name, got, tt.want)
		}
	}
}
