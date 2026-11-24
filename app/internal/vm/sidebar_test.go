// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func sidebarOver(t *testing.T, stored vm.SidebarFile) (*vm.Sidebar, *vm.Workspace) {
	t.Helper()
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	ws, _ := loadedWorkspace(t, b)
	return vm.NewSidebar(ws, stored), ws
}

// Before: there was no recent list. Opening the same request twice must not list it twice, and the newest goes first.
func TestSidebar_Given_a_request_opened_twice_When_listing_recent_Then_it_is_listed_once_newest_first(t *testing.T) {
	sb, _ := sidebarOver(t, vm.SidebarFile{})

	sb.NoteOpened("r:C1:R1")
	sb.NoteOpened("f:C1:F1")
	sb.NoteOpened("r:C1:R1")
	got := sb.Recent()

	if len(got) != 2 || got[0].ID != "r:C1:R1" || got[1].ID != "f:C1:F1" {
		t.Fatalf("recent = %+v", got)
	}
	if got[0].Where != "Shop / auth" || got[0].TypeName != "HTTP" || got[1].TypeName != "Flow" {
		t.Errorf("rows carry where and type: %+v", got)
	}
}

// Before: nothing was recorded. A collection, a folder or an environment is not something you work on, so it stays out.
func TestSidebar_Given_a_row_that_is_not_a_request_or_flow_When_opened_Then_recent_is_unchanged(t *testing.T) {
	sb, _ := sidebarOver(t, vm.SidebarFile{})

	changed := []bool{sb.NoteOpened("c:C1"), sb.NoteOpened("environments"), sb.NoteOpened("no-such-row")}

	if slices.Contains(changed, true) || len(sb.Recent()) != 0 {
		t.Fatalf("changed %v, recent %+v", changed, sb.Recent())
	}
}

func TestSidebar_Given_a_full_recent_list_When_opening_another_Then_the_oldest_drops_off(t *testing.T) {
	var stored vm.SidebarFile
	for i := range vm.MaxRecent {
		stored.Recent = append(stored.Recent, "r:C1:OLD"+strconv.Itoa(i))
	}
	sb, _ := sidebarOver(t, stored)

	sb.NoteOpened("r:C1:R1")

	got := sb.File().Recent
	if len(got) != vm.MaxRecent || got[0] != "r:C1:R1" || slices.Contains(got, "r:C1:OLD"+strconv.Itoa(vm.MaxRecent-1)) {
		t.Fatalf("recent %v", got)
	}
}

// Before: a request deleted since the last session would have been a dead row in Recent.
func TestSidebar_Given_a_stored_row_that_no_longer_exists_When_listing_Then_it_is_left_out(t *testing.T) {
	sb, _ := sidebarOver(t, vm.SidebarFile{Recent: []string{"r:C1:GONE", "r:C1:R3"}, Favourites: []model.ID{"UNMOUNTED", "C1"}})

	recent, stars := sb.Recent(), sb.Favourites()

	if len(recent) != 1 || recent[0].ID != "r:C1:R3" {
		t.Errorf("recent %+v", recent)
	}
	if !slices.Equal(stars, []string{"c:C1"}) {
		t.Errorf("favourites %v", stars)
	}
}

func TestSidebar_Given_a_collection_When_toggled_twice_Then_it_is_starred_then_not(t *testing.T) {
	sb, _ := sidebarOver(t, vm.SidebarFile{})

	first := sb.ToggleFavourite("c:C1")
	starred := sb.Favourites()
	second := sb.ToggleFavourite("c:C1")

	if !first || !second || !slices.Equal(starred, []string{"c:C1"}) || len(sb.Favourites()) != 0 {
		t.Fatalf("first %v second %v starred %v now %v", first, second, starred, sb.Favourites())
	}
}

func TestSidebar_Given_a_request_row_When_starred_Then_nothing_changes(t *testing.T) {
	sb, _ := sidebarOver(t, vm.SidebarFile{})

	changed := sb.ToggleFavourite("r:C1:R1")

	if changed || len(sb.File().Favourites) != 0 {
		t.Fatalf("a request was starred: %+v", sb.File())
	}
}

// Before: Drafts is where untitled try-outs live; starring it would pin a place that empties itself.
func TestSidebar_Given_the_drafts_space_When_starred_Then_it_is_refused(t *testing.T) {
	scratch := vmtest.SampleCollection()
	scratch.Manifest.ID, scratch.Manifest.Name = "D1", "Scratch"
	b := &vmtest.Backend{Cols: []model.Collection{scratch}, DraftIDs: map[model.ID]bool{"D1": true}}
	ws, _ := loadedWorkspace(t, b)
	sb := vm.NewSidebar(ws, vm.SidebarFile{})

	changed := sb.ToggleFavourite("c:D1")

	if changed || len(sb.Favourites()) != 0 {
		t.Fatalf("the drafts space was starred: %v", sb.Favourites())
	}
}

func TestSessionStore_Given_a_saved_sidebar_When_loaded_Then_it_comes_back(t *testing.T) {
	store := vm.NewSessionStore(t.TempDir())
	want := vm.SidebarFile{Favourites: []model.ID{"C1"}, Recent: []string{"r:C1:R1"}}

	saveErr := store.SaveSidebar(want)
	got, loadErr := store.LoadSidebar()

	if saveErr != nil || loadErr != nil || !slices.Equal(got.Favourites, want.Favourites) || !slices.Equal(got.Recent, want.Recent) {
		t.Fatalf("got %+v (%v, %v)", got, saveErr, loadErr)
	}
}

// Before: n/a (new). A damaged file must come back as an error AND an empty sidebar, so the window can say so and carry on.
func TestSessionStore_Given_a_damaged_sidebar_file_When_loaded_Then_it_is_an_error_and_empty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sidebar.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := vm.NewSessionStore(dir).LoadSidebar()

	if err == nil || len(got.Favourites) != 0 || len(got.Recent) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}
