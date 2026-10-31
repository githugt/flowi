// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"reflect"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestNode_Arrows(t *testing.T) {
	tests := []struct {
		name     string
		node     model.Node
		previous string
		want     []model.Arrow
	}{
		{"no arrows follows the step before on success", model.Node{}, "a", []model.Arrow{{From: "a", OnSuccess: true}}},
		{"the first step has no arrows", model.Node{}, "", nil},
		{"an unlinked step does not follow the step before it", model.Node{Unlinked: true}, "a", nil},
		{"an arrow of its own wins over unlinked", model.Node{Unlinked: true, Needs: []string{"x"}}, "a", []model.Arrow{{From: "x", OnSuccess: true}}},
		{"needs are success arrows", model.Node{Needs: []string{"x", "y"}}, "a", []model.Arrow{{From: "x", OnSuccess: true}, {From: "y", OnSuccess: true}}},
		{"a failure arrow alone replaces the one from the step before", model.Node{AfterFailureOf: []string{"x"}}, "a", []model.Arrow{{From: "x", OnFailure: true}}},
		{"the same step in both lists is an arrow taken either way", model.Node{Needs: []string{"x"}, AfterFailureOf: []string{"x"}}, "a", []model.Arrow{{From: "x", OnSuccess: true, OnFailure: true}}},
		{"success and failure arrows from different steps", model.Node{Needs: []string{"x"}, AfterFailureOf: []string{"y"}}, "a", []model.Arrow{{From: "x", OnSuccess: true}, {From: "y", OnFailure: true}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.node.Arrows(tc.previous)

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
