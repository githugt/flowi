// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm/vmtest"
)

// twoCollections is the sample Shop collection (auth/login, auth/verify,
// health, the flow signup) and Billing (invoices/list).
func twoCollections(t *testing.T) (*App, *published, *vmtest.Backend) {
	t.Helper()
	billing := model.Collection{Root: "/b", Manifest: model.Manifest{Meta: model.Meta{ID: "C2", Name: "Billing"}, File: "/b/flowi.yaml"},
		Requests: []model.Request{{Meta: model.Meta{ID: "R7", Name: "List"}, Path: "invoices/list", File: "/b/requests/invoices/list.req.yaml",
			Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "https://b/invoices"}}}}}
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection(), billing}}
	pub := &published{}
	app := NewApp(b, NewInlineActor(), pub.emit)
	app.Start(t.Context())
	return app, pub, b
}

var errFake = errors.New("disk full")

type published struct{ last map[string]any }

func (p *published) emit(name string, data any) {
	if p.last == nil {
		p.last = map[string]any{}
	}
	p.last[name] = data
}

// rowOf finds a tree row by kind and label.
func rowOf(t *testing.T, app *App, kind, label string) string {
	t.Helper()
	var walk func([]TreeNode) string
	walk = func(nodes []TreeNode) string {
		for _, n := range nodes {
			if n.Kind == kind && n.Label == label {
				return n.ID
			}
			if id := walk(n.Children); id != "" {
				return id
			}
		}
		return ""
	}
	id := walk(app.Workspace().Tree().Roots)
	if id == "" {
		t.Fatalf("no %s row %q", kind, label)
	}
	return id
}

func TestRenamedPath_should_keep_the_folder(t *testing.T) {
	tests := []struct{ name, old, given, want string }{
		{name: "a request in a folder", old: "auth/login", given: "sign-in", want: "auth/sign-in"},
		{name: "a request at the top", old: "health", given: "ping", want: "ping"},
		{name: "a path with a slash goes where it says", old: "auth/login", given: "users/login", want: "users/login"},
		{name: "a folder", old: "auth/tokens", given: "keys", want: "auth/keys"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renamedPath(tt.old, tt.given)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorkspaceService_Menu_should_offer_Move_to_for_requests_only(t *testing.T) {
	app, _, _ := twoCollections(t)
	login, verify, flow := rowOf(t, app, "request", "Login"), rowOf(t, app, "request", "Verify"), rowOf(t, app, "flow", "signup")
	tests := []struct {
		name  string
		ids   []string
		want  []string // the actions, in order
		other bool     // Move to… must not be there
	}{
		{name: "one request", ids: []string{login}, want: []string{"open", "duplicate", "rename", "move", "delete"}},
		{name: "two requests", ids: []string{login, verify}, want: []string{"flowFromRequests", "move", "delete"}},
		{name: "a request and a flow", ids: []string{login, flow}, other: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.Workspace().Menu(tt.ids)

			var actions []string
			for _, e := range got {
				actions = append(actions, e.Action)
				if e.Action == "move" && (e.Dialog != "move" || e.Label != "Move to…") {
					t.Fatalf("move entry %+v", e)
				}
			}
			if tt.other {
				for _, a := range actions {
					if a == "move" {
						t.Fatalf("actions %v offer move", actions)
					}
				}
				return
			}
			if len(actions) != len(tt.want) {
				t.Fatalf("actions %v, want %v", actions, tt.want)
			}
			for i := range actions {
				if actions[i] != tt.want[i] {
					t.Fatalf("actions %v, want %v", actions, tt.want)
				}
			}
		})
	}
}

func TestWorkspaceService_MoveTargets_should_list_collections_and_their_request_folders(t *testing.T) {
	app, _, _ := twoCollections(t)

	got := app.Workspace().MoveTargets()

	var labels []string
	for _, m := range got {
		labels = append(labels, m.Label)
	}
	want := []string{"Shop", "Shop / auth", "Billing", "Billing / invoices"}
	if len(labels) != len(want) {
		t.Fatalf("targets %v, want %v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("targets %v, want %v", labels, want)
		}
	}
	if got[1].Depth != 1 || got[0].Depth != 0 {
		t.Fatalf("depths %+v", got)
	}
}

func TestWorkspaceService_MoveInto_should_move_each_request_keeping_its_name(t *testing.T) {
	tests := []struct {
		name      string
		requests  []string // labels
		into      [2]string
		wantMoves []string
		wantError bool
	}{
		{name: "to another collection's folder", requests: []string{"Login"}, into: [2]string{"folder", "invoices"}, wantMoves: []string{"C2/invoices/login"}},
		{name: "two to another collection's top", requests: []string{"Login", "Health"}, into: [2]string{"collection", "Billing"}, wantMoves: []string{"C2/login", "C2/health"}},
		{name: "to where it already is", requests: []string{"Login"}, into: [2]string{"folder", "auth"}},
		{name: "onto a row that holds no requests", requests: []string{"Login"}, into: [2]string{"flow", "signup"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, pub, b := twoCollections(t)
			var ids []string
			for _, l := range tt.requests {
				ids = append(ids, rowOf(t, app, "request", l))
			}
			into := rowOf(t, app, tt.into[0], tt.into[1])

			app.Workspace().MoveInto(ids, into)

			var got []string
			for _, m := range b.Moves {
				got = append(got, m[1])
			}
			if len(got) != len(tt.wantMoves) {
				t.Fatalf("moves %v, want %v", got, tt.wantMoves)
			}
			for i := range got {
				if got[i] != tt.wantMoves[i] {
					t.Fatalf("moves %v, want %v", got, tt.wantMoves)
				}
			}
			n, noticed := pub.last["notice"].(Notice)
			if noticed != tt.wantError || (noticed && n.Level != "error") {
				t.Fatalf("notice %+v, want an error %v", pub.last["notice"], tt.wantError)
			}
		})
	}
}

func TestRequestService_Keep_should_store_open_edits_before_a_move(t *testing.T) {
	tests := []struct {
		name      string
		edit      bool
		failWrite bool
		want      bool
		wantStore bool
	}{
		{name: "an open request with an edit", edit: true, want: true, wantStore: true},
		{name: "an open request without one", want: true},
		{name: "an edit that cannot be written", edit: true, failWrite: true, want: false, wantStore: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, b := twoCollections(t)
			node := rowOf(t, app, "request", "Login")
			if _, err := app.Requests().Open(node); err != nil {
				t.Fatal(err)
			}
			if tt.edit {
				app.Requests().SetURL(node, "https://elsewhere/login")
			}
			if tt.failWrite {
				b.EditErr = errFake
			}

			got := app.Requests().Keep([]string{node, "not-open"})

			if got != tt.want || b.Called("StoreEdit") != tt.wantStore {
				t.Fatalf("kept %v (stored %v), want %v (stored %v)", got, b.Called("StoreEdit"), tt.want, tt.wantStore)
			}
		})
	}
}
