// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
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

func TestApplyImportReport_Given_a_different_request_at_the_same_path_and_keep_mine_Then_the_import_goes_beside_it(t *testing.T) {
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
	// Before: the WebSocket echo was dropped ("kept yours") or, with Overwrite, replaced the HTTP one: two operations that
	// share a name could not both be had. Now the import is named echo-2 and says so.
	if got := namesOf(report.Written, PlannedRequest); !slices.Equal(got, []string{"Echo (send a message, get it back)", "Receive ticks"}) {
		t.Errorf("written requests %v", got)
	}
	if len(report.Kept) != 0 || len(report.Written) != 3 {
		t.Errorf("report %+v", report)
	}
	if got := requestAt(t, w, c, "echo"); got.Name != "echo" || got.Versions[0].Method != "POST" {
		t.Errorf("the request that was there is %+v", got)
	}
	if got := requestAt(t, w, c, "echo-2"); got.Versions[0].Async == nil {
		t.Errorf("echo-2 is %+v", got)
	}
	var renamed []string
	for _, d := range plan.Diags {
		if d.Code == diag.CodeImportRenamed {
			renamed = append(renamed, d.Args["path"]+" -> "+d.Args["name"])
		}
	}
	if !slices.Equal(renamed, []string{"echo -> echo-2"}) {
		t.Errorf("the plan says %v", renamed)
	}
}

// Before: importing the same document again with Keep mine put nothing new down; a re-import must stay a re-import.
func TestApplyImportReport_Given_the_same_document_twice_and_keep_mine_Then_the_second_is_kept_not_renamed(t *testing.T) {
	w, c := petsCollection(t)
	first, err := w.PreviewImportSource(pastedWebSocket(t), c)
	if err == nil {
		err = w.ApplyImport(first, ApplyOptions{})
	}
	if err != nil {
		t.Fatal(err)
	}
	again, err := w.PreviewImportSource(pastedWebSocket(t), c)
	if err != nil {
		t.Fatal(err)
	}

	report, err := w.ApplyImportReport(again, ApplyOptions{SkipExisting: true})

	if err != nil {
		t.Fatal(err)
	}
	if len(report.Written) != 0 || len(report.Kept) != 3 {
		t.Fatalf("report %+v", report)
	}
	if rels := relsOf(again); slices.Contains(rels, "requests/echo-2.req.yaml") {
		t.Fatalf("a re-import was renamed: %v", rels)
	}
}

func TestPreviewImportSource_Given_a_different_request_at_the_same_path_Then_the_plan_puts_the_import_beside_it_and_says_so(t *testing.T) {
	w, c := petsCollection(t)
	// A request titled "Subscribe to flowi.demo.ticks" holds the file the WebSocket operation receiveTicks would write.
	existing := model.Request{Meta: model.Meta{ID: w.newID(), Name: "Subscribe to flowi.demo.ticks"}, Path: "receiveTicks", Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "http://localhost/ticks"}}}
	writeRequest(t, w, c, existing)

	plan, err := w.PreviewImportSource(pastedWebSocket(t), c)

	if err != nil {
		t.Fatal(err)
	}
	// Before: the plan said "exists (Subscribe to flowi.demo.ticks): yours is kept", so the operation was lost.
	got := map[string]bool{}
	for _, f := range plan.Files {
		got[f.Rel] = f.Exists
	}
	if exists, planned := got["requests/receiveTicks-2.req.yaml"]; !planned || exists {
		t.Errorf("files %v, want a new receiveTicks-2", got)
	}
	if _, planned := got["requests/receiveTicks.req.yaml"]; planned {
		t.Errorf("the plan still writes over the request that is there: %v", got)
	}
	msg := ""
	for _, d := range plan.Diags {
		if d.Code == diag.CodeImportRenamed {
			msg, _ = diag.English().Message(d)
		}
	}
	if !strings.Contains(msg, "receiveTicks") || !strings.Contains(msg, "receiveTicks-2") {
		t.Errorf("message %q", msg)
	}
}

// Before: re-importing an AsyncAPI document with Overwrite replaced each request wholesale, so the person's title and
// checks were gone; only an OpenAPI import merged.
func TestApplyImportReport_Given_overwrite_and_a_changed_asyncapi_document_Then_the_request_keeps_what_the_person_added_and_takes_the_documents_change(t *testing.T) {
	w, c := petsCollection(t)
	first, err := w.PreviewImportSource(pastedWebSocket(t), c)
	if err == nil {
		err = w.ApplyImport(first, ApplyOptions{})
	}
	if err != nil {
		t.Fatal(err)
	}
	before := requestAt(t, w, c, "echo")
	edited := before
	edited.Name = "mine"
	edited.Versions = slices.Clone(before.Versions)
	edited.Versions[0].Assert = []string{"status == 101"}
	writeRequest(t, w, c, edited)
	changed := pastedWebSocket(t)
	changed.Data = []byte(strings.Replace(string(changed.Data), "address: /ws\n", "address: /ws-v2\n", 1))
	again, err := w.PreviewImportSource(changed, c)
	if err != nil {
		t.Fatal(err)
	}

	report, err := w.ApplyImportReport(again, ApplyOptions{Overwrite: true})

	if err != nil {
		t.Fatal(err)
	}
	if len(report.Replaced) == 0 {
		t.Fatalf("nothing was replaced: %+v", report)
	}
	got := requestAt(t, w, c, "echo")
	if got.Name != "mine" || !slices.Equal(got.Versions[0].Assert, []string{"status == 101"}) {
		t.Errorf("what the person added is gone: name %q asserts %v", got.Name, got.Versions[0].Assert)
	}
	if reflect.DeepEqual(got.Versions[0].Async, before.Versions[0].Async) {
		t.Errorf("the document's new address was not taken: %+v", got.Versions[0].Async)
	}
}

// A re-import of the same document changes nothing: what the person edited is theirs, and the rest is what it was.
func TestApplyImportReport_Given_overwrite_and_the_same_asyncapi_document_Then_an_edited_request_is_unchanged(t *testing.T) {
	w, c := petsCollection(t)
	first, err := w.PreviewImportSource(pastedWebSocket(t), c)
	if err == nil {
		err = w.ApplyImport(first, ApplyOptions{})
	}
	if err != nil {
		t.Fatal(err)
	}
	edited := requestAt(t, w, c, "echo")
	edited.Name = "mine"
	writeRequest(t, w, c, edited)
	again, err := w.PreviewImportSource(pastedWebSocket(t), c)
	if err != nil {
		t.Fatal(err)
	}

	report, err := w.ApplyImportReport(again, ApplyOptions{Overwrite: true})

	if err != nil {
		t.Fatal(err)
	}
	if len(report.Replaced) != 0 || len(report.Written) != 0 || requestAt(t, w, c, "echo").Name != "mine" {
		t.Fatalf("report %+v", report)
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
