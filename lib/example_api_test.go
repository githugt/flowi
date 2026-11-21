// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/workspace"
)

func TestOpenExample_should_open_a_collection_that_logs_in_against_its_own_demo_server(t *testing.T) {
	// Before: the example collection and the demo API were embedded but only tests used them, so a new user saw an empty app.
	w, e := tokenWorkspace(t)
	t.Cleanup(w.CloseExample)

	ex, err := w.OpenExample()
	if err != nil {
		t.Fatal(err)
	}
	target, err := w.FindRequest("auth/login", string(ex.Collection))
	if err != nil {
		t.Fatal(err)
	}
	s, err := w.SendRequest(t.Context(), target, SendOptions{Env: ex.Env, With: map[string]string{"email": "ada@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	res := s.Wait()

	if res.Status != engine.StatusPassed || res.Record == nil || res.Record.Status != http.StatusOK {
		t.Fatalf("status %s diags %v record %+v", res.Status, res.Diags, res.Record)
	}
	if !strings.HasPrefix(ex.ServerURL, "http://127.0.0.1:") {
		t.Fatalf("the demo server must listen on the loopback address: %s", ex.ServerURL)
	}
	if _, err := os.Stat(filepath.Join(e.home, "config", "workspaces", workspace.DefaultName, "vault.enc")); err == nil {
		t.Fatal("opening the example made a vault: the demo password must not need one")
	}
}

func TestOpenExample_Given_it_is_open_Then_a_second_call_returns_the_same_and_close_removes_it(t *testing.T) {
	w, _ := tokenWorkspace(t)
	t.Cleanup(w.CloseExample)
	first, err := w.OpenExample()
	if err != nil {
		t.Fatal(err)
	}

	again, err := w.OpenExample()
	count := len(w.Collections())
	dir := w.example.dir
	w.CloseExample()

	if err != nil || again != first || count != 1 {
		t.Fatalf("again %+v (%v), first %+v, collections %d", again, err, first, count)
	}
	if len(w.Collections()) != 0 {
		t.Fatalf("collections left: %d", len(w.Collections()))
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("the example's folder is still there")
	}
	if resp, err := http.Get(first.ServerURL + "/v1/config"); err == nil { //nolint:noctx // proving the server is gone
		resp.Body.Close()
		t.Fatal("the demo server still answers")
	}
}

func TestOpenExample_should_point_the_demo_environment_at_the_new_port_each_time_and_not_mount_anything(t *testing.T) {
	w, e := tokenWorkspace(t)
	first, err := w.OpenExample()
	if err != nil {
		t.Fatal(err)
	}
	w.CloseExample()
	second, err := w.OpenExample()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.CloseExample)

	var base string
	for _, env := range w.Environments() {
		if env.Name == ExampleEnvironment {
			for _, v := range env.Vars {
				if v.Name == "base_url" {
					base = v.Value
				}
			}
		}
	}
	reopened := e.open(t)

	if base != second.ServerURL || first.ServerURL == "" {
		t.Fatalf("env base_url %q, second server %q", base, second.ServerURL)
	}
	if len(reopened.Collections()) != 0 {
		t.Fatalf("the example was mounted into the workspace file: %d collections", len(reopened.Collections()))
	}
}
