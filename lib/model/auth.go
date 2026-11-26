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
	AuthHTTPSig   = "httpsig" // RFC 9421 HTTP Message Signatures
	AuthDPoP      = "dpop"    // RFC 9449 proof of possession
)

// Auth is a version's authentication scheme and its parameters, in file
// order. Every value is a template, so credentials come from the vault
// ({{secret.client_secret}}) and never sit in the file.
//
// Use names a named authentication of the collection instead: the version
// then carries no scheme or parameters of its own, and the named one is
// applied when the request is sent.
type Auth struct {
	Type   string
	Params []KeyValue
	Use    string
}

// NamedAuth is an authentication defined once in a collection
// (auths/<name>.auth.yaml) and used by request versions by name.
type NamedAuth struct {
	Meta
	Auth Auth
	File string
}

// NamedAuth returns the collection's named authentication called name.
func (c Collection) NamedAuth(name string) (NamedAuth, bool) {
	for _, a := range c.Auths {
		if a.Name == name {
			return a, true
		}
	}
	return NamedAuth{}, false
}

// AuthTypes lists the schemes in documentation order.
func AuthTypes() []string {
	return []string{AuthAPIKey, AuthDigest, AuthOAuth2, AuthOAuth1, AuthAWSSigV4, AuthJWT, AuthHawk, AuthMutualTLS, AuthNTLM, AuthHTTPSig, AuthDPoP}
}

// AsyncAuthTypes are the schemes of the auth block a message request of protocol p can use: the ones that add a
// credential to the connection (a header, a query parameter, a token) or present a client certificate before the
// connection opens. WebSocket and SSE open with an HTTP request, so they take the same block as HTTP minus the schemes
// that sign the request or answer a 401 (they have no request body to sign and no second try). Every other message
// protocol takes its credentials in its bindings (nil).
func AsyncAuthTypes(p string) []string {
	switch p {
	case ProtocolWS, ProtocolSSE:
		return []string{AuthAPIKey, AuthOAuth2, AuthJWT, AuthMutualTLS}
	}
	return nil
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
		return []string{"grant", "token_url", "client_id", "client_secret", "scope", "audience", "username", "password", "refresh_token", "client_auth", "prefix", "authorization_url", "device_url", "redirect_port", "pkce", "authorization_details"}
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
	case AuthHTTPSig:
		return []string{"algorithm", "key_id", "secret", "private_key", "components", "label", "include_alg", "expires_in", "nonce", "tag", "digest"}
	case AuthDPoP:
		return []string{"private_key", "algorithm", "access_token", "nonce"}
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
	case AuthHTTPSig:
		return []string{"secret", "private_key"}
	case AuthDPoP:
		return []string{"private_key", "access_token"}
	}
	return nil
}
