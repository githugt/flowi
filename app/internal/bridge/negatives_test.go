// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func negativesApp(t *testing.T) (*bridge.App, *vmtest.Backend) {
	t.Helper()
	b := &vmtest.Backend{
		Cols:      []model.Collection{vmtest.SampleCollection()},
		Negatives: []model.Overlay{{Name: "no-email", Description: "email missing"}},
	}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	return app, b
}

func TestNegatives_should_list_the_variants_with_what_each_does(t *testing.T) {
	app, _ := negativesApp(t)

	got := app.Blueprints().Negatives("r:C1:R1")

	if got.Error != "" || len(got.Variants) != 1 || got.Variants[0].Name != "no-email" || got.Variants[0].Description != "email missing" {
		t.Fatalf("got %+v", got)
	}
}

func TestNegatives_Given_a_flow_row_Then_the_error_is_in_the_view_and_the_list_is_empty_not_null(t *testing.T) {
	app, _ := negativesApp(t)

	got := app.Blueprints().Negatives("f:C1:F1")

	if got.Error == "" || got.Variants == nil {
		t.Fatalf("got %+v", got)
	}
}

func TestAcceptNegatives_should_report_what_was_saved_and_what_was_left_alone(t *testing.T) {
	app, b := negativesApp(t)

	got := app.Blueprints().AcceptNegatives("r:C1:R1", []string{"no-email"})

	if got.Error != "" || len(got.Saved) != 1 || got.Skipped == nil || !b.Called("AcceptNegatives C1 no-email") {
		t.Fatalf("got %+v calls %v", got, b.Calls)
	}
}

func TestAcceptNegatives_Given_an_open_request_When_variants_are_saved_Then_it_reads_its_overlays_again(t *testing.T) {
	// Before: the saved variants were on disk but the open request kept the overlay list it was opened with, so they
	// were missing from its overlay picker until the app was restarted.
	app, b := negativesApp(t)
	req, _ := findNode(app.Workspace().Tree().Roots, "request")
	if _, err := app.Requests().Open(req.ID); err != nil {
		t.Fatal(err)
	}
	readsBefore := len(b.CallsStarting("Overlays"))

	app.Blueprints().AcceptNegatives(req.ID, []string{"no-email"})

	if got := len(b.CallsStarting("Overlays")); got <= readsBefore {
		t.Fatalf("overlay reads: %d before, %d after saving; the open request did not reload", readsBefore, got)
	}
}
