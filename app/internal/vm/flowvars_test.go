// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/model"
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

func TestScanFlow_Given_an_empty_flow_Then_nothing(t *testing.T) {
	got := ScanFlow(model.Flow{})

	if len(got.Set)+len(got.Read)+len(got.Unset)+len(got.Secrets) != 0 {
		t.Fatalf("%+v", got)
	}
}
