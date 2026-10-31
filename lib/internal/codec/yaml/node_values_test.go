// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"bytes"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

const setValuesFlow = `id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ
nodes:
  - id: save
    type: set
    values:
      - name: token
        from: response
        in: body
        path: $.data.token
      - name: trace
        from: response
        in: header
        header: X-Trace
      - name: user
        from: generate
        value: "{{$first-name}}"
        seed: 7
      - name: total
        value: "{{vars.a + 1}}"
      - name: raw
        from: literal
        value: "{{not evaluated}}"
`

func TestDecodeFlow_Given_a_set_with_values_When_saved_and_loaded_Then_it_round_trips(t *testing.T) {
	c := newCodec()

	f, diags := c.DecodeFlow("f.flow.yaml", "f", []byte(setValuesFlow))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, []byte(setValuesFlow)) {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, setValuesFlow)
	}
	vals := f.Nodes[0].Values
	if len(vals) != 5 || vals[0] != (model.SetValue{Name: "token", From: "response", Where: "body", Path: "$.data.token"}) {
		t.Fatalf("values %+v", vals)
	}
	if vals[2].Seed != 7 || vals[2].Value != "{{$first-name}}" || vals[4].From != "literal" {
		t.Fatalf("values %+v", vals)
	}
}

func TestNode_Given_a_colour_When_saved_and_loaded_Then_it_round_trips(t *testing.T) {
	const src = "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: pause\n    type: delay\n    color: pink\n    duration: 2s\n"
	c := newCodec()

	f, diags := c.DecodeFlow("f.flow.yaml", "f", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	if err != nil || f.Nodes[0].Color != "pink" || !bytes.Equal(out, []byte(src)) {
		t.Fatalf("err %v colour %q, output:\n%s", err, f.Nodes[0].Color, out)
	}
}

func TestFlow_Given_inputs_When_saved_Then_their_order_and_types_are_kept(t *testing.T) {
	const src = `id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ
inputs:
  zeta:
    type: string
    default: z
  alpha:
    type: number
  mid:
    default: "{{env.base}}"
outputs:
  user: "{{vars.user}}"
  count:
    type: number
    value: "{{vars.count}}"
nodes:
  - id: "n"
    type: note
    text: x
`
	c := newCodec()

	f, diags := c.DecodeFlow("f.flow.yaml", "f", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	if err != nil || !bytes.Equal(out, []byte(src)) {
		t.Fatalf("err %v, output:\n%s", err, out)
	}
	if got := []string{f.Inputs[0].Name, f.Inputs[1].Name, f.Inputs[2].Name}; got[0] != "zeta" || got[1] != "alpha" || got[2] != "mid" {
		t.Fatalf("input order %v", got)
	}
	if f.Inputs[0].Type != "string" || f.Inputs[1].Type != "number" || f.Outputs[1] != (model.Output{Name: "count", Type: "number", Value: "{{vars.count}}"}) {
		t.Fatalf("inputs %+v outputs %+v", f.Inputs, f.Outputs)
	}
}

func TestNode_Given_a_step_that_runs_after_a_failure_When_saved_and_loaded_Then_it_round_trips(t *testing.T) {
	const src = "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: login\n    type: delay\n    duration: 1s\n  - id: alert\n    type: delay\n    needs:\n      - login\n    after_failure_of:\n      - login\n    duration: 1s\n"
	c := newCodec()

	f, diags := c.DecodeFlow("f.flow.yaml", "f", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	if err != nil || len(f.Nodes[1].AfterFailureOf) != 1 || f.Nodes[1].AfterFailureOf[0] != "login" || !bytes.Equal(out, []byte(src)) {
		t.Fatalf("err %v after_failure_of %v, output:\n%s", err, f.Nodes[1].AfterFailureOf, out)
	}
}

func TestNode_Given_an_unlinked_step_When_saved_and_loaded_Then_it_round_trips(t *testing.T) {
	const src = "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: a\n    type: delay\n    duration: 1s\n  - id: b\n    type: delay\n    unlinked: true\n    duration: 1s\n"
	c := newCodec()

	f, diags := c.DecodeFlow("f.flow.yaml", "f", []byte(src))
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFlow(f)

	if err != nil || !f.Nodes[1].Unlinked || !bytes.Equal(out, []byte(src)) {
		t.Fatalf("err %v unlinked %v, output:\n%s", err, f.Nodes[1].Unlinked, out)
	}
}
