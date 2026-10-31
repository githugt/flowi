// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/store"
)

func descriptionFile(w *Workspace) string {
	return filepath.Join(w.Collections()[0].Root, filepath.FromSlash(store.GeneratedDescription))
}

func readDescription(t *testing.T, w *Workspace) string {
	t.Helper()
	data, err := os.ReadFile(descriptionFile(w))
	if err != nil {
		t.Fatalf("no generated description: %v", err)
	}
	return string(data)
}

func TestSaveRequest_should_write_the_generated_description_with_its_operation(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target, err := w.NewRequest(c.Manifest.ID, "pets/list")
	if err != nil {
		t.Fatal(err)
	}

	text := readDescription(t, w)

	if !strings.Contains(text, "operationId: pets_list") || !strings.Contains(text, "x-flowi-request: "+string(target.Request)) {
		t.Fatalf("description lacks the new request:\n%s", text)
	}
}

func TestMoveRequest_should_rename_the_operation_but_not_another_requests_id(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	login, _ := w.FindRequest("auth/login", "")
	before := readDescription(t, w)

	_, err := w.MoveRequest(login, c.Manifest.ID, "auth/signin")

	after := readDescription(t, w)
	if err != nil || !strings.Contains(after, "operationId: auth_signin") || strings.Contains(after, "operationId: auth_login\n") || !strings.Contains(before, "operationId: auth_verify") || !strings.Contains(after, "operationId: auth_verify") {
		t.Fatalf("err %v\nbefore:\n%s\nafter:\n%s", err, before, after)
	}
}

func TestLearnSchemas_should_add_the_response_schema_to_the_description(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")

	if _, err := w.LearnSchemas(target, "1", recordWith(200, nil, map[string]any{"token": "t"})); err != nil {
		t.Fatal(err)
	}

	if text := readDescription(t, w); !strings.Contains(text, "token:") || !strings.Contains(text, `"200":`) {
		t.Fatalf("description lacks the learned schema:\n%s", text)
	}
}

func TestRegenerateDescription_Given_nothing_changed_Then_the_file_is_not_touched(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	if err := w.RegenerateDescription(c.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(descriptionFile(w), old, old); err != nil {
		t.Fatal(err)
	}

	err := w.RegenerateDescription(c.Manifest.ID)

	info, _ := os.Stat(descriptionFile(w))
	if err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("err %v mtime %v, want %v", err, info.ModTime(), old)
	}
}

func TestRegenerateDescription_Given_no_requests_and_no_flows_Then_no_file(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	dir := t.TempDir()
	if _, err := w.InitCollection(dir, "Empty", true); err != nil {
		t.Fatal(err)
	}
	c := w.Collections()[0]

	err := w.RegenerateDescription(c.Manifest.ID)

	if _, statErr := os.Stat(filepath.Join(c.Root, filepath.FromSlash(store.GeneratedDescription))); err != nil || !os.IsNotExist(statErr) {
		t.Fatalf("err %v stat %v", err, statErr)
	}
}

func asyncFile(w *Workspace) string {
	return filepath.Join(w.Collections()[0].Root, filepath.FromSlash(store.GeneratedAsyncDescription))
}

func saveAsync(t *testing.T, w *Workspace, path string, a *model.Async) RequestTarget {
	t.Helper()
	c := w.Collections()[0]
	target, err := w.NewRequest(c.Manifest.ID, path)
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	r.Versions = []model.RequestVersion{{Label: "1", Async: a}}
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestSaveRequest_should_write_the_asyncapi_description_for_async_requests_only(t *testing.T) {
	w := demoWorkspace(t)
	if _, err := os.Stat(asyncFile(w)); !os.IsNotExist(err) {
		t.Fatalf("no async request yet, but %v", err)
	}

	target := saveAsync(t, w, "orders/placed", &model.Async{Protocol: "mqtt", Server: "mqtt://b:1883", Channel: "orders/placed", Send: &model.AsyncSend{}})

	data, err := os.ReadFile(asyncFile(w))
	if err != nil || !strings.Contains(string(data), "orders_placed") || !strings.Contains(string(data), "x-flowi-request: "+string(target.Request)) {
		t.Fatalf("err %v\n%s", err, data)
	}
	if strings.Contains(readDescription(t, w), "orders_placed") {
		t.Fatal("the async operation leaked into the OpenAPI description")
	}
}

func TestSaveRequest_should_remove_the_asyncapi_description_when_no_async_request_is_left(t *testing.T) {
	w := demoWorkspace(t)
	target := saveAsync(t, w, "orders/placed", &model.Async{Protocol: "ws", Server: "ws://h", Channel: "c", Send: &model.AsyncSend{}})
	_, r, _ := w.requestFor(target)
	r.Versions = []model.RequestVersion{{Label: "1", Method: "GET", URL: "http://x"}}

	err := w.SaveRequest(target, r, SaveOptions{})

	if _, statErr := os.Stat(asyncFile(w)); err != nil || !os.IsNotExist(statErr) {
		t.Fatalf("err %v stat %v", err, statErr)
	}
}
