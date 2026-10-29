// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/internal/vm"
)

// tokens renders spans as "part:text" separated by "|"; key/value spans
// carry their pair index ("key#1:b").
func tokens(url string, known func(string) bool) string {
	var out []string
	for _, s := range vm.TokenizeURL(url, known) {
		label := s.Part.String()
		if s.Pair >= 0 {
			label += "#" + string(rune('0'+s.Pair))
		}
		out = append(out, label+":"+s.Text)
	}
	return strings.Join(out, "|")
}

func everythingKnown(string) bool { return true }

func TestTokenizeURL_should_split_every_part_of_a_full_url(t *testing.T) {
	got := tokens("https://www.example.com:8080/some-url/abc?key1=value1&key2=value2#fragment", everythingKnown)

	want := "scheme:https|separator:://|subdomain:www.|host:example.com|port::8080|path:/some-url/abc|query-mark:?|key#0:key1|equals:=|value#0:value1|query-mark:&|key#1:key2|equals:=|value#1:value2|fragment-mark:#|fragment:fragment"
	if got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
}

func TestTokenizeURL_should_mark_variables_wherever_they_are(t *testing.T) {
	got := tokens("{{env.base_url}}/users/{{id}}/x?k={{v}}", everythingKnown)

	want := "variable:{{env.base_url}}|path:/users/|variable:{{id}}|path:/x|query-mark:?|key#0:k|equals:=|variable#0:{{v}}"
	if got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
}

func TestTokenizeURL_should_mark_unknown_variables(t *testing.T) {
	known := func(name string) bool { return name == "env.base_url" }

	got := tokens("{{env.base_url}}/{{nope}}/{{$no-such}}", known)

	want := "variable:{{env.base_url}}|path:/|unknown-variable:{{nope}}|path:/|unknown-variable:{{$no-such}}"
	if got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
}

func TestTokenizeURL_should_handle_a_host_without_a_scheme(t *testing.T) {
	got := tokens("localhost:3000/api", everythingKnown)

	want := "host:localhost|port::3000|path:/api"
	if got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
}

func TestTokenizeURL_should_treat_a_leading_path_as_a_path(t *testing.T) {
	got := tokens("/api/v1?x=1", everythingKnown)

	want := "path:/api/v1|query-mark:?|key#0:x|equals:=|value#0:1"
	if got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
}

func TestTokenizeURL_should_split_a_fragment_that_holds_pairs_like_a_query(t *testing.T) {
	got := tokens("https://a.test/#a=1&b=2", everythingKnown)

	want := "scheme:https|separator:://|host:a.test|path:/|fragment-mark:#|key#0:a|equals:=|value#0:1|query-mark:&|key#1:b|equals:=|value#1:2"
	if got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
}

func TestTokenizeURL_should_keep_a_key_without_a_value_and_empty_pairs(t *testing.T) {
	got := tokens("https://a.test/?flag&&k=", everythingKnown)

	want := "scheme:https|separator:://|host:a.test|path:/|query-mark:?|key#0:flag|query-mark:&&|key#1:k|equals:="
	if got != want {
		t.Fatalf("\ngot  %s\nwant %s", got, want)
	}
}

func TestTokenizeURL_spans_should_always_join_back_to_the_input(t *testing.T) {
	inputs := []string{"", "{{", "{{}}", "}}{{", "https://", "://x", "a?b#c?d", "{{a}}{{b}}", "https://user:pw@h.test/p", "#", "?", "&=&=", "{{ env.x }}/a", "ünï://çödé/路径?ключ=значение#фраг"}
	for _, in := range inputs {
		var b strings.Builder
		for _, s := range vm.TokenizeURL(in, everythingKnown) {
			b.WriteString(s.Text)
			if s.Text == "" {
				t.Errorf("%q: empty span", in)
			}
		}
		if b.String() != in {
			t.Errorf("%q: spans join to %q", in, b.String())
		}
	}
}

func TestNameChecker_should_know_environment_names_runtime_namespaces_and_generators(t *testing.T) {
	check := vm.NewNameChecker([]string{"base_url", "region"}, []string{"password"})
	tests := map[string]bool{
		"env.base_url": true, "env.nope": false, "secret.password": true, "secret.nope": false,
		"region": true, "email": false, " env.base_url ": true,
		"vars.token": true, "with.x": true, "inputs.a": true, "nodes.n.out": true, "response.body.id": true, "request.url": true, "status": true, "collection_var.region": true,
		"$uuid-v4": true, "$no-such-generator": false, "$int(1, 5)": true,
	}
	for name, want := range tests {
		if got := check(name); got != want {
			t.Errorf("%q: got %v want %v", name, got, want)
		}
	}
}

// Review finding 4: expressions the checker cannot judge must not be red.
func TestNameChecker_should_not_flag_expressions_it_cannot_judge(t *testing.T) {
	check := vm.NewNameChecker([]string{"x"}, nil)
	for _, expr := range []string{`upper(env.x)`, `"literal"`, `1`, `env["x"]`, `env.missing ?? "d"`, `env.x | upper`, `a + b`} {
		if !check(expr) {
			t.Errorf("%q was flagged", expr)
		}
	}
	if check("env.missing") || check("nope") {
		t.Error("plain unknown names must still be flagged")
	}
}
