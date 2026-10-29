// SPDX-License-Identifier: Apache-2.0

package model

import (
	"slices"
	"testing"
)

func TestValidMethod_Given_names_Then_only_http_tokens_are_valid(t *testing.T) {
	tests := map[string]bool{
		"GET": true, "QUERY": true, "PROPFIND": true, "VERSION-CONTROL": true, "X-CUSTOM": true, "purge": true,
		"": false, "GET POST": false, "GET\n": false, "A/B": false, "GÉT": false, "A,B": false, "A(B)": false,
	}
	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			if got := ValidMethod(name); got != want {
				t.Fatalf("ValidMethod(%q) = %v, want %v", name, got, want)
			}
		})
	}
}

func TestMethods_Then_query_is_a_standard_method_and_the_lists_do_not_overlap(t *testing.T) {
	std, more := StandardMethods(), MoreMethods()

	if !slices.Contains(std, "QUERY") || slices.Index(std, "GET") != 0 {
		t.Errorf("standard %v", std)
	}
	for _, m := range more {
		if slices.Contains(std, m) {
			t.Errorf("%s is in both lists", m)
		}
	}
	for _, want := range []string{"PROPFIND", "MKCOL", "REPORT", "SEARCH", "PURGE"} {
		if !slices.Contains(more, want) {
			t.Errorf("more lacks %s: %v", want, more)
		}
	}
}

func TestKnownMethod_Given_the_wrong_case_Then_it_is_known_and_the_right_spelling_is_offered(t *testing.T) {
	right, known := KnownMethod("query")

	if !known || right != "QUERY" {
		t.Fatalf("got %q %v", right, known)
	}
	if _, known := KnownMethod("FROBNICATE"); known {
		t.Error("an unknown name is not known")
	}
}
