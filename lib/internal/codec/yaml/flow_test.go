// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func TestDecodeFlow_should_round_trip_canonical_fixture(t *testing.T) {
	c := newCodec()
	data := readFixture(t, "valid/user-creation.flow.yaml")

	f, diags := c.DecodeFlow("user-creation.flow.yaml", "user-creation", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, data)
	}
	if f.Run == nil || f.Run.MaxDuration != 10*time.Minute || f.Execution.MaxConcurrency != 4 {
		t.Fatalf("unexpected flow settings %+v %+v", f.Run, f.Execution)
	}
	verify := f.Nodes[2]
	if verify.Request == nil || verify.Request.Request.Collection != "identity-api" || verify.Request.Version != "1.1" || verify.Request.Hooks.Skip[0] != "post" {
		t.Fatalf("unexpected verify node %+v", verify.Request)
	}
	if len(f.Nodes[3].Branches) != 2 || len(f.Nodes[4].Then) != 1 || f.Nodes[5].Body[0].Subflow == nil {
		t.Fatalf("containers not decoded: %+v", f.Nodes[3:])
	}
}

func TestDecodeFlow_should_round_trip_the_flowi_overlay_of_a_request_step(t *testing.T) {
	// Arrange
	data := []byte("id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: call\n    type: request\n    request:\n      id: 01J9ZB3M4N5P6Q7R8S9T0VW000\n    overlay: vip\n")
	c := newCodec()

	// Act
	f, diags := c.DecodeFlow("f.flow.yaml", "f", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	// Assert
	if err != nil || f.Nodes[0].Request.Overlay != "vip" || !bytes.Contains(out, []byte("overlay: vip")) {
		t.Fatalf("err %v overlay %+v, output:\n%s", err, f.Nodes[0].Request, out)
	}
}

func TestDecodeFlow_should_keep_unknown_node_keys_as_ordered_params(t *testing.T) {
	data := []byte("id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: d\n    type: delay\n    duration: 2s\n    jitter: 500ms\n    tags: [a, b]\n")

	f, diags := newCodec().DecodeFlow("f.flow.yaml", "f", data)

	requireNoDiagnostics(t, diags)
	p := f.Nodes[0].Params
	if len(p) != 3 || p[0].Key != "duration" || p[1].Key != "jitter" || p[2].Key != "tags" {
		t.Fatalf("params %+v", p)
	}
}

func TestDecodeFlow_invalid(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode diag.Code
		wantPath string
	}{
		{name: "bad_timeout", body: "nodes:\n  - id: a\n    type: log\n    timeout: soon\n", wantCode: diag.CodeYAMLType, wantPath: "$.nodes[0].timeout"},
		{name: "ref_without_id", body: "nodes:\n  - id: a\n    type: request\n    request:\n      path: x\n", wantCode: diag.CodeIDMissing, wantPath: "$.nodes[0].request"},
		{name: "unknown_flow_key", body: "colour: red\n", wantCode: diag.CodeYAMLUnknownKey, wantPath: "$.colour"},
		{name: "unknown_branch_key", body: "nodes:\n  - id: p\n    type: parallel\n    branches:\n      - id: a\n        steps: []\n", wantCode: diag.CodeYAMLUnknownKey, wantPath: "$.nodes[0].branches[0].steps"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\n" + tt.body)

			_, diags := newCodec().DecodeFlow("f.flow.yaml", "f", data)

			if len(diags) != 1 || diags[0].Code != tt.wantCode || diags[0].Path != tt.wantPath {
				t.Fatalf("got %+v", diags)
			}
		})
	}
}

func TestDecodeFlow_should_read_outputs_and_rate_limit_scalar_and_per_host(t *testing.T) {
	tests := []struct {
		name      string
		execution string
		wantAll   string
		wantHosts []model.KeyValue
	}{
		{name: "scalar", execution: "  rate_limit: 20/s\n", wantAll: "20/s"},
		{name: "per_host", execution: "  rate_limit:\n    api.acme.dev: 5/s\n    auth.acme.dev: 1/s\n",
			wantHosts: []model.KeyValue{{Key: "api.acme.dev", Value: "5/s"}, {Key: "auth.acme.dev", Value: "1/s"}}},
		{name: "per_host_with_default", execution: "  rate_limit:\n    \"*\": 50/s\n    api.acme.dev: 5/s\n",
			wantAll: "50/s", wantHosts: []model.KeyValue{{Key: "api.acme.dev", Value: "5/s"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\noutputs:\n  token: \"{{vars.token}}\"\nexecution:\n" + tt.execution + "nodes: []\n")

			f, diags := newCodec().DecodeFlow("f.flow.yaml", "f", data)

			requireNoDiagnostics(t, diags)
			if len(f.Outputs) != 1 || f.Outputs[0] != (model.Output{Name: "token", Value: "{{vars.token}}"}) {
				t.Fatalf("outputs %+v", f.Outputs)
			}
			if f.Execution.RateLimit != tt.wantAll || !slices.Equal(f.Execution.HostRateLimits, tt.wantHosts) {
				t.Fatalf("execution %+v", f.Execution)
			}
		})
	}
}

func TestEncodeFlow_should_round_trip_outputs_and_host_rate_limits(t *testing.T) {
	for _, data := range []string{
		"id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\noutputs:\n  token: \"{{vars.token}}\"\nexecution:\n  rate_limit: 20/s\n",
		"id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nexecution:\n  rate_limit:\n    \"*\": 50/s\n    api.acme.dev: 5/s\n",
	} {
		c := newCodec()
		f, diags := c.DecodeFlow("f.flow.yaml", "f", []byte(data))
		requireNoDiagnostics(t, diags)

		out, err := c.EncodeFlow(f)

		if err != nil {
			t.Fatal(err)
		}
		if string(out) != data {
			t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, data)
		}
	}
}

func TestFlowNode_attach_should_round_trip_and_stay_out_of_params(t *testing.T) {
	data := []byte("id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: pick\n    type: set\n    attach: login\n")

	f, diags := newCodec().DecodeFlow("f.flow.yaml", "f", data)
	requireNoDiagnostics(t, diags)
	out, err := newCodec().EncodeFlow(f)

	if err != nil {
		t.Fatal(err)
	}
	if f.Nodes[0].Attach != "login" || len(f.Nodes[0].Params) != 0 || !bytes.Contains(out, []byte("attach: login")) {
		t.Fatalf("attach %q, params %+v, output:\n%s", f.Nodes[0].Attach, f.Nodes[0].Params, out)
	}
}

func TestDecodeFlow_should_round_trip_the_connection_a_request_step_opens_uses_and_closes(t *testing.T) {
	// Arrange
	data := []byte("id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: join\n    type: request\n    request:\n      id: 01J9ZB3M4N5P6Q7R8S9T0VW000\n    open: chat\n  - id: ask\n    type: request\n    request:\n      id: 01J9ZB3M4N5P6Q7R8S9T0VW001\n    over: chat\n    close: true\n")

	c := newCodec()

	// Act
	f, diags := c.DecodeFlow("f.flow.yaml", "f", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if f.Nodes[0].Request.Open != "chat" || f.Nodes[1].Request.Over != "chat" || !f.Nodes[1].Request.Close {
		t.Fatalf("nodes %+v %+v", f.Nodes[0].Request, f.Nodes[1].Request)
	}
	for _, want := range []string{"open: chat", "over: chat", "close: true"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
