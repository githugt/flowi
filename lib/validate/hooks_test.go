// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// tokenPattern builds the auth pattern: the collection's pre-hook flow
// "hooks/token" calls request "auth/token", which (as every request in the
// collection) has that same collection pre-hook.
func tokenPattern(t *testing.T, nested bool) model.Collection {
	t.Helper()
	f := newFixture(t)
	c := f.collection("c")
	token := f.request(&c, "auth/token", "1")
	hook := f.flow(&c, "hooks/token", requestNode("get", token))
	c.Manifest.Hooks.Pre = []model.HookStep{{Flow: &model.Ref{ID: hook.ID, Path: hook.Path}}}
	if nested {
		c.Manifest.Settings = model.Settings{"hooks.nested": true}
	}
	return c
}

func TestValidate_Given_hook_cycle_and_nested_false_Then_no_error(t *testing.T) {
	c := tokenPattern(t, false)

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}
