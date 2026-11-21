// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchImportSource_should_return_the_body_and_the_file_name(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/openapi.json" {
			_, _ = w.Write([]byte(`{"openapi":"3.0.0"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	data, name, err := FetchImportSource(context.Background(), srv.URL+"/v1/openapi.json")
	_, _, missing := FetchImportSource(context.Background(), srv.URL+"/nope")
	_, _, notURL := FetchImportSource(context.Background(), "ftp://x/y")

	if err != nil || string(data) != `{"openapi":"3.0.0"}` || name != "openapi.json" {
		t.Fatalf("data %q name %q err %v", data, name, err)
	}
	if missing == nil || !strings.Contains(missing.Error(), "404") {
		t.Fatalf("missing: %v", missing)
	}
	if notURL == nil {
		t.Fatal("a non-http URL must be refused")
	}
}

func TestIsImportURL(t *testing.T) {
	if !IsImportURL("https://a/b.json") || !IsImportURL("http://a") || IsImportURL("./a.json") || IsImportURL("C:/a.json") {
		t.Fatal("IsImportURL")
	}
}

// Before: a login page or error page answered with 200 was imported as if it were a description, and the user got a
// parse error about HTML.
func TestFetchImportSource_should_refuse_a_web_page(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>sign in</html>"))
	}))
	defer srv.Close()

	_, _, err := FetchImportSource(context.Background(), srv.URL+"/openapi.json")

	if err == nil || !strings.Contains(err.Error(), "web page") {
		t.Fatalf("got %v", err)
	}
}

func TestFetchImportSource_should_not_follow_a_redirect_from_https_to_http(t *testing.T) {
	secure, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://a.example/spec", http.NoBody)
	plain, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://a.example/spec", http.NoBody)
	upgrade, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://b.example/spec", http.NoBody)

	down := newFetchClient().CheckRedirect(plain, []*http.Request{secure})
	up := newFetchClient().CheckRedirect(upgrade, []*http.Request{plain})
	same := newFetchClient().CheckRedirect(upgrade, []*http.Request{secure})

	if down == nil || !strings.Contains(down.Error(), "not encrypted") {
		t.Fatalf("https to http: %v", down)
	}
	if up != nil || same != nil {
		t.Fatalf("http to https: %v, https to https: %v", up, same)
	}
}
