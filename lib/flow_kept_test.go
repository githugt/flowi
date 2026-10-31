// SPDX-License-Identifier: Apache-2.0

package flowi

import "testing"

func TestKeptFlowEdits_should_return_what_KeepFlowEdits_stored_once(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target := FlowTarget{Collection: c.Manifest.ID, Flow: c.Flows[0].ID}
	edited := c.Flows[0]
	edited.Nodes = edited.Nodes[:1]

	if err := w.KeepFlowEdits(target, edited); err != nil {
		t.Fatal(err)
	}
	text, ok, err := w.KeptFlowEdits(target)
	_, again, _ := w.KeptFlowEdits(target)

	if err != nil || !ok || text == "" {
		t.Fatalf("kept: %q %v %v", text, ok, err)
	}
	back, diags, err := w.ParseFlowSource(target, text)
	if err != nil || len(diags) > 0 || len(back.Nodes) != 1 {
		t.Fatalf("read back: %d nodes, %v, %v", len(back.Nodes), diags, err)
	}
	if again {
		t.Error("the edits were kept a second time")
	}
}

func TestKeptFlowEdits_Given_nothing_kept_Then_not_ok(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]

	_, ok, err := w.KeptFlowEdits(FlowTarget{Collection: c.Manifest.ID, Flow: c.Flows[0].ID})

	if ok || err != nil {
		t.Fatalf("ok %v err %v", ok, err)
	}
}
