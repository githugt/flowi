// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestSpecOrigin_should_name_the_overlay_that_sets_each_field(t *testing.T) {
	w, c := keptPets(t)
	root := collectionRoot(t, w, c)
	handWritten := `overlay: 1.0.0
info: {title: fixes, version: 1.0.0}
actions:
  - target: $.paths['/v1/pets'].get
    update: {description: From the fixes.}
`
	if err := os.WriteFile(filepath.Join(root, "descriptions", "pets.overlay.yaml"), []byte(handWritten), 0o600); err != nil {
		t.Fatal(err)
	}
	r := requestAt(t, w, c, "pets/listPets")
	target := RequestTarget{Collection: c, Request: r.ID}
	v := r.Versions[0]
	v.Docs = "From the fixes."
	v.Query = append(v.Query, model.KeyValue{Key: "page", Value: "2"})
	p, err := w.PreviewSpecOverlay(target, v)
	if err == nil {
		err = w.ApplySpecOverlay(p, all(p))
	}
	if err != nil {
		t.Fatal(err)
	}

	got, err := w.SpecOrigin(target, "1")

	want := []OverlayField{
		{Section: "docs", File: "descriptions/pets.overlay.yaml"},
		{Section: "query", Key: "page", File: "descriptions/pets.edits.overlay.yaml"},
	}
	if err != nil || got.Spec != "descriptions/pets.openapi.yaml" || got.Name != "pets" || !slices.Equal(got.Fields, want) {
		t.Fatalf("origin %+v err %v, want fields %v", got, err, want)
	}
}

func TestSpecOrigin_Given_a_request_from_no_spec_Then_ErrNotFromSpec(t *testing.T) {
	w, c := keptPets(t)
	target, err := w.NewRequest(c, "scratch/ping")
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.SpecOrigin(target, "1")

	if !errors.Is(err, ErrNotFromSpec) {
		t.Fatalf("err %v", err)
	}
}
