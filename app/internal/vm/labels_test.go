// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func labelledBackend() *vmtest.Backend {
	c := vmtest.SampleCollection()
	c.Folders = []model.Folder{{Dir: "requests/auth", Status: "in-development", Tags: []string{"auth"}}, {Dir: "flows", Status: "ghost"}}
	c.Requests[0].Tags = []string{"smoke"}
	c.Requests[1].Status = "review"
	c.Manifest.Statuses = []model.Status{{Name: "needs-design", Color: "pink", Icon: "bookmark"}}
	return &vmtest.Backend{Cols: []model.Collection{c}}
}

// Before: a tree row knew its label and its verb and nothing about where the work stood.
func TestWorkspaceVM_tree_rows_should_carry_the_resolved_status_and_the_tags_that_apply(t *testing.T) {
	ws, _ := loadedWorkspace(t, labelledBackend())

	inherits := ws.Node("r:C1:R1")
	own := ws.Node("r:C1:R2")
	none := ws.Node("r:C1:R3")
	folder := ws.Node("c:C1/requests/auth")

	if inherits.Status.Name != "in-development" || inherits.Status.Icon != "code" || inherits.StatusFrom != "requests/auth" || inherits.StatusUnknown {
		t.Errorf("inherited: %+v from %q", inherits.Status, inherits.StatusFrom)
	}
	if !slices.Equal(inherits.Tags, []string{"auth", "smoke"}) {
		t.Errorf("tags of an item under a folder with tags: %v", inherits.Tags)
	}
	if own.Status.Name != "review" || own.StatusFrom != "" {
		t.Errorf("own: %+v from %q", own.Status, own.StatusFrom)
	}
	if none.Status.Name != "" {
		t.Errorf("none: %+v", none.Status)
	}
	if folder.Status.Name != "in-development" || folder.StatusFrom != "" || !slices.Equal(folder.Tags, []string{"auth"}) {
		t.Errorf("folder: %+v %v", folder.Status, folder.Tags)
	}
}

func TestWorkspaceVM_a_status_the_collection_does_not_have_should_show_as_unknown_not_as_none(t *testing.T) {
	ws, _ := loadedWorkspace(t, labelledBackend())

	flow := ws.Node("f:C1:F1")

	if flow.Status.Name != "ghost" || !flow.StatusUnknown || flow.Status.Icon != "alert" || flow.StatusFrom != "flows" {
		t.Fatalf("%+v unknown %v from %q", flow.Status, flow.StatusUnknown, flow.StatusFrom)
	}
}

func TestWorkspaceVM_Labels_should_give_the_own_values_what_is_inherited_and_the_choices(t *testing.T) {
	ws, _ := loadedWorkspace(t, labelledBackend())

	l, err := ws.Labels("r:C1:R1")
	folder, ferr := ws.Labels("c:C1/requests/auth")
	_, bad := ws.Labels("c:C1")

	if err != nil || ferr != nil {
		t.Fatal(err, ferr)
	}
	if !slices.Equal(l.Tags, []string{"smoke"}) || l.Status != "" || l.Inherited.Name != "in-development" || l.InheritedFrom != "requests/auth" {
		t.Errorf("request: %+v", l)
	}
	if !slices.Equal(l.FolderTags, []string{"auth"}) {
		t.Errorf("folder tags: %v", l.FolderTags)
	}
	if len(l.Statuses) != 6 || l.Statuses[5].Name != "needs-design" {
		t.Errorf("statuses: %+v", l.Statuses)
	}
	if len(l.Suggest) != 2 || l.Suggest[0].Tag != "auth" {
		t.Errorf("suggest: %+v", l.Suggest)
	}
	if folder.Status != "in-development" || !slices.Equal(folder.Tags, []string{"auth"}) || len(folder.FolderTags) != 0 {
		t.Errorf("folder: %+v", folder)
	}
	if !errors.Is(bad, vm.ErrNoLabels) {
		t.Errorf("a collection row has no labels: %v", bad)
	}
}

func TestWorkspaceVM_SetLabels_should_write_through_the_backend_and_show_the_tree_again(t *testing.T) {
	b := labelledBackend()
	ws, w := loadedWorkspace(t, b)
	got := errors.New("not called")

	ws.SetLabels("r:C1:R3", []string{"slow", "slow", "needs review"}, "testing", false, func(err error) { got = err })
	w.Wait()
	ws.SetLabels("c:C1/requests/auth", []string{"auth"}, "review", false, func(error) {})
	ws.SetLabels("f:C1:F1", nil, "", false, func(error) {})
	w.Wait()

	if got != nil {
		t.Fatal(got)
	}
	if !b.Called("SetRequestLabels R3 slow,slow,needs review testing") || !b.Called("SetFolderLabels requests/auth auth review") || !b.Called("SetFlowLabels F1  ") {
		t.Fatalf("calls %v", b.Calls)
	}
	if n := ws.Node("r:C1:R3"); n.Status.Name != "testing" || !slices.Equal(n.Tags, []string{"needs-review", "slow"}) {
		t.Errorf("the tree row: %+v %v", n.Status, n.Tags)
	}
	if n := ws.Node("r:C1:R1"); n.Status.Name != "review" || n.StatusFrom != "requests/auth" {
		t.Errorf("a row under the folder takes the folder's new status: %+v from %q", n.Status, n.StatusFrom)
	}
}

func TestWorkspaceVM_SetLabels_Given_an_unknown_status_Then_the_error_comes_back_and_nothing_changes(t *testing.T) {
	ws, w := loadedWorkspace(t, labelledBackend())
	var got error

	ws.SetLabels("r:C1:R3", nil, "revew", false, func(err error) { got = err })
	w.Wait()

	if got == nil || ws.Node("r:C1:R3").Status.Name != "" {
		t.Fatalf("error %v status %+v", got, ws.Node("r:C1:R3").Status)
	}
}

func TestWorkspaceVM_AddStatus_should_add_one_keep_the_others_and_make_it_usable(t *testing.T) {
	b := labelledBackend()
	ws, w := loadedWorkspace(t, b)
	added, used := errors.New("x"), errors.New("x")

	ws.AddStatus("r:C1:R3", model.Status{Name: "blocked", Color: "red", Icon: "pause"}, false, func(err error) { added = err })
	w.Wait()
	ws.SetLabels("r:C1:R3", nil, "blocked", false, func(err error) { used = err })
	w.Wait()

	if added != nil || used != nil {
		t.Fatal(added, used)
	}
	list := b.Cols[0].Manifest.Statuses
	if len(list) != 2 || list[0].Name != "needs-design" || list[1].Name != "blocked" {
		t.Fatalf("statuses %+v", list)
	}
	if n := ws.Node("r:C1:R3"); n.Status.Icon != "pause" || n.Status.Color != "red" {
		t.Fatalf("row %+v", n.Status)
	}
}

// Before: a change to the tags or the status counted as a change on disk, so an open request with edits asked which
// version to keep, and an open flow with edits was refused its next save.
func TestRequestVM_Reloaded_Given_only_the_labels_changed_under_edits_Then_the_edits_stay_and_there_is_no_conflict(t *testing.T) {
	b := sampleBackend()
	r, w := newRequestVM(t, b)
	disk := b.Cols[0].Requests[0]
	disk.Tags, disk.Status = []string{"smoke"}, "review"

	r.Edit(func(v *model.RequestVersion) { v.URL = "{{env.base_url}}/signin" })
	r.Reloaded(b.Cols[0], disk, true)
	r.Save(false)
	w.Wait()

	if r.ChangedOnDisk() || !r.Dirty() && len(b.Saved) != 1 {
		t.Fatalf("changed on disk %v, saved %d", r.ChangedOnDisk(), len(b.Saved))
	}
	saved := b.Saved[0]
	if saved.Status != "review" || !slices.Equal(saved.Tags, []string{"smoke"}) || saved.Versions[0].URL != "{{env.base_url}}/signin" {
		t.Fatalf("saved %+v: the edit kept, and the labels written meanwhile not undone", saved)
	}
}

func TestFlowDoc_Reloaded_Given_only_the_labels_changed_under_edits_Then_the_edits_stay_undo_does_not_bring_them_back(t *testing.T) {
	d, b, w := newFlowDoc(t)
	disk := b.Cols[0].Flows[0]
	disk.Tags, disk.Status = []string{"nightly"}, "testing"

	d.Delete("a")
	d.Reloaded(b.Cols[0], disk, true)
	flagged := d.ChangedOnDisk()
	d.Undo()
	d.Delete("b")
	d.Save(false)
	w.Wait()

	if flagged || d.ChangedOnDisk() {
		t.Fatalf("flagged %v / after save %v", flagged, d.ChangedOnDisk())
	}
	if len(b.SavedFlows) != 1 || b.SavedFlows[0].Status != "testing" || !slices.Equal(b.SavedFlows[0].Tags, []string{"nightly"}) {
		t.Fatalf("saved %+v", b.SavedFlows)
	}
}
