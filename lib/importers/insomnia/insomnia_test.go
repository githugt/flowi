// SPDX-License-Identifier: Apache-2.0

package insomnia

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers/importerstest"
)

func TestImport_corpus(t *testing.T) {
	var files []string
	for _, pattern := range []string{"api-*.json", "api-*.yaml"} {
		found, err := filepath.Glob(filepath.Join("testdata", pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, found...)
	}
	if len(files) == 0 {
		t.Fatal("no corpus")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			r, fatal := Import(src, filepath.Base(file))

			name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
			importerstest.Golden(t, name+".golden", importerstest.Render(t, r, fatal))
		})
	}
}

func TestImport_should_never_write_credentials(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "api-v4.json"))
	if err != nil {
		t.Fatal(err)
	}

	r, _ := Import(src, "api-v4.json")
	out := importerstest.Render(t, r, nil)

	for _, secret := range []string{"sk-live-123", "literal-key", "hunter2", `"pw"`, "shh", "s3"} {
		if strings.Contains(out, secret) {
			t.Errorf("%q reached the result:\n%s", secret, out)
		}
	}
}

func TestImport_should_refuse_what_is_not_an_export(t *testing.T) {
	cases := map[string]string{"json": `{"hello": "world"}`, "list": `[1, 2]`}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, fatal := Import([]byte(src), "x.json")

			// Assert
			if len(fatal) != 1 || (fatal[0].Code != diag.CodeImportInvalid && fatal[0].Code != diag.CodeImportSyntax) {
				t.Errorf("fatal %v", fatal)
			}
		})
	}
}
