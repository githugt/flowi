// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func datasetBackend() *vmtest.Backend {
	b := asyncBackend()
	b.Cols[0].Requests[3].Versions[0].Async = nil
	b.Cols[0].Requests[3].Versions[0].Dataset = &model.Dataset{
		Driver: "postgres", Server: "postgres://app@db:5432/shop", Query: "SELECT id FROM customers WHERE id = $1", Params: []string{"{{id}}"}}
	return b
}

func TestVerb_should_be_SQL_for_a_dataset_version(t *testing.T) {
	if got := vm.Verb(model.RequestVersion{Dataset: &model.Dataset{}}); got != "SQL" {
		t.Fatalf("verb %q", got)
	}
}

func TestRequestVM_should_edit_the_dataset_block_without_touching_what_is_on_disk(t *testing.T) {
	r, _ := asyncRequestVM(t, datasetBackend())

	r.EditDataset(func(d *model.Dataset) { d.Query = "SELECT 2"; d.Params = append(d.Params, "x") })
	edited := r.Dirty() && r.Dataset().Query == "SELECT 2"
	r.Revert()
	back := r.Dataset()

	if !r.IsDataset() || r.IsAsync() || r.IsGRPC() || !edited {
		t.Fatalf("dataset %v async %v grpc %v edited %v", r.IsDataset(), r.IsAsync(), r.IsGRPC(), edited)
	}
	if back.Query != "SELECT id FROM customers WHERE id = $1" || len(back.Params) != 1 {
		t.Fatalf("revert did not restore the block: %+v", back)
	}
}

func TestRequestVM_Configured_should_mark_the_dataset_sections_that_have_content(t *testing.T) {
	r, _ := asyncRequestVM(t, datasetBackend())

	got := map[vm.Section]bool{vm.SectionQuery: r.Configured(vm.SectionQuery), vm.SectionValues: r.Configured(vm.SectionValues)}

	if !got[vm.SectionQuery] || !got[vm.SectionValues] {
		t.Fatalf("configured %v", got)
	}
}

func TestWorkspace_CreateDatasetRequest_should_create_through_the_backend_and_seed_the_hooks(t *testing.T) {
	b := sampleBackend()
	ws, w := loadedWorkspace(t, b)

	ws.CreateDatasetRequest("C1", "customers/list", "sqlite", func(string) {}, func(err error) { t.Fatal(err) })
	w.Wait()

	if !b.Called("NewDatasetRequest customers/list sqlite") || !b.Called("SeedRequestHooks new") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestMenuFor_should_offer_a_new_dataset_request_where_it_offers_a_new_grpc_one(t *testing.T) {
	ws, _, _ := shopWorkspace(t)

	for _, id := range []string{"c:C1", "c:C1/requests/auth"} {
		if got := actionsOf(ws.MenuFor([]string{id})); !slices.Contains(got, vm.ActNewDatasetRequest) {
			t.Errorf("%s menu %v lacks a new dataset request", id, got)
		}
	}
}
