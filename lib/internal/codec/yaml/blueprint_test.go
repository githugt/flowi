// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"bytes"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

const blueprintFile = `id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ
name: Standard onboarding
description: Log in, then create a user.
params:
  api:
    type: request-ref
    description: The login request
    default: auth/login
  email:
    type: string
    default: demo@example.com
  retries:
    type: int
  notify:
    type: bool
    default: "true"
  verify:
    type: flow-ref
inputs:
  note:
    default: hello
nodes:
  - id: login
    type: request
    request:
      param: api
  - id: verify
    type: subflow
    flow:
      param: verify
`

func TestDecodeBlueprint_should_read_params_and_param_refs(t *testing.T) {
	bp, diags := newCodec().DecodeBlueprint("standard.blueprint.yaml", "standard", []byte(blueprintFile))

	requireNoDiagnostics(t, diags)
	if bp.Name != "Standard onboarding" || bp.Path != "standard" || len(bp.Params) != 5 {
		t.Fatalf("blueprint %+v", bp)
	}
	api := bp.Params[0]
	if api.Name != "api" || api.Type != model.ParamRequestRef || api.Description != "The login request" || api.Default != "auth/login" {
		t.Fatalf("api %+v", api)
	}
	if bp.Params[2].Name != "retries" || bp.Params[2].Type != model.ParamInt || bp.Params[2].Default != "" {
		t.Fatalf("retries %+v", bp.Params[2])
	}
	login, verify := bp.Body.Nodes[0], bp.Body.Nodes[1]
	if login.Request == nil || login.Request.Request.Param != "api" || login.Request.Request.ID != "" {
		t.Fatalf("login %+v", login.Request)
	}
	if verify.Subflow == nil || verify.Subflow.Flow.Param != "verify" {
		t.Fatalf("verify %+v", verify.Subflow)
	}
	if len(bp.Body.Inputs) != 1 || bp.Body.Inputs[0].Name != "note" {
		t.Fatalf("inputs %+v", bp.Body.Inputs)
	}
}

func TestEncodeBlueprint_should_be_canonical_and_stable(t *testing.T) {
	c := newCodec()
	bp, diags := c.DecodeBlueprint("standard.blueprint.yaml", "standard", []byte(blueprintFile))
	requireNoDiagnostics(t, diags)

	first, err := c.EncodeBlueprint(bp)
	if err != nil {
		t.Fatal(err)
	}
	again, diags := c.DecodeBlueprint("standard.blueprint.yaml", "standard", first)
	requireNoDiagnostics(t, diags)
	second, err := c.EncodeBlueprint(again)

	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("not stable:\n%s\n---\n%s", first, second)
	}
	if !strings.Contains(string(first), "params:\n  api:\n    type: request-ref") || !strings.Contains(string(first), "request:\n      param: api") {
		t.Fatalf("encoding:\n%s", first)
	}
}

func TestDecodeBlueprint_should_report_each_problem_with_its_place(t *testing.T) {
	tests := []struct {
		name string
		edit func(string) string
		code diag.Code
	}{
		{"param without type", func(s string) string { return strings.Replace(s, "    type: string\n", "", 1) }, diag.CodeBlueprintParamType},
		{"unknown param key", func(s string) string {
			return strings.Replace(s, "    type: string\n", "    type: string\n    colour: red\n", 1)
		}, diag.CodeYAMLUnknownKey},
		{"unknown param type", func(s string) string { return strings.Replace(s, "type: bool", "type: money", 1) }, diag.CodeBlueprintParamType},
		{"param ref with extra keys", func(s string) string {
			return strings.Replace(s, "      param: api\n", "      param: api\n      path: x\n", 1)
		}, diag.CodeYAMLUnknownKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags := newCodec().DecodeBlueprint("b.blueprint.yaml", "b", []byte(tt.edit(blueprintFile)))

			if !hasCode(diags, tt.code) {
				t.Fatalf("want %s in %v", tt.code, diags)
			}
			for _, d := range diags {
				if d.Code == tt.code && d.Pos.Line == 0 {
					t.Fatalf("no position: %+v", d)
				}
			}
		})
	}
}

func TestDecodeFlow_Given_a_param_ref_outside_a_blueprint_Then_unknown_key(t *testing.T) {
	data := []byte("id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: a\n    type: request\n    request:\n      param: api\n")

	_, diags := newCodec().DecodeFlow("f.flow.yaml", "f", data)

	if !hasCode(diags, diag.CodeYAMLUnknownKey) {
		t.Fatalf("a flow must not accept blueprint references: %v", diags)
	}
}

func hasCode(l diag.List, c diag.Code) bool {
	for _, d := range l {
		if d.Code == c {
			return true
		}
	}
	return false
}

func TestDecodeBlueprint_should_round_trip_the_canonical_fixture(t *testing.T) {
	c := newCodec()
	data := readFixture(t, "valid/standard.blueprint.yaml")

	bp, diags := c.DecodeBlueprint("standard.blueprint.yaml", "standard", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeBlueprint(bp)

	if err != nil || !bytes.Equal(out, data) {
		t.Fatalf("round trip differs (err %v):\n--- got\n%s\n--- want\n%s", err, out, data)
	}
}

func TestBlueprint_should_round_trip_its_phase(t *testing.T) {
	src := "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nname: Sign\nphase: pre\nnodes:\n  - id: a\n    type: log\n    value: x\n"

	bp, diags := newCodec().DecodeBlueprint("sign.blueprint.yaml", "sign", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := newCodec().EncodeBlueprint(bp)

	if err != nil || bp.Phase != "pre" || !strings.Contains(string(out), "phase: pre") {
		t.Fatalf("phase %q err %v\n%s", bp.Phase, err, out)
	}
}

func TestBlueprint_Given_an_unknown_phase_Then_reports_it(t *testing.T) {
	src := "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nphase: middle\nnodes:\n  - id: a\n    type: log\n    value: x\n"

	_, diags := newCodec().DecodeBlueprint("sign.blueprint.yaml", "sign", []byte(src))

	if !diags.HasErrors() {
		t.Fatal("an unknown phase was accepted")
	}
}

func TestBlueprint_Given_no_phase_Then_the_encoding_has_none(t *testing.T) {
	bp, _ := newCodec().DecodeBlueprint("standard.blueprint.yaml", "standard", []byte(blueprintFile))

	out, _ := newCodec().EncodeBlueprint(bp)

	if strings.Contains(string(out), "phase:") {
		t.Fatalf("a phase appeared:\n%s", out)
	}
}

func TestBlueprint_Given_outcomes_When_saved_and_loaded_Then_they_round_trip(t *testing.T) {
	const src = "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nname: With outcomes\noutcomes:\n  - name: success\n    description: Fine\n  - name: gone\n    when: response.status == 410\n    color: amber\nnodes:\n  - id: only\n    type: log\n    value: done\n"
	c := newCodec()

	bp, diags := c.DecodeBlueprint("b.blueprint.yaml", "b", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeBlueprint(bp)

	if err != nil || len(bp.Outcomes) != 2 || bp.Outcomes[1].Color != "amber" || string(out) != src {
		t.Fatalf("err %v outcomes %+v, output:\n%s", err, bp.Outcomes, out)
	}
}
