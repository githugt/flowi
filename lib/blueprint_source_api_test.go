// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: a blueprint could be made and deleted, never changed.
func TestSaveBlueprintSource_Given_edited_text_Then_the_blueprint_changes_and_keeps_its_id(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	bt, err := w.CreateHookBlueprint(flow.Collection, "hooks/hello", "Hello", "post", []model.Node{{ID: "n1", Type: "log", Params: []model.Field{{Key: "value", Value: "hi"}}}})
	if err != nil {
		t.Fatal(err)
	}
	text, err := w.BlueprintSource(bt)
	if err != nil {
		t.Fatal(err)
	}

	diags, err := w.SaveBlueprintSource(bt, strings.Replace(text, "hi", "bye", 1))

	if err != nil || diags.HasErrors() {
		t.Fatalf("save: %v %v", err, diags)
	}
	got, err := w.Blueprint(bt)
	if err != nil || got.Body.Nodes[0].Params[0].Value != "bye" {
		t.Fatalf("blueprint %+v, %v", got, err)
	}
}

func TestSaveBlueprintSource_Given_text_with_problems_Then_nothing_is_written(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	bt, err := w.CreateHookBlueprint(flow.Collection, "hooks/hello", "Hello", "post", []model.Node{{ID: "n1", Type: "log", Params: []model.Field{{Key: "value", Value: "hi"}}}})
	if err != nil {
		t.Fatal(err)
	}
	text, _ := w.BlueprintSource(bt)

	broken, errBroken := w.SaveBlueprintSource(bt, "nodes: [")
	_, errID := w.SaveBlueprintSource(bt, strings.Replace(text, string(bt.Blueprint), "01JA0000000000000000009999", 1))

	if errBroken != nil || !broken.HasErrors() {
		t.Fatalf("broken text: %v %v, want problems", errBroken, broken)
	}
	if errID == nil {
		t.Fatal("a changed id was accepted")
	}
	after, _ := w.BlueprintSource(bt)
	if after != text {
		t.Fatalf("the file changed:\n%s", after)
	}
}
