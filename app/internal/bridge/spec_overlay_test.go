// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"errors"
	"slices"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
)

// fakeSpecs is SpecOverlays with canned answers; it records what was applied.
type fakeSpecs struct {
	origin    flowi.SpecOrigin
	originErr error
	plan      flowi.OverlayPlan
	update    flowi.ImportPlan
	applied   [][]int // chosen changes, per ApplySpecOverlay
	imported  []flowi.ImportPlan
	saw       model.RequestVersion // the version PreviewSpecOverlay was given
}

func (f *fakeSpecs) SpecOrigin(flowi.RequestTarget, string) (flowi.SpecOrigin, error) {
	return f.origin, f.originErr
}

func (f *fakeSpecs) PreviewSpecOverlay(_ flowi.RequestTarget, v model.RequestVersion) (flowi.OverlayPlan, error) {
	f.saw = v
	return f.plan, nil
}

func (f *fakeSpecs) ApplySpecOverlay(_ flowi.OverlayPlan, chosen []int) error {
	f.applied = append(f.applied, chosen)
	return nil
}

func (f *fakeSpecs) KeptSpecs(model.ID) ([]flowi.KeptSpec, error) {
	return []flowi.KeptSpec{{Name: "pets", Rel: "descriptions/pets.openapi.yaml", Overlays: []string{"/c/descriptions/pets.overlay.yaml"}, Edits: "/c/descriptions/pets.edits.overlay.yaml"}}, nil
}

func (f *fakeSpecs) PreviewSpecUpdate(model.ID, string, []byte) (flowi.ImportPlan, error) {
	return f.update, nil
}

func (f *fakeSpecs) ApplyImport(p flowi.ImportPlan, _ flowi.ApplyOptions) error {
	f.imported = append(f.imported, p)
	return nil
}

func TestRequestService_SpecOrigin_should_list_the_fields_overlays_set(t *testing.T) {
	tests := []struct {
		name     string
		specs    *fakeSpecs
		wantSpec string
		wantN    int
	}{
		{name: "from a kept spec", specs: &fakeSpecs{origin: flowi.SpecOrigin{Spec: "descriptions/pets.openapi.yaml", Name: "pets",
			Fields: []flowi.OverlayField{{Section: "query", Key: "page", File: "descriptions/pets.edits.overlay.yaml"}}}},
			wantSpec: "descriptions/pets.openapi.yaml", wantN: 1},
		{name: "from no spec", specs: &fakeSpecs{originErr: flowi.ErrNotFromSpec}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, v, _, _, _ := openLoginOptions(t, bridge.WithSpecOverlays(tt.specs))

			got := reqs.SpecOrigin(v.Node)

			if got.Spec != tt.wantSpec || len(got.Fields) != tt.wantN {
				t.Fatalf("origin %+v", got)
			}
		})
	}
}

func TestRequestService_SaveToOverlay_should_save_then_write_the_chosen_changes(t *testing.T) {
	specs := &fakeSpecs{plan: flowi.OverlayPlan{Spec: flowi.KeptSpec{Rel: "descriptions/pets.openapi.yaml", Edits: "/c/descriptions/pets.edits.overlay.yaml"},
		Changes: []flowi.OverlayChange{{What: "the docs"}, {What: "the body example"}}}}
	reqs, v, pub, _, _ := openLoginOptions(t, bridge.WithSpecOverlays(specs))
	edit := v.Current
	edit.Docs = "mine"
	reqs.Edit(v.Node, edit)

	err := reqs.SaveToOverlay(v.Node, []int{1})

	if err != nil || len(specs.applied) != 1 || !slices.Equal(specs.applied[0], []int{1}) || specs.saw.Docs != "mine" {
		t.Fatalf("err %v applied %v saw docs %q", err, specs.applied, specs.saw.Docs)
	}
	if got, _ := reqs.Open(v.Node); got.Dirty || !slices.Contains(pub.names, "spec-origin") {
		t.Fatalf("dirty %v events %v", got.Dirty, pub.names)
	}
}

func TestRequestService_PreviewOverlay_Given_no_specs_Then_an_error(t *testing.T) {
	reqs, v, _, _, _ := openLoginOptions(t)

	_, err := reqs.PreviewOverlay(v.Node)

	if err == nil {
		t.Fatal("want an error without specs")
	}
}

func TestManagerService_PreviewSpecUpdate_should_leave_out_files_it_writes_back_unchanged(t *testing.T) {
	specs := &fakeSpecs{update: flowi.ImportPlan{Files: []flowi.PlannedFile{
		{Rel: "requests/pets/getPet.req.yaml", Exists: true, Same: true},
		{Rel: "requests/pets/listPets.req.yaml", Exists: true},
		{Rel: "requests/owners/listOwners.req.yaml"},
		{Rel: "requests/pets/getPet.schemas/1-404.json", Exists: true, Remove: true},
	}}}
	_, _, _, app, _ := openLoginOptions(t, bridge.WithSpecOverlays(specs))
	mgr := app.Manager()

	got, err := mgr.PreviewSpecUpdate("C1", "pets", "")

	if err != nil || len(got.Changed) != 1 || len(got.Added) != 1 || len(got.Removed) != 1 {
		t.Fatalf("view %+v err %v", got, err)
	}
	if err := mgr.ApplySpecUpdate("C1", "pets"); err != nil || len(specs.imported) != 1 {
		t.Fatalf("apply err %v imported %d", err, len(specs.imported))
	}
}

func TestManagerService_ApplySpecUpdate_Given_no_preview_Then_an_error(t *testing.T) {
	_, _, _, app, _ := openLoginOptions(t, bridge.WithSpecOverlays(&fakeSpecs{}))

	err := app.Manager().ApplySpecUpdate("C1", "pets")

	if err == nil || errors.Is(err, flowi.ErrNoKeptSpec) {
		t.Fatalf("err %v, want: preview first", err)
	}
}

func TestManagerService_KeptSpecs_should_name_the_overlays_relative_to_the_collection(t *testing.T) {
	_, _, _, app, _ := openLoginOptions(t, bridge.WithSpecOverlays(&fakeSpecs{}))

	got, err := app.Manager().KeptSpecs("C1")

	if err != nil || len(got) != 1 || !slices.Equal(got[0].Overlays, []string{"descriptions/pets.overlay.yaml"}) {
		t.Fatalf("specs %+v err %v (the edits overlay does not exist, so it is not listed)", got, err)
	}
}
