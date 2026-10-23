// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"reflect"
	"testing"
)

func TestSchema_should_have_unique_names_and_valid_defaults(t *testing.T) {
	seen := map[string]bool{}

	for _, k := range Schema() {
		if seen[k.Name] {
			t.Fatalf("duplicate key %s", k.Name)
		}
		seen[k.Name] = true
		if _, err := ParseValue(k, k.Default); err != nil {
			t.Errorf("default of %s invalid: %v", k.Name, err)
		}
		if k.Description == "" {
			t.Errorf("%s has no description", k.Name)
		}
	}

	if len(seen) != 74 {
		t.Fatalf("got %d keys, want 74", len(seen))
	}
}

func TestLookup_should_find_key_by_name(t *testing.T) {
	k, ok := Lookup(Schema(), "http.max_redirects")

	if !ok || k.Kind != KindInt {
		t.Fatalf("got %+v, %v", k, ok)
	}
}

func TestSchema_should_include_retry_hooks_and_request_compression_keys_with_defaults(t *testing.T) {
	want := map[string]any{
		"http.request_compression": "none",
		"retry.max":                int64(0),
		"retry.backoff":            "exponential",
		"retry.jitter":             true,
		"retry.on":                 []any{"5xx", "timeout", "connection"},
		"hooks.nested":             false,
		"hooks.max_depth":          int64(3),
	}

	for name, def := range want {
		k, ok := Lookup(Schema(), name)

		if !ok {
			t.Errorf("%s missing", name)
			continue
		}
		if !reflect.DeepEqual(k.Default, def) {
			t.Errorf("%s default %#v, want %#v", name, k.Default, def)
		}
	}
}

func TestParseValue_Given_string_list_with_enum_When_element_not_allowed_Then_error(t *testing.T) {
	k, _ := Lookup(Schema(), "retry.on")

	_, errBad := ParseValue(k, []any{"5xx", "teapot"})
	got, errOK := ParseValue(k, "429,timeout")

	if errBad == nil {
		t.Fatal("want error for unknown element")
	}
	if errOK != nil || !reflect.DeepEqual(got, []string{"429", "timeout"}) {
		t.Fatalf("got %v, %v", got, errOK)
	}
}
