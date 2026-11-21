// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"
)

// scratchRequestAged makes a try-out request in Scratch and sets its file's age.
func scratchRequestAged(t *testing.T, w *Workspace, age time.Duration) (RequestTarget, string) {
	t.Helper()
	target, err := w.NewScratchRequest()
	if err != nil {
		t.Fatal(err)
	}
	_, r, err := w.requestFor(target)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age)
	if err := os.Chtimes(r.File, old, old); err != nil {
		t.Fatal(err)
	}
	return target, r.File
}

func scratchRequests(t *testing.T, w *Workspace) []model.Request {
	t.Helper()
	m, err := w.Scratch()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range w.Collections() {
		if c.Manifest.ID == m.ID {
			return c.Requests
		}
	}
	return nil
}

func TestTrashStaleDrafts_should_move_old_unused_drafts_to_the_trash_and_keep_the_rest(t *testing.T) {
	w := demoWorkspace(t)
	_, oldFile := scratchRequestAged(t, w, 30*24*time.Hour)
	inUse, _ := scratchRequestAged(t, w, 30*24*time.Hour) // old, but open in a tab
	scratchRequestAged(t, w, time.Hour)
	openID := inUse.Request

	moved, err := w.TrashStaleDrafts(7*24*time.Hour, func(kind string, id model.ID) bool { return kind == TrashRequest && id == openID })

	if err != nil || len(moved) != 1 || moved[0].Kind != TrashRequest || moved[0].Path == "" {
		t.Fatalf("moved %+v, %v", moved, err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("the old draft is still in Scratch: %v", err)
	}
	left := scratchRequests(t, w)
	if len(left) != 2 {
		t.Fatalf("Scratch holds %d requests, want the open one and the fresh one", len(left))
	}
	trash, err := w.ListTrash()
	if err != nil || len(trash) != 1 || trash[0].ID != moved[0].ID {
		t.Fatalf("trash %+v, %v", trash, err)
	}
}

func TestTrashStaleDrafts_Given_no_limit_Then_moves_nothing(t *testing.T) {
	w := demoWorkspace(t)
	scratchRequestAged(t, w, 400*24*time.Hour)

	moved, err := w.TrashStaleDrafts(0, nil)

	if err != nil || len(moved) != 0 || len(scratchRequests(t, w)) != 1 {
		t.Fatalf("moved %+v, %v", moved, err)
	}
}

func TestRestoreFromTrash_should_bring_the_draft_back_under_its_name_or_a_free_one(t *testing.T) {
	w := demoWorkspace(t)
	scratchRequestAged(t, w, 30*24*time.Hour)
	moved, _ := w.TrashStaleDrafts(24*time.Hour, nil)
	if len(moved) != 1 {
		t.Fatalf("moved %+v", moved)
	}
	first, err := w.RestoreFromTrash(moved[0].ID)
	again, _ := w.NewScratchRequest()
	_ = again

	if err != nil || first != moved[0].Path {
		t.Fatalf("restored as %q (was %q), %v", first, moved[0].Path, err)
	}
	if trash, _ := w.ListTrash(); len(trash) != 0 {
		t.Fatalf("the restored item is still in the trash: %+v", trash)
	}
	if got := scratchRequests(t, w); len(got) != 2 {
		t.Fatalf("Scratch holds %d requests, want the restored one and the new one", len(got))
	}
}

func TestDeleteFromTrash_and_EmptyTrash_should_remove_for_good_and_refuse_odd_ids(t *testing.T) {
	w := demoWorkspace(t)
	scratchRequestAged(t, w, 30*24*time.Hour)
	scratchRequestAged(t, w, 30*24*time.Hour)
	moved, _ := w.TrashStaleDrafts(24*time.Hour, nil)
	if len(moved) != 2 {
		t.Fatalf("moved %+v", moved)
	}

	errOne := w.DeleteFromTrash(moved[0].ID)
	errOdd := w.DeleteFromTrash("../x")
	afterOne, _ := w.ListTrash()
	errAll := w.EmptyTrash()
	afterAll, _ := w.ListTrash()

	if errOne != nil || errOdd == nil || len(afterOne) != 1 || errAll != nil || len(afterAll) != 0 {
		t.Fatalf("errors %v %v %v, left %d then %d", errOne, errOdd, errAll, len(afterOne), len(afterAll))
	}
	if _, err := w.RestoreFromTrash("../x"); err == nil {
		t.Fatal("an odd id was restored")
	}
}

// Before: RestoreFromTrash joined the Files and Path of item.json into paths
// unchecked, so an edited item.json could write outside Scratch. Now such an
// item is left out of the trash list and refused on restore.
func TestRestoreFromTrash_Given_a_tampered_item_When_restoring_Then_refuses_and_writes_nothing(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TrashedItem)
	}{
		{"file climbs out", func(i *TrashedItem) { i.Files = append(i.Files, "requests/../../../escape.txt") }},
		{"absolute file", func(i *TrashedItem) { i.Files = []string{"/tmp/escape.txt"} }},
		{"file outside the item's path", func(i *TrashedItem) { i.Files = []string{"requests/other.req.yaml"} }},
		{"path climbs out", func(i *TrashedItem) { i.Path = "../x" }},
		{"unknown kind", func(i *TrashedItem) { i.Kind = "secret" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := demoWorkspace(t)
			scratchRequestAged(t, w, 30*24*time.Hour)
			moved, _ := w.TrashStaleDrafts(24*time.Hour, nil)
			if len(moved) != 1 {
				t.Fatalf("moved %+v", moved)
			}
			item := moved[0]
			tt.mutate(&item)
			data, _ := json.Marshal(item)
			if err := os.WriteFile(filepath.Join(w.trashDir(), item.ID, "item.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := w.RestoreFromTrash(item.ID)
			listed, _ := w.ListTrash()
			deleteErr := w.DeleteFromTrash(item.ID)

			if err == nil || len(listed) != 0 {
				t.Fatalf("restore error %v, %d listed", err, len(listed))
			}
			if deleteErr != nil {
				t.Fatalf("a refused item must still be deletable: %v", deleteErr)
			}
		})
	}
}

// Before: deleting a request or flow in the tree removed its files at once and nothing could bring them back.
func TestDeleteRequestToTrash_should_keep_the_files_and_restore_them_to_the_same_collection(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)

	item, err := w.DeleteRequestToTrash(target)

	_, _, gone := w.requestFor(target)
	_, statErr := os.Stat(source.File)
	if err != nil || gone == nil || !os.IsNotExist(statErr) || item.Collection != target.Collection || item.Path != source.Path {
		t.Fatalf("err %v still found %v stat %v item %+v", err, gone == nil, statErr, item)
	}
	path, err := w.RestoreFromTrash(item.ID)
	back, backErr := os.Stat(source.File)
	if err != nil || path != source.Path || backErr != nil || back.Size() == 0 {
		t.Fatalf("restored as %q, %v; file %v", path, err, backErr)
	}
	if trash, _ := w.ListTrash(); len(trash) != 0 {
		t.Fatalf("the restored item is still in the trash: %+v", trash)
	}
}

func TestRestoreFromTrash_should_use_a_free_name_when_a_new_request_took_the_old_one(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)
	item, err := w.DeleteRequestToTrash(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.NewRequest(target.Collection, source.Path); err != nil {
		t.Fatalf("making a new request at the freed path: %v", err)
	}

	path, err := w.RestoreFromTrash(item.ID)

	if err != nil || path != source.Path+"-restored" {
		t.Fatalf("restored as %q, %v", path, err)
	}
}

func TestDeleteFlowToTrash_should_take_the_layout_along_and_bring_both_back(t *testing.T) {
	w := demoWorkspace(t)
	target, source := userCreation(t, w)
	layout := layoutFile(source.File)
	if err := os.WriteFile(layout, []byte("nodes: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	item, err := w.DeleteFlowToTrash(target)

	_, flowStat := os.Stat(source.File)
	_, layoutStat := os.Stat(layout)
	if err != nil || !os.IsNotExist(flowStat) || !os.IsNotExist(layoutStat) || len(item.Files) != 2 {
		t.Fatalf("err %v flow %v layout %v item %+v", err, flowStat, layoutStat, item)
	}
	if _, err := w.RestoreFromTrash(item.ID); err != nil {
		t.Fatal(err)
	}
	_, flowBack := os.Stat(source.File)
	_, layoutBack := os.Stat(layout)
	if flowBack != nil || layoutBack != nil {
		t.Fatalf("after restoring: flow %v layout %v", flowBack, layoutBack)
	}
}

func TestRestoreFromTrash_Given_the_collection_is_gone_Then_puts_the_deleted_request_in_Scratch(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	item, err := w.DeleteRequestToTrash(target)
	if err != nil {
		t.Fatal(err)
	}
	item.Collection = "no-such-collection"
	data, _ := json.Marshal(item)
	if err := os.WriteFile(filepath.Join(w.trashDir(), item.ID, "item.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	path, err := w.RestoreFromTrash(item.ID)

	if err != nil || path == "" || len(scratchRequests(t, w)) != 1 {
		t.Fatalf("restored %q, %v; Scratch holds %d", path, err, len(scratchRequests(t, w)))
	}
}

func TestDeleteRequestToTrash_Given_an_unknown_request_Then_nothing_is_trashed(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	target.Request = "nope"

	_, err := w.DeleteRequestToTrash(target)

	if err == nil {
		t.Fatal("an unknown request must be an error")
	}
	if trash, _ := w.ListTrash(); len(trash) != 0 {
		t.Fatalf("trash %+v", trash)
	}
}
