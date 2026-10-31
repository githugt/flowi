// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func labelsOf(targets []MoveTargetView) []string {
	var out []string
	for _, t := range targets {
		out = append(out, t.Label)
	}
	return out
}

func TestWorkspaceService_PlaceTargets_Given_requests_Then_the_same_places_as_Move(t *testing.T) {
	app, _, _ := twoCollections(t)

	got := labelsOf(app.Workspace().PlaceTargets("requests"))

	want := labelsOf(app.Workspace().MoveTargets())
	if !slices.Equal(got, want) {
		t.Fatalf("places %v, want %v", got, want)
	}
}

func TestWorkspaceService_PlaceTargets_Given_flows_Then_collections_and_flow_folders(t *testing.T) {
	app, _, _ := twoCollections(t)
	billing := rowOf(t, app, "collection", "Billing")
	app.Workspace().NewPlaceFolder(billing, "flows", "journeys")

	got := labelsOf(app.Workspace().PlaceTargets("flows"))

	want := []string{"Shop", "Billing", "Billing / journeys"}
	if !slices.Equal(got, want) {
		t.Fatalf("places %v, want %v", got, want)
	}
}

func TestWorkspaceService_NewAt_should_create_in_the_place_the_row_stands_for(t *testing.T) {
	tests := []struct {
		name, kind, place, text, protocol string
		wantCall                          string
	}{
		{name: "a request in a folder, named", kind: "request", place: "Shop / auth", text: "refresh", wantCall: "NewRequest auth/refresh"},
		{name: "a request in a folder, unnamed", kind: "request", place: "Shop / auth", wantCall: "NewRequest auth/untitled"},
		{name: "a request at the top of a collection", kind: "request", place: "Billing", text: "ping", wantCall: "NewRequest ping"},
		{name: "a message request", kind: "async", place: "Billing", text: "orders", protocol: "mqtt", wantCall: "NewAsyncRequest orders mqtt"},
		{name: "a gRPC request", kind: "grpc", place: "Billing", text: "get", wantCall: "NewGRPCRequest get"},
		{name: "a dataset query", kind: "dataset", place: "Billing", text: "customers", protocol: "sqlite", wantCall: "NewDatasetRequest customers sqlite"},
		{name: "a flow in a folder", kind: "flow", place: "Billing", text: "journeys/signup", wantCall: "NewFlow journeys/signup"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, pub, b := twoCollections(t)
			row := placeRow(t, app, tt.place)

			app.Workspace().NewAt(tt.kind, row, tt.text, tt.protocol)

			opened, _ := pub.last["open"].(string)
			if !b.Called(tt.wantCall) || opened == "" {
				t.Fatalf("calls %v, opened %q", b.Calls, opened)
			}
		})
	}
}

func TestWorkspaceService_NewAt_Given_no_place_Then_it_goes_to_Scratch(t *testing.T) {
	app, pub, b := twoCollections(t)

	app.Workspace().NewAt("grpc", "", "", "")

	opened, _ := pub.last["open"].(string)
	if !b.Called("Scratch") || !b.Called("NewGRPCRequest untitled") || opened == "" {
		t.Fatalf("calls %v, opened %q", b.Calls, opened)
	}
}

func TestWorkspaceService_NewAt_Given_a_row_that_is_not_a_place_Then_a_notice(t *testing.T) {
	app, pub, _ := twoCollections(t)
	request := rowOf(t, app, "request", "Login")

	app.Workspace().NewAt("request", request, "x", "")

	notice, _ := pub.last["notice"].(Notice)
	if notice.Level != "error" {
		t.Fatalf("notice %+v", notice)
	}
}

func TestWorkspaceService_NewPlaceCollection_should_make_a_draft_collection_and_name_its_row(t *testing.T) {
	app, _, b := twoCollections(t)

	row := app.Workspace().NewPlaceCollection("  Payments ")

	if row == "" || !b.Called("NewDraft Payments") || !slices.Contains(labelsOf(app.Workspace().PlaceTargets("requests")), "Payments") {
		t.Fatalf("row %q calls %v", row, b.Calls)
	}
}

func TestWorkspaceService_NewPlaceFolder_should_make_a_folder_below_the_place_and_name_its_row(t *testing.T) {
	app, _, b := twoCollections(t)
	auth := placeRow(t, app, "Shop / auth")

	row := app.Workspace().NewPlaceFolder(auth, "requests", "tokens")

	got := app.Workspace().PlaceTargets("requests")
	if row == "" || !b.Called("NewFolder requests/auth/tokens") || !slices.Contains(labelsOf(got), "Shop / auth / tokens") {
		t.Fatalf("row %q calls %v targets %v", row, b.Calls, labelsOf(got))
	}
	if placeOf := got[slices.IndexFunc(got, func(m MoveTargetView) bool { return m.Row == row })]; placeOf.Label != "Shop / auth / tokens" {
		t.Fatalf("the returned row %q is %q", row, placeOf.Label)
	}
}

// placeRow finds the row of a place by the label PlaceTargets gives it.
func placeRow(t *testing.T, app *App, label string) string {
	t.Helper()
	for _, p := range app.Workspace().PlaceTargets("requests") {
		if p.Label == label {
			return p.Row
		}
	}
	t.Fatalf("no place %q", label)
	return ""
}

// Before: a scheme added to the library without a label here showed as a blank entry in the Auth picker.
func TestAuthSchemeChoices_should_label_every_scheme_the_library_has(t *testing.T) {
	got := authSchemeChoices()

	if len(got) != len(model.AuthTypes()) {
		t.Fatalf("%d choices for %d schemes", len(got), len(model.AuthTypes()))
	}
	for _, c := range got {
		if strings.TrimSpace(c.Label) == "" {
			t.Errorf("%s has no label", c.ID)
		}
	}
}
