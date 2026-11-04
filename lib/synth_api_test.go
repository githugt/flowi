// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func petstoreWorkspace(t *testing.T) (*Workspace, model.ID) {
	t.Helper()
	e := newEnv(t)
	w := e.open(t)
	dir := t.TempDir()
	if _, err := w.InitCollection(dir, "Pets", true); err != nil {
		t.Fatal(err)
	}
	c := w.Collections()[len(w.Collections())-1].Manifest.ID
	importPetstore(t, w, c)
	return w, c
}

func TestSuggestFlows_should_propose_create_then_get_from_an_imported_description(t *testing.T) {
	w, c := petstoreWorkspace(t)

	got, err := w.SuggestFlows(c)

	if err != nil {
		t.Fatal(err)
	}
	var pets *FlowSuggestion
	for i := range got {
		if got[i].Name == "pets" {
			pets = &got[i]
		}
	}
	if pets == nil {
		t.Fatalf("no proposal for pets: %+v", got)
	}
	if len(pets.Steps) < 2 || pets.Steps[0].Method != "POST" {
		t.Fatalf("steps %+v", pets.Steps)
	}
	if len(pets.Flow.Nodes) != len(pets.Steps) {
		t.Fatalf("the draft has %d nodes for %d steps", len(pets.Flow.Nodes), len(pets.Steps))
	}
	reader := pets.Flow.Nodes[1]
	if reader.Request == nil || !slices.ContainsFunc(reader.Request.With, func(kv model.KeyValue) bool {
		return kv.Key == "petId" && kv.Value == "{{nodes."+pets.Flow.Nodes[0].ID+".body.id}}"
	}) {
		t.Fatalf("the reader should take petId from the create's body: %+v", reader)
	}
}

func TestAcceptSuggestion_should_save_the_draft_as_a_flow_that_validates(t *testing.T) {
	w, c := petstoreWorkspace(t)
	all, _ := w.SuggestFlows(c)
	var s FlowSuggestion
	for _, one := range all {
		if one.Name == "pets" {
			s = one
		}
	}

	target, err := w.AcceptSuggestion(c, s)

	if err != nil {
		t.Fatal(err)
	}
	_, saved, err := w.flowFor(target)
	if err != nil || len(saved.Nodes) != len(s.Flow.Nodes) {
		t.Fatalf("saved %v nodes, %v", len(saved.Nodes), err)
	}
	for _, d := range w.Validate() {
		if d.Severity == diag.SeverityError {
			t.Fatalf("the saved flow does not validate: %v", d)
		}
	}
	again, err := w.AcceptSuggestion(c, s)
	if err != nil || again.Flow == target.Flow {
		t.Fatalf("a second accept must make another flow, not overwrite: %v", err)
	}
}
