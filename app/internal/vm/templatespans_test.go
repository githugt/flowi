// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "testing"

func TestTemplateSpans_should_mark_known_and_unknown_variables_and_join_back_to_the_text(t *testing.T) {
	known := func(e string) bool { return e == "env.token" }
	text := "Bearer {{env.token}} and {{secret.x}} {{ broken"

	spans := TemplateSpans(text, known)

	joined := ""
	var parts []URLPart
	for _, s := range spans {
		joined += s.Text
		parts = append(parts, s.Part)
	}
	want := []URLPart{PartText, PartVariable, PartText, PartUnknownVariable, PartText}
	if joined != text || len(parts) != len(want) {
		t.Fatalf("joined %q parts %v", joined, parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Fatalf("parts %v want %v", parts, want)
		}
	}
}

func TestTemplateSpans_Given_no_checker_Then_one_plain_span(t *testing.T) {
	spans := TemplateSpans("a {{b}}", nil)

	if len(spans) != 1 || spans[0].Part != PartText {
		t.Fatalf("spans %+v", spans)
	}
}
