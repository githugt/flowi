// SPDX-License-Identifier: Apache-2.0

package bruno

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/importers/importerstest"
)

func TestParse_should_read_dictionary_text_and_list_blocks(t *testing.T) {
	src := "meta {\n  name: Get user\n  ~seq: 1\n}\n\nbody:json {\n  {\n    \"a\": {}\n  }\n}\n\nvars:secret [\n  password,\n  token\n]\n"

	blocks, err := parse([]byte(src))

	if err != nil || len(blocks) != 3 {
		t.Fatalf("blocks %+v err %v", blocks, err)
	}
	meta := blocks[0]
	if meta.name != "meta" || len(meta.entries) != 2 || meta.entries[0].value != "Get user" || !meta.entries[1].disabled || meta.entries[1].line != 3 {
		t.Fatalf("meta %+v", meta)
	}
	if blocks[1].text != "{\n  \"a\": {}\n}" {
		t.Fatalf("body %q", blocks[1].text)
	}
	if got := blocks[2].list; len(got) != 2 || got[0] != "password" || got[1] != "token" {
		t.Fatalf("list %v", got)
	}
}

func TestParse_should_point_at_the_problem(t *testing.T) {
	cases := map[string]int{
		"meta {\n  name: x\n":         1, // never closed: the block's line
		"stray text\n":                1,
		"get {\n  url https://x\n}\n": 2, // no colon
		"vars:secret [\n  a,\n":       1,
	}
	for src, line := range cases {
		_, err := parse([]byte(src))
		var se *syntaxError
		if !errors.As(err, &se) || se.line != line {
			t.Errorf("parse(%q) = %v, want an error at line %d", src, err, line)
		}
	}
}

func TestImport_corpus(t *testing.T) {
	r, fatal := Import(os.DirFS("testdata/shop"), "shop")

	importerstest.Golden(t, "shop.golden", importerstest.Render(t, r, fatal))
}

func TestImport_should_never_write_credentials(t *testing.T) {
	r, _ := Import(os.DirFS("testdata/shop"), "shop")

	out := importerstest.Render(t, r, nil)

	for _, secret := range []string{"ak_live_999", "hunter2"} {
		if strings.Contains(out, secret) {
			t.Errorf("%q reached the result", secret)
		}
	}
}

func TestImport_Given_a_folder_that_is_not_a_collection_Then_FLW_IMPORT_002(t *testing.T) {
	_, fatal := Import(os.DirFS(t.TempDir()), "empty")

	if len(fatal) != 1 || !strings.Contains(fatal[0].Args["detail"], "bruno.json") {
		t.Fatalf("fatal %v", fatal)
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"meta {\n", "a {\n}\n", "body:json {\n  x\n}\n", "l [\n", "x {\n  k: '''\n", "}\n", "a {\n  ~\n}"} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, src string) {
		parse([]byte(src)) //nolint:errcheck // must not panic or hang; errors are expected
	})
}

func TestImport_should_redact_bruno_queries_and_bodies(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("bruno.json", `{"version": "1", "name": "x", "type": "collection"}`)
	write("environments/local.bru", "vars {\n  accessToken: V1\n}\n")
	write("a.bru", "meta {\n  name: a\n}\n\npost {\n  url: https://api.example.com/a?token=Q1\n  body: json\n}\n\nparams:query {\n  token: Q1\n}\n\nbody:json {\n  {\"password\": \"B1\"}\n}\n")

	r, _ := Import(os.DirFS(dir), "x")
	out := importerstest.Render(t, r, nil)

	for _, secret := range []string{"V1", "Q1", "B1"} {
		if strings.Contains(out, secret) {
			t.Errorf("%q reached the result:\n%s", secret, out)
		}
	}
}
