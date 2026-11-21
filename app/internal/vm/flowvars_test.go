// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func param(k string, v any) model.Field { return model.Field{Key: k, Value: v} }

func TestScanFlow_should_list_written_read_and_unset_variables_and_secrets(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "s", Type: "set", Params: []model.Field{param("target", "vars.token"), param("value", "{{secret.api_key}}")}},
		{ID: "e", Type: "set", Values: []model.SetValue{{Name: "user_id", From: "response", Where: "body", Path: "$.id"}}},
		{ID: "g", Type: "set", Values: []model.SetValue{{Name: "email", From: "generate", Value: "{{$email}}"}}},
		{ID: "l", Type: "log", Params: []model.Field{param("value", "{{vars.token}} {{vars.user_id}} {{vars.missing}}")}},
	}}

	got := ScanFlow(f)

	if !slices.Equal(got.Set, []string{"email", "token", "user_id"}) {
		t.Fatalf("set %v", got.Set)
	}
	if !slices.Equal(got.Unset, []string{"missing"}) || !slices.Equal(got.Secrets, []string{"api_key"}) {
		t.Fatalf("unset %v secrets %v", got.Unset, got.Secrets)
	}
}

// Before: the variables bar listed names only, so finding who wrote or read a variable meant reading every step.
func TestScanFlow_should_say_which_steps_write_and_read_each_variable(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "login", Type: "set", Values: []model.SetValue{{Name: "token", From: "response", Where: "body", Path: "$.t"}}},
		{ID: "each", Type: "batch", Body: []model.Node{
			{ID: "call", Type: "log", Params: []model.Field{param("value", "{{vars.token}}")}},
			{ID: "bump", Type: "set", Params: []model.Field{param("target", "vars.count"), param("value", "{{vars.count}}")}},
		}},
	}}

	got := ScanFlow(f)

	if !slices.Equal(got.Writers["token"], []string{"login"}) || !slices.Equal(got.Writers["count"], []string{"bump"}) {
		t.Fatalf("writers %v", got.Writers)
	}
	if !slices.Equal(got.Readers["token"], []string{"call"}) || !slices.Equal(got.Readers["count"], []string{"bump"}) || len(got.Readers["token"]) != 1 {
		t.Fatalf("readers %v (the container must not count as a reader of what its children read)", got.Readers)
	}
}

func TestScanFlow_Given_an_empty_flow_Then_nothing(t *testing.T) {
	got := ScanFlow(model.Flow{})

	if len(got.Set)+len(got.Read)+len(got.Unset)+len(got.Secrets) != 0 {
		t.Fatalf("%+v", got)
	}
}
