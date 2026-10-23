// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 10, 6, 12, 30, 0, 0, time.UTC) }

func clock() time.Time { return fixedNow() }

func sequence(t *testing.T, seed uint64) []any {
	t.Helper()
	gen := NewGeneration(seed, clock).Func()
	calls := []struct {
		name string
		args []any
	}{
		{"uuid-v4", nil}, {"first-name", nil}, {"last-name", nil}, {"full-name", nil}, {"email", nil},
		{"city-name", nil}, {"phone", nil}, {"word", nil}, {"int", []any{1, 1000}}, {"float", []any{0, 1}}, {"bool", nil},
	}
	var out []any
	for range 2 {
		for _, c := range calls {
			v, err := gen(c.name, c.args)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			out = append(out, v)
		}
	}
	return out
}

func TestGeneration_Given_same_seed_Then_same_sequence(t *testing.T) {
	a := sequence(t, 42)
	b := sequence(t, 42)

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("seed 42 not reproducible:\n%v\n%v", a, b)
	}
}

func TestGeneration_Given_different_seed_Then_different_email(t *testing.T) {
	a, _ := NewGeneration(1, clock).Func()("email", nil)
	b, _ := NewGeneration(2, clock).Func()("email", nil)

	if a == b {
		t.Fatalf("both seeds gave %v", a)
	}
}

func TestGenerator_int_should_respect_inclusive_bounds(t *testing.T) {
	const seed = 7
	gen := NewGeneration(seed, clock).Func()
	seen := map[int]bool{}

	for range 1000 {
		v, err := gen("int", []any{1, 3})
		if err != nil {
			t.Fatal(err)
		}
		n := v.(int)
		if n < 1 || n > 3 {
			t.Fatalf("seed %d: %d out of [1,3]", seed, n)
		}
		seen[n] = true
	}

	if len(seen) != 3 {
		t.Fatalf("seed %d: saw %v, want all of 1..3", seed, seen)
	}
}

func TestGenerator_int_Given_bad_args_Then_error_names_signature(t *testing.T) {
	_, err := NewGeneration(1, clock).Func()("int", []any{"a"})

	if err == nil || !containsAll(err.Error(), "int", "(min, max)") {
		t.Fatalf("got %v", err)
	}
}

func TestGenerator_now_should_use_injected_clock_and_named_layouts(t *testing.T) {
	gen := NewGeneration(1, clock).Func()
	tests := []struct {
		args []any
		want string
	}{
		{nil, "2026-10-06T12:30:00Z"},
		{[]any{"RFC3339"}, "2026-10-06T12:30:00Z"},
		{[]any{"date"}, "2026-10-06"},
		{[]any{"time"}, "12:30:00"},
		{[]any{"2006/01/02"}, "2026/10/06"},
	}
	for _, tt := range tests {
		got, err := gen("now", tt.args)

		if err != nil || got != tt.want {
			t.Errorf("now%v = %v, %v; want %s", tt.args, got, err, tt.want)
		}
	}
}

func TestGenerator_timestamp_should_be_unix_seconds_of_clock(t *testing.T) {
	got, err := NewGeneration(1, clock).Func()("timestamp", nil)

	if err != nil || got != fixedNow().Unix() {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestGenerators_should_have_descriptions_and_unique_names(t *testing.T) {
	seen := map[string]bool{}

	for _, g := range Generators() {
		if seen[g.Name] || g.Description == "" || g.Fn == nil {
			t.Errorf("bad generator %+v", g)
		}
		seen[g.Name] = true
	}

	for _, want := range []string{"uuid-v4", "first-name", "last-name", "full-name", "email", "city-name", "int", "now"} {
		if !seen[want] {
			t.Errorf("missing %s", want)
		}
	}
}

func TestGeneration_Given_unknown_name_Then_UnknownGeneratorError(t *testing.T) {
	_, err := NewGeneration(1, clock).Func()("nope", nil)

	var ge *UnknownGeneratorError
	if !errors.As(err, &ge) || ge.Name != "nope" {
		t.Fatalf("got %v", err)
	}
}

func TestRender_should_use_generation_end_to_end(t *testing.T) {
	s := Scope{Gen: NewGeneration(9, clock).Func()}

	a, err := mustParse(t, "{{$first-name}}-{{$int(5, 5)}}").Render(s)

	if err != nil || fmt.Sprint(a.Value) == "" || !containsAll(a.Value.(string), "-5") {
		t.Fatalf("got %#v, %v", a.Value, err)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
