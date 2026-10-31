// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// bom is the UTF-8 byte order mark some editors put at the start of a file.
const bom = "\xef\xbb\xbf"

const matrixYAML = `openapi: 3.0.3
info: {title: examples, version: 1.0.0}
paths:
  /examples/{id}:
    parameters:
      - {name: id, in: path, required: true, schema: {type: string}}
    get:
      operationId: getExample
      parameters:
        - {name: expand, in: query, schema: {type: string}}
        - {name: X-Trace-Id, in: header, schema: {type: string}}
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {type: object, properties: {id: {type: string}}}
`

const matrixJSON = `{"openapi":"3.0.3","info":{"title":"examples","version":"1.0.0"},"paths":{"/examples/{id}":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"get":{"operationId":"getExample","parameters":[{"name":"expand","in":"query","schema":{"type":"string"}},{"name":"X-Trace-Id","in":"header","schema":{"type":"string"}}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string"}}}}}}}}}}}`

func TestImport_should_fill_path_query_and_header_parameters(t *testing.T) {
	r, diags := importDoc(t, matrixYAML)

	v := r.Requests[0].Versions[0]
	if len(diags) != 0 || !strings.HasSuffix(v.URL, "/examples/{{id}}") {
		t.Fatalf("url %q diags %v", v.URL, diags)
	}
	if len(v.Query) != 1 || v.Query[0].Key != "expand" || v.Query[0].Value != "{{expand}}" {
		t.Errorf("query %+v", v.Query)
	}
	if len(v.Headers) != 1 || v.Headers[0].Key != "X-Trace-Id" || v.Headers[0].Value == "" {
		t.Errorf("headers %+v", v.Headers)
	}
}

func TestImport_should_give_the_same_requests_for_YAML_JSON_a_BOM_and_Windows_line_endings(t *testing.T) {
	variants := map[string]string{
		"yaml":           matrixYAML,
		"json":           matrixJSON,
		"yaml with BOM":  bom + matrixYAML,
		"yaml with CRLF": strings.ReplaceAll(matrixYAML, "\n", "\r\n"),
		"json with BOM":  bom + matrixJSON,
	}
	want, _ := importDoc(t, matrixYAML)

	for name, doc := range variants {
		t.Run(name, func(t *testing.T) {
			got, diags := importDoc(t, doc)

			if diags.HasErrors() || len(got.Requests) != 1 {
				t.Fatalf("diags %v, %d requests", diags, len(got.Requests))
			}
			if g, w := fmt.Sprint(got.Requests[0].Versions), fmt.Sprint(want.Requests[0].Versions); g != w {
				t.Errorf("versions differ\n got %s\nwant %s", g, w)
			}
			if len(got.Schemas) != len(want.Schemas) {
				t.Errorf("schemas %d, want %d", len(got.Schemas), len(want.Schemas))
			}
		})
	}
}

func TestImport_should_finish_a_large_spec_quickly(t *testing.T) {
	var b strings.Builder
	b.WriteString("openapi: 3.0.3\ninfo: {title: big, version: 1.0.0}\ncomponents:\n  schemas:\n    Item: {type: object, properties: {id: {type: string}, name: {type: string}}}\npaths:\n")
	const operations = 600
	for i := 0; i < operations; i++ {
		fmt.Fprintf(&b, "  /things%d/{id}:\n    get:\n      operationId: getThing%d\n      parameters:\n        - {name: id, in: path, required: true, schema: {type: string}}\n      responses:\n        \"200\":\n          description: ok\n          content:\n            application/json:\n              schema: {$ref: '#/components/schemas/Item'}\n", i, i)
	}
	started := time.Now()

	r, diags := importDoc(t, b.String())

	if diags.HasErrors() || len(r.Requests) != operations {
		t.Fatalf("%d requests, diags %v", len(r.Requests), diags)
	}
	if took := time.Since(started); took > 10*time.Second {
		t.Errorf("%d operations took %s", operations, took)
	}
}

func TestImport_should_give_two_paths_that_differ_only_in_case_their_own_requests(t *testing.T) {
	r, _ := importDoc(t, `openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /Pets:
    get: {responses: {"200": {description: ok}}}
  /pets:
    get: {responses: {"200": {description: ok}}}
`)

	if len(r.Requests) != 2 || strings.EqualFold(r.Requests[0].Path, r.Requests[1].Path) {
		var paths []string
		for _, q := range r.Requests {
			paths = append(paths, q.Path)
		}
		t.Fatalf("paths %v: on a case-insensitive disk these would be one file", paths)
	}
}
