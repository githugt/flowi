// SPDX-License-Identifier: Apache-2.0

package postman

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/importers/importerstest"
)

func TestImport_corpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.json"))
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

			importerstest.Golden(t, strings.TrimSuffix(filepath.Base(file), ".json")+".golden", importerstest.Render(t, r, fatal))
		})
	}
}

func TestImport_should_never_write_credentials(t *testing.T) {
	for _, name := range []string{"shop.postman_collection.json", "staging.postman_environment.json"} {
		src, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}

		r, _ := Import(src, name)
		out := importerstest.Render(t, r, nil)

		for _, secret := range []string{"sk_live_123", "eyJ.live.token", "ak_live_999", "hunter2", "pa55"} {
			if strings.Contains(out, secret) {
				t.Errorf("%s: %q reached the result", name, secret)
			}
		}
	}
}

// Before: URL userinfo, query secrets, camelCase variables and literal
// passwords in raw bodies were written.
func TestImport_should_redact_urls_queries_variables_and_bodies(t *testing.T) {
	src := `{"info": {"name": "C", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "variable": [{"key": "accessToken", "value": "V1"}, {"key": "clientSecret", "value": "V2"}],
  "item": [{"name": "A", "request": {"method": "POST",
    "url": {"raw": "https://u:P1@api.example.com/v1/a?api_key=Q1", "query": [{"key": "api_key", "value": "Q1"}]},
    "body": {"mode": "raw", "raw": "{\"password\": \"B1\", \"email\": \"{{email}}\"}", "options": {"raw": {"language": "json"}}}}}]}`

	r, _ := Import([]byte(src), "c.json")
	out := importerstest.Render(t, r, nil)

	for _, secret := range []string{"P1", "Q1", "V1", "V2", "B1"} {
		if strings.Contains(out, secret) {
			t.Errorf("%q reached the result:\n%s", secret, out)
		}
	}
	if !strings.Contains(out, "{{env.email}}") {
		t.Fatalf("variables stay:\n%s", out)
	}
}
