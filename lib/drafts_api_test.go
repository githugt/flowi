// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/workspace"
)

func TestNewDraft_should_mount_a_collection_under_the_drafts_dir(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)

	m, err := w.NewDraft("Try-out")

	if err != nil {
		t.Fatal(err)
	}
	if !w.IsDraft(m.ID) {
		t.Fatal("not a draft")
	}
	root := w.Collections()[0].Root
	drafts, _ := filepath.EvalSymlinks(workspace.DraftsDir(e.opts.Paths, workspace.DefaultName))
	if !strings.HasPrefix(root, drafts) {
		t.Fatalf("root %s", root)
	}
}

func TestIsDraft_Given_a_collection_in_a_user_folder_Then_false(t *testing.T) {
	w := newEnv(t).open(t)
	m, err := w.InitCollection(filepath.Join(t.TempDir(), "mine"), "mine", true)
	if err != nil {
		t.Fatal(err)
	}

	got := w.IsDraft(m.ID)

	if got {
		t.Fatal("user folder reported as draft")
	}
}

func TestNewScratchRequest_should_create_Scratch_once_and_number_requests(t *testing.T) {
	w := newEnv(t).open(t)

	first, err1 := w.NewScratchRequest()
	second, err2 := w.NewScratchRequest()

	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if len(w.Collections()) != 1 || w.Collections()[0].Manifest.Name != "Scratch" || first.Collection != second.Collection {
		t.Fatalf("collections %+v", w.Collections())
	}
	paths := []string{}
	for _, r := range w.Collections()[0].Requests {
		paths = append(paths, r.Path)
	}
	slices.Sort(paths)
	if strings.Join(paths, ",") != "untitled,untitled-2" {
		t.Fatalf("paths %v", paths)
	}
}

func TestScratch_Given_unmounted_scratch_folder_Then_mounts_it_again(t *testing.T) {
	w := newEnv(t).open(t)
	target, err := w.NewScratchRequest()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unmount(target.Collection); err != nil {
		t.Fatal(err)
	}

	m, err := w.Scratch()

	if err != nil {
		t.Fatal(err)
	}
	if m.ID != target.Collection || len(w.Collections()[0].Requests) != 1 {
		t.Fatalf("got %s with %+v", m.ID, w.Collections())
	}
}

func TestDrafts_should_survive_reopening(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	m, err := w.NewDraft("kept")
	if err != nil {
		t.Fatal(err)
	}

	again := e.open(t)

	if !again.IsDraft(m.ID) || len(again.Collections()) != 1 {
		t.Fatalf("collections %+v", again.Collections())
	}
}

func TestSaveDraft_should_move_files_keep_id_and_stop_being_a_draft(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	target, err := w.NewScratchRequest()
	if err != nil {
		t.Fatal(err)
	}
	draftRoot := w.Collections()[0].Root
	dir := filepath.Join(t.TempDir(), "saved")

	err = w.SaveDraft(target.Collection, dir)

	if err != nil {
		t.Fatal(err)
	}
	if w.IsDraft(target.Collection) {
		t.Fatal("still a draft")
	}
	if _, err := os.Stat(filepath.Join(dir, "requests", "untitled.req.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(draftRoot); !os.IsNotExist(err) {
		t.Fatalf("draft dir left behind: %v", err)
	}
	again := e.open(t)
	if again.Collections()[0].Manifest.ID != target.Collection || len(again.Collections()[0].Requests) != 1 {
		t.Fatalf("reopened %+v", again.Collections())
	}
}

func TestSaveDraft_Given_a_bad_target_Then_refused_and_draft_untouched(t *testing.T) {
	w := newEnv(t).open(t)
	m, err := w.NewDraft("d")
	if err != nil {
		t.Fatal(err)
	}
	root := w.Collections()[0].Root
	full := t.TempDir()
	if err := os.WriteFile(filepath.Join(full, "keep.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"non-empty folder": full,
		"inside the draft": filepath.Join(root, "requests", "nested"),
		"the draft itself": root,
	}
	for name, dir := range cases {
		t.Run(name, func(t *testing.T) {
			err := w.SaveDraft(m.ID, dir)

			if err == nil {
				t.Fatal("accepted")
			}
			if !w.IsDraft(m.ID) {
				t.Fatal("no longer a draft")
			}
			if _, err := os.Stat(filepath.Join(root, "flowi.yaml")); err != nil {
				t.Fatal(err)
			}
		})
	}
	if data, _ := os.ReadFile(filepath.Join(full, "keep.txt")); string(data) != "mine" {
		t.Fatal("user file changed")
	}
}

func TestSaveDraft_Given_an_unwritable_target_Then_no_partial_folder(t *testing.T) {
	w := newEnv(t).open(t)
	m, err := w.NewDraft("d")
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) }) // let TempDir clean up

	err = w.SaveDraft(m.ID, filepath.Join(parent, "saved"))

	if err == nil {
		t.Skip("running with permissions that ignore mode bits")
	}
	if !w.IsDraft(m.ID) {
		t.Fatal("no longer a draft")
	}
	if _, err := os.Stat(filepath.Join(parent, "saved")); !os.IsNotExist(err) {
		t.Fatalf("partial folder: %v", err)
	}
}

func TestSaveDraft_Given_a_user_collection_Then_refused(t *testing.T) {
	w := newEnv(t).open(t)
	m, err := w.InitCollection(filepath.Join(t.TempDir(), "mine"), "mine", true)
	if err != nil {
		t.Fatal(err)
	}

	err = w.SaveDraft(m.ID, filepath.Join(t.TempDir(), "elsewhere"))

	if err == nil {
		t.Fatal("moved a user collection")
	}
}

func TestDiscardDraft_should_unmount_and_delete_only_drafts(t *testing.T) {
	w := newEnv(t).open(t)
	draft, err := w.NewDraft("d")
	if err != nil {
		t.Fatal(err)
	}
	draftRoot := w.Collections()[0].Root
	userDir := filepath.Join(t.TempDir(), "mine")
	mine, err := w.InitCollection(userDir, "mine", true)
	if err != nil {
		t.Fatal(err)
	}

	errDraft := w.DiscardDraft(draft.ID)
	errMine := w.DiscardDraft(mine.ID)

	if errDraft != nil {
		t.Fatal(errDraft)
	}
	if errMine == nil {
		t.Fatal("discarded a user collection")
	}
	if _, err := os.Stat(draftRoot); !os.IsNotExist(err) {
		t.Fatalf("draft dir left: %v", err)
	}
	if _, err := os.Stat(filepath.Join(userDir, "flowi.yaml")); err != nil {
		t.Fatal(err)
	}
	ids := []model.ID{}
	for _, c := range w.Collections() {
		ids = append(ids, c.Manifest.ID)
	}
	if len(ids) != 1 || ids[0] != mine.ID {
		t.Fatalf("mounted %v", ids)
	}
}

func TestNewScratchRequest_Given_concurrent_calls_Then_one_Scratch_and_distinct_names(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	const calls = 8
	errs := make(chan error, calls)

	var wg sync.WaitGroup
	for range calls {
		wg.Go(func() { _, err := w.NewScratchRequest(); errs <- err })
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	again := e.open(t)
	if len(again.Collections()) != 1 || len(again.Collections()[0].Requests) != calls {
		t.Fatalf("collections %+v", again.Collections())
	}
	if err := again.DiscardDraft(again.Collections()[0].Manifest.ID); err != nil {
		t.Fatalf("scratch unusable after reopen: %v", err)
	}
}

func TestSaveDraft_Given_a_target_inside_Flow_I_own_dir_Then_refused(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	a, err := w.NewDraft("a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.NewDraft("b"); err != nil {
		t.Fatal(err)
	}
	var bRoot string
	for _, c := range w.Collections() {
		if c.Manifest.ID != a.ID {
			bRoot = c.Root
		}
	}
	wsDir := workspace.Dir(e.opts.Paths, workspace.DefaultName)
	for name, dir := range map[string]string{
		"inside another draft": filepath.Join(bRoot, "sub"),
		"the drafts dir":       filepath.Join(wsDir, "drafts", "mine"),
		"the data dir":         filepath.Join(wsDir, "data", "x"),
	} {
		t.Run(name, func(t *testing.T) {
			err := w.SaveDraft(a.ID, dir)

			if err == nil {
				t.Fatalf("saved into %s", dir)
			}
			if !w.IsDraft(a.ID) {
				t.Fatal("no longer a draft")
			}
		})
	}
}

func TestDraftsDir_should_belong_to_the_default_user(t *testing.T) {
	e := newEnv(t)

	dir := workspace.DraftsDir(e.opts.Paths, workspace.DefaultName)

	if !strings.HasSuffix(filepath.ToSlash(dir), "/users/"+workspace.DefaultUser+"/drafts") {
		t.Fatalf("drafts dir %s", dir)
	}
}

// Before this change drafts lived directly under the workspace directory. They
// stay where they are (they are mounted by path) and stay drafts.
func TestIsDraft_Given_a_draft_from_before_users_Then_still_a_draft_and_Scratch_is_reused(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	legacyScratch := filepath.Join(workspace.Dir(e.opts.Paths, workspace.DefaultName), "drafts", "scratch")
	m, err := w.InitCollection(legacyScratch, "Scratch", true)
	if err != nil {
		t.Fatal(err)
	}

	again, scratchErr := w.Scratch()

	if scratchErr != nil || !w.IsDraft(m.ID) || again.ID != m.ID || len(w.Collections()) != 1 {
		t.Fatalf("err %v draft %v ids %s/%s collections %d", scratchErr, w.IsDraft(m.ID), again.ID, m.ID, len(w.Collections()))
	}
}
