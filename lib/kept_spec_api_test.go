// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func collectionRoot(t *testing.T, w *Workspace, c model.ID) string {
	t.Helper()
	i := slices.IndexFunc(w.Collections(), func(x model.Collection) bool { return x.Manifest.ID == c })
	if i < 0 {
		t.Fatalf("no collection %s", c)
	}
	return w.Collections()[i].Root
}

func writeDescriptions(t *testing.T, root string, names ...string) {
	t.Helper()
	dir := filepath.Join(root, "descriptions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x: 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestKeptSpecs_should_list_kept_specs_with_their_overlays_in_order(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeDescriptions(t, root, "pets.openapi.yaml", "pets.overlay-10.yaml", "pets.overlay.yaml", "pets.overlay-2.yaml",
		"pets.edits.overlay.yaml", "shop.openapi.json", "requests.openapi.yaml", "requests.asyncapi.yaml")
	dir := filepath.Join(root, "descriptions")

	got, err := w.KeptSpecs(c)

	if err != nil || len(got) != 2 {
		t.Fatalf("specs %+v err %v, want pets and shop (requests.openapi.yaml is generated)", got, err)
	}
	pets := got[0]
	wantOverlays := []string{filepath.Join(dir, "pets.overlay.yaml"), filepath.Join(dir, "pets.overlay-2.yaml"), filepath.Join(dir, "pets.overlay-10.yaml")}
	if pets.Name != "pets" || pets.Rel != "descriptions/pets.openapi.yaml" || !slices.Equal(pets.Overlays, wantOverlays) {
		t.Fatalf("pets %+v, want overlays %v", pets, wantOverlays)
	}
	if pets.Edits != filepath.Join(dir, "pets.edits.overlay.yaml") || got[1].Name != "shop" || len(got[1].Overlays) != 0 {
		t.Fatalf("edits %q, shop %+v", pets.Edits, got[1])
	}
}

func TestKeptSpecs_Given_no_descriptions_folder_Then_none(t *testing.T) {
	w, c := petsCollection(t)

	got, err := w.KeptSpecs(c)

	if err != nil || len(got) != 0 {
		t.Fatalf("specs %v err %v", got, err)
	}
}

func TestPreviewSpecUpdate_should_keep_a_saved_edit_and_take_a_new_operation(t *testing.T) {
	w, c := petsCollection(t)
	src := readFixture(t, "openapi/testdata/petstore30.yaml")
	plan, err := w.PreviewImportSource(ImportSource{Kind: ImportOpenAPI, Name: "pets.openapi.yaml", Data: src}, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	edited := requestAt(t, w, c, "pets/getPet")
	edited.Versions[0].Docs = "mine"
	edited.Versions[0].Assert = []string{"status == 200"}
	if err := w.SaveRequest(RequestTarget{Collection: c, Request: edited.ID}, edited, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	newer := bytes.Replace(src, []byte("paths:\n"), []byte(`paths:
  /v1/owners:
    get:
      operationId: listOwners
      tags: [owners]
      responses: {"200": {description: ok}}
`), 1)

	plan, err = w.PreviewSpecUpdate(c, "pets", newer)
	if err == nil {
		err = w.ApplyImport(plan, ApplyOptions{Overwrite: true})
	}

	if err != nil {
		t.Fatal(err)
	}
	after := requestAt(t, w, c, "pets/getPet")
	if after.Versions[0].Docs != "mine" || len(after.Versions[0].Assert) != 1 || after.ID != edited.ID {
		t.Fatalf("after %+v", after.Versions[0])
	}
	requestAt(t, w, c, "owners/listOwners")
}

func TestPreviewSpecUpdate_Given_an_unknown_name_Then_ErrNoKeptSpec(t *testing.T) {
	w, c := petsCollection(t)

	_, err := w.PreviewSpecUpdate(c, "nope", nil)

	if !errors.Is(err, ErrNoKeptSpec) {
		t.Fatalf("err %v", err)
	}
}
