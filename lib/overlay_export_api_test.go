// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/specoverlay"
)

func writeTestOverlay(t *testing.T, w *Workspace, c model.ID, ov model.Overlay) {
	t.Helper()
	if err := w.WriteOverlay(c, ov, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestExportOverlayAsSpec_should_carry_what_a_spec_can_hold_and_name_the_rest(t *testing.T) {
	// Arrange
	w, c := keptPets(t)
	target, err := w.FindRequest("pets/listPets", "")
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	writeTestOverlay(t, w, c, model.Overlay{Name: "documented", Requests: []model.RequestOverlay{{Request: r.ID, Path: r.Path, Ops: []model.OverlayOp{
		{Verb: model.OverlaySet, Path: "docs", Value: "Lists the pets, a page at a time."},
		{Verb: model.OverlaySet, Path: "query.page", Value: "2"},
		{Verb: model.OverlayAdd, Path: "assert", Value: "status == 200"},
	}}}})

	// Act
	got, err := w.ExportOverlayAsSpec(c, "documented")

	// Assert
	if err != nil || len(got.Documents) != 1 {
		t.Fatalf("documents %+v err %v", got.Documents, err)
	}
	doc := got.Documents[0]
	if doc.Spec.Name != "pets" || !strings.Contains(doc.YAML, "Lists the pets, a page at a time.") || !strings.Contains(doc.YAML, "name: page") {
		t.Fatalf("overlay:\n%s", doc.YAML)
	}
	if strings.Contains(doc.YAML, "extends") {
		t.Errorf("the overlay must not name the kept copy as the document it extends:\n%s", doc.YAML)
	}
	if len(got.NotCarried) == 0 || !strings.Contains(strings.Join(noteWhats2(got.NotCarried), "|"), "the checks") {
		t.Errorf("not carried: %+v", got.NotCarried)
	}
	if _, warnings := specoverlay.ApplyTo(readFixture(t, "openapi/testdata/petstore30.yaml"), "pets.openapi.yaml", [][]byte{[]byte(doc.YAML)}); warnings.HasErrors() {
		t.Errorf("the exported overlay does not apply: %v", warnings)
	}
}

func TestExportOverlayAsSpec_Given_nothing_a_spec_can_hold_Then_no_document_and_every_reason(t *testing.T) {
	// Arrange
	w, c := keptPets(t)
	writeTestOverlay(t, w, c, model.Overlay{Name: "slow", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "settings.timeouts.total", Value: "1s"}}})

	// Act
	got, err := w.ExportOverlayAsSpec(c, "slow")

	// Assert
	if err != nil || len(got.Documents) != 0 {
		t.Fatalf("documents %+v err %v", got.Documents, err)
	}
	if len(got.NotCarried) == 0 {
		t.Error("the user should be told why nothing was exported")
	}
}

func noteWhats2(ns []ExportNote) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.What)
	}
	return out
}

func TestExportOverlayAsSpec_Given_a_collection_with_no_kept_spec_Then_says_so(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := overlayLoginTarget(t, w)
	writeTestOverlay(t, w, target.Collection, model.Overlay{Name: "vip", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"}}})

	// Act
	got, err := w.ExportOverlayAsSpec(target.Collection, "vip")

	// Assert
	if err != nil || len(got.Documents) != 0 || len(got.NotCarried) != 1 || !strings.Contains(got.NotCarried[0].Why, "no request of this collection") {
		t.Fatalf("documents %v notes %+v err %v", got.Documents, got.NotCarried, err)
	}
}
