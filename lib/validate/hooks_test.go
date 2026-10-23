// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
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

func TestValidate_Given_hook_cycle_and_nested_true_Then_FLW_CYC_003_with_cycle_text(t *testing.T) {
	c := tokenPattern(t, true)

	got := Validate(Target{Collections: []model.Collection{c}})

	d := only(t, got, diag.CodeCycleHooks)
	if d.Args["cycle"] != "hooks/token → hooks/token" || d.File != c.Manifest.File || !strings.Contains(d.Path, "hooks") {
		t.Fatalf("diag %+v", d)
	}
}

func TestValidate_Given_hook_cycle_and_nested_false_Then_no_error(t *testing.T) {
	c := tokenPattern(t, false)

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}

func TestValidate_Given_two_flow_hook_cycle_through_version_hooks_Then_FLW_CYC_003(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	a := f.request(&c, "a", "1")
	b := f.request(&c, "b", "1")
	fa := f.flow(&c, "hooks/fa", requestNode("call-b", b))
	fb := f.flow(&c, "hooks/fb", requestNode("call-a", a))
	c.Requests[0].Versions[0].Hooks.Pre = []model.HookStep{{Flow: &model.Ref{ID: fa.ID, Path: fa.Path}}}
	c.Requests[1].Versions[0].Hooks.Post = []model.HookStep{{Flow: &model.Ref{ID: fb.ID, Path: fb.Path}}}
	c.Manifest.Settings = model.Settings{"hooks.nested": true}

	got := Validate(Target{Collections: []model.Collection{c}})

	d := only(t, got, diag.CodeCycleHooks)
	if d.Args["cycle"] != "hooks/fa → hooks/fb → hooks/fa" {
		t.Fatalf("cycle %q", d.Args["cycle"])
	}
}
