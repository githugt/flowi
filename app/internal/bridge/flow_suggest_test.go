// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func suggestingApp(t *testing.T) (*bridge.App, *published) {
	t.Helper()
	b := &vmtest.Backend{
		Cols: []model.Collection{vmtest.SampleCollection()},
		Suggestions: map[model.ID][]flowi.FlowSuggestion{"C1": {{
			Name: "pets", Path: "suggested/pets",
			Steps: []flowi.SuggestedStep{
				{ID: "create", Request: "pets/create", Method: "POST"},
				{ID: "get", Request: "pets/get", Method: "GET", Takes: []flowi.SuggestedValue{{Param: "petId", From: "create", Field: "id", Confidence: 0.9}}},
			},
		}}},
	}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	return app, pub
}

func TestSuggestions_should_show_each_step_with_what_it_takes_from_the_one_before(t *testing.T) {
	app, _ := suggestingApp(t)

	got := app.Blueprints().Suggestions("C1")

	if len(got) != 1 || len(got[0].Steps) != 2 {
		t.Fatalf("suggestions %+v", got)
	}
	take := got[0].Steps[1].Takes[0]
	if take.Param != "petId" || take.From != "create" || take.Field != "id" || take.Confidence != 0.9 {
		t.Fatalf("take %+v", take)
	}
	if got[0].Steps[0].Takes == nil {
		t.Fatal("takes must be empty, not null, for the frontend")
	}
}

func TestSuggestions_Given_a_collection_with_none_Then_an_empty_list_not_null(t *testing.T) {
	app, _ := suggestingApp(t)

	got := app.Blueprints().Suggestions("nothing")

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestAcceptSuggestion_should_ask_the_window_to_open_the_saved_flow(t *testing.T) {
	app, pub := suggestingApp(t)

	app.Blueprints().AcceptSuggestion("C1", "suggested/pets")

	if pub.last["open"] != "f:C1:new" {
		t.Fatalf("events %+v", pub)
	}
}
