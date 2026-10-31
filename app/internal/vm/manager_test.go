// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func newManager(t *testing.T) (*vm.Manager, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Manifest.DependsOn = []model.Dependency{{ID: "D1", Name: "identity", Repo: "https://example.com/identity.git"}, {ID: "C2", Name: "billing"}}
	other := model.Collection{Root: "/b", Manifest: model.Manifest{Meta: model.Meta{ID: "C2", Name: "billing"}}}
	b := &vmtest.Backend{Cols: []model.Collection{c, other}, MissingIDs: []model.ID{"GONE"},
		Paths: map[model.ID]string{"C1": "~/shop", "C2": "~/billing", "GONE": "~/old"}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	m := vm.NewManager(b, w, ws)
	m.Load()
	w.Wait()
	return m, b, w
}

func TestManagerVM_lists_collections_missing_and_dependency_status(t *testing.T) {
	m, _, _ := newManager(t)

	rows := m.Rows()

	if len(rows) != 3 || rows[0].Name != "Shop" || rows[0].Path != "~/shop" || rows[2].Path != "~/old" || !rows[2].Missing {
		t.Fatalf("rows %+v", rows)
	}
	deps := rows[0].Deps
	if len(deps) != 2 || deps[0].Mounted || deps[0].Repo == "" || !deps[1].Mounted {
		t.Fatalf("deps %+v", deps)
	}
}

func TestManagerVM_mount_unmount_relocate_init_call_the_backend_and_reload(t *testing.T) {
	m, b, w := newManager(t)

	m.Mount("/new")
	m.Unmount("C2")
	m.Relocate("GONE", "/found")
	m.Init("/fresh", "Fresh")
	w.Wait()

	for _, call := range []string{"Mount /new", "Unmount C2", "Relocate GONE /found", "InitCollection /fresh Fresh"} {
		if !b.Called(call) {
			t.Fatalf("missing %q in %v", call, b.Calls)
		}
	}
	if m.Err() != "" {
		t.Fatalf("err %q", m.Err())
	}
}

func TestManagerVM_NewCollection_should_create_in_the_location_or_in_flowis_own_folder(t *testing.T) {
	b := &vmtest.Backend{}
	w := vmtest.SyncWorker()
	m := vm.NewManager(b, w, vm.NewWorkspace(b, w))

	m.NewCollection("Pets", "")
	m.NewCollection("Shop", "/work/shop")
	w.Wait()

	if !slices.Contains(b.Calls, "NewDraft Pets") || !slices.Contains(b.Calls, "InitCollection /work/shop Shop") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestWorkspaceVM_InScratch_should_be_true_only_for_the_drafts_space(t *testing.T) {
	b := draftsBackend()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()

	if !ws.InScratch("r:S1:SR1") || ws.InScratch("r:C1:R1") {
		t.Fatal("scratch detection")
	}
}

func TestManagerVM_SaveDraftOr_Given_a_failure_Then_hands_the_error_to_the_caller(t *testing.T) {
	b := draftsBackend()
	b.NewErr = errBoom
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	m := vm.NewManager(b, w, ws)
	var failed error

	m.DiscardOr("S1", func(err error) { failed = err })
	w.Wait()

	if failed == nil {
		t.Fatal("the error must reach the caller, not only the manager screen")
	}
}

func TestManagerVM_open_bundle_calls_the_backend_and_reloads_so_the_replay_collection_shows(t *testing.T) {
	m, b, w := newManager(t)

	m.OpenBundle("/runs/login.flowi-run")
	w.Wait()

	if !b.Called("OpenBundle /runs/login.flowi-run") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestManagerVM_open_arazzo_calls_the_backend_with_the_reference_and_reloads(t *testing.T) {
	m, b, w := newManager(t)

	m.OpenArazzo("/docs/check.arazzo.yaml")
	w.Wait()

	if !b.Called("OpenArazzo /docs/check.arazzo.yaml") {
		t.Fatalf("calls %v", b.Calls)
	}
}
