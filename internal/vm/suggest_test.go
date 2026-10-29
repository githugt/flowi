// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"testing"
)

func suggestSource() SuggestSource {
	return SuggestSource{
		Env:        []string{"base_url", "token", "tenant"},
		Secrets:    []string{"password"},
		Vars:       []string{"id", "page"},
		Generators: []Generator{{Name: "uuid-v4", Detail: "Random UUID"}, {Name: "first-name"}, {Name: "email"}, {Name: "int", Detail: "(min, max)"}},
	}
}

func texts(c Completion) []string {
	var out []string
	for _, it := range c.Items {
		out = append(out, it.Insert)
	}
	return out
}

func TestSuggest_should_filter_generators_by_what_follows_the_dollar(t *testing.T) {
	tests := map[string]struct {
		text string
		want []string
	}{
		"all functions":    {"{{$", []string{"$email", "$first-name", "$int"}},
		"starting at uu":   {"{{$uu", []string{"$uuid-v4"}},
		"no match":         {"{{$zz", nil},
		"after other text": {"http://x/{{$e", []string{"$email"}},
	}
	for name, tc := range tests {
		got := texts(Suggest(tc.text, len(tc.text), suggestSource(), 3))
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v want %v", name, got, tc.want)
		}
	}
}

func TestSuggest_should_offer_variables_after_two_braces_and_keep_namespaces_apart(t *testing.T) {
	tests := map[string]struct {
		text string
		want []string
	}{
		"just braces":      {"{{", []string{"env.base_url", "env.tenant", "env.token"}},
		"env namespace":    {"{{env.t", []string{"env.tenant", "env.token"}},
		"env only":         {"{{env.", []string{"env.base_url", "env.tenant", "env.token"}},
		"request variable": {"{{var.", []string{"var.id", "var.page"}},
		"var prefix":       {"{{var.p", []string{"var.page"}},
		"secrets":          {"{{secret.", []string{"secret.password"}},
		"partial root":     {"{{va", []string{"var.id", "var.page"}},
		"spaces tolerated": {"{{  env.ba", []string{"env.base_url"}},
	}
	for name, tc := range tests {
		got := texts(Suggest(tc.text, len(tc.text), suggestSource(), 3))
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v want %v", name, got, tc.want)
		}
	}
}

func TestSuggest_should_stop_at_the_limit(t *testing.T) {
	got := texts(Suggest("{{", 2, suggestSource(), 2))

	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestSuggest_should_say_nothing_outside_an_open_template(t *testing.T) {
	for name, text := range map[string]string{"plain": "http://x/a", "closed": "{{env.base_url}}/a", "single brace": "{env"} {
		if got := Suggest(text, len(text), suggestSource(), 3); len(got.Items) != 0 {
			t.Errorf("%s: got %v", name, texts(got))
		}
	}
}

func TestSuggest_should_use_the_text_before_the_caret_only(t *testing.T) {
	text := "{{$uu}} and {{env.t}}"

	got := Suggest(text, len("{{$uu"), suggestSource(), 3)

	if !slices.Equal(texts(got), []string{"$uuid-v4"}) {
		t.Fatalf("got %v", texts(got))
	}
}

func TestSuggest_should_offer_the_collections_variables_in_their_own_namespace(t *testing.T) {
	src := suggestSource()
	src.Collection = []string{"region", "api_base"}

	got := texts(Suggest("{{collection_var.", len("{{collection_var."), src, 10))

	if !slices.Equal(got, []string{"collection_var.api_base", "collection_var.region"}) {
		t.Fatalf("got %v", got)
	}
}
