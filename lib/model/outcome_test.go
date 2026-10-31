// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"reflect"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestRequestVersion_EffectiveOutcomes(t *testing.T) {
	custom := []model.Outcome{{Name: "ok", When: "status in [200, 201]"}}

	tests := []struct {
		name string
		v    model.RequestVersion
		want []model.Outcome
	}{
		{"none written means success and failure", model.RequestVersion{}, []model.Outcome{{Name: "success"}, {Name: "failure"}}},
		{"an empty list means the same", model.RequestVersion{Outcomes: []model.Outcome{}}, []model.Outcome{{Name: "success"}, {Name: "failure"}}},
		{"a written list is the whole list: the defaults are not added back", model.RequestVersion{Outcomes: custom}, custom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.v.EffectiveOutcomes()

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestValidOutcomeName(t *testing.T) {
	for name, want := range map[string]bool{"ok": true, "not-found": true, "server_down": true, "a1": true, "": false, "has space": false, "a:b": false, "-x": false} {
		if got := model.ValidOutcomeName(name); got != want {
			t.Errorf("%q: got %v, want %v", name, got, want)
		}
	}
}

func TestNode_Arrows_with_named_outcomes(t *testing.T) {
	n := model.Node{AfterOutcome: []string{"login:not-found", "login:auth", "pay:failure", "pay:success"}}

	got := n.Arrows("x")

	want := []model.Arrow{
		{From: "login", On: []string{"not-found", "auth"}},
		{From: "pay", OnSuccess: true, OnFailure: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestArrow_Takes(t *testing.T) {
	a := model.Arrow{OnSuccess: true, On: []string{"not-found"}}

	for outcome, want := range map[string]bool{"success": true, "failure": false, "not-found": true, "auth": false} {
		if got := a.Takes(outcome); got != want {
			t.Errorf("%s: got %v, want %v", outcome, got, want)
		}
	}
}
