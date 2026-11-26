// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// connected is a collection with one message request of the protocol, with the auth block given.
func connected(f *fixture, protocol, server string, auth *model.Auth) model.Collection {
	c := f.collection("c")
	c.Requests = append(c.Requests, model.Request{
		Meta: model.Meta{ID: f.ids.New(), Name: "r"}, Path: "r", File: c.Root + "/requests/r.req.yaml",
		Versions: []model.RequestVersion{{Label: "1", Auth: auth, Async: &model.Async{Protocol: protocol, Server: server, Receive: &model.AsyncReceive{Count: 1}}}},
	})
	return c
}

func apiKeyAuth(value string) *model.Auth {
	return &model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "name", Value: "X-Key"}, {Key: "value", Value: value}}}
}

// Before: any auth block on a message request was FLW-AUTH-001 ("a message request sets its credentials in its
// binding"), although WebSocket and SSE have no credential bindings at all.
func TestCheckAuth_Given_a_ws_or_sse_request_has_a_usable_auth_block_When_validated_Then_no_diagnostic(t *testing.T) {
	for _, protocol := range []string{"ws", "sse"} {
		for _, auth := range []*model.Auth{
			apiKeyAuth("{{secret.key}}"),
			{Type: model.AuthOAuth2, Params: []model.KeyValue{{Key: "grant", Value: "client_credentials"}, {Key: "token_url", Value: "https://idp/t"}}},
			{Type: model.AuthJWT, Params: []model.KeyValue{{Key: "secret", Value: "{{secret.jwt}}"}}},
			{Type: model.AuthMutualTLS, Params: []model.KeyValue{{Key: "cert", Value: "c.pem"}, {Key: "key", Value: "c.key"}}},
		} {
			f := newFixture(t)
			c := connected(f, protocol, "https://h/x", auth)

			diags := Validate(Target{Collections: []model.Collection{c}})

			for _, d := range diags {
				if d.Code == diag.CodeAuthInvalid {
					t.Errorf("%s %s: %v", protocol, auth.Type, d)
				}
			}
		}
	}
}

func TestCheckAuth_Given_a_ws_request_uses_a_named_authentication_When_validated_Then_it_is_checked_as_that_scheme(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := connected(f, "ws", "wss://h/x", &model.Auth{Use: "good"})
	f.namedAuth(&c, "good", *apiKeyAuth("{{secret.key}}"))
	f.namedAuth(&c, "signs", model.Auth{Type: model.AuthDigest})
	bad := connected(f, "sse", "https://h/x", &model.Auth{Use: "signs"})
	c.Requests = append(c.Requests, bad.Requests...)
	c.Requests[1].Path, c.Requests[1].File = "r2", c.Root+"/requests/r2.req.yaml"

	// Act
	diags := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	// Only the digest scheme of the second request is refused: it answers a 401, and a handshake has no second try.
	var invalid diag.List
	for _, d := range diags {
		if d.Code == diag.CodeAuthInvalid {
			invalid = append(invalid, d)
		}
	}
	if len(invalid) != 1 || invalid[0].File != c.Requests[1].File {
		t.Fatalf("diags %v", diags)
	}
}

func TestCheckAuth_Given_a_ws_or_sse_request_has_a_scheme_that_signs_or_challenges_When_validated_Then_FLW_AUTH_001_says_which_it_can_use(t *testing.T) {
	for _, typ := range []string{model.AuthDigest, model.AuthOAuth1, model.AuthAWSSigV4, model.AuthHawk, model.AuthNTLM, model.AuthHTTPSig, model.AuthDPoP} {
		f := newFixture(t)
		c := connected(f, "ws", "wss://h/x", &model.Auth{Type: typ})

		diags := Validate(Target{Collections: []model.Collection{c}})

		d := only(t, diags, diag.CodeAuthInvalid)
		if d.Path != "$.versions.1.auth" || d.Args["detail"] == "" {
			t.Errorf("%s: %+v", typ, d)
		}
	}
}

func TestCheckAuth_Given_a_message_protocol_with_credential_bindings_has_an_auth_block_When_validated_Then_it_is_still_refused(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := connected(f, "mqtt", "mqtt://h", apiKeyAuth("{{secret.key}}"))

	// Act
	diags := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	d := only(t, diags, diag.CodeAuthInvalid)
	if d.Path != "$.versions.1.auth" {
		t.Fatalf("%+v", d)
	}
}

func TestCheckAuth_Given_a_ws_auth_block_has_an_unknown_parameter_When_validated_Then_FLW_AUTH_001(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := connected(f, "sse", "https://h/x", &model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "nope", Value: "x"}}})

	// Act
	diags := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	only(t, diags, diag.CodeAuthInvalid)
}

// Before: the check read only the url field, which a WebSocket or SSE request does not have, so a token sent to
// ws://stream.example.com went unremarked.
func TestLint_Given_a_ws_or_sse_request_sends_a_credential_in_clear_text_to_a_remote_host_Then_FLW_LINT_005_names_the_server(t *testing.T) {
	tests := []struct {
		name, protocol, server string
		auth                   *model.Auth
		headers                []model.KeyValue
		want                   bool
	}{
		{name: "ws apikey", protocol: "ws", server: "ws://stream.example.com", auth: apiKeyAuth("{{secret.key}}"), want: true},
		{name: "sse apikey", protocol: "sse", server: "http://stream.example.com/events", auth: apiKeyAuth("{{secret.key}}"), want: true},
		{name: "ws bearer header", protocol: "ws", server: "ws://stream.example.com", headers: []model.KeyValue{{Key: "Authorization", Value: "Bearer {{secret.t}}"}}, want: true},
		{name: "ws mtls only", protocol: "ws", server: "ws://stream.example.com", auth: &model.Auth{Type: model.AuthMutualTLS}},
		{name: "wss", protocol: "ws", server: "wss://stream.example.com", auth: apiKeyAuth("{{secret.key}}")},
		{name: "localhost", protocol: "sse", server: "http://localhost:9000", auth: apiKeyAuth("{{secret.key}}")},
		{name: "templated host", protocol: "ws", server: "ws://{{env.host}}", auth: apiKeyAuth("{{secret.key}}")},
		{name: "no credential", protocol: "ws", server: "ws://stream.example.com"},
		{name: "mqtt is not read", protocol: "mqtt", server: "mqtt://stream.example.com", headers: []model.KeyValue{{Key: "Authorization", Value: "Bearer {{secret.t}}"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := connected(f, tt.protocol, tt.server, nil)
			c.Requests[0].Versions[0].Headers = tt.headers
			if tt.auth != nil && tt.protocol != "mqtt" {
				c.Requests[0].Versions[0].Auth = tt.auth
			}

			var found *diag.Diagnostic
			for _, d := range Validate(Target{Collections: []model.Collection{c}}) {
				if d.Code == diag.CodeLintPlainKey {
					found = &d
				}
			}

			if (found != nil) != tt.want {
				t.Fatalf("warned %v, want %v", found != nil, tt.want)
			}
			if found != nil && found.Path != "$.versions.1.async.server" {
				t.Errorf("path %q", found.Path)
			}
		})
	}
}

func TestLint_Given_an_auth_block_sends_a_credential_over_plain_http_Then_FLW_LINT_005_names_the_url(t *testing.T) {
	// Before: only headers, query and user:password were counted, so an apikey in an auth block went unremarked.
	f := newFixture(t)
	c := f.collection("c")
	f.request(&c, "r", "1")
	c.Requests[0].Versions[0].URL = "http://api.example.com/x"
	c.Requests[0].Versions[0].Auth = apiKeyAuth("{{secret.key}}")

	diags := Validate(Target{Collections: []model.Collection{c}})

	if d := only(t, diags, diag.CodeLintPlainKey); d.Path != "$.versions.1.url" {
		t.Fatalf("%+v", d)
	}
}

func TestLint_Given_a_credential_is_typed_into_an_auth_block_Then_FLW_LINT_004_warns_unless_it_is_a_template(t *testing.T) {
	// Before: an API key typed into the Auth tab reached git through the file with no warning, where the same key typed
	// as a header was reported.
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"typed", "abc123", true},
		{"secret reference", "{{secret.key}}", false},
		{"variable reference", "{{env.key}}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := connected(f, "ws", "wss://h/x", apiKeyAuth(tt.value))
			f.namedAuth(&c, "shared", *apiKeyAuth(tt.value))

			var paths []string
			for _, d := range Validate(Target{Collections: []model.Collection{c}}) {
				if d.Code == diag.CodeLintLiteralKey {
					paths = append(paths, d.Path)
				}
			}

			if tt.want && len(paths) != 2 {
				t.Fatalf("paths %v, want the request's and the named authentication's", paths)
			}
			if !tt.want && len(paths) != 0 {
				t.Fatalf("paths %v", paths)
			}
		})
	}
}
