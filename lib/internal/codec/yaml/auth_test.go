// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"bytes"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: there was no file for a named authentication.
func TestDecodeAuth_Given_the_canonical_fixture_When_decoded_and_encoded_Then_the_bytes_are_the_same(t *testing.T) {
	data := readFixture(t, "valid/api-key.auth.yaml")
	c := newCodec()

	a, diags := c.DecodeAuth("api-key.auth.yaml", data)

	if len(diags) != 0 || a.Name != "api-key" || a.Auth.Type != model.AuthAPIKey || len(a.Auth.Params) != 3 || a.Auth.Params[1].Value != "{{secret.api_key}}" {
		t.Fatalf("auth %+v diags %v", a, diags)
	}
	out, err := c.EncodeAuth(a)
	if err != nil || !bytes.Equal(out, data) {
		t.Fatalf("err %v\n%s", err, out)
	}
}

// Before: auth: {use: name} lost the name: it was read as a parameter called "use".
func TestDecodeRequest_Given_a_version_uses_a_named_auth_When_decoded_and_encoded_Then_use_is_kept(t *testing.T) {
	data := readFixture(t, "valid/named-auth.req.yaml")
	c := newCodec()

	r, diags := c.DecodeRequest("named-auth.req.yaml", "named-auth", data)

	if len(diags) != 0 || r.Versions[0].Auth.Use != "api-key" || r.Versions[0].Auth.Type != "" || len(r.Versions[0].Auth.Params) != 0 ||
		r.Versions[1].Auth.Use != "" || r.Versions[1].Auth.Type != model.AuthDigest {
		t.Fatalf("versions %+v diags %v", r.Versions, diags)
	}
	out, err := c.EncodeRequest(r)
	if err != nil || !bytes.Equal(out, data) {
		t.Fatalf("err %v\n%s", err, out)
	}
}
