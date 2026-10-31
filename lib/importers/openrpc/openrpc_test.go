// SPDX-License-Identifier: Apache-2.0

package openrpc

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

			name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
			importerstest.Golden(t, name+".golden", importerstest.Render(t, r, fatal))
		})
	}
}

func TestImport_should_refuse_what_is_not_an_OpenRPC_document(t *testing.T) {
	cases := map[string]string{"json": `{"hello": "world"}`, "openapi": `{"openapi": "3.0.0", "paths": {}}`, "no methods": `{"openrpc": "1.2.6", "info": {}}`}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, fatal := Import([]byte(src), "x.json")

			if len(fatal) == 0 || fatal[0].Code != diag.CodeImportInvalid {
				t.Fatalf("got %v", fatal)
			}
		})
	}
}

func TestImport_should_report_a_file_that_does_not_parse_as_a_syntax_problem(t *testing.T) {
	_, fatal := Import([]byte("{\"openrpc\": "), "x.json")

	if len(fatal) == 0 || fatal[0].Code != diag.CodeImportSyntax {
		t.Fatalf("got %v", fatal)
	}
}
