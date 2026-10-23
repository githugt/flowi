// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"testing"

	"github.com/githugt/flowi-lib/internal/nodes"
)

func TestFrame_scope_should_merge_vars_and_nodes_up_the_chain_and_isolate_own_layers(t *testing.T) {
	r := &runner{secrets: &secretTracker{}}
	root := newRootFrame(&flowScope{env: environment{values: map[string]any{"base": "u"}}}, nil)
	root.vars.Set("shared", 1)
	root.outputs.put("login", map[string]any{"token": "t"})

	iter := root.child(nodes.Lane{Label: "[2]", OwnVars: true, OwnNodes: true, Item: "x", Index: 2, HasItem: true}, nil, "users")
	iter.vars.Set("mine", 2)
	iter.outputs.put("create", map[string]any{"status": 201})
	shared := root.child(nodes.Lane{Label: "then"}, nil, "cond")
	shared.vars.Set("fromBranch", 3)

	sc := iter.scope(r).Values
	vars := sc["vars"].(map[string]any)
	ns := sc["nodes"].(map[string]any)
	if vars["shared"] != 1 || vars["mine"] != 2 || ns["login"] == nil || ns["create"] == nil || sc["item"] != "x" || sc["index"] != 2 {
		t.Fatalf("scope %v", sc)
	}
	if _, leaked := root.vars.Get("mine"); leaked {
		t.Fatal("iteration var leaked to the root")
	}
	if _, leaked := root.outputs.snapshot()["create"]; leaked {
		t.Fatal("iteration output leaked to the root")
	}
	if v, _ := root.vars.Get("fromBranch"); v != 3 {
		t.Fatal("shared lane must write to the enclosing vars")
	}
	if iter.path("create") != "users[2]/create" || shared.path("log") != "cond/then/log" || root.path("a") != "a" {
		t.Fatalf("paths %q %q %q", iter.path("create"), shared.path("log"), root.path("a"))
	}
	if iter.parentPath != "users" || iter.lane != "[2]" {
		t.Fatalf("parent %q lane %q", iter.parentPath, iter.lane)
	}
}

func TestOutputs_should_panic_on_second_write(t *testing.T) {
	o := newOutputs(nil)
	o.put("a", map[string]any{})

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	o.put("a", map[string]any{})
}
