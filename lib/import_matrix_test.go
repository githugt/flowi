// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"
)

// The import matrix: one fixture of an example service (REST, secure REST,
// events) in every format Flowi reads, each checked against what the import
// is expected to make of it (see import-test-matrix.md next to the sources).

const matrixDir = "testdata/my-example-app"

func matrixSource(t *testing.T, kind ImportKind, file string) ImportSource {
	t.Helper()
	return ImportSource{Kind: kind, Name: file, Data: readFixture(t, filepath.Join(matrixDir, file))}
}

// importMatrix previews src into a new collection and applies it.
func importMatrix(t *testing.T, w *Workspace, c model.ID, src ImportSource) ImportPlan {
	t.Helper()
	plan, err := w.PreviewImportSource(src, c)
	if err != nil {
		t.Fatalf("importing %s: %v", src.Name, err)
	}
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatalf("applying %s: %v", src.Name, err)
	}
	return plan
}

// requestPaths are the paths of the collection's requests, in order.
func requestPaths(t *testing.T, w *Workspace, c model.ID) []string {
	t.Helper()
	var out []string
	for _, col := range w.Collections() {
		if col.Manifest.ID == c {
			for _, r := range col.Requests {
				out = append(out, r.Path)
			}
		}
	}
	slices.Sort(out)
	return out
}

// planText is everything the plan would write, to look for text that must not be in it.
func planText(p ImportPlan) string {
	var b strings.Builder
	for _, data := range p.data {
		b.Write(data)
	}
	return b.String()
}

func diagCodes(l diag.List) map[diag.Code]int {
	out := map[diag.Code]int{}
	for _, d := range l {
		out[d.Code]++
	}
	return out
}

func TestImportMatrix_curl_should_make_a_request_per_command_and_keep_no_credential(t *testing.T) {
	w, c := petsCollection(t)

	plan := importMatrix(t, w, c, matrixSource(t, ImportCurl, "curl-commands.txt"))

	if got := len(requestPaths(t, w, c)); got != 6 {
		t.Errorf("%d requests, want one for each of the 6 commands (bash and Windows quoting)", got)
	}
	if text := planText(plan); strings.Contains(text, "hunter2") || strings.Contains(text, "key-literal-123") {
		t.Error("a credential from a command reached a file")
	}
}

func TestImportMatrix_OpenAPI_should_make_five_requests_with_contracts_from_every_version_of_the_format(t *testing.T) {
	for _, file := range []string{"openapi-2.0.yaml", "openapi-3.0.yaml", "openapi-3.1.yaml"} {
		t.Run(file, func(t *testing.T) {
			w, c := petsCollection(t)

			plan := importMatrix(t, w, c, matrixSource(t, ImportOpenAPI, file))

			paths := requestPaths(t, w, c)
			if len(paths) != 5 {
				t.Fatalf("requests %v, want 5", paths)
			}
			schemas := 0
			for _, rel := range relsOf(plan) {
				if strings.Contains(rel, ".schemas/") {
					schemas++
				}
			}
			if schemas == 0 {
				t.Errorf("no response schema was kept as a contract: %v", relsOf(plan))
			}
			create := requestAt(t, w, c, "createExample")
			if create.Versions[0].Method != "POST" || !strings.HasSuffix(create.Versions[0].URL, "/examples") || create.Versions[0].Body.JSON == "" {
				t.Errorf("createExample: %+v", create.Versions[0])
			}
		})
	}
}

func TestImportMatrix_OpenAPI_with_gaps_should_import_name_by_method_and_path_and_warn_about_each_missing_operationId(t *testing.T) {
	w, c := petsCollection(t)

	plan := importMatrix(t, w, c, matrixSource(t, ImportOpenAPI, "openapi-gaps.yaml"))

	paths := requestPaths(t, w, c)
	if len(paths) != 3 {
		t.Fatalf("requests %v, want 3 (the two paths that differ in case stay apart)", paths)
	}
	lower := map[string]bool{}
	for _, p := range paths {
		lower[strings.ToLower(p)] = true
	}
	if len(lower) != 3 {
		t.Errorf("%v would collide on a case-insensitive disk", paths)
	}
	if n := diagCodes(plan.Diags)[diag.CodeImportNoOpID]; n != 3 {
		t.Errorf("%d warnings about a missing operationId, want 3 (%v)", n, diagCodes(plan.Diags))
	}
}

func TestImportMatrix_OpenAPI_split_over_files_should_follow_the_files_next_to_it(t *testing.T) {
	w, c := petsCollection(t)
	src := matrixSource(t, ImportOpenAPI, "openapi-split/openapi.yaml")
	src.RefDir = filepath.Join(matrixDir, "openapi-split")

	plan := importMatrix(t, w, c, src)

	create := requestAt(t, w, c, "createExample")
	if !strings.Contains(create.Versions[0].Body.JSON, `"name"`) {
		t.Errorf("the body should come from schemas/example.yaml: %q", create.Versions[0].Body.JSON)
	}
	if !slices.Contains(relsOf(plan), "descriptions/openapi.files/schemas/example.yaml") {
		t.Errorf("the file should be kept for the update: %v", relsOf(plan))
	}
}

func TestImportMatrix_Postman_should_make_folders_variables_and_examples_and_report_the_script(t *testing.T) {
	w, c := petsCollection(t)

	plan := importMatrix(t, w, c, matrixSource(t, ImportPostman, "postman-collection.json"))

	paths := requestPaths(t, w, c)
	if !slices.Contains(paths, "REST/Create") || !slices.Contains(paths, "Secure/Get-secure") {
		t.Errorf("requests %v, want them in their folders", paths)
	}
	if got := requestAt(t, w, c, "REST/Create").Versions[0].URL; got != "{{env.baseUrl}}/examples" {
		t.Errorf("url %q", got)
	}
	if !slices.ContainsFunc(relsOf(plan), func(rel string) bool { return strings.Contains(rel, ".examples/") }) {
		t.Errorf("the saved response should be an example: %v", relsOf(plan))
	}
	if diagCodes(plan.Diags)[diag.CodeImportUnmapped] == 0 && !slices.ContainsFunc(plan.Items, func(it importers.Item) bool { return it.Diags.HasErrors() || len(it.Diags) > 0 }) {
		t.Error("the pre-request script should be reported")
	}
	if strings.Contains(planText(plan), "hunter2") {
		t.Error("a password reached a file")
	}
}

func TestImportMatrix_Postman_environment_should_not_write_a_secret_value(t *testing.T) {
	w, c := petsCollection(t)

	plan := importMatrix(t, w, c, matrixSource(t, ImportPostman, "postman-environment.json"))

	text := planText(plan)
	if strings.Contains(text, "key-literal-123") || !strings.Contains(text, "baseUrl") {
		t.Errorf("environment text:\n%s", text)
	}
}

func TestImportMatrix_Bruno_should_keep_auth_asserts_and_post_response_variables(t *testing.T) {
	w, c := petsCollection(t)

	importMatrix(t, w, c, ImportSource{Kind: ImportBruno, Dir: filepath.Join(matrixDir, "bruno")})

	req := requestAt(t, w, c, "get-example")
	v := req.Versions[0]
	if len(v.Assert) != 1 || len(v.Extract) != 1 || v.Extract[0].Name != "exampleName" {
		t.Errorf("asserts %v, extracts %+v", v.Assert, v.Extract)
	}
	if !slices.ContainsFunc(v.Headers, func(h model.KeyValue) bool { return h.Key == "Authorization" }) {
		t.Errorf("the basic auth should become an Authorization header: %+v", v.Headers)
	}
}

func TestImportMatrix_Arazzo_should_make_a_flow_whose_steps_call_the_imported_requests(t *testing.T) {
	for _, file := range []string{"examples.arazzo.yaml", "examples-1.1.arazzo.yaml"} {
		t.Run(file, func(t *testing.T) {
			w, c := petsCollection(t)
			importMatrix(t, w, c, matrixSource(t, ImportOpenAPI, "openapi-3.0.yaml"))

			plan := importMatrix(t, w, c, matrixSource(t, ImportArazzo, file))

			flows := 0
			for _, rel := range relsOf(plan) {
				if strings.HasPrefix(rel, "flows/arazzo/") {
					flows++
				}
			}
			if flows != 1 {
				t.Errorf("flow files %v, want one", relsOf(plan))
			}
			if diagCodes(plan.Diags)[diag.CodeImportUnmapped] != 0 {
				t.Errorf("every step names an imported operation, but %v", plan.Diags)
			}
		})
	}
}

func TestImportMatrix_AsyncAPI_should_make_NATS_requests_from_both_versions_of_the_format(t *testing.T) {
	for file, wantRequests := range map[string]int{"asyncapi-3.0.yaml": 2, "asyncapi-2.6.yaml": 2} {
		t.Run(file, func(t *testing.T) {
			w, c := petsCollection(t)

			importMatrix(t, w, c, matrixSource(t, ImportAsyncAPI, file))

			paths := requestPaths(t, w, c)
			if len(paths) != wantRequests {
				t.Fatalf("requests %v, want %d", paths, wantRequests)
			}
			for _, p := range paths {
				v := requestAt(t, w, c, p).Versions[0]
				if v.Async == nil || v.Async.Protocol != model.ProtocolNATS {
					t.Errorf("%s: %+v, want a NATS request", p, v.Async)
				}
			}
		})
	}
}
