// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
)

const standaloneWorkflow = `arazzo: 1.0.1
info: {title: pets, version: 1.0.0}
sourceDescriptions: [{name: pets, url: ./pets.yaml, type: openapi}]
workflows:
  - workflowId: fetchName
    inputs:
      type: object
      properties:
        petId: {type: string}
    steps:
      - stepId: fetch
        operationId: getPet
        parameters:
          - {name: petId, in: path, value: $inputs.petId}
        successCriteria:
          - condition: $statusCode == 200
`

const standaloneOpenAPI = `openapi: 3.0.3
info: {title: pets, version: 1.0.0}
servers: [{url: SERVER}]
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      parameters:
        - {name: petId, in: path, required: true, schema: {type: string}}
      responses:
        "200": {description: ok}
`

func standaloneServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"rex"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeStandalone(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pets.yaml"), []byte(strings.Replace(standaloneOpenAPI, "SERVER", srv.URL, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "fetch.arazzo.yaml")
	if err := os.WriteFile(file, []byte(standaloneWorkflow), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestOpenArazzo_Given_a_standalone_document_Then_it_runs_against_its_source_description(t *testing.T) {
	// Arrange
	srv := standaloneServer(t)
	file := writeStandalone(t, srv)
	w := newEnv(t).open(t)

	// Act
	doc, err := w.OpenArazzo(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	run, err := w.RunFlow(context.Background(), doc.Flows[0], RunOptions{Env: doc.Env, Inputs: map[string]string{"petId": "7"}})
	if err != nil {
		t.Fatal(err)
	}
	for range run.Events() { //nolint:revive // draining the events lets the run end
	}
	res := run.Wait()

	// Assert
	if res.Status != engine.StatusPassed || len(res.Captured) != 1 || !strings.HasSuffix(res.Captured[0].Request.URL, "/pets/7") {
		t.Fatalf("status %s captured %+v diags %v", res.Status, res.Captured, res.Diags)
	}
}

func TestOpenArazzo_Given_it_is_closed_Then_the_workspace_has_no_trace_of_it(t *testing.T) {
	// Arrange
	file := writeStandalone(t, standaloneServer(t))
	w := newEnv(t).open(t)
	doc, err := w.OpenArazzo(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	root := w.Collections()[0].Root

	// Act
	doc.Close()

	// Assert
	if len(w.Collections()) != 0 {
		t.Fatalf("collections left: %d", len(w.Collections()))
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("temporary collection still on disk: %v", err)
	}
}

func TestOpenArazzo_Given_a_file_that_is_not_arazzo_Then_it_says_so_and_leaves_nothing_behind(t *testing.T) {
	// Arrange
	file := filepath.Join(t.TempDir(), "x.yaml")
	if err := os.WriteFile(file, []byte("hello: world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := newEnv(t).open(t)

	// Act
	_, err := w.OpenArazzo(context.Background(), file)

	// Assert
	if err == nil || len(w.Collections()) != 0 {
		t.Fatalf("err %v collections %d", err, len(w.Collections()))
	}
}

func TestKeepArazzo_Given_a_document_Then_its_flows_stay_in_the_workspace_and_run_against_its_own_server_without_an_environment(t *testing.T) {
	// Arrange
	srv := standaloneServer(t)
	file := writeStandalone(t, srv)
	w := newEnv(t).open(t)

	// Act
	doc, err := w.KeepArazzo(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	res := runToEnd(t, w, doc.Flows[0], RunOptions{Inputs: map[string]string{"petId": "7"}})

	// Assert
	if res.Status != engine.StatusPassed {
		t.Fatalf("status %s %v", res.Status, res.Diags)
	}
	cs := w.Collections()
	if len(cs) != 1 || !strings.Contains(cs[0].Manifest.Name, "Arazzo") {
		t.Fatalf("collections %+v: it should be there and not look like a mounted one", cs)
	}
}
