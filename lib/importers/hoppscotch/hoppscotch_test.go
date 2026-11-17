// SPDX-License-Identifier: Apache-2.0

package hoppscotch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers/importerstest"
)

func TestImport_corpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "api-*.json"))
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

			name := strings.TrimSuffix(filepath.Base(file), ".json")
			importerstest.Golden(t, name+".golden", importerstest.Render(t, r, fatal))
		})
	}
}

func TestImport_should_never_write_credentials(t *testing.T) {
	for _, file := range []string{"api-collection.json", "api-environment.json"} {
		src, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}

		r, _ := Import(src, file)
		out := importerstest.Render(t, r, nil)

		for _, secret := range []string{"literal-key", "hunter2", `"pw"`, "shh", "s3", "tok-12345"} {
			if strings.Contains(out, secret) {
				t.Errorf("%s: %q reached the result:\n%s", file, secret, out)
			}
		}
	}
}

func TestImport_should_refuse_what_is_not_an_export(t *testing.T) {
	cases := map[string]string{"object": `{"hello": "world"}`, "list": `[1, 2]`, "broken": `{`}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, fatal := Import([]byte(src), "x.json")

			if len(fatal) != 1 || (fatal[0].Code != diag.CodeImportInvalid && fatal[0].Code != diag.CodeImportSyntax) {
				t.Errorf("fatal %v", fatal)
			}
		})
	}
}

func TestImport_Given_a_scheme_auth_set_on_a_collection_and_a_folder_Then_requests_use_named_authentications(t *testing.T) {
	src := `{"name": "Shop", "auth": {"authType": "digest", "authActive": true, "username": "ada", "password": "pw"},
	  "requests": [
	    {"name": "A", "method": "GET", "endpoint": "https://example.com/a", "auth": {"authType": "inherit"}},
	    {"name": "Own", "method": "GET", "endpoint": "https://example.com/o", "auth": {"authType": "digest", "authActive": true, "username": "bob", "password": "x"}}],
	  "folders": [{"name": "Admin", "auth": {"authType": "digest", "authActive": true, "username": "root", "password": "y"},
	    "requests": [{"name": "C", "method": "GET", "endpoint": "https://example.com/c", "auth": {"authType": "inherit"}}]}]}`

	res, _ := Import([]byte(src), "shop.json")

	use := map[string]string{}
	for _, r := range res.Requests {
		if a := r.Versions[0].Auth; a != nil {
			use[r.Path] = a.Use + "/" + a.Type
		}
	}
	if use["A"] != "Shop-auth/" || use["Admin/C"] != "Admin-auth/" || use["Own"] != "/digest" || len(res.Auths) != 2 {
		t.Fatalf("got %v auths %+v", use, res.Auths)
	}
}
