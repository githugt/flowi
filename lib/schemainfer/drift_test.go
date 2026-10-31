// SPDX-License-Identifier: Apache-2.0

package schemainfer

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func sampleOf(t *testing.T, text string) any {
	t.Helper()
	d := json.NewDecoder(strings.NewReader(text))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func driftSummary(ds []Difference) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = string(d.Kind) + " " + d.Path
	}
	return out
}

func TestDrift_Given_a_response_like_the_one_the_schema_was_learned_from_Then_there_is_none(t *testing.T) {
	// Arrange
	schema := Infer(sampleOf(t, `{"id": 7, "name": "rex", "owner": {"handle": "ada"}, "tags": ["a"]}`))

	// Act
	got := Drift(schema, sampleOf(t, `{"id": 8, "name": "fido", "owner": {"handle": "bob"}, "tags": ["b", "c"]}`))

	// Assert
	if len(got) != 0 {
		t.Fatalf("drift %v", driftSummary(got))
	}
}

func TestDrift_Given_a_field_the_schema_does_not_know_Then_it_is_new_at_its_path(t *testing.T) {
	// Arrange
	schema := Infer(sampleOf(t, `{"id": 7, "owner": {"handle": "ada"}}`))

	// Act
	got := Drift(schema, sampleOf(t, `{"id": 7, "owner": {"handle": "ada", "email": "a@x.test"}, "region": "eu"}`))

	// Assert
	want := []string{"new $.owner.email", "new $.region"}
	if !slices.Equal(driftSummary(got), want) {
		t.Fatalf("drift %v, want %v", driftSummary(got), want)
	}
}

func TestDrift_Given_a_required_field_that_is_gone_Then_it_is_missing(t *testing.T) {
	// Arrange
	schema := Infer(sampleOf(t, `{"id": 7, "owner": {"handle": "ada"}}`))

	// Act
	got := Drift(schema, sampleOf(t, `{"owner": {}}`))

	// Assert
	want := []string{"missing $.id", "missing $.owner.handle"}
	if !slices.Equal(driftSummary(got), want) {
		t.Fatalf("drift %v, want %v", driftSummary(got), want)
	}
}

func TestDrift_Given_a_field_that_changed_type_Then_it_is_changed_and_says_from_what_to_what(t *testing.T) {
	// Arrange
	schema := Infer(sampleOf(t, `{"id": 7, "tags": ["a"]}`))

	// Act
	got := Drift(schema, sampleOf(t, `{"id": "seven", "tags": [1]}`))

	// Assert
	if !slices.Equal(driftSummary(got), []string{"changed $.id", "changed $.tags[*]"}) {
		t.Fatalf("drift %v", driftSummary(got))
	}
	if got[0].Detail != "was integer, now string" {
		t.Errorf("detail %q", got[0].Detail)
	}
}

func TestDrift_Given_an_integer_where_the_schema_says_number_Then_it_is_not_drift(t *testing.T) {
	// Arrange
	schema := Infer(sampleOf(t, `{"price": 1.5}`))

	// Act
	got := Drift(schema, sampleOf(t, `{"price": 2}`))

	// Assert
	if len(got) != 0 {
		t.Fatalf("an integer is a number: %v", driftSummary(got))
	}
}

func TestDrift_Given_a_nullable_field_and_an_optional_one_Then_null_and_absence_are_not_drift(t *testing.T) {
	// Arrange: a schema widened over two samples: "note" came and went, "next" was null and a string.
	schema, _ := Merge(Infer(sampleOf(t, `{"id": 1, "note": "x", "next": null}`)), sampleOf(t, `{"id": 2, "next": "u"}`))

	// Act
	got := Drift(schema, sampleOf(t, `{"id": 3, "next": null}`))

	// Assert
	if len(got) != 0 {
		t.Fatalf("drift %v", driftSummary(got))
	}
}

func TestDrift_Given_a_body_that_is_not_the_shape_the_schema_describes_Then_the_whole_body_changed(t *testing.T) {
	// Arrange
	schema := Infer(sampleOf(t, `{"id": 1}`))

	// Act
	got := Drift(schema, sampleOf(t, `["a"]`))

	// Assert
	if !slices.Equal(driftSummary(got), []string{"changed $"}) {
		t.Fatalf("drift %v", driftSummary(got))
	}
}
