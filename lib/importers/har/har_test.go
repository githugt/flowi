// SPDX-License-Identifier: Apache-2.0

package har

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/importers/importerstest"
)

func TestImport_corpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.har"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus: %v", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			r, fatal := Import(src, filepath.Base(file))

			importerstest.Golden(t, strings.TrimSuffix(filepath.Base(file), ".har")+".golden", importerstest.Render(t, r, fatal))
		})
	}
}

func TestImport_should_never_write_credentials(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "browser.har"))
	if err != nil {
		t.Fatal(err)
	}

	r, _ := Import(src, "browser.har")
	out := importerstest.Render(t, r, nil)

	for _, secret := range []string{"eyJ.secret.sig", "sid=abc", `"pw"`} {
		if strings.Contains(out, secret) {
			t.Errorf("%q reached the result:\n%s", secret, out)
		}
	}
}

func TestTemplatePath_should_name_id_segments(t *testing.T) {
	cases := map[string]string{
		"/v1/users/42":          "/v1/users/{{id}}",
		"/v1/users/42/orders/9": "/v1/users/{{id}}/orders/{{id_2}}",
		"/v1/items/3f2b8c1e-9d4a-4b7e-8f6a-1c2d3e4f5a6b": "/v1/items/{{id}}",
		"/v1/blobs/9f86d081884c7d659a2feaa0c55ad015":     "/v1/blobs/{{id}}",
		"/v1/users/me":   "/v1/users/me",
		"/v2/files/2026": "/v2/files/{{id}}",
	}
	for in, want := range cases {
		if got, _ := templatePath(in); got != want {
			t.Errorf("templatePath(%q) = %q, want %q", in, got, want)
		}
	}
}

// Before: camelCase keys, text/plain JSON bodies, urlencoded text without
// params, multipart password fields and query secrets were kept.
func TestImport_should_redact_every_recorded_credential(t *testing.T) {
	src := `{"log": {"entries": [
  {"_resourceType": "fetch", "request": {"method": "POST", "url": "https://u:P0@api.example.com/v1/a?access_token=Q1&page=1",
    "headers": [{"name": "X-AccessToken", "value": "H1"}], "queryString": [{"name": "access_token", "value": "Q1"}, {"name": "page", "value": "1"}],
    "postData": {"mimeType": "text/plain;charset=UTF-8", "text": "{\"password\":\"B1\"}"}},
   "response": {"status": 200, "headers": [], "content": {"mimeType": "application/json", "text": "{\"accessToken\":\"R1\",\"refreshToken\":\"R2\",\"jwt\":\"R3\",\"user\":{\"sessionId\":\"R4\"}}"}}},
  {"_resourceType": "fetch", "request": {"method": "POST", "url": "https://api.example.com/v1/b", "headers": [], "queryString": [],
    "postData": {"mimeType": "application/x-www-form-urlencoded", "text": "user=ada&password=B2"}},
   "response": {"status": 204, "headers": [], "content": {"mimeType": "x-unknown"}}},
  {"_resourceType": "fetch", "request": {"method": "POST", "url": "https://api.example.com/v1/c", "headers": [], "queryString": [],
    "postData": {"mimeType": "multipart/form-data; boundary=x", "params": [{"name": "password", "value": "B3"}]}},
   "response": {"status": 204, "headers": [], "content": {"mimeType": "x-unknown"}}}
]}}`

	r, _ := Import([]byte(src), "r.har")
	out := importerstest.Render(t, r, nil)

	for _, secret := range []string{"P0", "Q1", "H1", "B1", "B2", "B3", "R1", "R2", "R3", "R4"} {
		if strings.Contains(out, `"`+secret+`"`) || strings.Contains(out, "="+secret) || strings.Contains(out, ": "+secret) {
			t.Errorf("%q reached the result:\n%s", secret, out)
		}
	}
}

// Before: 20k polls of one endpoint took 45s. It takes about 1.5s with the race detector; the
// limit is wide enough for a busy machine (5s failed during a full parallel run) and still far
// under the old time.
func TestImport_should_stay_fast_on_long_recordings(t *testing.T) {
	entry := `{"_resourceType": "xhr", "request": {"method": "GET", "url": "https://api.example.com/v1/poll", "headers": [], "queryString": []}, "response": {"status": 204, "headers": [], "content": {"mimeType": "x-unknown"}}}`
	src := `{"log": {"entries": [` + strings.Repeat(entry+",", 19999) + entry + `]}}`
	done := make(chan struct{})

	go func() {
		defer close(done)
		Import([]byte(src), "long.har")
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("a long recording did not import in 20s")
	}
}
