// SPDX-License-Identifier: Apache-2.0

package template

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func scope(values map[string]any) Scope {
	return Scope{Values: values, Bare: []string{"with", "vars", "inputs", "env"}}
}

func mustParse(t *testing.T, src string) Template {
	t.Helper()
	tpl, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return tpl
}

func TestParse_should_split_text_and_expressions(t *testing.T) {
	tests := []struct {
		src    string
		static bool
		refs   []string
	}{
		{"plain text", true, nil},
		{"{{env.base_url}}/v1", false, []string{"env.base_url"}},
		{"{{a.b}}-{{c}}", false, []string{"a.b", "c"}},
		{"{{a}}{{b}}", false, []string{"a", "b"}},
		{`{{ x ?? "}}" }}`, false, []string{"x"}},
		{`{"a":{"b":1}}`, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			tpl := mustParse(t, tt.src)

			if tpl.IsStatic() != tt.static {
				t.Fatalf("IsStatic = %v", tpl.IsStatic())
			}
			if !reflect.DeepEqual(tpl.References(), tt.refs) {
				t.Fatalf("References = %v, want %v", tpl.References(), tt.refs)
			}
		})
	}
}

func TestParse_Given_unbalanced_braces_Then_SyntaxError_with_offset(t *testing.T) {
	_, err := Parse("abc {{ env.x ")

	var se *SyntaxError
	if !errors.As(err, &se) || se.Offset != 4 {
		t.Fatalf("got %v", err)
	}
}

func TestParse_Given_bad_expression_Then_SyntaxError(t *testing.T) {
	_, err := Parse("{{ a + }}")

	var se *SyntaxError
	if !errors.As(err, &se) || se.Detail == "" {
		t.Fatalf("got %v", err)
	}
}

func TestEval_Given_single_expression_Then_keeps_type(t *testing.T) {
	s := scope(map[string]any{"vars": map[string]any{"n": 3, "ok": true, "m": map[string]any{"k": "v"}}})
	tests := map[string]any{
		"{{vars.n}}":  3,
		"{{vars.ok}}": true,
		"{{vars.m}}":  map[string]any{"k": "v"},
	}
	for src, want := range tests {
		got, err := mustParse(t, src).Eval(s)

		if err != nil || !reflect.DeepEqual(got.Value, want) {
			t.Errorf("%s: got %#v, %v", src, got.Value, err)
		}
	}
}

func TestRender_Given_undefined_namespaced_ref_Then_UndefinedError_names_full_path(t *testing.T) {
	s := scope(map[string]any{"nodes": map[string]any{}})

	_, err := mustParse(t, "Bearer {{nodes.login.token}}").Render(s)

	var ue *UndefinedError
	if !errors.As(err, &ue) || ue.Name != "nodes.login.token" {
		t.Fatalf("got %v", err)
	}
}

func TestRender_Given_several_undefined_Then_all_reported(t *testing.T) {
	_, err := mustParse(t, "{{secret.a}}:{{secret.b}}").Render(scope(map[string]any{"secret": map[string]any{}}))

	if got := Undefined(err); !reflect.DeepEqual(got, []string{"secret.a", "secret.b"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRender_Given_bare_name_Then_resolves_with_before_vars_before_env(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]any
		want   string
	}{
		{"with wins", map[string]any{"with": map[string]any{"x": "w"}, "vars": map[string]any{"x": "v"}, "env": map[string]any{"x": "e"}}, "w"},
		{"vars next", map[string]any{"vars": map[string]any{"x": "v"}, "env": map[string]any{"x": "e"}}, "v"},
		{"env last", map[string]any{"env": map[string]any{"x": "e"}}, "e"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mustParse(t, "{{x}}").Render(scope(tt.values))

			if err != nil || got.Value != tt.want {
				t.Fatalf("got %#v, %v", got.Value, err)
			}
		})
	}
}

func TestRender_Given_undefined_bare_name_Then_UndefinedError_names_it(t *testing.T) {
	_, err := mustParse(t, "{{email}}").Render(scope(map[string]any{}))

	if got := Undefined(err); !reflect.DeepEqual(got, []string{"email"}) {
		t.Fatalf("got %v (%v)", got, err)
	}
}

func TestRender_Given_fallback_operator_Then_no_error(t *testing.T) {
	s := scope(map[string]any{"vars": map[string]any{}})
	for _, src := range []string{`{{vars.x ?? "d"}}`, `{{vars?.x?.y ?? "d"}}`} {
		got, err := mustParse(t, src).Render(s)

		if err != nil || got.Value != "d" {
			t.Errorf("%s: got %#v, %v", src, got.Value, err)
		}
	}
}

func TestRender_Given_lenient_scope_Then_undefined_is_nil(t *testing.T) {
	s := scope(map[string]any{})
	s.Lenient = true

	got, err := mustParse(t, "[{{vars.missing}}]").Render(s)

	if err != nil || got.Value != "[]" {
		t.Fatalf("got %#v, %v", got.Value, err)
	}
}

func TestRender_Given_secret_ref_Then_result_tainted(t *testing.T) {
	s := scope(map[string]any{"secret": map[string]any{"pw": "hunter2"}, "env": map[string]any{"u": "me"}})

	tainted, err := mustParse(t, "{{env.u}}:{{secret.pw}}").Render(s)
	clean, _ := mustParse(t, "{{env.u}}").Render(s)

	if err != nil || !tainted.Tainted || tainted.Value != "me:hunter2" {
		t.Fatalf("got %+v, %v", tainted, err)
	}
	if clean.Tainted {
		t.Fatal("clean render marked tainted")
	}
}

func TestRender_should_read_lazy_namespace(t *testing.T) {
	ns := lookupFunc(func(k string) (any, bool) { return "v-" + k, k != "missing" })
	s := scope(map[string]any{"secret": ns})

	got, err := mustParse(t, "{{secret.a}}").Render(s)
	_, missErr := mustParse(t, "{{secret.missing}}").Render(s)

	if err != nil || got.Value != "v-a" {
		t.Fatalf("got %#v, %v", got.Value, err)
	}
	if !reflect.DeepEqual(Undefined(missErr), []string{"secret.missing"}) {
		t.Fatalf("got %v", missErr)
	}
}

type lookupFunc func(string) (any, bool)

func (f lookupFunc) Lookup(k string) (any, bool) { return f(k) }

func TestCheck_should_compare_against_empty(t *testing.T) {
	tests := []struct {
		value any
		empty bool
	}{
		{nil, true}, {"", true}, {[]any{}, true}, {map[string]any{}, true}, {"x", false}, {0, false},
	}
	for _, tt := range tests {
		s := scope(map[string]any{"vars": map[string]any{"v": tt.value}})

		isEmpty, _, err1 := Check("vars.v == empty", s)
		notEmpty, _, err2 := Check("vars.v != empty", s)

		if err1 != nil || err2 != nil || isEmpty != tt.empty || notEmpty == tt.empty {
			t.Errorf("%#v: == %v, != %v, errs %v %v", tt.value, isEmpty, notEmpty, err1, err2)
		}
	}
}

func TestCheck_should_accept_duration_literals(t *testing.T) {
	s := scope(map[string]any{"response": map[string]any{"time": 500 * time.Millisecond}})

	fast, _, err := Check("response.time < 800ms", s)
	slow, _, err2 := Check("response.time > 1m30s", s)
	str, _, err3 := Check(`"800ms" == "800" + "ms"`, s)

	if err != nil || err2 != nil || err3 != nil || !fast || slow || !str {
		t.Fatalf("fast=%v slow=%v str=%v errs %v %v %v", fast, slow, str, err, err2, err3)
	}
}

func TestCheck_Given_non_boolean_Then_error(t *testing.T) {
	_, _, err := Check(`"x"`, scope(nil))

	if err == nil {
		t.Fatal("want error")
	}
}

func TestRender_should_call_generator_with_hyphenated_name_and_args(t *testing.T) {
	var calls []string
	s := scope(nil)
	s.Gen = func(name string, args []any) (any, error) {
		calls = append(calls, name+":"+strings.Trim(strings.ReplaceAll(strings.TrimSpace(strings.Join(toStrings(args), ",")), " ", ""), ","))
		return "g", nil
	}

	got, err := mustParse(t, `{{$uuid-v4}} {{$int(1, 100)}} {{$now()}} {{"$not-a-gen"}}`).Render(s)

	if err != nil || got.Value != "g g g $not-a-gen" {
		t.Fatalf("got %#v, %v", got.Value, err)
	}
	if !reflect.DeepEqual(calls, []string{"uuid-v4:", "int:1,100", "now:"}) {
		t.Fatalf("calls %v", calls)
	}
}

func toStrings(args []any) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = stringify(a)
	}
	return out
}

func TestRender_Given_no_generator_func_Then_UnknownGeneratorError(t *testing.T) {
	_, err := mustParse(t, "{{$email}}").Render(scope(nil))

	var ge *UnknownGeneratorError
	if !errors.As(err, &ge) || ge.Name != "email" {
		t.Fatalf("got %v", err)
	}
}

func TestRenderJSON_should_keep_valid_json_for_hostile_values(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		value any
		want  any
	}{
		{"quote", `{"a":"{{vars.v}}"}`, `say "hi"`, map[string]any{"a": `say "hi"`}},
		{"backslash", `{"a":"{{vars.v}}"}`, `C:\x`, map[string]any{"a": `C:\x`}},
		{"newline", `{"a":"{{vars.v}}"}`, "a\nb", map[string]any{"a": "a\nb"}},
		{"script", `{"a":"{{vars.v}}"}`, "</script>", map[string]any{"a": "</script>"}},
		{"emoji", `{"a":"{{vars.v}}"}`, "🙂", map[string]any{"a": "🙂"}},
		{"nul", `{"a":"{{vars.v}}"}`, "a\x00b", map[string]any{"a": "a\x00b"}},
		{"nil whole literal", `{"a":"{{vars.v}}"}`, nil, map[string]any{"a": nil}},
		{"number in quotes", `{"a":"{{vars.v}}"}`, 7, map[string]any{"a": "7"}},
		{"number bare", `{"a":{{vars.v}}}`, 7, map[string]any{"a": float64(7)}},
		{"map bare", `{"a":{{vars.v}}}`, map[string]any{"k": []any{"x"}}, map[string]any{"a": map[string]any{"k": []any{"x"}}}},
		{"inside larger string", `{"a":"Bearer {{vars.v}}!"}`, `t"k`, map[string]any{"a": `Bearer t"k!`}},
		{"quote in text before", `{"q":"\"","a":"{{vars.v}}"}`, "x", map[string]any{"q": `"`, "a": "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scope(map[string]any{"vars": map[string]any{"v": tt.value}})

			got, err := RenderJSON(tt.src, s)

			if err != nil {
				t.Fatal(err)
			}
			text := got.Value.(string)
			var decoded any
			if !json.Valid([]byte(text)) || json.Unmarshal([]byte(text), &decoded) != nil {
				t.Fatalf("invalid JSON %q", text)
			}
			if !reflect.DeepEqual(decoded, tt.want) {
				t.Fatalf("decoded %#v, want %#v", decoded, tt.want)
			}
		})
	}
}

func TestApplyEmpty(t *testing.T) {
	tests := []struct {
		policy  EmptyPolicy
		value   any
		want    string
		keep    bool
		wantErr bool
	}{
		{EmptyKeep, nil, "", true, false},
		{EmptyKeep, "", "", true, false},
		{EmptyOmit, nil, "", false, false},
		{EmptyOmit, "", "", false, false},
		{EmptyError, nil, "", false, true},
		{EmptyError, "", "", false, true},
		{EmptyOmit, "x", "x", true, false},
		{EmptyError, 5, "5", true, false},
	}
	for _, tt := range tests {
		got, keep, err := ApplyEmpty(tt.value, tt.policy)

		if got != tt.want || keep != tt.keep || (err != nil) != tt.wantErr {
			t.Errorf("%v/%#v: got %q %v %v", tt.policy, tt.value, got, keep, err)
		}
	}
}

// Before: secret.some-secret read as secret.some minus secret, failing as
// "undefined secret.some".
func TestRender_should_read_names_with_hyphens_after_a_namespace(t *testing.T) {
	scope := Scope{Values: map[string]any{
		"secret": map[string]any{"some-secret": "s3", "count": 5},
		"env":    map[string]any{"base-url": "http://x", "count": 5, "count-1": "one"},
	}}

	r, err := mustParse(t, "{{secret.some-secret}} {{env.base-url}}/a {{env.count-1}} {{ env.count - 1 }}").Render(scope)

	if got := stringify(r.Value); err != nil || got != "s3 http://x/a one 4" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestReferences_should_name_a_hyphenated_variable_whole(t *testing.T) {
	refs, err := References("{{secret.some-secret}}", false)

	if err != nil || len(refs) != 1 || len(refs[0].Path) != 2 || refs[0].Path[1] != "some-secret" {
		t.Fatalf("refs %+v err %v", refs, err)
	}
}
