// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "testing"

func insertsOf(items []Suggestion) map[string]string {
	out := map[string]string{}
	for _, s := range items {
		out[s.Insert] = s.Detail
	}
	return out
}

func TestExpressionVocabulary_should_offer_the_names_a_message_check_can_read(t *testing.T) {
	// Arrange
	want := []string{"messages", "all_messages", "body", "headers", "received", "outcome"}

	// Act
	got := insertsOf(ExpressionVocabulary(ExprMessageCheck))

	// Assert
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("missing %q in %v", name, got)
		}
	}
}

func TestExpressionVocabulary_should_stop_a_placeholder_name_at_its_dot(t *testing.T) {
	// Arrange / Act
	got := insertsOf(ExpressionVocabulary(ExprFlowCheck))

	// Assert
	for _, want := range []string{"nodes.", "vars.", "inputs.", "env.", "secret."} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for insert := range got {
		if insert == "nodes.<step>.<output>" || insert == "$generator" {
			t.Errorf("a placeholder or generator was offered as it is: %q", insert)
		}
	}
}

func TestExpressionVocabulary_should_offer_functions_with_an_open_bracket_and_the_empty_keyword(t *testing.T) {
	// Arrange / Act
	got := insertsOf(ExpressionVocabulary(ExprRequestCheck))

	// Assert
	for _, want := range []string{"len(", "map(", "upper(", "empty", "contains"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing %q", want)
		}
	}
}

func TestExpressionVocabulary_should_not_repeat_a_name(t *testing.T) {
	// Arrange / Act
	items := ExpressionVocabulary(ExprMessageFilter)

	// Assert
	seen := map[string]bool{}
	for _, s := range items {
		if seen[s.Insert] {
			t.Errorf("%q is offered twice", s.Insert)
		}
		seen[s.Insert] = true
	}
}

func TestExpressionVocabulary_should_offer_the_names_a_dataset_check_can_read(t *testing.T) {
	want := []string{"rows", "columns", "row_count", "truncated", "outcome", "body"}

	got := insertsOf(ExpressionVocabulary(ExprDatasetCheck))

	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("missing %q in %v", name, got)
		}
	}
	for _, not := range []string{"status", "messages", "received"} {
		if _, ok := got[not]; ok {
			t.Errorf("%q is not defined for a dataset check", not)
		}
	}
}
