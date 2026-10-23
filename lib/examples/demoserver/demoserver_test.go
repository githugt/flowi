// SPDX-License-Identifier: Apache-2.0

package demoserver_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/examples/demoserver"
	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/store"
	"github.com/githugt/flowi-lib/vault"
)

func server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(demoserver.Handler(demoserver.Options{Password: "demo"}))
	t.Cleanup(srv.Close)
	return srv
}

// post sends JSON and returns the status and the body (already closed).
func post(t *testing.T, url, body string, header map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data)
}

func TestHandler_login_then_verify_with_token(t *testing.T) {
	srv := server(t)

	login, body := post(t, srv.URL+"/v1/login", `{"email":"a@example.com","password":"demo"}`, nil)
	token := between(body, `"token":"`, `"`)
	sub := between(body, `"sub":"`, `"`)
	verify, _ := post(t, srv.URL+"/v1/verify", `{"subject":"`+sub+`","code":"123456"}`, map[string]string{"Authorization": "Bearer " + token, "X-Client-Id": "demo-client"})
	bad, _ := post(t, srv.URL+"/v1/verify", `{"subject":"`+sub+`","code":"123456"}`, map[string]string{"Authorization": "Bearer forged"})

	if login != 200 || token == "" || verify != 204 || bad != 401 {
		t.Fatalf("login %d %s, verify %d, forged %d", login, body, verify, bad)
	}
}

func between(s, start, end string) string {
	_, rest, ok := strings.Cut(s, start)
	if !ok {
		return ""
	}
	v, _, _ := strings.Cut(rest, end)
	return v
}

func TestHandler_login_Given_wrong_password_Then_401(t *testing.T) {
	status, _ := post(t, server(t).URL+"/v2/login", `{"email":"a@example.com","password":"nope"}`, nil)

	if status != http.StatusUnauthorized {
		t.Fatalf("status %d", status)
	}
}

func TestHandler_redirect_n_should_redirect_n_times(t *testing.T) {
	srv := server(t)
	var hops atomic.Int32
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { hops.Add(1); return nil }}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/redirect/3", nil)
	resp, err := client.Do(req)

	if err != nil || resp.StatusCode != 200 || hops.Load() != 3 {
		t.Fatalf("err %v status %v hops %d", err, resp, hops.Load())
	}
	_ = resp.Body.Close()
}

func TestHandler_helper_routes(t *testing.T) {
	srv := server(t)
	tests := []struct {
		path   string
		status int
		size   int
	}{
		{"/status/418", 418, -1},
		{"/bytes/1000", 200, 1000},
		{"/gzip", 200, -1},
		{"/cookie/set", 200, -1},
		{"/echo", 200, -1},
	}
	for _, tt := range tests {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+tt.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode != tt.status || (tt.size >= 0 && len(data) != tt.size) {
			t.Errorf("%s: status %d, %d bytes", tt.path, resp.StatusCode, len(data))
		}
	}
}

// exampleLookup serves the loaded example collection to the engine.
type exampleLookup struct{ c model.Collection }

func (l exampleLookup) Collection(model.ID) (model.Collection, bool) { return l.c, true }

func (l exampleLookup) Request(ref model.Ref, _ model.ID) (model.Collection, model.Request, bool) {
	for _, r := range l.c.Requests {
		if r.ID == ref.ID {
			return l.c, r, true
		}
	}
	return model.Collection{}, model.Request{}, false
}

func (l exampleLookup) Flow(ref model.Ref, _ model.ID) (model.Collection, model.Flow, bool) {
	for _, f := range l.c.Flows {
		if f.ID == ref.ID {
			return l.c, f, true
		}
	}
	return model.Collection{}, model.Flow{}, false
}

func (l exampleLookup) Settings(_ model.ID, folder string, version model.Settings) settings.Resolved {
	schema := settings.Schema()
	layers := []settings.LayerValues{}
	add := func(layer settings.Layer, s model.Settings) {
		lv, _ := settings.FromBlock(schema, layer, "", s)
		layers = append(layers, lv)
	}
	add(settings.LayerCollection, l.c.Manifest.Settings)
	for _, f := range l.c.Folders {
		if folder == f.Dir || strings.HasPrefix(folder, f.Dir+"/") {
			add(settings.LayerFolder, f.Settings)
		}
	}
	add(settings.LayerRequest, version)
	return settings.Resolve(schema, layers...)
}

// memKeys is an in-memory keychain.
type memKeys struct{ keys map[string]string }

func (k *memKeys) Get(s, a string) (string, error) {
	if v, ok := k.keys[s+a]; ok {
		return v, nil
	}
	return "", vault.ErrKeyNotFound
}

func (k *memKeys) Set(s, a, v string) error { k.keys[s+a] = v; return nil }

func TestHandler_should_serve_example_collection_flow_requests(t *testing.T) {
	srv := server(t)
	c, diags, err := store.New(yamlcodec.New(settings.Schema())).Load("../user-creation")
	if err != nil || diags.HasErrors() {
		t.Fatalf("load: %v %v", err, diags)
	}
	for i, e := range c.Environments {
		if e.Name == "local" {
			c.Environments[i].Vars[0].Value = srv.URL // base_url
		}
	}
	v := vault.New(vault.Options{File: filepath.Join(t.TempDir(), "vault.enc"), WorkspaceID: "01JA00000000000000000000W1", Keys: &memKeys{keys: map[string]string{}}})
	if err := v.Set(vault.Entry{Collection: c.Manifest.ID, Environment: "01JA0000000000000000000002", Name: "password"}, "demo"); err != nil {
		t.Fatal(err)
	}
	var seq atomic.Int64
	deps := engine.Deps{
		Lookup: exampleLookup{c}, Vault: v, Clock: time.Now,
		DataDir: func(model.ID) string { return t.TempDir() },
		NewID:   func() string { return fmt.Sprintf("01JD%022d", seq.Add(1)) },
	}
	send := func(path string, with map[string]string) engine.SendResult {
		t.Helper()
		for _, r := range c.Requests {
			if r.Path == path {
				s := engine.StartSend(context.Background(), deps, engine.SendInput{Collection: c, Request: r, Env: "local", With: with, PreferVersion: "2"})
				for range s.Events() {
				}
				res := s.Wait()
				if res.Status != engine.StatusPassed {
					t.Fatalf("%s: %s %v", path, res.Status, res.Diags)
				}
				return res
			}
		}
		t.Fatalf("no request %s", path)
		return engine.SendResult{}
	}

	login := send("auth/login", map[string]string{"email": "a@example.com"})
	token, sub := fmt.Sprint(login.Record.Outputs["token"]), fmt.Sprint(login.Record.Outputs["subject"])
	auth := map[string]string{"token": token, "subject": sub}
	send("auth/verify", map[string]string{"token": token, "subject": sub, "code": "123456"})
	send("onboarding/form-page-1", merge(auth, map[string]string{"first_name": "Ada", "last_name": "Lovelace"}))
	send("onboarding/form-page-2", merge(auth, map[string]string{"city": "London"}))

	if login.Record.Version != "2" || token == "" {
		t.Fatalf("login %+v", login.Record)
	}
}

func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func TestDemoServer_status_should_turn_ready_after_two_polls(t *testing.T) {
	srv := server(t)
	_, body := post(t, srv.URL+"/v2/login", `{"email": "a@b.c", "password": "demo"}`, nil)
	token := between(body, `"token":"`, `"`)
	sub := between(body, `"id":"`, `"`)
	poll := func(subject string) (int, string) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/v1/onboarding/"+subject+"/status", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(data)
	}

	var states []string
	for range 3 {
		_, b := poll(sub)
		states = append(states, between(b, `"state":"`, `"`))
	}
	other, _ := poll("u-someone-else")

	if fmt.Sprint(states) != "[pending pending ready]" || other != http.StatusBadRequest {
		t.Fatalf("states %v, other subject %d", states, other)
	}
}
