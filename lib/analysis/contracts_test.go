// SPDX-License-Identifier: Apache-2.0

package analysis_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// contractFixture is a collection with one request file; examples and
// saved responses are written per test.
type contractFixture struct {
	t       *testing.T
	root    string
	data    string
	coll    model.Collection
	request model.Request
}

func newContractFixture(t *testing.T) *contractFixture {
	t.Helper()
	root := t.TempDir()
	r := model.Request{Meta: model.Meta{ID: "01JA0000000000000000000010"}, Path: "auth/login",
		File: filepath.Join(root, "requests", "auth", "login.req.yaml")}
	c := model.Collection{Root: root, Manifest: model.Manifest{Meta: model.Meta{ID: "01JA0000000000000000000001"}}, Requests: []model.Request{r}}
	return &contractFixture{t: t, root: root, data: t.TempDir(), coll: c, request: r}
}

func (f *contractFixture) write(file, content string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *contractFixture) example(name, content string) string {
	file := filepath.Join(f.root, "requests", "auth", "login.examples", name)
	f.write(file, content)
	return file
}

func (f *contractFixture) saved(version, content string) string {
	file := filepath.Join(f.data, string(f.coll.Manifest.ID), "responses", string(f.request.ID), version, "last.json")
	f.write(file, content)
	return file
}

func (f *contractFixture) contract(version string) (analysis.Contract, diag.List) {
	src := analysis.FileContracts{DataDir: func(id model.ID) string { return filepath.Join(f.data, string(id)) }}
	return src.Contract(f.coll, f.request, version)
}

func sampleFiles(c analysis.Contract) []string {
	var out []string
	for _, s := range c.Samples {
		out = append(out, filepath.Base(s.File))
	}
	slices.Sort(out)
	return out
}

func TestFileContracts_Given_examples_for_two_versions_Then_only_matching_and_unversioned(t *testing.T) {
	f := newContractFixture(t)
	f.example("v1.json", `{"version": "1", "status": 200, "body": {"data": {"token": "t"}}}`)
	f.example("v2.json", `{"version": "2", "status": 200, "body": {"token": "t"}}`)
	f.example("any.json", `{"status": 404, "body": {"error": "x"}}`)

	got, diags := f.contract("2")

	if got.Source != "example" || !slices.Equal(sampleFiles(got), []string{"any.json", "v2.json"}) || len(diags) != 0 {
		t.Fatalf("got %+v %v, want example any.json + v2.json", got, diags)
	}
}

func TestFileContracts_Given_no_examples_Then_last_saved_response(t *testing.T) {
	f := newContractFixture(t)
	f.example("v1.json", `{"version": "1", "status": 200, "body": {}}`)
	f.saved("2", `{"status": 200, "headers": [{"name": "Set-Cookie", "value": "sid=1"}], "body": {"json": {"token": "t"}, "size": 13}}`)

	got, _ := f.contract("2")

	if got.Source != "response" || len(got.Samples) != 1 {
		t.Fatalf("got %+v, want the saved response", got)
	}
	s := got.Samples[0]
	if s.Status != 200 || !s.HeadersKnown || s.Headers.Get("set-cookie") != "sid=1" || s.Body.(map[string]any)["token"] != "t" {
		t.Fatalf("sample %+v", s)
	}
}

func TestFileContracts_Given_text_saved_response_Then_text_sample(t *testing.T) {
	f := newContractFixture(t)
	f.saved("1", `{"status": 200, "body": {"text": "ok", "size": 2}}`)

	got, _ := f.contract("1")

	if len(got.Samples) != 1 || got.Samples[0].Body != nil || got.Samples[0].Text != "ok" {
		t.Fatalf("got %+v, want a text sample", got)
	}
}

func TestFileContracts_Given_nothing_Then_empty_source(t *testing.T) {
	f := newContractFixture(t)

	got, diags := f.contract("1")

	if got.Source != "" || len(got.Samples) != 0 || len(diags) != 0 {
		t.Fatalf("got %+v %v, want no contract", got, diags)
	}
}

func TestFileContracts_Given_broken_example_Then_FLW_CONTRACT_003_and_skipped(t *testing.T) {
	f := newContractFixture(t)
	bad := f.example("bad.json", `{"status": 200, "body": `)
	f.example("good.json", `{"status": 200, "body": {"token": "t"}}`)

	got, diags := f.contract("1")

	if len(got.Samples) != 1 || len(diags) != 1 || diags[0].Code != diag.CodeContractExample || diags[0].File != bad || diags[0].Severity != diag.SeverityWarning {
		t.Fatalf("got %+v %v, want good.json and one warning on bad.json", got, diags)
	}
}

func TestFileContracts_Given_example_without_headers_Then_headers_unknown(t *testing.T) {
	f := newContractFixture(t)
	f.example("200.json", `{"status": 200, "body": {"token": "t"}}`)

	got, _ := f.contract("1")

	if len(got.Samples) != 1 || got.Samples[0].HeadersKnown {
		t.Fatalf("got %+v, want headers unknown", got)
	}
}

func TestFileContracts_should_ignore_non_json_files_in_examples(t *testing.T) {
	f := newContractFixture(t)
	f.example("README.md", "notes")
	f.example("200.json", `{"status": 200, "body": {}}`)

	got, diags := f.contract("1")

	if len(got.Samples) != 1 || len(diags) != 0 {
		t.Fatalf("got %+v %v", got, diags)
	}
}

func (f *contractFixture) schema(name, content string) string {
	file := filepath.Join(f.root, "requests", "auth", "login.schemas", name)
	f.write(file, content)
	return file
}

func TestFileContracts_Given_schema_Then_it_beats_examples(t *testing.T) {
	f := newContractFixture(t)
	f.example("200.json", `{"status": 200, "body": {"token": "t"}}`)
	f.schema("1-200.json", `{"type": "object", "properties": {"token": {"type": "string"}}}`)
	f.schema("2-200.json", `{"type": "object"}`)

	got, diags := f.contract("1")

	if got.Source != analysis.SourceSchema || len(got.Samples) != 1 || got.Samples[0].Status != 200 || got.Samples[0].Schema == nil || len(diags) != 0 {
		t.Fatalf("got %+v %v", got, diags)
	}
}
