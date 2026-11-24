// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

const bareSpec = `openapi: 3.0.3
info: {title: Pets, version: "1"}
paths:
  /pets:
    get:
      summary: List pets
      operationId: listPets
      tags: [pets]
      security: []
      responses: {"200": {description: ok}}
`

func writeKept(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "descriptions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func specCodes(l diag.List) []diag.Code {
	var out []diag.Code
	for _, d := range l {
		if IsSpecDiagnostic(d) {
			out = append(out, d.Code)
		}
	}
	return out
}

func TestLintSpecs_Given_a_kept_description_When_linted_Then_it_shows_in_Validate_too(t *testing.T) {
	w, c := petsCollection(t)
	writeKept(t, collectionRoot(t, w, c), "pets.openapi.yaml", bareSpec)

	got, err := w.LintSpecs(c)

	if err != nil || len(got) != 1 || got[0].Code != diag.CodeSpecInfo || got[0].Args["spec"] != "pets" {
		t.Fatalf("findings %v err %v, want the missing info.description", got, err)
	}
	if all := specCodes(w.Validate()); len(all) != 1 || all[0] != diag.CodeSpecInfo {
		t.Fatalf("Validate carries %v", all)
	}
}

// Before this, a description written in an overlay would have left the finding on the file it was written for.
func TestLintSpecs_Given_an_overlay_that_adds_the_description_When_linted_Then_it_is_clear(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, "pets.openapi.yaml", bareSpec)
	writeKept(t, root, "pets.edits.overlay.yaml", "overlay: 1.0.0\ninfo: {title: edits, version: 1.0.0}\nactions:\n  - target: $.info\n    update: {description: The pet shop}\n")

	got, err := w.LintSpecs(c)

	if err != nil || len(got) != 0 {
		t.Fatalf("findings %v err %v, want none", got, err)
	}
}

func TestLintSpecs_Given_a_lint_file_When_linted_Then_a_rule_is_off_another_an_error_and_a_typo_reported(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, "pets.openapi.yaml", bareSpec)
	writeKept(t, root, SpecLintFile, "rules:\n  info-description: off\n  path-case: error\n  typo: off\n")
	writeKept(t, root, "shop.openapi.yaml", "openapi: 3.0.3\ninfo: {title: Shop, version: '1'}\npaths:\n  /Shop:\n    get: {summary: s, operationId: s, tags: [s], security: [], responses: {'200': {description: ok}}}\n")

	got, err := w.LintSpecs(c)

	if err != nil || len(got) != 2 {
		t.Fatalf("findings %v err %v", got, err)
	}
	if got[0].Code != diag.CodeSpecLintConfig || got[1].Code != diag.CodeSpecPathCase || got[1].Severity != diag.SeverityError {
		t.Fatalf("findings = %+v", got)
	}
}

func TestLintSpecs_Given_a_changed_description_When_linted_again_Then_the_memo_does_not_hide_it(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, "pets.openapi.yaml", bareSpec)
	if got, _ := w.LintSpecs(c); len(got) != 1 {
		t.Fatalf("first: %v", got)
	}

	writeKept(t, root, "pets.openapi.yaml", bareSpec[:len(bareSpec)-1]+"\n  /dogs:\n    get: {responses: {'200': {description: ok}}}\n")
	got, _ := w.LintSpecs(c)

	if len(got) < 5 {
		t.Fatalf("second: %v, want the new operation's gaps as well", got)
	}
}

func TestLintSpecs_Given_an_unknown_collection_Then_an_error(t *testing.T) {
	w, _ := petsCollection(t)

	if _, err := w.LintSpecs(model.ID("nope")); err == nil {
		t.Fatal("no error")
	}
}

func TestSetSpecLintRule_Given_choices_When_set_and_read_back_Then_the_file_says_only_what_differs_from_the_default(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, "pets.openapi.yaml", bareSpec)

	if err := w.SetSpecLintRule(c, "path-case", "error"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetSpecLintRule(c, "operation-tags", "off"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetSpecLintRule(c, "operation-id", "warning"); err != nil { // the default: nothing to write
		t.Fatal(err)
	}
	rules, err := w.SpecLintRules(c)

	if err != nil {
		t.Fatal(err)
	}
	levels := map[string]string{}
	chosen := map[string]bool{}
	for _, r := range rules {
		levels[r.Name], chosen[r.Name] = r.Level, r.Chosen
	}
	if levels["path-case"] != "error" || levels["operation-tags"] != "off" || levels["operation-id"] != "warning" || chosen["operation-id"] || !chosen["path-case"] {
		t.Fatalf("levels %v chosen %v", levels, chosen)
	}
	data, _ := os.ReadFile(filepath.Join(root, "descriptions", SpecLintFile))
	if want := "# How seriously each rule about the API description is taken: off, warning or error.\nrules:\n  path-case: error\n  operation-tags: off\n"; string(data) != want {
		t.Fatalf("file:\n%s", data)
	}
	if got, _ := w.LintSpecs(c); len(specCodes(got)) != 1 { // only info-description is left: tags off, path ok
		t.Fatalf("findings %v", got)
	}
}

// Before: the lint file was written whole from the rules, so a comment a team wrote in it was lost on the first change.
func TestSetSpecLintRule_Given_a_file_with_comments_When_a_rule_is_set_Then_the_comments_stay(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, SpecLintFile, "# agreed in the API guild\nrules:\n  path-case: warning  # gateway needs it\n")

	if err := w.SetSpecLintRule(c, "path-case", "error"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetSpecLintRule(c, "operation-tags", "off"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "descriptions", SpecLintFile))
	if want := "# agreed in the API guild\nrules:\n  path-case: error  # gateway needs it\n  operation-tags: off\n"; string(data) != want {
		t.Fatalf("file:\n%s", data)
	}
}

// Before: a finding had a YAML path and no line, so it could not be jumped to.
func TestLintSpecs_Given_a_finding_When_validated_Then_it_has_the_line_it_is_on_in_the_kept_file(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, "pets.openapi.yaml", "openapi: 3.0.3\ninfo: {title: Pets, version: '1', description: d}\npaths:\n  /pets:\n    get:\n      summary: List\n      operationId: l\n      security: []\n      responses: {'200': {description: ok}}\n")

	var got []diag.Diagnostic
	for _, d := range w.Validate() {
		if IsSpecDiagnostic(d) {
			got = append(got, d)
		}
	}

	if len(got) != 1 || got[0].Code != diag.CodeSpecTags || got[0].Pos.Line != 5 {
		t.Fatalf("findings = %+v, want the missing tag on line 5 (the get)", got)
	}
}

func TestLintSpecs_Given_a_place_an_overlay_added_When_validated_Then_the_line_is_the_nearest_one_that_is_in_the_file(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, "pets.openapi.yaml", "openapi: 3.0.3\ninfo: {title: Pets, version: '1', description: d}\npaths:\n  /pets:\n    get:\n      summary: List\n      operationId: l\n      tags: [t]\n      security: []\n      responses: {'200': {description: ok}}\n")
	writeKept(t, root, "pets.edits.overlay.yaml", "overlay: 1.0.0\ninfo: {title: edits, version: 1.0.0}\nactions:\n  - target: $.paths\n    update: {/dogs: {get: {operationId: d, tags: [t], security: [], responses: {'200': {description: ok}}}}}\n")

	got := specDiagnostics(w.Validate())

	if len(got) != 1 || got[0].Code != diag.CodeSpecSummary || got[0].Path != "$.paths.'/dogs'.get" || got[0].Pos.Line != 3 {
		t.Fatalf("findings = %+v, want the new get, on the line of paths:", got)
	}
}

func specDiagnostics(l diag.List) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range l {
		if IsSpecDiagnostic(d) {
			out = append(out, d)
		}
	}
	return out
}

// Before: only OpenAPI was linted. AsyncAPI, OpenRPC and each .proto a collection kept are read now.
func TestLintSpecs_Given_asyncapi_openrpc_and_proto_kept_When_linted_Then_each_is_checked_by_its_own_fields(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, "orders.asyncapi.yaml", "asyncapi: 2.6.0\ninfo: {title: Orders, version: '1', description: d}\nchannels:\n  created:\n    publish: {summary: s, operationId: o, tags: [{name: t}]}\n")
	writeKept(t, root, "calc.openrpc.json", `{"openrpc":"1.2.6","info":{"title":"Calc","version":"1","description":"d"},"methods":[{"name":"add","tags":[{"name":"m"}],"result":{"name":"r","schema":{}}}]}`)
	if err := os.MkdirAll(filepath.Join(root, "protos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "protos", "shop.proto"), []byte("syntax = \"proto3\";\n// Shop.\nservice Shop {\n  rpc Buy(M) returns (M);\n}\nmessage M {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := w.LintSpecs(c)

	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]diag.Diagnostic{}
	for _, d := range got {
		byName[d.Args["spec"]] = d
	}
	if len(got) != 2 || byName["calc"].Code != diag.CodeSpecSummary || byName["calc"].Pos.Line != 1 {
		t.Fatalf("findings = %+v", got)
	}
	if p := byName["shop.proto"]; p.Code != diag.CodeSpecSummary || p.Pos.Line != 4 || p.Args["where"] != "Shop.Buy" {
		t.Fatalf("proto finding = %+v", p)
	}
}

func TestSetSpecLintRule_Given_the_default_level_When_set_Then_the_choice_is_forgotten(t *testing.T) {
	w, c := petsCollection(t)
	if err := w.SetSpecLintRule(c, "path-case", "error"); err != nil {
		t.Fatal(err)
	}

	if err := w.SetSpecLintRule(c, "path-case", ""); err != nil {
		t.Fatal(err)
	}
	rules, _ := w.SpecLintRules(c)

	for _, r := range rules {
		if r.Chosen {
			t.Fatalf("%s is still chosen", r.Name)
		}
	}
}

func TestSetSpecLintRule_Given_a_wrong_name_or_level_Then_nothing_is_written(t *testing.T) {
	w, c := petsCollection(t)

	if err := w.SetSpecLintRule(c, "made-up", "off"); err == nil {
		t.Fatal("a made-up rule was accepted")
	}
	if err := w.SetSpecLintRule(c, "path-case", "loud"); err == nil {
		t.Fatal("a made-up level was accepted")
	}
	if _, err := os.Stat(filepath.Join(collectionRoot(t, w, c), "descriptions", SpecLintFile)); err == nil {
		t.Fatal("a file was written")
	}
}

// Before: nothing could hand a collection's API description to someone outside Flowi; the generated one sat in
// descriptions/ and a kept one was without the overlays that corrected it.
func TestExportDescription_Given_a_kept_description_with_an_overlay_When_exported_Then_the_overlay_is_in_it(t *testing.T) {
	w, c := petsCollection(t)
	root := collectionRoot(t, w, c)
	writeKept(t, root, "pets.openapi.yaml", bareSpec)
	writeKept(t, root, "pets.edits.overlay.yaml", "overlay: 1.0.0\ninfo: {title: edits, version: 1.0.0}\nactions:\n  - target: $.info\n    update: {description: The pet shop}\n")
	writeKept(t, root, "requests.openapi.yaml", "openapi: 3.0.3\ninfo: {title: generated, version: '1'}\npaths: {}\n")
	writeKept(t, root, "orders.asyncapi.yaml", "asyncapi: 2.6.0\ninfo: {title: o, version: '1'}\nchannels: {}\n")

	items, err := w.ExportItems(c)
	got, gotErr := w.ExportDescription(c, "kept:pets.openapi.yaml")
	generated, _ := w.ExportDescription(c, ExportGenerated)

	if err != nil || gotErr != nil {
		t.Fatalf("items err %v, export err %v", err, gotErr)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if want := []string{"generated", "kept:pets.openapi.yaml", "kept:orders.asyncapi.yaml"}; len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("items %v, want %v (the generated one first, no lint file, no proto)", ids, want)
	}
	if !strings.Contains(string(got), "The pet shop") || !strings.Contains(string(generated), "generated") {
		t.Fatalf("kept:\n%s\ngenerated:\n%s", got, generated)
	}
	if _, err := w.ExportDescription(c, "kept:nope.openapi.yaml"); !errors.Is(err, ErrNoExport) {
		t.Fatalf("err = %v", err)
	}
	if _, err := w.ExportItems("nope"); err == nil {
		t.Fatal("an unknown collection gave no error")
	}
}
