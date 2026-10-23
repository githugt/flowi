// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/examples"
	"github.com/githugt/flowi-lib/examples/demoserver"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"

	"github.com/githugt/flowi/internal/backend"
	"github.com/githugt/flowi/internal/uithread"
	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

// openExample opens a fresh workspace with the example collection mounted
// and pointed at an in-process demo server.
func openExample(t *testing.T) vm.Backend {
	t.Helper()
	srv := httptest.NewServer(demoserver.Handler(demoserver.Options{Password: "demo"}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	err := fs.WalkDir(examples.UserCreation(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(examples.UserCreation(), p)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, "example", p)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, []byte(strings.ReplaceAll(string(data), "http://localhost:8080", srv.URL)), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	// Environments are global: the example's become the workspace's.
	if err := os.MkdirAll(filepath.Join(dir, "config", "workspaces", "default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "example", "environments"), filepath.Join(dir, "config", "workspaces", "default", "environments")); err != nil {
		t.Fatal(err)
	}
	environ := []string{"FLOWI_CONFIG_DIR=" + filepath.Join(dir, "config"), "FLOWI_VAULT_KEY=test-only", "FLOWI_SECRET_LOCAL_PASSWORD=demo"}
	paths, err := settings.DiscoverPaths(environ)
	if err != nil {
		t.Fatal(err)
	}
	ws, _, err := flowi.OpenWorkspace(context.Background(), flowi.WorkspaceOptions{Paths: paths, Environ: environ})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.Mount(filepath.Join(dir, "example")); err != nil {
		t.Fatal(err)
	}
	return backend.New(ws, uithread.NewGuard(true)) // never marked: tests have no UI goroutine
}

func TestSendVM_integration_against_demoserver(t *testing.T) {
	b := openExample(t)
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	c := ws.Collections()[0]
	var login model.Request
	for _, r := range c.Requests {
		if r.Path == "auth/login" {
			login = r
		}
	}
	req := vm.NewRequest(b, w, c, login)
	history := vm.NewHistory()
	send := vm.NewSend(b, w, ws, vm.NewRunLog(), history)
	ws.SelectEnv("local")

	send.Send(req, vm.SendOptions{With: []model.KeyValue{{Key: "email", Value: "ada@example.com"}}})
	w.Wait()

	last, ok := send.Last()
	if !ok || last.Result.Status != engine.StatusPassed || last.Result.Record.Status != 200 || send.Err() != "" {
		t.Fatalf("last %+v err %q", last.Result, send.Err())
	}
}

func TestRequestVM_integration_save_and_extract_as_variable(t *testing.T) {
	b := openExample(t)
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	c := ws.Collections()[0]
	login := c.Requests[0]
	for _, r := range c.Requests {
		if r.Path == "auth/login" {
			login = r
		}
	}
	req := vm.NewRequest(b, w, c, login)

	err := req.ExtractAsVariable("expires", "$.data.expires")
	w.Wait()

	data, _ := os.ReadFile(login.File)
	if err != nil || req.Err() != "" || !strings.Contains(string(data), "expires:") || !strings.Contains(string(data), "$.data.expires") {
		t.Fatalf("err %v %q, file:\n%s", err, req.Err(), data)
	}
}

func TestImportWizard_integration_imports_openapi_into_the_collection(t *testing.T) {
	b := openExample(t)
	spec := filepath.Join(t.TempDir(), "pets.yaml")
	doc := `openapi: 3.0.3
info: {title: Pets, version: 1.0.0}
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      tags: [pets]
      parameters:
        - {name: petId, in: path, required: true, schema: {type: integer}}
      responses:
        "200":
          description: a pet
          content:
            application/json:
              schema: {type: object, properties: {name: {type: string}}}
`
	if err := os.WriteFile(spec, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	iw := vm.NewImportWizard(b, w, ws)
	iw.Load()
	w.Wait()
	iw.SetFile(spec)

	iw.Preview()
	w.Wait()
	iw.Import(vm.ImportRefuseExisting, false)
	w.Wait()

	if iw.Err() != "" || iw.Done() != "Imported 3 files into User creation" {
		t.Fatalf("err %q done %q rows %+v", iw.Err(), iw.Done(), iw.Rows())
	}
	found := false
	for _, r := range b.Collections()[0].Requests {
		found = found || (r.Path == "pets/getPet" && r.Operation == "getPet")
	}
	if !found {
		t.Fatal("the imported request is loaded")
	}
}

func TestIntegration_scratch_request_then_save_draft_to_folder(t *testing.T) {
	b := openExample(t)
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	var opened string

	ws.NewScratchRequest(func(id string) { opened = id }, func(err error) { t.Error(err) })
	w.Wait()

	n := ws.Node(opened)
	if n.Kind != vm.NodeRequest || !ws.Node("c:"+string(n.Collection)).Draft {
		t.Fatalf("opened %q: %+v", opened, n)
	}
	m := vm.NewManager(b, w, ws)
	dir := filepath.Join(t.TempDir(), "kept")
	m.SaveDraft(n.Collection, dir)
	w.Wait()
	if m.Err() != "" || ws.Node("c:"+string(n.Collection)).Draft {
		t.Fatalf("err %q, node %+v", m.Err(), ws.Node("c:"+string(n.Collection)))
	}
	if _, err := os.Stat(filepath.Join(dir, "requests", "untitled.req.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_replayer_serves_examples_and_stops(t *testing.T) {
	b := openExample(t)
	q, w := newUIQueue()
	r := vm.NewReplayer(b, w)
	t.Cleanup(func() {
		closed := make(chan struct{})
		go func() { r.Close(); close(closed) }()
		for {
			select {
			case f := <-q.posted:
				f()
			case <-closed:
				return
			}
		}
	})

	r.Start(vm.ReplayerForm{Addr: "127.0.0.1:0"})
	q.run(1)
	if r.State() != vm.ReplayerRunning || r.Err() != "" {
		t.Fatalf("state %v err %q", r.State(), r.Err())
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+r.Addr()+"/v1/login", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	q.run(1)

	entries := r.Entries()
	if res.StatusCode != 200 || len(entries) != 1 || entries[0].Request != "auth/login" {
		t.Fatalf("status %d entries %+v", res.StatusCode, entries)
	}
	r.Stop()
	q.run(1)
	if r.State() != vm.ReplayerStopped {
		t.Fatalf("state %v", r.State())
	}
}

func TestWorkspaceVM_integration_CreateRequest_without_a_name_makes_an_untitled_request(t *testing.T) {
	b := openExample(t)
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	cid := ws.Collections()[0].Manifest.ID
	var opened string
	var failed error

	ws.CreateRequest(cid, "", func(id string) { opened = id }, func(err error) { failed = err })
	w.Wait()

	if failed != nil || opened == "" {
		t.Fatalf("opened %q err %v", opened, failed)
	}
}

func TestImportWizard_integration_imports_from_a_url(t *testing.T) {
	b := openExample(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("openapi: 3.0.3\ninfo: {title: Pets, version: 1.0.0}\npaths:\n  /pets:\n    get:\n      operationId: listPets\n      responses: {\"200\": {description: ok}}\n"))
	}))
	defer srv.Close()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	iw := vm.NewImportWizard(b, w, ws)
	iw.Load()
	w.Wait()
	iw.SetFile(srv.URL + "/api/openapi.yaml")

	iw.Preview()
	w.Wait()
	iw.Import(vm.ImportRefuseExisting, false)
	w.Wait()

	found := false
	for _, r := range b.Collections()[0].Requests {
		found = found || r.Operation == "listPets"
	}
	if iw.Err() != "" || !found {
		t.Fatalf("err %q rows %+v", iw.Err(), iw.Rows())
	}
}

func TestImportWizard_integration_applies_an_overlay_before_importing(t *testing.T) {
	b := openExample(t)
	dir := t.TempDir()
	spec := filepath.Join(dir, "things.yaml")
	overlay := filepath.Join(dir, "fixes.overlay.yaml")
	if err := os.WriteFile(spec, []byte("openapi: 3.0.3\ninfo: {title: t, version: '1'}\npaths:\n  /things:\n    get:\n      responses: {'200': {description: ok}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, []byte("overlay: 1.0.0\ninfo: {title: fixes, version: 1.0.0}\nactions:\n  - target: $.paths['/things'].get\n    update: {operationId: listThings}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	iw := vm.NewImportWizard(b, w, ws)
	iw.Load()
	w.Wait()
	iw.SetFile(spec)
	iw.SetOverlay(overlay)

	iw.Preview()
	w.Wait()
	iw.Import(vm.ImportRefuseExisting, false)
	w.Wait()

	found := false
	for _, r := range b.Collections()[0].Requests {
		found = found || r.Operation == "listThings"
	}
	if iw.Err() != "" || !found || !vm.SupportsOverlay(flowi.ImportAsyncAPI) || vm.SupportsOverlay(flowi.ImportCurl) {
		t.Fatalf("err %q found %v rows %+v", iw.Err(), found, iw.Rows())
	}
}
