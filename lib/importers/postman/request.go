// SPDX-License-Identifier: Apache-2.0

package postman

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/model"
)

// translator turns one request's parts into Flowi values.
type translator struct {
	importers.Translator
	it       *importers.Item
	userinfo bool // the URL held user:password@, now stripped
}

func (t *translator) text(s string) string { return t.Text(s) }

func (t *translator) reportUnknown() { t.ReportUnknown() }

// pathVar is a Postman path variable segment (/users/:id).
var pathVar = regexp.MustCompile(`/:([A-Za-z_][A-Za-z0-9_-]*)`)

// url returns the URL without its query, and the query. A path variable
// with a value in the collection keeps that value; one without becomes
// {{name}}, filled by the caller (with).
func (t *translator) url(u urlParts) (string, []model.KeyValue) {
	raw, had := importers.StripUserinfo(u.raw)
	t.userinfo = had
	base, rawQuery, _ := strings.Cut(raw, "?")
	base = pathVar.ReplaceAllStringFunc(t.text(base), func(m string) string {
		name := m[2:]
		for _, v := range u.vars {
			if v.Key == name && v.Value != "" {
				return "/" + t.text(string(v.Value))
			}
		}
		return "/{{" + name + "}}"
	})
	var query []model.KeyValue
	if u.hasQuery {
		for _, q := range u.query {
			if !q.Disabled && q.Key != "" {
				query = append(query, model.KeyValue{Key: t.text(q.Key), Value: t.credential("query parameter "+q.Key, q.Key, string(q.Value))})
			}
		}
		return base, query
	}
	for _, part := range strings.Split(rawQuery, "&") {
		if k, v, _ := strings.Cut(part, "="); k != "" {
			query = append(query, model.KeyValue{Key: t.text(k), Value: t.credential("query parameter "+k, k, v)})
		}
	}
	return base, query
}

func (t *translator) credential(where, name, value string) string {
	return t.Credential(where, name, value)
}

func (t *translator) auth(a *auth, v *model.RequestVersion) {
	if a == nil {
		return
	}
	switch a.Type {
	case "", "noauth":
	case "bearer":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Bearer(a.param("token"))})
	case "basic":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Basic(a.param("username"), a.param("password"))})
	case "apikey":
		t.apiKey(a, v)
	case "digest", "hawk", "oauth1", "awsv4", "oauth2":
		t.authBlock(a, v)
	default:
		t.it.Unmapped("auth type "+a.Type, "left out", "set the credentials up with headers or a pre-hook")
	}
}

// authBlock maps the schemes Flowi signs or negotiates itself to an auth
// block. An OAuth 2.0 token already fetched, or a grant that needs a browser,
// has no block and is reported.
func (t *translator) authBlock(a *auth, v *model.RequestVersion) {
	switch a.Type {
	case "digest":
		v.Auth = t.AuthBlock(model.AuthDigest, "username", a.param("username"), "password", a.param("password"))
	case "hawk":
		v.Auth = t.AuthBlock(model.AuthHawk, "id", a.param("authId"), "key", a.param("authKey"),
			"algorithm", strings.ToLower(strings.ReplaceAll(a.param("algorithm"), "-", "")), "ext", a.param("extraData"), "app", a.param("app"), "dlg", a.param("delegation"))
	case "oauth1":
		v.Auth = t.AuthBlock(model.AuthOAuth1, "consumer_key", a.param("consumerKey"), "consumer_secret", a.param("consumerSecret"),
			"token", a.param("token"), "token_secret", a.param("tokenSecret"), "signature_method", a.param("signatureMethod"),
			"realm", a.param("realm"), "verifier", a.param("verifier"), "callback", a.param("callback"))
	case "awsv4":
		v.Auth = t.AuthBlock(model.AuthAWSSigV4, "access_key_id", a.param("accessKey"), "secret_access_key", a.param("secretKey"),
			"session_token", a.param("sessionToken"), "region", a.param("region"), "service", a.param("service"))
	case "oauth2":
		t.oauth2(a, v)
	}
}

func (t *translator) oauth2(a *auth, v *model.RequestVersion) {
	switch a.param("grant_type") {
	case "authorization_code", "authorization_code_with_pkce":
		t.authorizationCode(a, v)
		return
	}
	grant := map[string]string{"client_credentials": "client_credentials", "password_credentials": "password"}[a.param("grant_type")]
	if grant == "" {
		t.it.Unmapped("auth type oauth2", "left out", "a token fetched in Postman or a grant Flowi does not have (implicit) has no Flowi equivalent; use client credentials, password or authorization code, or send the token as a Bearer header")
		return
	}
	clientAuth := "basic"
	if a.param("client_authentication") == "body" {
		clientAuth = "body"
	}
	v.Auth = t.AuthBlock(model.AuthOAuth2, "grant", grant, "token_url", a.param("accessTokenUrl"), "client_id", a.param("clientId"),
		"client_secret", a.param("clientSecret"), "scope", a.param("scope"), "username", a.param("username"), "password", a.param("password"),
		"client_auth", clientAuth)
}

// authorizationCode maps Postman's authorization code grant. Flowi always
// redirects to the loopback address, so the redirect address Postman used is
// reported: the provider must be told about Flowi's.
func (t *translator) authorizationCode(a *auth, v *model.RequestVersion) {
	pkce := "off"
	if a.param("grant_type") == "authorization_code_with_pkce" {
		pkce = "on"
	}
	v.Auth = t.AuthBlock(model.AuthOAuth2, "grant", "authorization_code", "authorization_url", a.param("authUrl"), "token_url", a.param("accessTokenUrl"),
		"client_id", a.param("clientId"), "client_secret", a.param("clientSecret"), "scope", a.param("scope"), "pkce", pkce)
	if redirect := a.param("redirect_uri"); redirect != "" {
		t.it.Unmapped("OAuth 2.0 redirect address "+redirect, "not used", "Flowi redirects to http://127.0.0.1:<port>/callback; register that address with the provider and set redirect_port to its port")
	}
}

// apiKey adds the key as a header (default) or query entry. The value is
// always treated as a credential.
func (t *translator) apiKey(a *auth, v *model.RequestVersion) {
	key := cmpOr(a.param("key"), "api_key")
	out := t.APIKey(key, a.param("value"))
	if a.param("in") == "query" {
		v.Query = append(v.Query, model.KeyValue{Key: key, Value: out})
		return
	}
	v.Headers = append(v.Headers, model.KeyValue{Key: key, Value: out})
}

func (t *translator) body(b *body, v *model.RequestVersion) model.Body {
	if b == nil {
		return model.Body{}
	}
	switch b.Mode {
	case "", "none":
	case "raw":
		if b.Raw == "" {
			return model.Body{}
		}
		text := t.it.RedactBodyText(t.text(b.Raw), "the request body")
		if b.Options != nil && b.Options.Raw != nil && b.Options.Raw.Language == "json" {
			return model.Body{JSON: text}
		}
		return model.Body{Text: text}
	case "urlencoded":
		var form []model.KeyValue
		for _, f := range b.URLEncoded {
			if !f.Disabled && f.Key != "" {
				form = append(form, model.KeyValue{Key: t.text(f.Key), Value: t.credential("form field "+f.Key, f.Key, string(f.Value))})
			}
		}
		return model.Body{Form: form}
	case "formdata":
		return model.Body{Multipart: t.formData(b.FormData)}
	case "file":
		if b.File == nil || b.File.Src == "" {
			return model.Body{}
		}
		t.it.Unmapped("file body "+string(b.File.Src), "kept with its path", "the path is on the machine that exported the collection")
		return model.Body{File: importers.Literal(string(b.File.Src))}
	case "graphql":
		return t.graphQL(b, v)
	default:
		t.it.Unmapped("body mode "+b.Mode, "left out", "Flowi has no equivalent")
	}
	return model.Body{}
}

func (t *translator) formData(fields []kv) []model.Field {
	var parts []model.Field
	for _, f := range fields {
		if f.Disabled || f.Key == "" {
			continue
		}
		if f.Type == "file" {
			t.it.Unmapped("file part "+f.Key, "kept with its path", "the path is on the machine that exported the collection")
			parts = append(parts, model.Field{Key: f.Key, Value: []model.Field{{Key: "file", Value: importers.Literal(string(f.Src))}}})
			continue
		}
		parts = append(parts, model.Field{Key: f.Key, Value: t.credential("form field "+f.Key, f.Key, string(f.Value))})
	}
	return parts
}

// graphQL sends {"query", "variables"} as JSON, as Postman does.
func (t *translator) graphQL(b *body, v *model.RequestVersion) model.Body {
	if b.GraphQL == nil {
		return model.Body{}
	}
	var vars any = map[string]any{}
	if strings.TrimSpace(b.GraphQL.Variables) != "" {
		if err := json.Unmarshal([]byte(b.GraphQL.Variables), &vars); err != nil {
			vars = b.GraphQL.Variables
		}
	}
	data, err := json.Marshal(map[string]any{"query": b.GraphQL.Query, "variables": vars})
	if err != nil {
		t.it.Unmapped("GraphQL body", "left out", err.Error())
		return model.Body{}
	}
	if !slices.ContainsFunc(v.Headers, func(h model.KeyValue) bool { return strings.EqualFold(h.Key, "Content-Type") }) {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Content-Type", Value: "application/json"})
	}
	return model.Body{JSON: t.text(string(data))}
}
