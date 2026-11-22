// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/store"
)

// kindsOf lists "kind name" of the files of a plan, in plan order.
func kindsOf(p ImportPlan) []string {
	var out []string
	for _, f := range p.Files {
		out = append(out, string(f.Kind)+" "+f.Name)
	}
	return out
}

func pastedWebSocket(t *testing.T) ImportSource {
	t.Helper()
	return ImportSource{Kind: ImportAsyncAPI, Name: PastedSourceName, Data: readFixture(t, "asyncapi/testdata/websocket-echo.yaml")}
}

func TestPreviewImportSource_Given_a_pasted_websocket_document_Then_the_plan_names_two_requests_and_one_document(t *testing.T) {
	w, c := petsCollection(t)

	plan, err := w.PreviewImportSource(pastedWebSocket(t), c)

	if err != nil {
		t.Fatal(err)
	}
	// Before: the kept copy was named after the temporary file the window wrote the paste to
	// (descriptions/flowi-paste-2749985858.asyncapi.yaml); the plan had no kinds, so a screen could
	// only count files and could not tell the document from a request.
	want := []string{
		"document ",
		"request Echo (send a message, get it back)",
		"request Receive ticks",
	}
	if got := kindsOf(plan); !slices.Equal(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	if rels := relsOf(plan); rels[0] != "descriptions/Flowi-manual-test-WebSocket.asyncapi.yaml" {
		t.Fatalf("the document is kept under its title: %v", rels)
	}
}

func TestApplyImportReport_Given_a_request_of_the_same_path_and_keep_mine_Then_it_is_reported_as_kept_not_written(t *testing.T) {
	w, c := petsCollection(t)
	// A request called echo is already there, like the HTTP one of a demo server.
	existing := model.Request{Meta: model.Meta{ID: w.newID(), Name: "echo"}, Path: "echo", Versions: []model.RequestVersion{{Label: "1", Method: "POST", URL: "http://localhost:8080/echo"}}}
	writeRequest(t, w, c, existing)
	plan, err := w.PreviewImportSource(pastedWebSocket(t), c)
	if err != nil {
		t.Fatal(err)
	}

	report, err := w.ApplyImportReport(plan, ApplyOptions{SkipExisting: true})

	if err != nil {
		t.Fatal(err)
	}
	// Before: ApplyImport said nothing; the window printed "Imported 2 files" (3 planned, 1 existing) while one
	// request was written and the other was kept, and none of it said which.
	if got := namesOf(report.Written, PlannedRequest); !slices.Equal(got, []string{"Receive ticks"}) {
		t.Errorf("written requests %v", got)
	}
	if got := namesOf(report.Kept, PlannedRequest); !slices.Equal(got, []string{"echo"}) {
		t.Errorf("kept requests %v", got)
	}
	if len(report.Written) != 2 || len(report.Kept) != 1 || len(report.Replaced)+len(report.Unchanged)+len(report.Removed) != 0 {
		t.Errorf("report %+v", report)
	}
	if got := requestAt(t, w, c, "echo"); got.Name != "echo" {
		t.Errorf("the request kept is %q", got.Name)
	}
}

func TestPreviewImportSource_Given_a_different_request_at_the_same_path_Then_the_plan_names_the_one_in_the_way(t *testing.T) {
	w, c := petsCollection(t)
	// A NATS request called "Subscribe to flowi.demo.ticks" has the operation id receiveTicks, so it holds the file
	// the WebSocket operation of the same id would write.
	existing := model.Request{Meta: model.Meta{ID: w.newID(), Name: "Subscribe to flowi.demo.ticks"}, Path: "receiveTicks", Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "http://localhost/ticks"}}}
	writeRequest(t, w, c, existing)

	plan, err := w.PreviewImportSource(pastedWebSocket(t), c)

	if err != nil {
		t.Fatal(err)
	}
	// Before: the plan only said the file existed, so a person who could not find "receiveTicks" in the sidebar
	// (it is listed under the title of the NATS request) could not tell where the clash was.
	got := map[string]string{}
	for _, f := range plan.Files {
		got[f.Rel] = f.ExistingName
	}
	if got["requests/receiveTicks.req.yaml"] != "Subscribe to flowi.demo.ticks" {
		t.Errorf("existing names %v", got)
	}
	if got["requests/echo.req.yaml"] != "" {
		t.Errorf("a new file has no existing name: %v", got)
	}
}

func TestApplyImportReport_Given_overwrite_Then_the_request_is_replaced_and_an_identical_one_unchanged(t *testing.T) {
	w, c := petsCollection(t)
	first, err := w.PreviewImportSource(pastedWebSocket(t), c)
	if err == nil {
		err = w.ApplyImport(first, ApplyOptions{})
	}
	if err != nil {
		t.Fatal(err)
	}
	writeRequest(t, w, c, model.Request{Meta: model.Meta{ID: w.newID(), Name: "mine"}, Path: "echo", Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "http://x"}}})
	again, err := w.PreviewImportSource(pastedWebSocket(t), c)
	if err != nil {
		t.Fatal(err)
	}

	report, err := w.ApplyImportReport(again, ApplyOptions{Overwrite: true})

	if err != nil {
		t.Fatal(err)
	}
	if got := namesOf(report.Replaced, PlannedRequest); !slices.Equal(got, []string{"Echo (send a message, get it back)"}) {
		t.Errorf("replaced %v", got)
	}
	if len(report.Unchanged) != 2 || len(report.Written) != 0 { // the document and receiveTicks are what they were
		t.Errorf("report %+v", report)
	}
}

func TestPreviewImportSource_Given_a_target_folder_Then_requests_land_in_it_nested(t *testing.T) {
	w, c := petsCollection(t)
	src := pastedWebSocket(t)
	src.Into = "/demo/websocket/"

	plan, err := w.PreviewImportSource(src, c)
	if err == nil {
		err = w.ApplyImport(plan, ApplyOptions{})
	}

	if err != nil {
		t.Fatal(err)
	}
	rels := relsOf(plan)
	for _, want := range []string{"requests/demo/websocket/echo.req.yaml", "requests/demo/websocket/receiveTicks.req.yaml"} {
		if !slices.Contains(rels, want) {
			t.Errorf("plan lacks %s: %v", want, rels)
		}
	}
	if plan.Into != "demo/websocket" {
		t.Errorf("Into %q", plan.Into)
	}
	if got := requestAt(t, w, c, "demo/websocket/echo"); got.Operation != "echo" {
		t.Errorf("request %+v", got)
	}
	// Documents are not moved: they stay where later imports and the overlay screen look for them.
	if !slices.ContainsFunc(rels, func(r string) bool { return strings.HasPrefix(r, "descriptions/") }) {
		t.Errorf("the document is kept in descriptions/: %v", rels)
	}
}

func TestPreviewImportSource_Given_a_target_folder_Then_openapi_requests_schemas_and_flows_move_together(t *testing.T) {
	w, c := petsCollection(t)
	src := ImportSource{Kind: ImportOpenAPI, Name: "petstore.yaml", Data: readFixture(t, "openapi/testdata/petstore30.yaml"), Into: "api/v1"}

	plan, err := w.PreviewImportSource(src, c)
	if err == nil {
		err = w.ApplyImport(plan, ApplyOptions{})
	}

	if err != nil {
		t.Fatal(err)
	}
	rels := relsOf(plan)
	for _, want := range []string{"requests/api/v1/pets/getPet.req.yaml", "requests/api/v1/pets/getPet.schemas/1-200.json", "flows/api/v1/suggested/createPet-to-getPet.arazzo.yaml"} {
		if !slices.Contains(rels, want) {
			t.Errorf("plan lacks %s: %v", want, rels)
		}
	}
	// The suggested flow calls the moved requests.
	get := requestAt(t, w, c, "api/v1/pets/getPet")
	f := flowAt(t, w, c, "api/v1/suggested/createPet-to-getPet")
	var called []model.ID
	for _, n := range f.Nodes {
		if n.Request != nil {
			called = append(called, n.Request.Request.ID)
		}
	}
	if !slices.Contains(called, get.ID) {
		t.Errorf("the flow calls %v, not the request at its new path (%s)", called, get.ID)
	}
}

func TestPreviewImportSource_Given_a_target_folder_Then_har_flows_still_call_their_requests(t *testing.T) {
	w, c := petsCollection(t)
	src := ImportSource{Kind: ImportHAR, Name: "browser.har", Data: readFixture(t, "importers/har/testdata/browser.har"), Into: "captures"}

	plan, err := w.PreviewImportSource(src, c)
	if err == nil {
		err = w.ApplyImport(plan, ApplyOptions{})
	}

	if err != nil {
		t.Fatal(err)
	}
	users := requestAt(t, w, c, "captures/har/api.example.com/get-v1-users-id")
	if flowAt(t, w, c, "captures/har/browser").Nodes[1].Request.Request.ID != users.ID {
		t.Fatal("the flow calls the imported request")
	}
}

func TestPreviewImportSource_Given_a_target_folder_Then_arazzo_flows_land_in_it(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c)
	src := ImportSource{Kind: ImportArazzo, Name: "fetch.arazzo.yaml", Data: []byte(fetchNameWorkflow), Into: "workflows"}

	plan, err := w.PreviewImportSource(src, c)

	if err != nil {
		t.Fatal(err)
	}
	if rels := relsOf(plan); !slices.ContainsFunc(rels, func(r string) bool {
		return strings.HasPrefix(r, "flows/workflows/") && strings.HasSuffix(r, "fetchName.arazzo.yaml")
	}) {
		t.Fatalf("the flow goes into the folder: %v", rels)
	}
}

func TestPreviewImportSource_Given_a_folder_that_is_not_a_path_Then_it_is_refused_and_nothing_is_planned(t *testing.T) {
	w, c := petsCollection(t)
	for _, into := range []string{"../outside", "a/../b", "a/.hidden", "a b/c d?"} {
		src := pastedWebSocket(t)
		src.Into = into

		_, err := w.PreviewImportSource(src, c)

		if err == nil {
			t.Errorf("into %q was accepted", into)
		}
	}
}

func TestSpecBaseName_Given_pasted_text_Then_the_title_names_the_kept_document(t *testing.T) {
	cases := []struct {
		name string
		src  ImportSource
		want string
	}{
		{"title", ImportSource{Name: PastedSourceName, Data: []byte("openapi: 3.0.0\ninfo: {title: Pet Store, version: '1'}\npaths: {}\n")}, "Pet-Store"},
		{"no title", ImportSource{Name: PastedSourceName, Data: []byte("openapi: 3.0.0\n")}, "imported"},
		{"not a document", ImportSource{Name: PastedSourceName, Data: []byte(":\t::")}, "imported"},
		{"a file keeps its own name", ImportSource{Name: "/x/pets.openapi.yaml", Data: []byte("info: {title: Other}")}, "pets"},
	}
	for _, tc := range cases {
		if got := specBaseName(tc.src); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// namesOf is the names of the files of kind in list.
func namesOf(list []PlannedFile, kind PlannedKind) []string {
	var out []string
	for _, f := range list {
		if f.Kind == kind {
			out = append(out, f.Name)
		}
	}
	return out
}

// writeRequest saves req to its collection through the public write path.
func writeRequest(t *testing.T, w *Workspace, c model.ID, req model.Request) {
	t.Helper()
	for _, col := range w.Collections() {
		if col.Manifest.ID != c {
			continue
		}
		req.File = filepath.Join(col.Root, "requests", filepath.FromSlash(req.Path)+".req.yaml")
		data, err := w.store.EncodeRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(req.File), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(req.File, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Reload(Change{Root: col.Root, File: col.Root, Op: store.ChangeRescan}); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no collection %s", c)
}
