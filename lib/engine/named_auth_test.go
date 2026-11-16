// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func collectionWithAuth() model.Collection {
	return model.Collection{Auths: []model.NamedAuth{{Meta: model.Meta{Name: "partner-key"},
		Auth: model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "value", Value: "{{secret.key}}"}}}}}}
}

// Before: a version that named an authentication kept an auth block with no scheme, which signs nothing.
func TestWithNamedAuth_Given_a_version_uses_a_name_the_collection_has_When_resolved_Then_it_carries_that_auth(t *testing.T) {
	v := model.RequestVersion{Auth: &model.Auth{Use: "partner-key"}}

	got, diags := withNamedAuth(collectionWithAuth(), v)

	if len(diags) != 0 || got.Auth.Use != "" || got.Auth.Type != model.AuthAPIKey || got.Auth.Params[0].Value != "{{secret.key}}" {
		t.Fatalf("auth %+v diags %v", got.Auth, diags)
	}
	if v.Auth.Use != "partner-key" {
		t.Fatalf("the stored version was changed: %+v", v.Auth)
	}
}

// Before: a name the collection does not have was not noticed, and the request went out with no authentication.
func TestWithNamedAuth_Given_a_version_uses_an_unknown_name_When_resolved_Then_FLW_AUTH_002(t *testing.T) {
	v := model.RequestVersion{Auth: &model.Auth{Use: "gone"}}

	_, diags := withNamedAuth(collectionWithAuth(), v)

	if len(diags) != 1 || diags[0].Code != diag.CodeAuthUnknown || !diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
}

// Before: n/a (guards the inline form staying as it was).
func TestWithNamedAuth_Given_an_inline_auth_When_resolved_Then_it_is_left_alone(t *testing.T) {
	v := model.RequestVersion{Auth: &model.Auth{Type: model.AuthDigest}}

	got, diags := withNamedAuth(collectionWithAuth(), v)

	if len(diags) != 0 || got.Auth != v.Auth {
		t.Fatalf("auth %+v diags %v", got.Auth, diags)
	}
}
