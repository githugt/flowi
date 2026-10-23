// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"slices"
	"testing"
	"time"
)

func key(t *testing.T, name string) Key {
	t.Helper()
	k, ok := Lookup(Schema(), name)
	if !ok {
		t.Fatalf("no key %s", name)
	}
	return k
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		raw     any
		want    any
		wantErr bool
	}{
		{name: "duration_string", key: "timeout", raw: "5s", want: 5 * time.Second},
		{name: "duration_without_unit", key: "timeout", raw: "30", wantErr: true},
		{name: "bool_true", key: "http.follow_redirects", raw: true, want: true},
		{name: "bool_env_string", key: "http.follow_redirects", raw: "false", want: false},
		{name: "bool_yes_rejected", key: "http.follow_redirects", raw: "yes", wantErr: true},
		{name: "int_yaml", key: "http.max_redirects", raw: int64(5), want: int64(5)},
		{name: "int_env_string", key: "http.max_redirects", raw: "7", want: int64(7)},
		{name: "int_above_max", key: "http.max_redirects", raw: int64(101), wantErr: true},
		{name: "int_below_min", key: "run.max_concurrency", raw: int64(0), wantErr: true},
		{name: "enum_valid", key: "appearance.theme", raw: "dark", want: "dark"},
		{name: "enum_invalid", key: "appearance.theme", raw: "blue", wantErr: true},
		{name: "string", key: "proxy.http", raw: "http://p:3128", want: "http://p:3128"},
		{name: "string_wrong_type", key: "proxy.http", raw: int64(3), wantErr: true},
		{name: "list_yaml", key: "tls.ca_files", raw: []any{"a.pem", "b.pem"}, want: []string{"a.pem", "b.pem"}},
		{name: "list_env_comma", key: "tls.ca_files", raw: "a.pem,b.pem", want: []string{"a.pem", "b.pem"}},
		{name: "list_wrong_item", key: "tls.ca_files", raw: []any{int64(1)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseValue(key(t, tt.key), tt.raw)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if list, ok := tt.want.([]string); ok {
				if !slices.Equal(got.([]string), list) {
					t.Fatalf("got %v, want %v", got, list)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// Lists typed by hand ("5xx, 429") have spaces after the commas; before,
// " 429" was kept as is and failed the enum check.
func TestParseValue_Given_list_with_spaces_Then_items_trimmed(t *testing.T) {
	k, _ := Lookup(Schema(), "retry.on")

	got, err := ParseValue(k, "5xx, 429 ,timeout")

	if err != nil || !slices.Equal(got.([]string), []string{"5xx", "429", "timeout"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}
