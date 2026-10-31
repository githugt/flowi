// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestURLWithQuery_should_append_the_params_that_have_a_name(t *testing.T) {
	tests := map[string]struct {
		base  string
		query []model.KeyValue
		want  string
	}{
		"none":            {"http://x/a", nil, "http://x/a"},
		"two":             {"http://x/a", []model.KeyValue{{Key: "a", Value: "1"}, {Key: "b", Value: "{{env.x}}"}}, "http://x/a?a=1&b={{env.x}}"},
		"empty row":       {"http://x/a", []model.KeyValue{{Key: "", Value: ""}, {Key: "a", Value: ""}}, "http://x/a?a="},
		"base has a ?":    {"http://x/a?z=9", []model.KeyValue{{Key: "a", Value: "1"}}, "http://x/a?z=9&a=1"},
		"only empty rows": {"http://x/a", []model.KeyValue{{}}, "http://x/a"},
	}
	for name, tc := range tests {
		if got := URLWithQuery(tc.base, tc.query); got != tc.want {
			t.Errorf("%s: got %q want %q", name, got, tc.want)
		}
	}
}

func TestSplitURL_should_move_the_query_string_into_params_and_keep_their_descriptions(t *testing.T) {
	old := []model.KeyValue{{Key: "page", Value: "1", Description: "which page"}}

	base, query := SplitURL("http://x/a?page=2&sort=name&flag&t={{env.a}}?b=1&c={{ x ?? \"&\" }}", old)

	want := []model.KeyValue{{Key: "page", Value: "2", Description: "which page"}, {Key: "sort", Value: "name"}, {Key: "flag"},
		{Key: "t", Value: "{{env.a}}?b=1"}, {Key: "c", Value: `{{ x ?? "&" }}`}}
	if base != "http://x/a" || len(query) != len(want) {
		t.Fatalf("base %q query %+v", base, query)
	}
	for i := range want {
		if query[i] != want[i] {
			t.Errorf("param %d: got %+v want %+v", i, query[i], want[i])
		}
	}
}

func TestSplitURL_Given_no_query_Then_params_are_empty(t *testing.T) {
	base, query := SplitURL("{{env.base_url}}/users", []model.KeyValue{{Key: "a", Value: "1"}})

	if base != "{{env.base_url}}/users" || len(query) != 0 {
		t.Fatalf("base %q query %+v", base, query)
	}
}

// A ? inside {{ }} belongs to the expression, not to the query string.
func TestSplitURL_should_not_split_inside_a_template(t *testing.T) {
	base, query := SplitURL(`http://x/{{ a ? "b" : "c" }}?q=1`, nil)

	if base != `http://x/{{ a ? "b" : "c" }}` || len(query) != 1 || query[0].Key != "q" {
		t.Fatalf("base %q query %+v", base, query)
	}
}
