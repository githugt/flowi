// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func sidebarApp(t *testing.T, values map[string]any, session string) (*bridge.App, *published) {
	t.Helper()
	pub := &published{}
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Session: session, Values: values}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	// Starring and opening are saved by a background writer into the session folder; wait for it before the test's
	// temp folder is removed, which fails with "directory not empty" when a write is still going.
	// Before: a test that starred without calling Shutdown failed on the CI runners.
	t.Cleanup(app.Shutdown)
	return app, pub
}

// Before: there was no Recent tab and no stars. A star and what was opened must come back after a restart.
func TestWorkspaceService_Given_a_star_and_opened_rows_When_the_app_restarts_Then_they_come_back(t *testing.T) {
	dir := t.TempDir()
	keep := map[string]any{"ui.keep_history": true}
	app, _ := sidebarApp(t, keep, dir)
	ws := app.Workspace()

	ws.ToggleFavourite("c:C1")
	ws.NoteOpened("r:C1:R3")
	ws.NoteOpened("f:C1:F1")
	app.Shutdown()
	again, _ := sidebarApp(t, keep, dir)
	view := again.Workspace().Sidebar()

	if !slices.Equal(view.Favourites, []string{"c:C1"}) {
		t.Errorf("favourites %v", view.Favourites)
	}
	if len(view.Recent) != 2 || view.Recent[0].ID != "f:C1:F1" || view.Recent[1].Kind != "request" || view.Recent[1].Where != "Shop" {
		t.Errorf("recent %+v", view.Recent)
	}
}

// Before: n/a (new). Someone who keeps no history does not expect a list of what they opened on disk; their stars stay.
func TestWorkspaceService_Given_history_is_not_kept_When_the_app_restarts_Then_only_the_stars_come_back(t *testing.T) {
	dir := t.TempDir()
	app, _ := sidebarApp(t, nil, dir)
	app.Workspace().ToggleFavourite("c:C1")
	app.Workspace().NoteOpened("r:C1:R3")
	app.Shutdown()

	again, _ := sidebarApp(t, nil, dir)
	view := again.Workspace().Sidebar()

	if len(view.Favourites) != 1 || len(view.Recent) != 0 {
		t.Fatalf("view %+v", view)
	}
}

func TestWorkspaceService_Given_a_change_When_starring_Then_the_window_is_told(t *testing.T) {
	app, pub := sidebarApp(t, nil, t.TempDir())

	app.Workspace().ToggleFavourite("c:C1")

	view, ok := pub.last["sidebar"].(bridge.SidebarView)
	if !ok || len(view.Favourites) != 1 {
		t.Fatalf("event %+v", pub.last["sidebar"])
	}
}

// Before: n/a (new). A damaged file must say so with its code and what to do, and the sidebar must still start.
func TestWorkspaceService_Given_a_damaged_sidebar_file_When_started_Then_a_coded_notice_says_what_to_do_and_it_starts_empty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sidebar.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	app, pub := sidebarApp(t, nil, dir)
	view := app.Workspace().Sidebar()

	notice, ok := pub.last["notice"].(bridge.Notice)
	if !ok || !strings.HasPrefix(notice.Text, "FLW-SIDEBAR-002") || !strings.Contains(notice.Text, "delete sidebar.json") {
		t.Fatalf("notice %+v", pub.last["notice"])
	}
	if len(view.Favourites) != 0 || len(view.Recent) != 0 {
		t.Errorf("view %+v", view)
	}
}

func TestWorkspaceService_Given_an_unwritable_folder_When_starring_Then_a_coded_notice_is_sent(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	app, pub := sidebarApp(t, nil, filepath.Join(blocker, "inside")) // a folder cannot be made under a file

	app.Workspace().ToggleFavourite("c:C1")
	app.Shutdown()

	notice, ok := pub.last["notice"].(bridge.Notice)
	if !ok || !strings.HasPrefix(notice.Text, "FLW-SIDEBAR-001") || !strings.Contains(notice.Text, "writable") {
		t.Fatalf("notice %+v", pub.last["notice"])
	}
}

func TestWorkspaceService_Hover_should_give_the_card_with_facts_and_lists_that_are_never_nil(t *testing.T) {
	app, _ := sidebarApp(t, nil, t.TempDir())

	card := app.Workspace().Hover("r:C1:R1")

	if card.Verb != "POST" || card.Target != "{{env.base_url}}/login" || card.Where != "Shop / auth" || card.Facts == nil || card.Problems == nil {
		t.Fatalf("card %+v", card)
	}
}

func TestWorkspaceService_Tree_should_carry_each_row_s_type_and_place(t *testing.T) {
	app, _ := sidebarApp(t, nil, t.TempDir())

	req, _ := findNode(app.Workspace().Tree().Roots, "request")

	if req.Type != "http" || req.TypeName != "HTTP" || req.Where == "" {
		t.Fatalf("request %+v", req)
	}
}

// Before: n/a (new). The sidebar marks the selected environment and counts what is unresolved: a secret with no value.
func TestEnvironmentService_States_should_mark_the_selected_environment_and_an_unset_secret(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Session: t.TempDir(), Values: map[string]any{"defaults.environment": "staging"}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	states := app.Environments().States()

	if len(states) == 0 || states[0].Name != "staging" || !states[0].Active || len(states[0].Issues) != 1 || states[0].Issues[0].Hint == "" {
		t.Fatalf("states %+v", states)
	}
}
