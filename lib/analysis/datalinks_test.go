// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func param(key string, value any) model.Field { return model.Field{Key: key, Value: value} }

func TestDataLinks_should_link_the_reader_of_an_output_or_a_variable_to_its_maker(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{}},
		{ID: "grab", Type: "extract", Params: []model.Field{param("token", "{{nodes.login.body.token}}")}},
		{ID: "shout", Type: "transform", Params: []model.Field{param("value", "{{upper(vars.token)}}"), param("into", "loud")}},
		{ID: "show", Type: "log", Params: []model.Field{param("value", "{{vars.loud}} and {{nodes.shout.value}}")}},
	}}

	got := DataLinks(f)

	want := []DataLink{
		{From: "login", To: "grab", Via: "login.body"},
		{From: "grab", To: "shout", Via: "token"},
		{From: "shout", To: "show", Via: "loud"},
		{From: "shout", To: "show", Via: "shout.value"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Behaviour: values a generate node makes are fresh, not taken from another
// step, so they are not drawn as a link; what a generate node reads still is.
func TestDataLinks_should_not_link_generated_values_but_link_what_a_generator_reads(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{}},
		{ID: "make", Type: "generate", Params: []model.Field{param("email", "{{$email}}"), param("tag", "{{nodes.login.status}}")}},
		{ID: "use", Type: "log", Params: []model.Field{param("value", "{{vars.email}}")}},
	}}

	got := DataLinks(f)

	want := []DataLink{{From: "login", To: "make", Via: "login.status"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDataLinks_should_find_readers_inside_containers_and_ignore_unknown_nodes(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "list", Type: "request", Request: &model.RequestCall{}},
		{ID: "each", Type: "batch", Params: []model.Field{param("over", "{{nodes.list.body.items}}")}, Body: []model.Node{
			{ID: "say", Type: "log", Params: []model.Field{param("value", "{{nodes.nobody.x}} {{item}}")}},
		}},
	}}

	got := DataLinks(f)

	want := []DataLink{{From: "list", To: "each", Via: "list.body"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
