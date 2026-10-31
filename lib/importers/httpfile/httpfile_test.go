// SPDX-License-Identifier: Apache-2.0

package httpfile

import (
	"os"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"
)

func load(t *testing.T) importers.Result {
	t.Helper()
	data, err := os.ReadFile("testdata/examples.http")
	if err != nil {
		t.Fatal(err)
	}
	res, diags := Import(data, "examples.http")
	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	return res
}

func requestAt(res importers.Result, path string) model.Request {
	for _, r := range res.Requests {
		if r.Path == path {
			return r
		}
	}
	return model.Request{}
}

func TestImport_should_read_requests_headers_bodies_and_variables(t *testing.T) {
	res := load(t)

	create := requestAt(res, "http/createExample").Versions[0]
	get := requestAt(res, "http/get-examples-42").Versions[0]

	if len(res.Requests) != 4 {
		t.Fatalf("requests %d", len(res.Requests))
	}
	if create.Method != "POST" || create.URL != "{{env.host}}/examples" || !strings.Contains(create.Body.JSON, `"name": "first"`) {
		t.Fatalf("create %+v", create)
	}
	if create.Headers[1].Value != "{{$uuid-v4}}" {
		t.Fatalf("dynamic variable: %+v", create.Headers)
	}
	if get.URL != "{{env.host}}/examples/42?verbose=true" || get.Method != "GET" {
		t.Fatalf("get %+v", get)
	}
	env := res.Environments[0]
	if env.Name != "examples" || len(env.Vars) != 2 || env.Vars[0].Value != "http://localhost:8080" || !env.Vars[1].Secret || env.Vars[1].Value != "" {
		t.Fatalf("environment %+v", env)
	}
}

func TestImport_should_redact_credentials_and_report_scripts(t *testing.T) {
	res := load(t)

	secure := requestAt(res, "http/get-secure-examples-42").Versions[0]
	form := requestAt(res, "http/post-login").Versions[0]

	if secure.Headers[0].Value != "{{secret.authorization}}" || secure.Headers[1].Value != "{{env.apiKey}}" && secure.Headers[1].Value != "{{secret.apiKey}}" {
		t.Fatalf("headers %+v", secure.Headers)
	}
	if len(form.Body.Form) != 2 || form.Body.Form[1].Value != importers.Redacted && form.Body.Form[1].Value != "hunter2" {
		t.Fatalf("form %+v", form.Body.Form)
	}
	var unmapped, sensitive int
	for _, it := range res.Items {
		for _, d := range it.Diags {
			switch d.Code {
			case diag.CodeImportUnmapped:
				unmapped++
			case diag.CodeImportSensitive:
				sensitive++
			}
		}
	}
	if unmapped != 1 || sensitive < 2 {
		t.Fatalf("unmapped %d sensitive %d", unmapped, sensitive)
	}
}

func TestImport_Given_no_request_Then_FLW_IMPORT_002(t *testing.T) {
	_, diags := Import([]byte("# just a comment\n@x = 1\n"), "empty.http")

	if len(diags) != 1 || diags[0].Code != diag.CodeImportInvalid {
		t.Fatalf("diags %v", diags)
	}
}
