// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import "testing"

func TestExplain_should_tell_the_open_flow_in_words_with_the_step_each_line_is_about(t *testing.T) {
	flows, v, _ := openTwoUnits(t)

	got := flows.Explain(v.Node)

	if len(got.Sentences) == 0 {
		t.Fatalf("explanation %+v", got)
	}
	for _, s := range got.Sentences {
		if s.Text == "" {
			t.Fatalf("an empty sentence in %+v", got.Sentences)
		}
	}
	if got.Problems == nil {
		t.Fatal("problems must be empty, not null, for the frontend")
	}
}

func TestExplain_Given_an_unknown_flow_Then_empty_lists_not_null(t *testing.T) {
	flows, _, _ := openTwoUnits(t)

	got := flows.Explain("no-such-flow")

	if got.Sentences == nil || got.Problems == nil || len(got.Sentences) != 0 {
		t.Fatalf("got %+v", got)
	}
}
