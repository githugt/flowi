// SPDX-License-Identifier: Apache-2.0

package flowi

import "testing"

func TestExplainFlow_should_tell_a_saved_flow_in_words(t *testing.T) {
	w, c := petstoreWorkspace(t)
	all, _ := w.SuggestFlows(c)
	target, err := w.AcceptSuggestion(c, all[0])
	if err != nil {
		t.Fatal(err)
	}

	got, err := w.ExplainFlow(target)

	if err != nil || len(got.Sentences) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got.Sentences[1].Text != "Call pets/getPet, with petId = create.body.id" {
		t.Fatalf("sentence %q", got.Sentences[1].Text)
	}
}

func TestExplainFlow_Given_an_unknown_flow_Then_an_error(t *testing.T) {
	w, c := petstoreWorkspace(t)

	_, err := w.ExplainFlow(FlowTarget{Collection: c, Flow: "nope"})

	if err == nil {
		t.Fatal("want an error")
	}
}
