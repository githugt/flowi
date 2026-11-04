// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
)

func TestHopViewsOf_should_say_known_values_as_text_and_leave_unknown_ones_blank(t *testing.T) {
	hops := []flowi.ValueHop{
		{Node: "grab", Role: "made", Value: "t-1", Known: true},
		{Node: "show", Role: "read", Value: map[string]any{"a": 1}, Known: true},
		{Node: "late", Role: "read"},
	}

	got := hopViewsOf(hops)

	if got[0].Value != "t-1" || got[1].Value != `{"a":1}` || got[2].Known || got[2].Value != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestHopViewsOf_should_cut_a_long_value_short(t *testing.T) {
	long := strings.Repeat("é", maxHopText+20)

	got := hopViewsOf([]flowi.ValueHop{{Node: "n", Role: "made", Value: long, Known: true}})

	if r := []rune(got[0].Value); len(r) != maxHopText+1 || !strings.HasSuffix(got[0].Value, "…") {
		t.Fatalf("%d runes: %q", len(r), got[0].Value)
	}
}
