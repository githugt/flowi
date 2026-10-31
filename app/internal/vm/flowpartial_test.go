// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestPartialNodes(t *testing.T) {
	list := []model.Node{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", Needs: []string{"a"}},
		{ID: "d", Type: "condition", Cases: []model.Case{{Nodes: []model.Node{{ID: "inner"}}}}},
	}
	tests := []struct {
		name string
		id   string
		kind RunScope
		want []string
		ok   bool
	}{
		{"only", "b", ScopeOnly, []string{"b"}, true},
		{"until follows the chain", "b", ScopeUntil, []string{"a", "b"}, true},
		{"until follows needs", "c", ScopeUntil, []string{"a", "c"}, true},
		{"from takes only what waits on it", "b", ScopeFrom, []string{"b"}, true},
		{"from a need takes its dependants", "a", ScopeFrom, []string{"a", "b", "c", "d"}, true},
		{"a step in a lane stands for its holder", "inner", ScopeOnly, []string{"d"}, true},
		{"unknown", "zzz", ScopeOnly, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PartialNodes(list, tt.id, tt.kind)

			if ok != tt.ok || !slices.Equal(got, tt.want) {
				t.Fatalf("got %v %v, want %v %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
