// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func (f *fixture) namedAuth(c *model.Collection, name string, auth model.Auth) {
	c.Auths = append(c.Auths, model.NamedAuth{Meta: model.Meta{ID: f.ids.New(), Name: name}, Auth: auth, File: c.Root + "/auths/" + name + ".auth.yaml"})
}

func useAuth(c *model.Collection, auth model.Auth) {
	c.Requests[0].Versions[0].Auth = &auth
}

// Before: auth: {use: name} was reported as an unknown scheme "".
func TestCheckAuth_Given_a_version_uses_a_named_auth_the_collection_has_When_validated_Then_no_diagnostic(t *testing.T) {
	f := newFixture(t)
	c := f.collection("a")
	f.request(&c, "r", "1")
	f.namedAuth(&c, "key", model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "value", Value: "k"}}})
	useAuth(&c, model.Auth{Use: "key"})

	diags := Validate(Target{Collections: []model.Collection{c}})

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
}

// Before: nothing said that a request named an authentication that does not exist.
func TestCheckAuth_Given_a_version_uses_an_unknown_name_When_validated_Then_FLW_AUTH_002_at_the_auth_block(t *testing.T) {
	f := newFixture(t)
	c := f.collection("a")
	r := f.request(&c, "r", "1")
	useAuth(&c, model.Auth{Use: "gone"})

	diags := Validate(Target{Collections: []model.Collection{c}})

	if len(diags) != 1 || diags[0].Code != diag.CodeAuthUnknown || diags[0].File != r.File || diags[0].Path != "$.versions.1.auth" {
		t.Fatalf("diags %+v", diags)
	}
}

// Before: named authentications were not checked at all.
func TestCheckAuth_Given_bad_named_auths_or_a_use_with_parameters_When_validated_Then_FLW_AUTH_001_for_each(t *testing.T) {
	f := newFixture(t)
	c := f.collection("a")
	f.request(&c, "r", "1")
	f.namedAuth(&c, "key", model.Auth{Type: "magic"})
	f.namedAuth(&c, "key", model.Auth{Type: model.AuthDigest, Params: []model.KeyValue{{Key: "nope", Value: "x"}}})
	f.namedAuth(&c, "chain", model.Auth{Use: "key"})
	useAuth(&c, model.Auth{Use: "key", Type: model.AuthDigest})

	diags := Validate(Target{Collections: []model.Collection{c}})

	// an unknown scheme; an unknown parameter and a repeated name; a chain; a use with a type.
	if got := codesOf(diags); len(got) != 5 || slices.ContainsFunc(got, func(c diag.Code) bool { return c != diag.CodeAuthInvalid }) {
		t.Fatalf("diags %v", diags)
	}
}
