// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

func apiKeyAuth(value string) model.Auth {
	return model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "name", Value: "X-Api-Key"}, {Key: "value", Value: value}, {Key: "in", Value: "header"}}}
}

// echoWithAuth makes a request to the demo server's /echo whose only version
// has the given auth block.
func echoWithAuth(t *testing.T, w *Workspace, auth *model.Auth) RequestTarget {
	t.Helper()
	c := w.Collections()[0]
	target, err := w.NewRequest(c.Manifest.ID, "named/echo")
	if err != nil {
		t.Fatal(err)
	}
	_, r, err := w.requestFor(target)
	if err != nil {
		t.Fatal(err)
	}
	r.Versions[0].URL = "{{base_url}}/echo"
	r.Versions[0].Auth = auth
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return target
}

// Before: a collection had no place to keep an authentication, so nothing could be created or listed.
func TestNewNamedAuth_Given_a_name_and_a_scheme_When_created_Then_the_file_is_written_and_it_is_listed(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]

	target, err := w.NewNamedAuth(c.Manifest.ID, "partner-key", "The partner API key", apiKeyAuth("{{secret.password}}"))

	if err != nil {
		t.Fatal(err)
	}
	list, err := w.NamedAuths(c.Manifest.ID)
	if err != nil || len(list) != 1 || list[0].ID != target.Auth || list[0].Name != "partner-key" || list[0].Auth.Type != model.AuthAPIKey || len(list[0].Auth.Params) != 3 {
		t.Fatalf("list %+v err %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(c.Root, "auths", "partner-key.auth.yaml")); err != nil {
		t.Fatal(err)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("the workspace has errors after creating an authentication: %v", diags)
	}
}

// Before: nothing stopped two authentications from sharing the name requests use them by.
func TestNewNamedAuth_Given_a_name_that_exists_or_has_a_folder_When_created_Then_FLW_WRITE_002(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	if _, err := w.NewNamedAuth(c.Manifest.ID, "partner-key", "", apiKeyAuth("k")); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"partner-key", "team/key", ""} {
		_, err := w.NewNamedAuth(c.Manifest.ID, name, "", apiKeyAuth("k"))

		var le *diag.ListError
		if !errors.As(err, &le) || le.List[0].Code != diag.CodeWriteRequestPath {
			t.Fatalf("name %q: got %v", name, err)
		}
	}
}

// Before: a saved authentication could not be changed without editing its file by hand.
func TestSaveNamedAuth_Given_new_parameters_When_saved_Then_they_are_kept_and_the_name_is_not_changed(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target, _ := w.NewNamedAuth(c.Manifest.ID, "partner-key", "", apiKeyAuth("old"))

	err := w.SaveNamedAuth(target, model.NamedAuth{Meta: model.Meta{Name: "renamed", Description: "now digest"},
		Auth: model.Auth{Type: model.AuthDigest, Params: []model.KeyValue{{Key: "username", Value: "ada"}}}}, SaveOptions{})

	if err != nil {
		t.Fatal(err)
	}
	a, err := w.NamedAuth(target)
	if err != nil || a.Name != "partner-key" || a.Description != "now digest" || a.Auth.Type != model.AuthDigest || a.Auth.Params[0].Value != "ada" {
		t.Fatalf("auth %+v err %v", a, err)
	}
}

// Before: deleting had nothing to report about the requests left pointing at the authentication.
func TestDeleteNamedAuth_Given_a_request_uses_it_When_deleted_Then_uses_were_reported_and_validation_names_the_request(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target, _ := w.NewNamedAuth(c.Manifest.ID, "partner-key", "", apiKeyAuth("k"))
	req := echoWithAuth(t, w, &model.Auth{Use: "partner-key"})
	uses, err := w.NamedAuthUses(target)
	if err != nil || len(uses) != 1 || uses[0].Request != req || uses[0].Path != "named/echo" || uses[0].Version != "1" {
		t.Fatalf("uses %+v err %v", uses, err)
	}

	err = w.DeleteNamedAuth(target)

	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.NamedAuth(target); err == nil {
		t.Fatal("the authentication is still there")
	}
	var found bool
	for _, d := range w.Validate() {
		found = found || d.Code == diag.CodeAuthUnknown
	}
	if !found {
		t.Fatalf("no FLW-AUTH-002 in %v", w.Validate())
	}
	_, err = w.SendRequest(context.Background(), req, SendOptions{Env: "local"})
	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeAuthUnknown {
		t.Fatalf("send: got %v", err)
	}
}

// Before: auth: {use: name} was read as an auth block with no scheme, and the request went out without credentials.
func TestSendRequest_Given_a_version_uses_a_named_auth_When_sent_Then_the_named_auth_is_applied(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	if _, err := w.NewNamedAuth(c.Manifest.ID, "partner-key", "", apiKeyAuth("{{secret.password}}")); err != nil {
		t.Fatal(err)
	}
	req := echoWithAuth(t, w, &model.Auth{Use: "partner-key"})

	s, err := w.SendRequest(context.Background(), req, SendOptions{Env: "local"})

	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)
	if res.Status != engine.StatusPassed || res.Record.Status != 200 {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
	var sent string
	for _, h := range res.Record.Request.Header {
		if h.Name == "X-Api-Key" {
			sent = h.Value
		}
	}
	if sent == "" {
		t.Fatalf("X-Api-Key was not sent: %+v", res.Record.Request.Header)
	}
}

// Before: an authentication could not be renamed; deleting and recreating it left its requests detached.
func TestRenameNamedAuth_Given_requests_use_it_When_renamed_Then_they_follow_the_new_name(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target, err := w.NewNamedAuth(c.Manifest.ID, "partner-key", "", apiKeyAuth("{{secret.password}}"))
	if err != nil {
		t.Fatal(err)
	}
	req := echoWithAuth(t, w, &model.Auth{Use: "partner-key"})

	err = w.RenameNamedAuth(target, "partner", SaveOptions{})

	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(req)
	uses, usesErr := w.NamedAuthUses(target)
	if r.Versions[0].Auth == nil || r.Versions[0].Auth.Use != "partner" || usesErr != nil || len(uses) != 1 {
		t.Fatalf("auth %+v, uses %v (%v)", r.Versions[0].Auth, uses, usesErr)
	}
	if _, err := w.NamedAuth(target); err != nil {
		t.Fatalf("the id no longer resolves: %v", err)
	}
	if err := w.RenameNamedAuth(target, "a/b", SaveOptions{}); err == nil {
		t.Fatal("a name with a folder was accepted")
	}
}
