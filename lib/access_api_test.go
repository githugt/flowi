// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

// accessServer keeps notes. A note belongs to the user who made it (header X-User). leaky says whether another
// user may read it: the bug the check is for.
func accessServer(t *testing.T, leaky bool) *httptest.Server {
	t.Helper()
	owner := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		user := r.Header.Get("X-User")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/notes":
			owner = user
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"note-77"}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/notes/"):
			if user != owner && !leaky {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"not yours"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"note-77","text":"secret plans"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// accessWorkspace builds a collection with a flow: create a note, read it, and a health check that reads
// nothing from earlier steps. Identities are overlays that set X-User.
func accessWorkspace(t *testing.T, srv *httptest.Server) (*Workspace, FlowTarget) {
	t.Helper()
	w := newEnv(t).open(t)
	m, err := w.InitCollection(t.TempDir(), "access", true)
	if err != nil {
		t.Fatal(err)
	}
	reqs := map[string]model.RequestVersion{
		"create": {Label: "1", Method: "POST", URL: "{{env.base_url}}/notes", Extract: []model.Extraction{{Name: "id", From: "body", Path: "$.id"}}},
		"read":   {Label: "1", Method: "GET", URL: "{{env.base_url}}/notes/{{with.id}}"},
		"health": {Label: "1", Method: "GET", URL: "{{env.base_url}}/health"},
	}
	refs := map[string]model.Ref{}
	for name, v := range reqs {
		target, err := w.NewRequest(m.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		r := requestOf(t, w, target)
		r.Versions = []model.RequestVersion{v}
		if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
			t.Fatal(err)
		}
		refs[name] = model.Ref{ID: r.ID, Path: name}
	}
	for user, name := range map[string]string{"a": "alice", "b": "bob"} {
		ov := model.Overlay{Name: name, Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-User", Value: user}}}
		if err := w.WriteOverlay(m.ID, ov, SaveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	envFile, err := w.NewEnvironment("local")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range w.Environments() {
		if e.File == envFile {
			e.Vars = []model.EnvVar{{Name: "base_url", Value: srv.URL}}
			if err := w.SaveEnvironment(e, SaveOptions{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	target, err := w.NewFlow(m.ID, "notes")
	if err != nil {
		t.Fatal(err)
	}
	_, flow, _ := w.flowFor(target)
	flow.Nodes = []model.Node{
		{ID: "create", Type: "request", Request: &model.RequestCall{Request: refs["create"]}},
		{ID: "read", Type: "request", Request: &model.RequestCall{Request: refs["read"], With: []model.KeyValue{{Key: "id", Value: "{{nodes.create.id}}"}}}},
		{ID: "health", Type: "request", Request: &model.RequestCall{Request: refs["health"]}},
	}
	if err := w.SaveFlow(target, flow, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return w, target
}

func TestCheckAccess_Given_a_server_that_keeps_notes_private_Then_the_other_user_is_denied(t *testing.T) {
	// Arrange
	w, target := accessWorkspace(t, accessServer(t, false))

	// Act
	rep, err := w.CheckAccess(context.Background(), target, AccessOptions{Env: "local", Owner: "alice", Others: []string{"bob"}})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Steps) != 1 || rep.Steps[0].ID != "read" {
		t.Fatalf("only the step that reads what an earlier step made is tried; got %+v", rep.Steps)
	}
	att := rep.Steps[0].Attempts[0]
	if rep.Steps[0].Owner != 200 || att.Identity != "bob" || att.Status != 403 || att.Verdict != AccessDenied || rep.Exposed() {
		t.Fatalf("report %+v", rep)
	}
}

func TestCheckAccess_Given_a_server_that_leaks_notes_Then_the_other_user_reading_them_is_an_exposure(t *testing.T) {
	// Arrange
	w, target := accessWorkspace(t, accessServer(t, true))

	// Act
	rep, err := w.CheckAccess(context.Background(), target, AccessOptions{Env: "local", Owner: "alice", Others: []string{"bob"}})

	// Assert
	if err != nil || !rep.Exposed() {
		t.Fatalf("err %v report %+v", err, rep)
	}
	if got := rep.Steps[0].Attempts[0]; got.Status != 200 || got.Verdict != AccessAllowed {
		t.Fatalf("attempt %+v", got)
	}
}

func TestCheckAccess_Given_named_steps_Then_exactly_those_are_tried(t *testing.T) {
	// Arrange
	w, target := accessWorkspace(t, accessServer(t, false))

	// Act
	rep, err := w.CheckAccess(context.Background(), target, AccessOptions{Env: "local", Owner: "alice", Others: []string{"bob"}, Steps: []string{"health"}})

	// Assert: the health check is public, so another user may read it: that is "allowed", and it is the person's call.
	if err != nil || len(rep.Steps) != 1 || rep.Steps[0].ID != "health" || rep.Steps[0].Attempts[0].Verdict != AccessAllowed {
		t.Fatalf("err %v report %+v", err, rep)
	}
}

func TestCheckAccess_Given_a_flow_that_fails_as_the_owner_Then_it_says_the_owner_run_must_pass_first(t *testing.T) {
	// Arrange
	srv := accessServer(t, false)
	w, target := accessWorkspace(t, srv)
	srv.Close()

	// Act
	_, err := w.CheckAccess(context.Background(), target, AccessOptions{Env: "local", Owner: "alice", Others: []string{"bob"}})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "as alice") {
		t.Fatalf("err %v", err)
	}
}

func TestCheckAccess_Given_an_unknown_identity_Then_FLW_OVL_002(t *testing.T) {
	w, target := accessWorkspace(t, accessServer(t, false))

	_, err := w.CheckAccess(context.Background(), target, AccessOptions{Env: "local", Owner: "alice", Others: []string{"mallory"}})

	if err == nil || !strings.Contains(err.Error(), "mallory") {
		t.Fatalf("err %v", err)
	}
}
