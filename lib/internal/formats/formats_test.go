// SPDX-License-Identifier: Apache-2.0

package formats

import (
	"slices"
	"testing"
)

func TestLookup_should_know_the_registered_formats_and_not_others(t *testing.T) {
	tests := []struct {
		name       string
		wantKnown  bool
		wantTypes  []string
		deprecated bool
	}{
		{name: "date-time", wantKnown: true, wantTypes: []string{"string"}},
		{name: "int64", wantKnown: true, wantTypes: []string{"number", "string"}},
		{name: "byte", wantKnown: true, wantTypes: []string{"string"}, deprecated: true},
		{name: "uuid", wantKnown: true, wantTypes: []string{"string"}},
		{name: "phone-number", wantKnown: false},
		{name: "DATE", wantKnown: false}, // names are case-sensitive
		{name: "", wantKnown: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known := Lookup(tt.name)

			if known != tt.wantKnown || (known && (!slices.Equal(got.Types, tt.wantTypes) || got.Deprecated != tt.deprecated || got.Meaning == "")) {
				t.Fatalf("known %v got %+v", known, got)
			}
		})
	}
}

func TestInvalid_should_give_a_value_for_each_format_that_has_a_clear_wrong_one_and_only_for_registered_names(t *testing.T) {
	for _, name := range InvalidSamples() {
		if _, known := Lookup(name); !known {
			t.Errorf("%q has a bad sample but is not registered", name)
		}
		if sample, ok := Invalid(name); !ok || sample == "" {
			t.Errorf("%q: no sample", name)
		}
	}
	if _, ok := Invalid("phone-number"); ok {
		t.Error("an unregistered format has no sample")
	}
	for _, name := range []string{"date-time", "date", "email", "uuid", "uri", "ipv4", "ipv6", "hostname", "time", "duration"} {
		if _, ok := Invalid(name); !ok {
			t.Errorf("%q should have a sample", name)
		}
	}
}

func TestNames_should_list_every_registered_format_once_sorted(t *testing.T) {
	names := Names()

	if len(names) < 50 || !slices.IsSorted(names) || len(slices.Compact(slices.Clone(names))) != len(names) {
		t.Fatalf("%d names, sorted %v", len(names), slices.IsSorted(names))
	}
}
