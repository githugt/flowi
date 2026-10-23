// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"

	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/model"
)

const fetchNameWorkflow = `arazzo: 1.0.1
info: {title: pets, version: 1.0.0}
sourceDescriptions: [{name: pets, url: ./pets.yaml, type: openapi}]
workflows:
  - workflowId: fetchName
    steps:
      - stepId: fetch
        operationId: getPet
        parameters:
          - {name: petId, in: path, value: $inputs.petId}
        successCriteria:
          - condition: $statusCode == 200
        outputs:
          name: $response.body#/name
`

func petsCollection(t *testing.T) (*Workspace, model.ID) {
	t.Helper()
	w := newEnv(t).open(t)
	m, err := w.InitCollection(filepath.Join(t.TempDir(), "pets"), "pets", true)
	if err != nil {
		t.Fatal(err)
	}
	return w, m.ID
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func importPetstore(t *testing.T, w *Workspace, c model.ID) {
	t.Helper()
	plan, err := w.PreviewImport(ImportOpenAPI, readFixture(t, "openapi/testdata/petstore30.yaml"), c)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
}

func relsOf(p ImportPlan) []string {
	var out []string
	for _, f := range p.Files {
		out = append(out, f.Rel)
	}
	return out
}

func requestAt(t *testing.T, w *Workspace, c model.ID, path string) model.Request {
	t.Helper()
	for _, col := range w.Collections() {
		if col.Manifest.ID != c {
			continue
		}
		if i := slices.IndexFunc(col.Requests, func(r model.Request) bool { return r.Path == path }); i >= 0 {
			return col.Requests[i]
		}
	}
	t.Fatalf("no request %s", path)
	return model.Request{}
}

func TestCollectionResolver_should_know_the_operation_only_of_calls_certain_to_run_its_version(t *testing.T) {
	versions := []model.RequestVersion{{Label: "1"}, {Label: "2"}}
	c := model.Collection{Requests: []model.Request{
		{Meta: model.Meta{ID: "R1"}, Operation: "getPet", Versions: versions},
		{Meta: model.Meta{ID: "R2"}, Operation: "listPets", Versions: versions[:1]},
	}}
	r := collectionResolver{c: c}
	cases := []struct {
		call model.RequestCall
		want bool
	}{
		{model.RequestCall{Request: model.Ref{ID: "R1"}, Version: "1"}, true},
		{model.RequestCall{Request: model.Ref{ID: "R1"}, Version: "2"}, false},
		{model.RequestCall{Request: model.Ref{ID: "R1"}}, false},
		{model.RequestCall{Request: model.Ref{ID: "R2"}}, true},
		{model.RequestCall{Request: model.Ref{ID: "R2", Collection: "other"}}, false},
	}

	for _, tc := range cases {
		_, ok := r.OperationOf(tc.call)

		if ok != tc.want {
			t.Errorf("OperationOf(%+v) = %v, want %v", tc.call, ok, tc.want)
		}
	}
}

func TestPreviewImport_openapi_should_plan_requests_schemas_and_flows_without_writing(t *testing.T) {
	w, c := petsCollection(t)

	plan, err := w.PreviewImport(ImportOpenAPI, readFixture(t, "openapi/testdata/petstore30.yaml"), c)

	if err != nil {
		t.Fatal(err)
	}
	rels := relsOf(plan)
	for _, want := range []string{"requests/pets/getPet.req.yaml", "requests/pets/getPet.schemas/1-200.json", "flows/suggested/createPet-to-getPet.arazzo.yaml"} {
		if !slices.Contains(rels, want) {
			t.Errorf("plan lacks %s: %v", want, rels)
		}
	}
	if slices.ContainsFunc(plan.Files, func(f PlannedFile) bool { return f.Exists }) {
		t.Fatal("nothing exists yet")
	}
	if _, err := os.Stat(filepath.Join(w.Collections()[0].Root, "requests", "pets")); err == nil {
		t.Fatal("a preview must not write")
	}
}

func TestApplyImport_should_write_and_load_requests(t *testing.T) {
	w, c := petsCollection(t)

	importPetstore(t, w, c)

	if r := requestAt(t, w, c, "pets/getPet"); r.Operation != "getPet" || len(r.Versions) != 2 {
		t.Fatalf("request %+v", r)
	}
}

func TestApplyImport_Given_existing_files_Then_FLW_IMPORT_003_unless_overwrite_or_skip(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c)
	edited := requestAt(t, w, c, "pets/getPet")
	edited.Description = "mine"
	if err := w.SaveRequest(RequestTarget{Collection: c, Request: edited.ID}, edited, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	plan, err := w.PreviewImport(ImportOpenAPI, readFixture(t, "openapi/testdata/petstore30.yaml"), c)
	if err != nil {
		t.Fatal(err)
	}

	refused := w.ApplyImport(plan, ApplyOptions{})
	afterRefusal := requestAt(t, w, c, "pets/getPet")
	skipped := w.ApplyImport(plan, ApplyOptions{SkipExisting: true})
	afterSkip := requestAt(t, w, c, "pets/getPet")
	overwritten := w.ApplyImport(plan, ApplyOptions{Overwrite: true})
	afterOverwrite := requestAt(t, w, c, "pets/getPet")

	if !slices.Equal(codesOfErr(refused), []diag.Code{diag.CodeImportExists}) || afterRefusal.Description != "mine" {
		t.Fatalf("refused %v, description %q", refused, afterRefusal.Description)
	}
	if skipped != nil || afterSkip.Description != "mine" {
		t.Fatalf("skipped %v, description %q", skipped, afterSkip.Description)
	}
	// Overwriting used to replace the saved description with the spec's, so
	// re-importing lost every edit. It is now merged against the kept spec:
	// the spec did not change the description, so the edit stays.
	if overwritten != nil || afterOverwrite.Description != "mine" || afterOverwrite.ID != edited.ID {
		t.Fatalf("overwritten %v, request %+v (id must be kept: %s)", overwritten, afterOverwrite, edited.ID)
	}
}

func TestImportArazzo_should_resolve_operations_from_the_collection(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c)

	plan, err := w.PreviewImport(ImportArazzo, []byte(fetchNameWorkflow), c)
	if err != nil {
		t.Fatal(err)
	}
	err = w.ApplyImport(plan, ApplyOptions{})

	if err != nil || !slices.Equal(relsOf(plan), []string{"flows/arazzo/fetchName.arazzo.yaml"}) || len(plan.Diags) != 0 {
		t.Fatalf("err %v rels %v diags %v", err, relsOf(plan), plan.Diags)
	}
	f := flowAt(t, w, c, "arazzo/fetchName")
	if f.Nodes[0].Request.Request.ID != requestAt(t, w, c, "pets/getPet").ID {
		t.Fatalf("node %+v", f.Nodes[0])
	}
}

func flowAt(t *testing.T, w *Workspace, c model.ID, path string) model.Flow {
	t.Helper()
	for _, col := range w.Collections() {
		if i := slices.IndexFunc(col.Flows, func(f model.Flow) bool { return f.Path == path }); col.Manifest.ID == c && i >= 0 {
			return col.Flows[i]
		}
	}
	t.Fatalf("no flow %s", path)
	return model.Flow{}
}

// The M5 acceptance test: removing a field from an imported response
// schema marks the flow that reads it ✗.
func TestImport_Given_a_field_removed_from_the_schema_Then_its_consumer_is_broken(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c)
	plan, _ := w.PreviewImport(ImportArazzo, []byte(fetchNameWorkflow), c)
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	get := requestAt(t, w, c, "pets/getPet")
	target := RequestTarget{Collection: c, Request: get.ID}

	before, _, err := w.Impact(target, "1")
	if err != nil {
		t.Fatal(err)
	}
	removeProperty(t, filepath.Join(strings.TrimSuffix(get.File, ".req.yaml")+".schemas", "1-200.json"), "name")
	after, _, err := w.Impact(target, "1")

	if err != nil || before.Status != analysis.Compatible || after.Status != analysis.Broken {
		t.Fatalf("err %v, before %s, after %s", err, before.Status, after.Status)
	}
}

// removeProperty deletes a property everywhere in a JSON schema file.
func removeProperty(t *testing.T, file, name string) {
	t.Helper()
	var schema any
	if err := json.Unmarshal(readFixture(t, file), &schema); err != nil {
		t.Fatal(err)
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if props, ok := x["properties"].(map[string]any); ok {
				delete(props, name)
			}
			for _, item := range x {
				walk(item)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(schema)
	data, _ := json.Marshal(schema)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExportArazzo_should_export_a_flow_and_the_subflows_it_calls(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c)
	doc := fetchNameWorkflow + `  - workflowId: outer
    steps:
      - stepId: inner
        workflowId: fetchName
        parameters:
          - {name: petId, value: $inputs.id}
`
	plan, _ := w.PreviewImport(ImportArazzo, []byte(doc), c)
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	f := flowAt(t, w, c, "arazzo/outer")

	out, diags, err := w.ExportArazzo(FlowTarget{Collection: c, Flow: f.ID})

	if err != nil || len(diags) != 0 {
		t.Fatalf("err %v diags %v", err, diags)
	}
	if _, err := libopenapi.NewArazzoDocument(out); err != nil || !strings.Contains(string(out), "operationId: $sourceDescriptions.flowi.getPet") ||
		!strings.Contains(string(out), "workflowId: fetchName") {
		t.Fatalf("%v\n%s", err, out)
	}
}

func TestFindCollection_should_default_to_the_only_collection_and_resolve_names(t *testing.T) {
	w, c := petsCollection(t)

	byDefault, errDefault := w.FindCollection("")
	byName, errName := w.FindCollection("pets")
	_, errUnknown := w.FindCollection("nope")

	if errDefault != nil || errName != nil || byDefault != c || byName != c {
		t.Fatalf("default %v %v, name %v %v", byDefault, errDefault, byName, errName)
	}
	if !slices.Equal(codesOfErr(errUnknown), []diag.Code{diag.CodeWSUnknownCollection}) {
		t.Fatalf("unknown %v", errUnknown)
	}
}

func TestPreviewImportNew_should_plan_into_a_directory_without_creating_it(t *testing.T) {
	w := newEnv(t).open(t)
	dir := filepath.Join(t.TempDir(), "new")

	plan, err := w.PreviewImportNew(ImportOpenAPI, readFixture(t, "openapi/testdata/petstore30.yaml"), dir)

	if err != nil || !slices.Contains(relsOf(plan), "requests/pets/getPet.req.yaml") {
		t.Fatalf("err %v rels %v", err, relsOf(plan))
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("a preview must not create the directory")
	}
	if err := w.ApplyImport(plan, ApplyOptions{}); !slices.Equal(codesOfErr(err), []diag.Code{diag.CodeWSUnknownCollection}) {
		t.Fatalf("a preview of a new collection cannot be applied: %v", err)
	}
}

// Defence in depth for document-derived names: whatever a transform
// produces, a plan never reaches outside the collection.
func TestPlanOf_Given_a_file_outside_the_collection_Then_it_is_refused(t *testing.T) {
	root := t.TempDir()
	c := model.Collection{Root: filepath.Join(root, "c")}
	files := map[string][]byte{
		filepath.Join(root, "c", "requests", "a.req.yaml"): nil,
		filepath.Join(root, "c", "..", "evil.json"):        nil,
	}

	_, err := planOf(c, files, nil, "")

	if err == nil || !strings.Contains(err.Error(), "outside the collection") {
		t.Fatalf("err %v", err)
	}
}

// Before: the containment check compared path strings only, so a symlink
// inside the collection (from a cloned repository, say) carried an import's
// write outside it.
func TestPlanOf_Given_a_symlink_pointing_outside_the_collection_Then_it_is_refused(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	collection := filepath.Join(root, "c")
	if err := os.MkdirAll(filepath.Join(collection, "requests"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(collection, "requests", "linked")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	c := model.Collection{Root: collection}
	files := map[string][]byte{filepath.Join(collection, "requests", "linked", "a.req.yaml"): nil}

	_, err := planOf(c, files, nil, "")

	if err == nil || !strings.Contains(err.Error(), "outside the collection") {
		t.Fatalf("err %v", err)
	}
}

// Before: overwriting kept schema files the document no longer has, so
// a stale contract kept consumers ✓.
func TestApplyImport_Given_overwrite_Then_schemas_the_document_no_longer_has_are_removed(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c)
	dir := strings.TrimSuffix(requestAt(t, w, c, "pets/getPet").File, ".req.yaml") + ".schemas"
	stale := filepath.Join(dir, "1-299.json")
	if err := os.WriteFile(stale, []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := w.PreviewImport(ImportOpenAPI, readFixture(t, "openapi/testdata/petstore30.yaml"), c)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(plan.Files, func(f PlannedFile) bool { return f.File == stale })
	if i < 0 || !plan.Files[i].Remove || !plan.Files[i].Exists {
		t.Fatalf("the stale schema is planned for removal: %+v", plan.Files)
	}
	if err := w.ApplyImport(plan, ApplyOptions{SkipExisting: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatal("skipping existing files keeps it")
	}
	if err := w.ApplyImport(plan, ApplyOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("overwriting removes it")
	}
}

func TestPlanResult_should_plan_requests_examples_environments_flows_and_keep_the_report(t *testing.T) {
	w, c := petsCollection(t)
	w.mu.RLock()
	col, _ := w.collection(c)
	w.mu.RUnlock()
	r := importers.Result{
		Requests:     []model.Request{{Meta: model.Meta{Name: "login"}, Path: "auth/login", Versions: []model.RequestVersion{{Label: "1", Method: "POST"}}}},
		Examples:     []importers.Example{{RequestPath: "auth/login", Name: "har-200", Status: 200, Body: map[string]any{"token": "t"}}},
		Environments: []model.Environment{{Meta: model.Meta{Name: "staging"}, Vars: []model.EnvVar{{Name: "base_url", Value: "https://x"}}}},
		Flows: []model.Flow{{Meta: model.Meta{Name: "journey"}, Path: "har/journey", Nodes: []model.Node{
			{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{Path: "auth/login"}}}}}},
		Items: []importers.Item{{Name: "login"}, {Name: "broken", Skipped: true}},
	}

	plan, err := w.planResult(col, r)

	if err != nil {
		t.Fatal(err)
	}
	want := []string{"environments/staging.yaml", "flows/har/journey.arazzo.yaml", "requests/auth/login.examples/har-200.json", "requests/auth/login.req.yaml"}
	if !slices.Equal(relsOf(plan), want) || len(plan.Items) != 2 {
		t.Fatalf("rels %v items %d", relsOf(plan), len(plan.Items))
	}
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	login := requestAt(t, w, c, "auth/login")
	if login.ID == "" || flowAt(t, w, c, "har/journey").Nodes[0].Request.Request.ID != login.ID {
		t.Fatalf("the flow calls the imported request by id: %+v", flowAt(t, w, c, "har/journey").Nodes[0].Request)
	}
}

func TestPreviewImportSource_curl_should_plan_the_good_commands_and_report_the_rest(t *testing.T) {
	w, c := petsCollection(t)
	src := ImportSource{Kind: ImportCurl, Name: "paste.txt", Data: []byte("curl 'https://x/a\ncurl https://x/v1/b -H 'Authorization: Bearer t0k'\n")}

	plan, err := w.PreviewImportSource(src, c)

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(relsOf(plan), []string{"requests/curl/get-v1-b.req.yaml"}) || len(plan.Items) != 2 || !plan.Items[0].Skipped {
		t.Fatalf("rels %v items %+v", relsOf(plan), plan.Items)
	}
	if plan.Items[0].Diags[0].File != "paste.txt" || len(plan.Items[0].Diags[0].Excerpt) != 2 {
		t.Fatalf("the report names the source and shows the excerpt: %+v", plan.Items[0].Diags[0])
	}
}

func TestPreviewImportSource_har_should_write_requests_examples_and_the_flow(t *testing.T) {
	w, c := petsCollection(t)
	src := ImportSource{Kind: ImportHAR, Name: "browser.har", Data: readFixture(t, "importers/har/testdata/browser.har")}

	plan, err := w.PreviewImportSource(src, c)
	if err != nil {
		t.Fatal(err)
	}
	err = w.ApplyImport(plan, ApplyOptions{})

	if err != nil || !slices.Contains(relsOf(plan), "requests/har/api.example.com/get-v1-users-id.examples/har-200.json") {
		t.Fatalf("err %v rels %v", err, relsOf(plan))
	}
	users := requestAt(t, w, c, "har/api.example.com/get-v1-users-id")
	if flowAt(t, w, c, "har/browser").Nodes[1].Request.Request.ID != users.ID {
		t.Fatal("the flow calls the imported request")
	}
}

func TestPreviewImportSource_postman_should_write_requests_environments_and_examples(t *testing.T) {
	w, c := petsCollection(t)
	src := ImportSource{Kind: ImportPostman, Name: "shop.json", Data: readFixture(t, "importers/postman/testdata/shop.postman_collection.json")}

	plan, err := w.PreviewImportSource(src, c)
	if err != nil {
		t.Fatal(err)
	}
	err = w.ApplyImport(plan, ApplyOptions{})

	for _, want := range []string{"requests/Auth/Login.req.yaml", "requests/Auth/Login.examples/OK.json", "environments/shop-api.yaml"} {
		if !slices.Contains(relsOf(plan), want) {
			t.Errorf("missing %s in %v", want, relsOf(plan))
		}
	}
	if err != nil || requestAt(t, w, c, "Users/Get-user").Versions[0].URL != "{{env.base_url}}/v1/users/42" {
		t.Fatalf("err %v", err)
	}
}

func TestPreviewImportSource_bruno_should_import_a_collection_folder(t *testing.T) {
	w, c := petsCollection(t)
	src := ImportSource{Kind: ImportBruno, Dir: "importers/bruno/testdata/shop"}

	plan, err := w.PreviewImportSource(src, c)
	if err != nil {
		t.Fatal(err)
	}
	err = w.ApplyImport(plan, ApplyOptions{})

	if err != nil || !slices.Contains(relsOf(plan), "environments/local.yaml") ||
		requestAt(t, w, c, "users/get-user").Versions[0].Extract[0].Name != "userName" {
		t.Fatalf("err %v rels %v", err, relsOf(plan))
	}
}

func TestPreviewImportSource_mermaid_should_resolve_against_the_collection(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c)
	src := ImportSource{Kind: ImportMermaid, Name: "adopt.mmd", Data: []byte("flowchart LR\n  get[req: pets/getPet@1] --> wait[delay: 1s] --> todo[Pay the shelter]\n")}

	plan, err := w.PreviewImportSource(src, c)
	if err != nil {
		t.Fatal(err)
	}
	err = w.ApplyImport(plan, ApplyOptions{})

	f := flowAt(t, w, c, "mermaid/adopt")
	if err != nil || f.Nodes[0].Request.Request.ID != requestAt(t, w, c, "pets/getPet").ID || f.Nodes[2].Type != "note" {
		t.Fatalf("err %v nodes %+v", err, f.Nodes)
	}
	if len(plan.Diags) != 1 || plan.Diags[0].Code != diag.CodeMermaidPlaceholder {
		t.Fatalf("diags %v", plan.Diags)
	}
}

func TestExportMermaid_should_write_the_flow(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}

	out, err := w.ExportMermaid(target)

	if err != nil || !strings.HasPrefix(out, "flowchart LR\n") || !strings.Contains(out, `login["req: auth/login"]`) {
		t.Fatalf("err %v\n%s", err, out)
	}
}

func TestPreviewImportSourceNew_should_plan_a_folder_import_without_creating_anything(t *testing.T) {
	w := newEnv(t).open(t)
	dir := filepath.Join(t.TempDir(), "shop")

	plan, err := w.PreviewImportSourceNew(ImportSource{Kind: ImportBruno, Dir: "importers/bruno/testdata/shop"}, dir)

	if err != nil || !slices.Contains(relsOf(plan), "requests/users/get-user.req.yaml") {
		t.Fatalf("err %v rels %v", err, relsOf(plan))
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("a preview must not create the directory")
	}
}

// Review finding 10: re-importing replaced the request file and dropped its
// dirty-edit slot, which the rewrite rule says must be preserved.
func TestApplyImport_Overwrite_should_keep_the_dirty_edit_slot(t *testing.T) {
	w, c := petsCollection(t)
	importPetstore(t, w, c)
	r := requestAt(t, w, c, "pets/getPet")
	target := RequestTarget{Collection: c, Request: r.ID}
	if err := w.StoreEdit(target, r.Versions[0].Label, editOf("http://my-edit.example"), SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	plan, err := w.PreviewImport(ImportOpenAPI, readFixture(t, "openapi/testdata/petstore30.yaml"), c)
	if err != nil {
		t.Fatal(err)
	}

	err = w.ApplyImport(plan, ApplyOptions{Overwrite: true})

	after := requestAt(t, w, c, "pets/getPet")
	if err != nil || after.Edit == nil || after.Edit.URL != "http://my-edit.example" || after.EditBase != r.Versions[0].Label {
		t.Fatalf("err %v edit %+v base %q", err, after.Edit, after.EditBase)
	}
}

func TestPreviewImportSource_openapi_should_keep_the_document_in_descriptions(t *testing.T) {
	w, c := petsCollection(t)
	data := readFixture(t, "openapi/testdata/petstore30.yaml")

	plan, err := w.PreviewImportSource(ImportSource{Kind: ImportOpenAPI, Name: "/specs/Pet Store.openapi.yaml", Data: data}, c)

	if err != nil || !slices.Contains(relsOf(plan), "descriptions/Pet-Store.openapi.yaml") {
		t.Fatalf("files %v, err %v", relsOf(plan), err)
	}
	if plain, _ := w.PreviewImport(ImportOpenAPI, data, c); !slices.Contains(relsOf(plain), "descriptions/imported.openapi.yaml") {
		t.Fatalf("an unnamed source: %v", relsOf(plain))
	}
}

func TestImportAsyncAPI_should_write_async_requests_and_keep_the_document(t *testing.T) {
	w, c := petsCollection(t)
	data := readFixture(t, "asyncapi/testdata/streetlights3.yaml")

	plan, err := w.PreviewImportSource(ImportSource{Kind: ImportAsyncAPI, Name: "streetlights.yaml", Data: data}, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	got := requestAt(t, w, c, "turnOn")
	if got.Versions[0].Async == nil || got.Versions[0].Async.Protocol != "mqtt" {
		t.Fatalf("request %+v", got)
	}
	if !slices.Contains(relsOf(plan), "descriptions/streetlights.asyncapi.yaml") {
		t.Fatalf("files %v", relsOf(plan))
	}
}

func TestImport_should_apply_overlays_before_mapping_and_keep_both_documents(t *testing.T) {
	w, c := petsCollection(t)
	spec := "openapi: 3.0.3\ninfo: {title: t, version: '1'}\npaths:\n  /things:\n    get:\n      responses: {'200': {description: ok}}\n"
	overlay := "overlay: 1.0.0\ninfo: {title: fixes, version: 1.0.0}\nactions:\n  - target: $.paths['/things'].get\n    update: {operationId: listThings}\n"
	asyncSpec := "asyncapi: 3.0.0\nservers: {s: {host: 'localhost:4222', protocol: nats}}\nchannels: {c: {address: things}}\noperations: {o: {action: send, channel: {$ref: '#/channels/c'}}}\n"
	asyncOverlay := "overlay: 1.0.0\ninfo: {title: fixes, version: 1.0.0}\nactions:\n  - target: $.operations\n    update: {sendThing: {action: send, channel: {$ref: '#/channels/c'}}}\n  - target: $.operations.o\n    remove: true\n"

	plan, err := w.PreviewImportSource(ImportSource{Kind: ImportOpenAPI, Name: "things.yaml", Data: []byte(spec), Overlays: [][]byte{[]byte(overlay)}}, c)
	if err != nil {
		t.Fatal(err)
	}
	asyncPlan, asyncErr := w.PreviewImportSource(ImportSource{Kind: ImportAsyncAPI, Name: "things.asyncapi.yaml", Data: []byte(asyncSpec), Overlays: [][]byte{[]byte(asyncOverlay)}}, c)

	if !slices.Contains(relsOf(plan), "requests/listThings.req.yaml") || !slices.Contains(relsOf(plan), "descriptions/things.overlay.yaml") || !slices.Contains(relsOf(plan), "descriptions/things.openapi.yaml") {
		t.Fatalf("files %v", relsOf(plan))
	}
	if asyncErr != nil || !slices.Contains(relsOf(asyncPlan), "requests/sendThing.req.yaml") {
		t.Fatalf("async files %v, err %v", relsOf(asyncPlan), asyncErr)
	}
	if _, err := w.PreviewImportSource(ImportSource{Kind: ImportOpenAPI, Data: []byte(spec), Overlays: [][]byte{[]byte("not: overlay")}}, c); err == nil {
		t.Fatal("an invalid overlay must refuse the import")
	}
}
