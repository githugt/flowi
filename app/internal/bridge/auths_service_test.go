// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func authsFixture(t *testing.T) (*bridge.App, *vmtest.Backend, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Auths = []model.NamedAuth{{Meta: model.Meta{ID: "A1", Name: "partner-key", Description: "The partner key"},
		Auth: model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "name", Value: "X-Api-Key"}, {Key: "value", Value: "{{secret.partner}}"}}}}}
	c.Requests[2].Versions[0].Auth = &model.Auth{Use: "partner-key"}
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	return app, b, pub
}

// Before: the app had no screen for named authentications, so nothing listed them.
func TestAuthsService_State_Given_a_collection_with_a_named_auth_When_read_Then_it_is_listed_with_its_fields_and_uses(t *testing.T) {
	app, _, _ := authsFixture(t)

	v := app.Auths().State()

	if len(v.Auths) != 1 || v.Auths[0].Name != "partner-key" || v.Auths[0].Scheme != "scheme:apikey" || v.Auths[0].SchemeLabel != "API key" ||
		!slices.Equal(v.Auths[0].Uses, []string{"health"}) {
		t.Fatalf("auths %+v", v.Auths)
	}
	value := v.Auths[0].Fields[1]
	if value.Name != "value" || value.Value != "{{secret.partner}}" || !value.Secret || len(v.Forms["scheme:digest"]) != 2 || len(v.Collections) != 1 {
		t.Fatalf("fields %+v forms %+v", v.Auths[0].Fields, v.Forms["scheme:digest"])
	}
}

// Before: an authentication could only be made by writing its file by hand.
func TestAuthsService_Create_Given_a_name_and_a_scheme_When_created_Then_the_backend_is_asked_and_the_screen_shows_it(t *testing.T) {
	app, b, pub := authsFixture(t)

	app.Auths().Create("C1", " admin ", "scheme:digest")

	if !slices.Contains(b.Calls, "NewNamedAuth admin digest") {
		t.Fatalf("calls %v", b.Calls)
	}
	v, ok := pub.last["auths"].(bridge.AuthsView)
	if !ok || len(v.Auths) != 2 || v.Auths[1].Name != "admin" || v.Auths[1].Scheme != "scheme:digest" {
		t.Fatalf("published %+v", pub.last["auths"])
	}
}

// Before: n/a (a new guard): a nameless authentication cannot be used by any request.
func TestAuthsService_Create_Given_no_name_When_created_Then_an_error_notice_and_nothing_is_written(t *testing.T) {
	app, b, pub := authsFixture(t)

	app.Auths().Create("C1", "  ", "scheme:digest")

	n, ok := pub.last["notice"].(bridge.Notice)
	if !ok || n.Level != "error" || slices.ContainsFunc(b.Calls, func(c string) bool { return c == "NewNamedAuth  digest" }) {
		t.Fatalf("notice %+v calls %v", pub.last["notice"], b.Calls)
	}
}

// Before: changing a stored authentication meant editing the file.
func TestAuthsService_Save_Given_a_new_scheme_and_values_When_saved_Then_the_parameters_are_written_in_form_order(t *testing.T) {
	app, _, _ := authsFixture(t)
	svc := app.Auths()

	svc.Save("C1/A1", "Now digest", "scheme:digest", map[string]string{"password": "{{secret.pw}}", "username": "ada", "realm": ""})

	a := svc.State().Auths[0]
	if a.Description != "Now digest" || a.Scheme != "scheme:digest" || len(a.Fields) != 2 || a.Fields[0].Value != "ada" || a.Fields[1].Value != "{{secret.pw}}" {
		t.Fatalf("auth %+v", a)
	}
}

// Before: n/a (new): deleting removes it from the screen.
func TestAuthsService_Delete_Given_a_named_auth_When_deleted_Then_it_is_gone_and_the_request_says_its_auth_is_missing(t *testing.T) {
	app, b, _ := authsFixture(t)

	app.Auths().Delete("C1/A1")

	if !slices.Contains(b.Calls, "DeleteNamedAuth A1") || len(app.Auths().State().Auths) != 0 {
		t.Fatalf("calls %v auths %+v", b.Calls, app.Auths().State().Auths)
	}
}

// Before: an authentication could only be renamed by deleting it and making it again.
func TestAuthsService_Rename_Given_a_new_name_When_renamed_Then_the_backend_is_asked_and_the_screen_shows_it(t *testing.T) {
	app, b, pub := authsFixture(t)

	app.Auths().Rename("C1/A1", " partner ")

	if !slices.Contains(b.Calls, "RenameNamedAuth A1 partner") {
		t.Fatalf("calls %v", b.Calls)
	}
	v, ok := pub.last["auths"].(bridge.AuthsView)
	if !ok || len(v.Auths) != 1 || v.Auths[0].Name != "partner" {
		t.Fatalf("published %+v", pub.last["auths"])
	}
}
