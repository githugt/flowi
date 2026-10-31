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
