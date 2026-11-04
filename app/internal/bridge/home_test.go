// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestWelcomeService_Home_should_count_the_workspace_and_suggest_what_is_unused(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	v := app.Welcome().Home()

	if v.Collections != 1 || v.Requests == 0 {
		t.Fatalf("counts %+v", v)
	}
	var ids []string
	for _, i := range v.Ideas {
		ids = append(ids, i.ID)
		if i.Title == "" || i.Action == "" {
			t.Errorf("incomplete idea %+v", i)
		}
	}
	if len(ids) == 0 {
		t.Fatalf("a workspace with no schedules, hooks or tracing should have ideas: %+v", v)
	}
}

func TestWelcomeService_Home_Given_tracing_on_Then_the_tracing_idea_goes_away(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit, bridge.WithTracer(&recordingTracer{}))
	app.Start(t.Context())
	app.Tracing().Save("http://localhost:4318", nil, false)

	v := app.Welcome().Home()

	for _, i := range v.Ideas {
		if i.ID == "tracing" {
			t.Fatalf("tracing is on, yet suggested: %+v", v.Ideas)
		}
	}
	if !v.TracingOn {
		t.Fatalf("%+v", v)
	}
}

func TestWelcomeService_Home_Given_flows_to_suggest_Then_the_first_idea_opens_the_review_in_that_collection(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Flows = nil
	b := &vmtest.Backend{
		Cols:        []model.Collection{c},
		Suggestions: map[model.ID][]flowi.FlowSuggestion{"C1": {{Name: "pets", Path: "suggested/pets"}}},
	}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	v := app.Welcome().Home()

	if len(v.Ideas) == 0 || v.Ideas[0].ID != "suggested-flows" || v.Ideas[0].Action != "suggest-flows:C1" {
		t.Fatalf("ideas %+v", v.Ideas)
	}
}
