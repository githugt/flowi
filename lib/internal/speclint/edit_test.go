// SPDX-License-Identifier: Apache-2.0

package speclint

import "testing"

func TestEdit_Given_a_commented_file_When_a_rule_is_set_Then_every_comment_stays(t *testing.T) {
	old := "# our house rules\n\nrules:\n  # paths are lowercase here\n  path-case: error   # since the 2.0 gateway\n  operation-tags: off\n\n# end\n"
	cases := []struct{ name, rule, level, want string }{
		{"change keeps the trailing comment", "path-case", "warning",
			"# our house rules\n\nrules:\n  # paths are lowercase here\n  path-case: warning   # since the 2.0 gateway\n  operation-tags: off\n\n# end\n"},
		{"add goes after the last entry", "operation-id", "error",
			"# our house rules\n\nrules:\n  # paths are lowercase here\n  path-case: error   # since the 2.0 gateway\n  operation-tags: off\n  operation-id: error\n\n# end\n"},
		{"forget drops the line only", "operation-tags", "",
			"# our house rules\n\nrules:\n  # paths are lowercase here\n  path-case: error   # since the 2.0 gateway\n\n# end\n"},
		{"the rule's own level counts as forgetting it", "operation-tags", "note",
			"# our house rules\n\nrules:\n  # paths are lowercase here\n  path-case: error   # since the 2.0 gateway\n\n# end\n"},
	}
	for _, c := range cases {
		if got := string(Edit([]byte(old), c.rule, c.level)); got != c.want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestEdit_Given_the_only_entry_When_it_is_forgotten_Then_the_rules_are_an_empty_mapping(t *testing.T) {
	old := "# why\nrules:  # the list\n  path-case: error\n"

	got := string(Edit([]byte(old), "path-case", ""))

	if want := "# why\nrules: {} # the list\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEdit_Given_an_empty_or_missing_rules_key_When_a_rule_is_set_Then_it_is_added_under_the_comments(t *testing.T) {
	if got, want := string(Edit([]byte("# note\nrules: {}\n"), "path-case", "error")), "# note\nrules:\n  path-case: error\n"; got != want {
		t.Errorf("empty mapping: got %q want %q", got, want)
	}
	if got, want := string(Edit([]byte("# only a note\n"), "path-case", "error")), "# only a note\nrules:\n  path-case: error\n"; got != want {
		t.Errorf("no key: got %q want %q", got, want)
	}
	if got, want := string(Edit(nil, "path-case", "error")), "# How seriously each rule about the API description is taken: off, note, warning or error.\nrules:\n  path-case: error\n"; got != want {
		t.Errorf("no file: got %q want %q", got, want)
	}
}

// Before: the lint file was written whole from the rules, so every comment in it was lost on the first change.
func TestEdit_Given_a_flow_style_mapping_When_a_rule_is_set_Then_it_is_rewritten_keeping_the_header(t *testing.T) {
	old := "# header\nrules: {operation-tags: off}\n"

	got := string(Edit([]byte(old), "path-case", "error"))

	if want := "# header\nrules:\n  operation-tags: off\n  path-case: error\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEdit_Given_a_quoted_name_When_it_is_changed_Then_it_is_found(t *testing.T) {
	got := string(Edit([]byte("rules:\n  'path-case': off\n"), "path-case", "error"))

	if want := "rules:\n  'path-case': error\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
