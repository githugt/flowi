// SPDX-License-Identifier: Apache-2.0

package model

// The authentication schemes a request version can name in its auth block
// (beyond a plain Authorization header, which the Auth tab also writes).
const (
	AuthAPIKey    = "apikey"
	AuthDigest    = "digest"
	AuthOAuth2    = "oauth2"
	AuthOAuth1    = "oauth1"
	AuthAWSSigV4  = "aws_sigv4"
	AuthJWT       = "jwt"
	AuthHawk      = "hawk"
	AuthMutualTLS = "mtls"
	AuthNTLM      = "ntlm"
)

// Auth is a version's authentication scheme and its parameters, in file
// order. Every value is a template, so credentials come from the vault
// ({{secret.client_secret}}) and never sit in the file.
type Auth struct {
	Type   string
	Params []KeyValue
}

// AuthTypes lists the schemes in documentation order.
func AuthTypes() []string {
	return []string{AuthAPIKey, AuthDigest, AuthOAuth2, AuthOAuth1, AuthAWSSigV4, AuthJWT, AuthHawk, AuthMutualTLS, AuthNTLM}
}

// AuthParams lists the parameter names a scheme understands, in the order a
// form shows them; nil for an unknown scheme.
func AuthParams(authType string) []string {
	switch authType {
	case AuthAPIKey:
		return []string{"name", "value", "in"}
	case AuthDigest:
		return []string{"username", "password"}
	case AuthOAuth2:
		return []string{"grant", "token_url", "client_id", "client_secret", "scope", "audience", "username", "password", "refresh_token", "client_auth", "prefix", "authorization_url", "device_url", "redirect_port", "pkce"}
	case AuthOAuth1:
		return []string{"consumer_key", "consumer_secret", "token", "token_secret", "signature_method", "realm", "verifier", "callback"}
	case AuthAWSSigV4:
		return []string{"access_key_id", "secret_access_key", "session_token", "region", "service"}
	case AuthJWT:
		return []string{"algorithm", "secret", "private_key", "claims", "expires_in", "header", "prefix"}
	case AuthHawk:
		return []string{"id", "key", "algorithm", "ext", "app", "dlg"}
	case AuthMutualTLS:
		return []string{"cert", "key"}
	case AuthNTLM:
		return []string{"username", "password", "domain", "workstation"}
	}
	return nil
}

// AuthSecretParams are the parameters of a scheme that hold credentials, which
// a description or a snippet must not write out.
func AuthSecretParams(authType string) []string {
	switch authType {
	case AuthAPIKey:
		return []string{"value"}
	case AuthDigest:
		return []string{"password"}
	case AuthOAuth2:
		return []string{"client_secret", "password", "refresh_token"}
	case AuthOAuth1:
		return []string{"consumer_secret", "token_secret"}
	case AuthAWSSigV4:
		return []string{"secret_access_key", "session_token"}
	case AuthJWT:
		return []string{"secret", "private_key"}
	case AuthHawk:
		return []string{"key"}
	case AuthNTLM:
		return []string{"password"}
	}
	return nil
}
