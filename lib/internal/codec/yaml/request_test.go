// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
)

const requestHeader = "id: 01J9ZA3M4N5P6Q7R8S9T0VWXYZ\nname: R\n"

func TestDecodeRequest_should_round_trip_canonical_fixture(t *testing.T) {
	c := newCodec()
	data := readFixture(t, "valid/login.req.yaml")

	r, diags := c.DecodeRequest("login.req.yaml", "auth/login", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, data)
	}
	if r.Path != "auth/login" || r.Default != "1" || len(r.Versions) != 3 || len(r.Hooks.Pre) != 1 {
		t.Fatalf("unexpected request %+v", r)
	}
	if v := r.Versions[0]; len(v.Extract) != 2 || v.Extract[0].Name != "token" || v.Headers[0].Key != "Content-Type" {
		t.Fatalf("unexpected version %+v", v)
	}
}

func TestDecodeRequest_should_keep_unquoted_float_like_version_keys_literal(t *testing.T) {
	data := []byte(requestHeader + "versions:\n  1.10:\n    method: GET\n    url: http://x\n  2.0:\n    method: GET\n    url: http://y\n")

	r, diags := newCodec().DecodeRequest("r.req.yaml", "r", data)

	requireNoDiagnostics(t, diags)
	if r.Versions[0].Label != "1.10" || r.Versions[1].Label != "2.0" {
		t.Fatalf("labels %q %q", r.Versions[0].Label, r.Versions[1].Label)
	}
}

func TestEncodeRequest_should_sort_versions(t *testing.T) {
	data := []byte(requestHeader + "versions:\n  \"2\":\n    url: a\n  \"1.10\":\n    url: b\n  \"1.2\":\n    url: c\n")
	c := newCodec()
	r, diags := c.DecodeRequest("r.req.yaml", "r", data)
	requireNoDiagnostics(t, diags)

	out, err := c.EncodeRequest(r)

	if err != nil {
		t.Fatal(err)
	}
	i12, i110, i2 := strings.Index(string(out), `"1.2"`), strings.Index(string(out), `"1.10"`), strings.Index(string(out), `"2"`)
	if i12 >= i110 || i110 >= i2 {
		t.Fatalf("versions not sorted:\n%s", out)
	}
}

func TestDecodeRequest_invalid(t *testing.T) {
	tests := []struct {
		name     string
		versions string
		wantCode diag.Code
		wantPath string
	}{
		{name: "bad_label", versions: "\n  v1:\n    url: a\n", wantCode: diag.CodeVersionInvalid, wantPath: "$.versions.v1"},
		{name: "equal_labels", versions: "\n  \"1\":\n    url: a\n  \"1.0\":\n    url: b\n", wantCode: diag.CodeVersionDuplicate, wantPath: "$.versions.'1.0'"},
		{name: "no_versions", versions: " {}\n", wantCode: diag.CodeVersionNone, wantPath: "$.versions"},
		{name: "two_body_kinds", versions: "\n  \"1\":\n    url: a\n    body:\n      json: '{}'\n      text: hi\n", wantCode: diag.CodeYAMLType, wantPath: "$.versions.1.body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(requestHeader + "versions:" + tt.versions)

			_, diags := newCodec().DecodeRequest("r.req.yaml", "r", data)

			if len(diags) != 1 || diags[0].Code != tt.wantCode || diags[0].Path != tt.wantPath {
				t.Fatalf("got %+v", diags)
			}
		})
	}
}

func TestDecodeRequest_should_require_versions_key(t *testing.T) {
	_, diags := newCodec().DecodeRequest("r.req.yaml", "r", []byte(requestHeader))

	if len(diags) != 1 || diags[0].Code != diag.CodeVersionNone {
		t.Fatalf("got %+v", diags)
	}
}

func TestDecode_should_accept_unquoted_version_values(t *testing.T) {
	req := []byte(requestHeader + "default: 2.0\nversions:\n  1.10:\n    url: a\n  2.0:\n    url: b\n")
	flow := []byte("id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nprefer_version: 2\nnodes:\n  - id: a\n    type: request\n    request:\n      id: 01J9ZA3M4N5P6Q7R8S9T0VWXYZ\n    version: 1.10\n")
	c := newCodec()

	r, rdiags := c.DecodeRequest("r.req.yaml", "r", req)
	f, fdiags := c.DecodeFlow("f.flow.yaml", "f", flow)

	requireNoDiagnostics(t, rdiags)
	requireNoDiagnostics(t, fdiags)
	if r.Default != "2.0" || f.PreferVersion != "2" || f.Nodes[0].Request.Version != "1.10" {
		t.Fatalf("default %q prefer %q version %q", r.Default, f.PreferVersion, f.Nodes[0].Request.Version)
	}
}

func TestDecodeRequest_should_round_trip_operation(t *testing.T) {
	c := newCodec()
	src := "id: 01JA0000000000000000000010\nname: Get pet\noperation: getPet\nversions:\n  \"1\":\n    method: GET\n    url: \"{{env.base_url}}/pets/{{petId}}\"\n"

	r, diags := c.DecodeRequest("x.req.yaml", "pets/get", []byte(src))
	out, err := c.EncodeRequest(r)

	if len(diags) != 0 || err != nil || r.Operation != "getPet" || string(out) != src {
		t.Fatalf("diags %v err %v operation %q\n%s", diags, err, r.Operation, out)
	}
}

func TestRequest_should_round_trip_the_dirty_edit(t *testing.T) {
	data := []byte(requestHeader + "versions:\n  \"1\":\n    method: GET\n    url: http://x\nedit:\n  method: POST\n  url: http://y\nedit_base: \"1\"\n")
	c := newCodec()

	r, diags := c.DecodeRequest("r.req.yaml", "r", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	if err != nil {
		t.Fatal(err)
	}
	if r.Edit == nil || r.Edit.URL != "http://y" || r.Edit.Method != "POST" || r.EditBase != "1" {
		t.Fatalf("edit not decoded: %+v base %q", r.Edit, r.EditBase)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, data)
	}
}

func TestRequest_should_round_trip_the_overlay_an_edit_was_made_with(t *testing.T) {
	// Arrange
	data := []byte(requestHeader + "versions:\n  \"1\":\n    method: GET\n    url: http://x\nedit:\n  method: GET\n  url: http://y\nedit_base: \"1\"\nedit_overlay: vip\n")
	c := newCodec()

	// Act
	r, diags := c.DecodeRequest("r.req.yaml", "r", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	// Assert
	if err != nil || r.EditOverlay != "vip" || !bytes.Equal(out, data) {
		t.Fatalf("err %v overlay %q, output:\n%s", err, r.EditOverlay, out)
	}
}

func TestRequest_should_round_trip_message_hooks(t *testing.T) {
	// Arrange
	data := []byte(requestHeader + "versions:\n  \"1\":\n    method: GET\n    url: http://x\n    hooks:\n      message:\n        - inline:\n            - id: ok\n              type: assert\n              check: body.ok\n")
	c := newCodec()

	// Act
	r, diags := c.DecodeRequest("r.req.yaml", "r", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	// Assert
	if err != nil || len(r.Versions[0].Hooks.Message) != 1 || !bytes.Equal(out, data) {
		t.Fatalf("err %v hooks %+v, output:\n%s", err, r.Versions[0].Hooks, out)
	}
}

func TestEncodeRequest_should_omit_the_edit_when_absent(t *testing.T) {
	data := []byte(requestHeader + "versions:\n  \"1\":\n    url: a\n")
	c := newCodec()
	r, _ := c.DecodeRequest("r.req.yaml", "r", data)

	out, err := c.EncodeRequest(r)

	if err != nil || strings.Contains(string(out), "edit") {
		t.Fatalf("err %v, output:\n%s", err, out)
	}
}

func TestRequest_should_round_trip_docs_variables_and_field_descriptions(t *testing.T) {
	data := []byte(requestHeader + `versions:
  "1":
    method: GET
    url: http://x/{{var.id}}
    query:
      page: "1"
      sort: name
    headers:
      X-Trace: abc
    variables:
      id: "7"
    docs: |-
      # Get thing

      Returns it.
    notes:
      query:
        page: which page to read
      headers:
        X-Trace: ties the call to a trace
      variables:
        id: the thing to read
`)
	c := newCodec()

	r, diags := c.DecodeRequest("r.req.yaml", "r", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	if err != nil {
		t.Fatal(err)
	}
	v := r.Versions[0]
	if v.Docs != "# Get thing\n\nReturns it." || len(v.Variables) != 1 || v.Variables[0].Description != "the thing to read" ||
		v.Query[0].Description != "which page to read" || v.Query[1].Description != "" || v.Headers[0].Description != "ties the call to a trace" {
		t.Fatalf("version %+v", v)
	}
	// The encoder indents the blank line inside the docs block; compare what
	// the file reads back as, not its bytes.
	again, diags := c.DecodeRequest("r.req.yaml", "r", out)
	requireNoDiagnostics(t, diags)
	if !reflect.DeepEqual(again.Versions, r.Versions) {
		t.Fatalf("re-read differs:\n%+v\n%+v", again.Versions, r.Versions)
	}
}

const asyncRequest = requestHeader + `versions:
  "1":
    async:
      protocol: mqtt
      server: "{{env.broker}}"
      channel: orders/placed
      send:
        body:
          json: "{\"id\": 1}"
        headers:
          type: order
      receive:
        count: 2
        timeout: 10s
        match: body.id == 1
      binding:
        qos: "1"
        retain: "true"
`

func TestDecodeRequest_should_read_and_round_trip_an_async_version(t *testing.T) {
	c := newCodec()

	r, diags := c.DecodeRequest("r.req.yaml", "r", []byte(asyncRequest))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	if err != nil {
		t.Fatal(err)
	}
	a := r.Versions[0].Async
	if a == nil || a.Protocol != "mqtt" || a.Channel != "orders/placed" || a.Server != "{{env.broker}}" {
		t.Fatalf("async %+v", a)
	}
	if a.Send == nil || a.Send.Body.JSON != `{"id": 1}` || a.Send.Headers[0].Key != "type" {
		t.Fatalf("send %+v", a.Send)
	}
	if a.Receive == nil || a.Receive.Count != 2 || a.Receive.Timeout.String() != "10s" || a.Receive.Match != "body.id == 1" {
		t.Fatalf("receive %+v", a.Receive)
	}
	if len(a.Binding) != 2 || a.Binding[0].Key != "qos" || a.Binding[1].Value != "true" {
		t.Fatalf("binding %+v", a.Binding)
	}
	if string(out) != asyncRequest {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, asyncRequest)
	}
}

const grpcRequest = requestHeader + `versions:
  "1":
    headers:
      authorization: Bearer {{secret.token}}
    grpc:
      server: "{{env.api}}"
      method: acme.orders.v1.Orders/GetOrder
      body:
        json: "{\"id\": \"7\"}"
      proto:
        - protos/orders.proto
      import:
        - protos
      receive:
        count: 3
        timeout: 10s
        match: body.status == "ok"
`

func TestDecodeRequest_should_read_and_round_trip_a_grpc_version(t *testing.T) {
	c := newCodec()

	r, diags := c.DecodeRequest("r.req.yaml", "r", []byte(grpcRequest))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	if err != nil {
		t.Fatal(err)
	}
	g := r.Versions[0].GRPC
	if g == nil || g.Server != "{{env.api}}" || g.Method != "acme.orders.v1.Orders/GetOrder" || g.Body.JSON != `{"id": "7"}` {
		t.Fatalf("grpc %+v", g)
	}
	if len(g.Proto) != 1 || g.Proto[0] != "protos/orders.proto" || len(g.Import) != 1 || g.Import[0] != "protos" {
		t.Fatalf("proto %v import %v", g.Proto, g.Import)
	}
	if g.Receive == nil || g.Receive.Count != 3 || g.Receive.Timeout.String() != "10s" || g.Receive.Match != `body.status == "ok"` {
		t.Fatalf("receive %+v", g.Receive)
	}
	if string(out) != grpcRequest {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, grpcRequest)
	}
}

func TestDecodeRequest_should_round_trip_a_grpc_web_version(t *testing.T) {
	// Arrange
	text := requestHeader + "versions:\n  \"1\":\n    grpc:\n      server: https://proxy.example/api\n      method: acme.orders.v1.Orders/GetOrder\n      proto:\n        - protos/orders.proto\n      web: true\n"
	c := newCodec()

	// Act
	r, diags := c.DecodeRequest("r.req.yaml", "r", []byte(text))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	// Assert
	if err != nil || !r.Versions[0].GRPC.Web || string(out) != text {
		t.Fatalf("err %v web %v, output:\n%s", err, r.Versions[0].GRPC.Web, out)
	}
}

func TestDecodeRequest_should_leave_http_versions_without_async(t *testing.T) {
	data := []byte(requestHeader + "versions:\n  \"1\":\n    url: http://x\n")

	r, diags := newCodec().DecodeRequest("r.req.yaml", "r", data)

	requireNoDiagnostics(t, diags)
	if r.Versions[0].Async != nil {
		t.Fatalf("async %+v", r.Versions[0].Async)
	}
}

func TestDecodeRequest_should_reject_unknown_async_keys(t *testing.T) {
	data := []byte(requestHeader + "versions:\n  \"1\":\n    async:\n      protocol: ws\n      bogus: 1\n")

	_, diags := newCodec().DecodeRequest("r.req.yaml", "r", data)

	if len(diags) == 0 {
		t.Fatal("want a diagnostic for the unknown key")
	}
}

func TestDecodeRequest_should_read_and_write_the_name_of_a_version(t *testing.T) {
	c := newCodec()
	data := []byte("id: 01JZZZZZZZZZZZZZZZZZZZZZZ1\nversions:\n  \"1\":\n    name: Before the tax change\n    method: GET\n    url: https://example.test/a\n")

	r, diags := c.DecodeRequest("a.req.yaml", "a", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	if err != nil || r.Versions[0].Name != "Before the tax change" || !bytes.Contains(out, []byte("name: Before the tax change")) {
		t.Fatalf("name %q, encoded:\n%s (%v)", r.Versions[0].Name, out, err)
	}
}

func TestRequest_should_round_trip_an_auth_block(t *testing.T) {
	// Arrange
	data := []byte(requestHeader + "versions:\n  \"1\":\n    method: GET\n    url: http://x\n    auth:\n      type: oauth2\n      grant: client_credentials\n      token_url: \"{{env.auth_url}}/token\"\n      client_id: abc\n      client_secret: \"{{secret.client_secret}}\"\n")
	c := newCodec()

	// Act
	r, diags := c.DecodeRequest("r.req.yaml", "r", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	// Assert
	a := r.Versions[0].Auth
	if err != nil || a == nil || a.Type != "oauth2" || len(a.Params) != 4 || a.Params[3].Key != "client_secret" || !bytes.Equal(out, data) {
		t.Fatalf("err %v auth %+v, output:\n%s", err, a, out)
	}
}

// Behaviour change: a multipart body is a mapping keyed by part name, so two parts of the same name (several files
// sent as "files") were written as two "files:" keys, which the loader refuses and the request could no longer be
// opened. Repeated names are now written as a list of one-entry mappings, in order; unique names stay a mapping.
func TestEncodeRequest_Given_two_parts_with_one_name_Then_they_are_written_as_a_list_and_read_back(t *testing.T) {
	// Arrange
	data := []byte(requestHeader + "versions:\n  \"1\":\n    method: post\n    url: https://x.dev/up\n    body:\n      multipart:\n        - files:\n            file: a.txt\n        - note: hi\n        - files:\n            file: b.txt\n")
	c := newCodec()

	// Act
	r, diags := c.DecodeRequest("r.req.yaml", "r", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	parts := r.Versions[0].Body.Multipart
	if len(parts) != 3 || parts[0].Key != "files" || parts[1].Key != "note" || parts[2].Key != "files" {
		t.Fatalf("parts %+v", parts)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, data)
	}
}

func TestEncodeRequest_Given_unique_part_names_Then_multipart_stays_a_mapping(t *testing.T) {
	data := []byte(requestHeader + "versions:\n  \"1\":\n    method: post\n    url: https://x.dev/up\n    body:\n      multipart:\n        note: hi\n        doc:\n          file: a.txt\n")
	c := newCodec()

	r, diags := c.DecodeRequest("r.req.yaml", "r", data)
	requireNoDiagnostics(t, diags)
	out, _ := c.EncodeRequest(r)

	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, data)
	}
}

func TestDecodeRequest_Given_a_multipart_list_item_with_two_keys_Then_it_is_refused(t *testing.T) {
	data := []byte(requestHeader + "versions:\n  \"1\":\n    method: post\n    url: https://x.dev/up\n    body:\n      multipart:\n        - a: 1\n          b: 2\n")

	_, diags := newCodec().DecodeRequest("r.req.yaml", "r", data)

	if len(diags) == 0 {
		t.Fatal("a part holds one name; two keys in one item should be reported")
	}
}

const datasetRequest = requestHeader + `versions:
  "1":
    dataset:
      driver: postgres
      server: postgres://flowi:{{secret.db_password}}@{{env.db_host}}:5432/shop
      query: SELECT id, name FROM customers WHERE id = $1 AND region = $2
      params:
        - "{{var.id}}"
        - eu
      limit: 50
    extract:
      first_name:
        from: body
        path: $.rows[0].name
`

func TestDecodeRequest_should_read_and_round_trip_a_dataset_version(t *testing.T) {
	c := newCodec()

	r, diags := c.DecodeRequest("r.req.yaml", "r", []byte(datasetRequest))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	if err != nil {
		t.Fatal(err)
	}
	d := r.Versions[0].Dataset
	if d == nil || d.Driver != "postgres" || d.Limit != 50 || d.Query != "SELECT id, name FROM customers WHERE id = $1 AND region = $2" {
		t.Fatalf("dataset %+v", d)
	}
	if len(d.Params) != 2 || d.Params[0] != "{{var.id}}" || d.Params[1] != "eu" {
		t.Fatalf("params %v", d.Params)
	}
	if string(out) != datasetRequest {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, datasetRequest)
	}
}

func TestDecodeRequest_Given_a_number_param_should_keep_it_as_text(t *testing.T) {
	text := requestHeader + "versions:\n  \"1\":\n    dataset:\n      driver: sqlite\n      server: shop.db\n      query: SELECT * FROM t WHERE id = ?\n      params:\n        - 42\n"

	r, diags := newCodec().DecodeRequest("r.req.yaml", "r", []byte(text))

	requireNoDiagnostics(t, diags)
	if got := r.Versions[0].Dataset.Params; len(got) != 1 || got[0] != "42" {
		t.Fatalf("params %v", got)
	}
}

func TestRequest_Given_named_outcomes_When_saved_and_loaded_Then_they_round_trip_in_order(t *testing.T) {
	src := requestHeader + `versions:
  "1":
    method: GET
    url: https://example.test/x
    outcomes:
      - name: success
      - name: failure
        disabled: true
      - name: ok
        when: response.status in [200, 201]
      - name: not-found
        when: response.status == 404
`
	c := newCodec()

	r, diags := c.DecodeRequest("x.req.yaml", "x", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	got := r.Versions[0].Outcomes
	if err != nil || len(got) != 4 || got[1].Name != "failure" || !got[1].Disabled || got[2].When != "response.status in [200, 201]" || got[3].Name != "not-found" {
		t.Fatalf("err %v outcomes %+v", err, got)
	}
	if !bytes.Equal(out, []byte(src)) {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, src)
	}
}

func TestNode_Given_after_outcome_When_saved_and_loaded_Then_it_round_trips(t *testing.T) {
	const src = "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: login\n    type: delay\n    duration: 1s\n  - id: missing\n    type: delay\n    after_outcome:\n      - login:not-found\n      - login:auth\n    duration: 1s\n"
	c := newCodec()

	f, diags := c.DecodeFlow("f.flow.yaml", "f", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	if err != nil || len(f.Nodes[1].AfterOutcome) != 2 || !bytes.Equal(out, []byte(src)) {
		t.Fatalf("err %v after_outcome %v, output:\n%s", err, f.Nodes[1].AfterOutcome, out)
	}
}

func TestRequest_Given_outcome_colour_and_description_When_saved_and_loaded_Then_they_round_trip(t *testing.T) {
	src := requestHeader + `versions:
  "1":
    method: GET
    url: https://example.test/x
    outcomes:
      - name: success
        color: green
        description: What we want
      - name: not-found
        when: response.status == 404
        color: amber
        description: The thing is gone
`
	c := newCodec()

	r, diags := c.DecodeRequest("x.req.yaml", "x", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeRequest(r)

	got := r.Versions[0].Outcomes
	if err != nil || got[0].Color != "green" || got[1].Description != "The thing is gone" || !bytes.Equal(out, []byte(src)) {
		t.Fatalf("err %v outcomes %+v, output:\n%s", err, got, out)
	}
}

func TestNode_Given_a_step_with_its_own_outcomes_When_saved_and_loaded_Then_they_round_trip(t *testing.T) {
	const src = "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: call\n    type: request\n    request:\n      id: 01J9ZA3M4N5P6Q7R8S9T0VWXYZ\n      path: auth/login\n    outcomes:\n      - name: success\n      - name: gone\n        when: response.status == 410\n        color: pink\n"
	c := newCodec()

	f, diags := c.DecodeFlow("f.flow.yaml", "f", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	got := f.Nodes[0].Request.Outcomes
	if err != nil || len(got) != 2 || got[1].Name != "gone" || got[1].Color != "pink" || !bytes.Equal(out, []byte(src)) {
		t.Fatalf("err %v outcomes %+v, output:\n%s", err, got, out)
	}
}
