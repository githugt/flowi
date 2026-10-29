// SPDX-License-Identifier: Apache-2.0

package fuzzcases

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func body() map[string]any {
	return map[string]any{"name": "rex", "age": json.Number("3"), "tags": []any{"a"}, "owner": map[string]any{"email": "a@x.test"}, "vaccinated": true}
}

func names(cs []Case) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func TestCases_Given_a_valid_body_Then_every_case_differs_from_it_and_has_a_name_that_says_how(t *testing.T) {
	// Act
	cs := Cases(body(), nil, 500)

	// Assert
	if len(cs) < 20 {
		t.Fatalf("only %d cases: %v", len(cs), names(cs))
	}
	for _, c := range cs {
		if c.Name == "" || reflect.DeepEqual(c.Body, body()) {
			t.Errorf("case %q does not change the body", c.Name)
		}
	}
}

func TestCases_Given_the_same_input_Then_the_same_cases_come_in_the_same_order(t *testing.T) {
	// Act
	first, second := names(Cases(body(), nil, 500)), names(Cases(body(), nil, 500))

	// Assert
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("the order must be stable so a failing case can be found again:\n%v\n%v", first, second)
	}
}

func TestCases_Given_a_limit_Then_the_first_cases_cover_different_fields_not_just_the_first_one(t *testing.T) {
	// Act
	cs := Cases(body(), nil, 8)

	// Assert
	if len(cs) != 8 {
		t.Fatalf("got %d", len(cs))
	}
	fields := map[string]bool{}
	for _, c := range cs {
		fields[strings.SplitN(c.Name, ":", 2)[0]] = true
	}
	if len(fields) < 5 {
		t.Fatalf("eight cases should touch at least five of the five fields' paths; touched %v from %v", fields, names(cs))
	}
}

func TestCases_Given_each_kind_of_field_Then_the_expected_mutations_are_there(t *testing.T) {
	// Act
	got := strings.Join(names(Cases(body(), nil, 500)), "\n")

	// Assert
	for _, want := range []string{
		"name: null", "name: a number instead of text", "name: empty text", "name: very long text", "name: SQL quote", "name: markup", "name: path traversal", "name: header injection",
		"age: text instead of a number", "age: zero", "age: negative", "age: past 32 bits",
		"tags: an object instead of a list", "tags: empty list",
		"owner.email: null", "vaccinated: text instead of true or false",
		"name: removed", "owner.email: removed", "an unknown field",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no case %q in:\n%s", want, got)
		}
	}
}

func TestCases_Given_a_schema_with_bounds_Then_one_case_goes_just_past_each(t *testing.T) {
	// Arrange
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"age":   map[string]any{"type": "integer", "minimum": 0.0, "maximum": 30.0},
		"name":  map[string]any{"type": "string", "maxLength": 10.0},
		"state": map[string]any{"type": "string", "enum": []any{"new", "old"}},
	}}
	b := map[string]any{"age": json.Number("5"), "name": "rex", "state": "new"}

	// Act
	cs := Cases(b, schema, 500)

	// Assert
	got := map[string]any{}
	for _, c := range cs {
		got[c.Name] = c.Body
	}
	if m, _ := got["age: below the minimum"].(map[string]any); m["age"] != json.Number("-1") {
		t.Errorf("below the minimum: %v", got["age: below the minimum"])
	}
	if m, _ := got["age: above the maximum"].(map[string]any); m["age"] != json.Number("31") {
		t.Errorf("above the maximum: %v", got["age: above the maximum"])
	}
	if m, _ := got["name: longer than the maximum"].(map[string]any); len(m["name"].(string)) != 11 {
		t.Errorf("longer than the maximum: %v", got["name: longer than the maximum"])
	}
	if _, ok := got["state: not one of the allowed values"]; !ok {
		t.Errorf("no enum case in %v", names(cs))
	}
}

func TestCases_Given_a_body_that_is_not_an_object_or_list_Then_it_still_gets_cases_for_the_value(t *testing.T) {
	// Act
	cs := Cases("hello", nil, 500)

	// Assert
	if len(cs) == 0 {
		t.Fatal("a plain value should still be mutated")
	}
}
