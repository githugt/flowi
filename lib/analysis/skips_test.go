// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func skipFlow(reader string) model.Flow {
	return model.Flow{Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{}},
		{ID: "pick", Type: "set", Values: []model.SetValue{{Name: "token", Value: "{{nodes.login.body.token}}"}}},
		{ID: "use", Type: "log", Params: []model.Field{param("value", reader)}},
	}}
}

func TestSkipReaders_Given_a_step_read_by_a_later_template_Then_the_reader_is_listed_as_required(t *testing.T) {
	f := skipFlow("{{nodes.login.status}} {{vars.token}}")

	got := SkipReaders(f, []string{"login"})

	want := []SkipReader{
		{Skipped: "login", Reader: "pick", Reads: "nodes.login.body.token", Required: true},
		{Skipped: "login", Reader: "use", Reads: "nodes.login.status", Required: true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSkipReaders_Given_a_default_in_the_template_Then_it_is_not_required(t *testing.T) {
	f := skipFlow("{{nodes.login.status ?? 0}}")

	got := SkipReaders(f, []string{"pick"})

	if len(got) != 0 {
		t.Fatalf("got %+v, want no reader (the only reader of pick is vars.token, absent here)", got)
	}
	got = SkipReaders(skipFlow("{{vars.token ?? 'none'}}"), []string{"pick"})
	want := []SkipReader{{Skipped: "pick", Reader: "use", Reads: "vars.token", Required: false}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSkipReaders_Given_a_skipped_reader_Then_it_is_not_reported(t *testing.T) {
	f := skipFlow("{{nodes.login.status}}")

	got := SkipReaders(f, []string{"login", "pick", "use"})

	if len(got) != 0 {
		t.Fatalf("got %+v, want none: every reader is skipped too", got)
	}
}

func TestSkipReaders_Given_a_skipped_container_Then_what_is_inside_counts_as_skipped(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "each", Type: "batch", Body: []model.Node{{ID: "call", Type: "request", Request: &model.RequestCall{}}}},
		{ID: "use", Type: "log", Params: []model.Field{param("value", "{{nodes.call.status}}")}},
	}}

	got := SkipReaders(f, []string{"each"})

	want := []SkipReader{{Skipped: "call", Reader: "use", Reads: "nodes.call.status", Required: true}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
