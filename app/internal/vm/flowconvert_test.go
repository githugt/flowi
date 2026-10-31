// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func TestStepCatalog_Then_it_lists_ten_entries_in_five_groups(t *testing.T) {
	// Act
	entries := StepCatalog()

	// Assert
	groups := map[string]bool{}
	for _, e := range entries {
		groups[e.Group] = true
		if e.Name == "" || e.Description == "" || e.Type == "" {
			t.Errorf("entry %+v is missing its name, description or type", e)
		}
	}
	if len(entries) != 10 || len(groups) != 5 {
		t.Fatalf("%d entries in %d groups: %+v", len(entries), len(groups), entries)
	}
}

func TestStepCatalog_Then_every_entry_makes_a_node_type_the_library_knows(t *testing.T) {
	// Arrange
	known := model.NodeTypes()

	// Act
	entries := StepCatalog()

	// Assert
	for _, e := range entries {
		if !slices.Contains(known, model.NodeType(e.Type)) {
			t.Errorf("entry %q makes %q, which is not a node type", e.Name, e.Type)
		}
	}
}

func TestSearchSteps_Given_a_word_from_a_description_Then_the_entry_is_found(t *testing.T) {
	// Act
	byName := SearchSteps("rout")
	byDescription := SearchSteps("repeat")
	none := SearchSteps("zzzz")

	// Assert
	if len(byName) != 1 || byName[0].Name != "Route" {
		t.Errorf("by name: %+v", byName)
	}
	if len(byDescription) == 0 || byDescription[0].Name != "Loop" {
		t.Errorf("by description: %+v", byDescription)
	}
	if len(none) != 0 {
		t.Errorf("no match should find nothing: %+v", none)
	}
	if len(SearchSteps("  ")) != 10 {
		t.Errorf("a blank search should list every entry")
	}
}

func TestConvertStep_Given_request_to_url_mode_Then_the_saved_reference_is_kept_as_a_note_and_the_lost_fields_are_named(t *testing.T) {
	// Arrange
	n := model.Node{
		ID: "login", Type: "request", Name: "Log in", Color: "teal", Needs: []string{"seed"},
		Request: &model.RequestCall{
			Request: model.Ref{Collection: "identity-api", Path: "auth/login"},
			Version: "1.1",
			Headers: []model.KeyValue{{Key: "X-Trace", Value: "{{vars.trace}}"}},
			With:    []model.KeyValue{{Key: "user", Value: "{{vars.user}}"}},
		},
	}

	// Act
	got, lost := ConvertStep(n, StepEntry{Type: "request", Mode: "url"})

	// Assert
	if got.Type != "fetch" || got.Request != nil {
		t.Fatalf("type %q request %+v", got.Type, got.Request)
	}
	if got.ID != "login" || got.Name != "Log in" || got.Color != "teal" || !slices.Equal(got.Needs, []string{"seed"}) {
		t.Errorf("identity not kept: %+v", got)
	}
	if !strings.Contains(got.Description, "identity-api") || !strings.Contains(got.Description, "auth/login") {
		t.Errorf("the saved reference should be kept as a note, got description %q", got.Description)
	}
	headers, _ := ParamValue(got, "headers").([]model.Field)
	if len(headers) != 1 || headers[0].Key != "X-Trace" {
		t.Errorf("headers should carry over: %+v", got.Params)
	}
	for _, want := range []string{"request", "version", "with"} {
		if !slices.Contains(lost, want) {
			t.Errorf("%q should be named as lost, got %v", want, lost)
		}
	}
	if slices.Contains(lost, "headers") {
		t.Errorf("headers carried over and must not be named as lost: %v", lost)
	}
}

func TestConvertStep_Given_loop_over_to_until_Then_fields_that_carry_over_are_kept(t *testing.T) {
	// Arrange
	body := []model.Node{{ID: "poll", Type: "request"}}
	n := model.Node{
		ID: "each", Type: "batch", Body: body,
		Params: []model.Field{{Key: "over", Value: "{{vars.ids}}"}, {Key: "concurrency", Value: int64(3)}},
	}

	// Act
	got, lost := ConvertStep(n, StepEntry{Type: "batch", Mode: "until"})

	// Assert
	if got.Type != "run_till" || len(got.Body) != 1 || got.Body[0].ID != "poll" {
		t.Fatalf("type %q body %+v", got.Type, got.Body)
	}
	if ParamValue(got, "over") != nil || ParamValue(got, "concurrency") != nil {
		t.Errorf("fields the new mode does not use should be gone: %+v", got.Params)
	}
	for _, want := range []string{"over", "concurrency"} {
		if !slices.Contains(lost, want) {
			t.Errorf("%q should be named as lost, got %v", want, lost)
		}
	}
}

func TestConvertStep_Given_the_same_kind_Then_nothing_changes_and_nothing_is_lost(t *testing.T) {
	// Arrange
	n := model.Node{ID: "w", Type: "delay", Params: []model.Field{{Key: "duration", Value: "2s"}}}

	// Act
	got, lost := ConvertStep(n, StepEntry{Type: "delay"})

	// Assert
	if got.Type != "delay" || ParamValue(got, "duration") != "2s" || len(lost) != 0 {
		t.Fatalf("got %+v lost %v", got, lost)
	}
}

func TestConvertStep_Given_a_step_with_nested_steps_to_a_kind_without_them_Then_the_nested_steps_are_named_as_lost(t *testing.T) {
	// Arrange
	n := model.Node{ID: "each", Type: "batch", Body: []model.Node{{ID: "a", Type: "log"}}}

	// Act
	got, lost := ConvertStep(n, StepEntry{Type: "delay"})

	// Assert
	if got.Type != "delay" || len(got.Body) != 0 || !slices.Contains(lost, "steps inside") {
		t.Fatalf("got %+v lost %v", got, lost)
	}
}
