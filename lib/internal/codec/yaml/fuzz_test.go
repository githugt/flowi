// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"bytes"
	"os"
	"testing"
)

// The invariants for every decoder: never panic, and for input without
// errors, encode → decode → encode is stable.

func FuzzDecodeFlow(f *testing.F) {
	seed, err := os.ReadFile("testdata/valid/user-creation.flow.yaml")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte("id: x\nnodes: [{id: a, type: log}]\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		c := newCodec()
		flow, diags := c.DecodeFlow("f.flow.yaml", "f", data)
		if diags.HasErrors() {
			return
		}
		first, err := c.EncodeFlow(flow)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		again, diags := c.DecodeFlow("f.flow.yaml", "f", first)
		if diags.HasErrors() {
			t.Fatalf("re-decode of encoded flow failed: %+v\n%s", diags, first)
		}
		second, _ := c.EncodeFlow(again)
		if !bytes.Equal(first, second) {
			t.Fatalf("unstable encoding:\n%s\n---\n%s", first, second)
		}
	})
}

func FuzzDecodeRequest(f *testing.F) {
	seed, err := os.ReadFile("testdata/valid/login.req.yaml")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Fuzz(func(t *testing.T, data []byte) {
		c := newCodec()
		r, diags := c.DecodeRequest("r.req.yaml", "r", data)
		if diags.HasErrors() {
			return
		}
		first, err := c.EncodeRequest(r)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		again, diags := c.DecodeRequest("r.req.yaml", "r", first)
		if diags.HasErrors() {
			t.Fatalf("re-decode of encoded request failed: %+v\n%s", diags, first)
		}
		second, _ := c.EncodeRequest(again)
		if !bytes.Equal(first, second) {
			t.Fatalf("unstable encoding:\n%s\n---\n%s", first, second)
		}
	})
}
