// SPDX-License-Identifier: Apache-2.0

package model

import "testing"

func TestParseExpect_should_read_statuses_ranges_classes_and_lists(t *testing.T) {
	tests := []struct {
		in      string
		match   []int
		miss    []int
		written string
	}{
		{"401", []int{401}, []int{200, 400, 402}, "401"},
		{"400-499", []int{400, 450, 499}, []int{399, 500}, "400-499"},
		{"4xx", []int{404}, []int{200, 500}, "400-499"},
		{" 200 , 404-410 ", []int{200, 404, 410}, []int{201, 411}, "200, 404-410"},
	}
	for _, tt := range tests {
		e, err := ParseExpect(tt.in)

		if err != nil || !e.Set() {
			t.Fatalf("%q: %v set=%v", tt.in, err, e.Set())
		}
		for _, s := range tt.match {
			if !e.Matches(s) {
				t.Errorf("%q should match %d", tt.in, s)
			}
		}
		for _, s := range tt.miss {
			if e.Matches(s) {
				t.Errorf("%q should not match %d", tt.in, s)
			}
		}
		if e.String() != tt.written {
			t.Errorf("%q written %q, want %q", tt.in, e.String(), tt.written)
		}
	}
}

func TestParseExpect_should_refuse_what_is_not_a_status(t *testing.T) {
	for _, in := range []string{"ok", "99", "600", "499-400", "6xx", "401,", "4x", "-", "200-"} {
		if _, err := ParseExpect(in); err == nil {
			t.Errorf("%q was accepted", in)
		}
	}
	if e, err := ParseExpect("  "); err != nil || e.Set() {
		t.Errorf("blank is no expectation: %v %v", e, err)
	}
}
