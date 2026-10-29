// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

// keptPets imports the petstore as pets.openapi.yaml, kept in descriptions/.
func keptPets(t *testing.T) (*Workspace, model.ID) {
	t.Helper()
	w, c := petsCollection(t)
	plan, err := w.PreviewImportSource(ImportSource{Kind: ImportOpenAPI, Name: "pets.openapi.yaml", Data: readFixture(t, "openapi/testdata/petstore30.yaml")}, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	return w, c
}

func whats(chs []OverlayChange) []string {
	var out []string
	for _, ch := range chs {
		out = append(out, ch.What)
	}
	return out
}

func noteWhats(ns []OverlayNote) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.What)
	}
	return out
}

func all(p OverlayPlan) []int {
	out := make([]int, len(p.Changes))
	for i := range out {
		out[i] = i
	}
	return out
}

func TestPreviewSpecOverlay_should_say_what_the_overlay_carries_and_what_stays_in_Flow_I(t *testing.T) {
	tests := []struct {
		name      string
		request   string
		edit      func(v *model.RequestVersion)
		wantWhat  []string
		wantNotes []string
	}{
		{
			name:    "docs and parameters",
			request: "pets/listPets",
			edit: func(v *model.RequestVersion) {
				v.Docs = "Lists the pets, a page at a time."
				v.Query = []model.KeyValue{{Key: "limit", Value: "20"}, {Key: "page", Value: "2"}}
				v.Headers = nil
				v.Assert = []string{"status == 200"}
			},
			wantWhat:  []string{"the docs", "query parameter page is added", "header parameter X-Trace is removed"},
			wantNotes: []string{"query parameter limit", "the checks"},
		},
		{
			name:     "a JSON body",
			request:  "pets/createPet",
			edit:     func(v *model.RequestVersion) { v.Body = model.Body{JSON: `{"name": "Rex"}`} },
			wantWhat: []string{"the body example"},
		},
		{
			name:      "a body with values only Flowi fills in",
			request:   "pets/createPet",
			edit:      func(v *model.RequestVersion) { v.Body = model.Body{JSON: `{"name": "{{name}}", "tag": "dog"}`} },
			wantNotes: []string{"the body"},
		},
		{
			name:      "a new URL",
			request:   "pets/listPets",
			edit:      func(v *model.RequestVersion) { v.URL = "{{env.base_url}}/v1/animals" },
			wantNotes: []string{"the method or URL"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, c := keptPets(t)
			r := requestAt(t, w, c, tt.request)
			v := r.Versions[0]
			tt.edit(&v)

			p, err := w.PreviewSpecOverlay(RequestTarget{Collection: c, Request: r.ID}, v)

			if err != nil {
				t.Fatal(err)
			}
			if got := whats(p.Changes); !slices.Equal(got, tt.wantWhat) {
				t.Fatalf("changes %v, want %v", got, tt.wantWhat)
			}
			if got := noteWhats(p.NotCarried); !slices.Equal(got, tt.wantNotes) {
				t.Fatalf("notes %v, want %v", got, tt.wantNotes)
			}
		})
	}
}

func TestApplySpecOverlay_should_survive_an_update_from_the_spec(t *testing.T) {
	w, c := keptPets(t)
	r := requestAt(t, w, c, "pets/listPets")
	target := RequestTarget{Collection: c, Request: r.ID}
	r.Versions[0].Docs = "Lists the pets."
	r.Versions[0].Query = append(r.Versions[0].Query, model.KeyValue{Key: "page", Value: "2"})
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	p, err := w.PreviewSpecOverlay(target, r.Versions[0])
	if err != nil {
		t.Fatal(err)
	}

	err = w.ApplySpecOverlay(p, all(p))

	if err != nil {
		t.Fatal(err)
	}
	kept, _ := keptSpecNamed(collectionRoot(t, w, c), "pets")
	fresh, err := importKept(kept)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(fresh.Requests, func(x model.Request) bool { return x.Path == "pets/listPets" })
	v := fresh.Requests[i].Versions[0]
	if v.Docs != "Lists the pets." || !slices.ContainsFunc(v.Query, func(kv model.KeyValue) bool { return kv.Key == "page" }) {
		t.Fatalf("a fresh import of the spec with its overlays: %+v", v)
	}
	again, err := w.PreviewSpecOverlay(target, requestAt(t, w, c, "pets/listPets").Versions[0])
	if err != nil || len(again.Changes) != 2 || slices.ContainsFunc(again.Changes, func(ch OverlayChange) bool { return !ch.Saved }) {
		t.Fatalf("after saving, both changes are there and saved: %+v %v", again.Changes, err)
	}
}

func TestApplySpecOverlay_should_replace_what_it_said_about_the_operation(t *testing.T) {
	w, c := keptPets(t)
	r := requestAt(t, w, c, "pets/listPets")
	target := RequestTarget{Collection: c, Request: r.ID}
	v := r.Versions[0]
	v.Docs = "first"
	p, _ := w.PreviewSpecOverlay(target, v)
	if err := w.ApplySpecOverlay(p, all(p)); err != nil {
		t.Fatal(err)
	}
	v.Docs = "second"
	p, _ = w.PreviewSpecOverlay(target, v)
	if err := w.ApplySpecOverlay(p, all(p)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(p.Spec.Edits)

	if err != nil || strings.Contains(string(data), "first") || strings.Count(string(data), "target:") != 1 {
		t.Fatalf("err %v overlay:\n%s", err, data)
	}
}

func TestApplySpecOverlay_Given_nothing_chosen_Then_the_overlay_is_removed(t *testing.T) {
	w, c := keptPets(t)
	r := requestAt(t, w, c, "pets/listPets")
	target := RequestTarget{Collection: c, Request: r.ID}
	v := r.Versions[0]
	v.Docs = "mine"
	p, _ := w.PreviewSpecOverlay(target, v)
	if err := w.ApplySpecOverlay(p, all(p)); err != nil {
		t.Fatal(err)
	}

	err := w.ApplySpecOverlay(p, nil)

	if _, statErr := os.Stat(p.Spec.Edits); err != nil || !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("err %v, the overlay is still there: %v", err, statErr)
	}
}

func TestPreviewSpecOverlay_Given_a_request_from_no_spec_Then_ErrNotFromSpec(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c) // kept as imported.openapi.yaml
	if err := os.Remove(collectionRoot(t, w, c) + "/descriptions/imported.openapi.yaml"); err != nil {
		t.Fatal(err)
	}
	r := requestAt(t, w, c, "pets/listPets")

	_, err := w.PreviewSpecOverlay(RequestTarget{Collection: c, Request: r.ID}, r.Versions[0])

	if !errors.Is(err, ErrNotFromSpec) {
		t.Fatalf("err %v", err)
	}
}

func TestApplySpecOverlay_Given_an_operation_without_parameters_Then_one_action_adds_them_all(t *testing.T) {
	w, c := keptPets(t)
	r := requestAt(t, w, c, "pets/createPet")
	v := r.Versions[0]
	v.Query = []model.KeyValue{{Key: "dryRun", Value: "true"}}
	v.Headers = []model.KeyValue{{Key: "X-Request", Value: "{{request_id}}"}}
	p, err := w.PreviewSpecOverlay(RequestTarget{Collection: c, Request: r.ID}, v)
	if err != nil {
		t.Fatal(err)
	}

	err = w.ApplySpecOverlay(p, all(p))

	kept, _ := keptSpecNamed(collectionRoot(t, w, c), "pets")
	fresh, importErr := importKept(kept)
	data, _ := os.ReadFile(p.Spec.Edits)
	if err != nil || importErr != nil || strings.Count(string(data), "target:") != 1 {
		t.Fatalf("err %v %v overlay:\n%s", err, importErr, data)
	}
	got := fresh.Requests[slices.IndexFunc(fresh.Requests, func(x model.Request) bool { return x.Path == "pets/createPet" })].Versions[0]
	if len(got.Query) != 1 || len(got.Headers) != 1 || strings.Contains(string(data), "request_id") {
		t.Fatalf("query %v headers %v; a {{…}} value is not written as an example:\n%s", got.Query, got.Headers, data)
	}
}
