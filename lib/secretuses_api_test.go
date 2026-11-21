// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: the Secrets screen showed a name and a state, and nothing said what a secret was used by.
func TestSecretUses_should_find_the_requests_flows_and_auths_that_write_a_secret(t *testing.T) {
	w := demoWorkspace(t)
	uses := w.SecretUses()
	if len(uses) == 0 {
		t.Fatal("the demo collection uses at least one secret")
	}
	for name, list := range uses {
		if name == "" || len(list) == 0 {
			t.Fatalf("empty entry %q: %v", name, list)
		}
		sorted := slices.IsSortedFunc(list, func(a, b SecretUse) int {
			if a.Kind != b.Kind {
				if a.Kind < b.Kind {
					return -1
				}
				return 1
			}
			switch {
			case a.Name < b.Name:
				return -1
			case a.Name > b.Name:
				return 1
			}
			return 0
		})
		if !sorted {
			t.Fatalf("%s not sorted: %v", name, list)
		}
	}
}

func TestSecretUses_should_read_a_secret_in_the_text_of_a_request(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	f.Nodes = append(f.Nodes, model.Node{ID: "peek", Type: "log", Params: []model.Field{{Key: "value", Value: "{{secret.only_here}}"}}})
	if err := w.SaveFlow(target, f, SaveOptions{DropComments: true}); err != nil {
		t.Fatal(err)
	}

	got := w.SecretUses()["only_here"]

	if len(got) != 1 || got[0].Kind != UseFlow || got[0].ID != f.ID || got[0].Collection != target.Collection {
		t.Fatalf("got %+v", got)
	}
}
