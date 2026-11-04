// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func traceFlow() model.Flow {
	return model.Flow{Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{}},
		{ID: "grab", Type: "set", Values: []model.SetValue{{Name: "token", Value: "{{nodes.login.body.token}}"}, {Name: "email", From: "generate", Value: "{{$email}}"}}},
		{ID: "again", Type: "set", Values: []model.SetValue{{Name: "token", Value: "{{vars.token}}-2"}}},
		{ID: "show", Type: "log", Params: []model.Field{param("value", "{{vars.token}} {{vars.email}} {{inputs.region}}")}},
	}}
}

func TestTrace_should_name_who_makes_and_who_reads_a_variable(t *testing.T) {
	got := Trace(traceFlow(), "token")

	want := ValueTrace{Name: "vars.token", Kind: TraceVariable, Producers: []string{"grab", "again"}, Readers: []string{"again", "show"}}
	if got.Name != want.Name || got.Kind != want.Kind || !slices.Equal(got.Producers, want.Producers) || !slices.Equal(got.Readers, want.Readers) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestTrace_should_accept_the_forms_a_value_is_written_in(t *testing.T) {
	for _, name := range []string{"token", "vars.token", "{{vars.token}}", " token "} {
		if got := Trace(traceFlow(), name); got.Name != "vars.token" {
			t.Errorf("Trace(%q).Name = %q", name, got.Name)
		}
	}
}

func TestTrace_should_follow_a_node_output_to_its_readers(t *testing.T) {
	got := Trace(traceFlow(), "login.body.token")

	if got.Kind != TraceOutput || !slices.Equal(got.Producers, []string{"login"}) || !slices.Equal(got.Readers, []string{"grab"}) {
		t.Fatalf("got %+v", got)
	}
}

func TestTrace_should_say_a_generated_variable_is_fresh_and_an_input_has_no_maker(t *testing.T) {
	email := Trace(traceFlow(), "email")
	region := Trace(traceFlow(), "inputs.region")

	if !email.Generated || !slices.Equal(email.Producers, []string{"grab"}) {
		t.Errorf("email %+v", email)
	}
	if region.Kind != TraceInput || len(region.Producers) != 0 || !slices.Equal(region.Readers, []string{"show"}) {
		t.Errorf("region %+v", region)
	}
}

func TestTrace_Given_a_name_nothing_makes_or_reads_Then_it_is_empty_not_an_error(t *testing.T) {
	got := Trace(traceFlow(), "ghost")

	if len(got.Producers) != 0 || len(got.Readers) != 0 {
		t.Fatalf("got %+v", got)
	}
}
