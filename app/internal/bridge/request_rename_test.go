// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// Before: an open request was never told its file was reloaded, so after a rename its view kept the old path.
func TestRequestService_Given_an_open_request_When_it_is_renamed_from_the_tree_Then_its_view_has_the_new_path(t *testing.T) {
	_, v, pub, app, _ := openLoginOptions(t)

	app.Workspace().Do("rename", []string{v.Node}, "sign-in", "")

	got, ok := pub.last["request"].(bridge.RequestView)
	if !ok || got.Node != v.Node || got.Path != "auth/sign-in" || got.Error != "" || got.OnDisk {
		t.Fatalf("view after rename: %+v", pub.last["request"])
	}
}

// Before: keeping a request under a new name in the collection it was already in (a draft saved in Drafts) asked
// the window to open and then close the same tab, which left the tab with nothing in it.
func TestRequestService_SaveTo_Given_the_same_collection_and_a_new_name_When_saved_Then_the_tab_is_not_closed(t *testing.T) {
	reqs, v, pub, _, _ := openLoginOptions(t)

	reqs.SaveTo(v.Node, reqs.PlaceOf(v.Node), "sign-in")

	for _, name := range pub.names {
		if name == "close" || name == "open" {
			t.Fatalf("the window was told to %s a tab: %v", name, pub.names)
		}
	}
	got, ok := pub.last["request"].(bridge.RequestView)
	if !ok || got.Node != v.Node || got.Path != "auth/sign-in" || got.Error != "" {
		t.Fatalf("view after saving: %+v", pub.last["request"])
	}
}

// Before: the "Leave secrets out" toggle of a code snippet started on whatever ui.snippet_exclude_secrets said, and
// that setting was off by default, so "Copy as curl" put live secrets on the clipboard.
func TestRequestService_Open_Given_the_snippet_setting_When_a_request_opens_Then_the_toggle_starts_as_the_setting(t *testing.T) {
	_, byDefault, _, _, _ := openLoginOptions(t)
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Values: map[string]any{"ui.snippet_exclude_secrets": false}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	turnedOff, err := app.Requests().Open("r:C1:R1")

	if err != nil || !byDefault.Snippet.Exclude || turnedOff.Snippet.Exclude {
		t.Fatalf("err %v, by default %v, turned off %v", err, byDefault.Snippet.Exclude, turnedOff.Snippet.Exclude)
	}
}
