// SPDX-License-Identifier: Apache-2.0

package insomnia

import (
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"
)

// request imports one request into folders.
func (im *importer) request(r map[string]any, folders []string) {
	name := str(r["name"])
	path := strings.Join(append(slices.Clone(folders), cmpOr(importers.FileName(name), "request")), "/")
	rep := importers.Item{Name: path, Source: im.at()}
	if str(r["url"]) == "" {
		rep.Skip("the request has no URL")
		im.result.Items = append(im.result.Items, rep)
		return
	}
	path = im.taken.Unique(path)
	rep.Name = path
	t := &translator{Translator: importers.Translator{Item: &rep}}
	v := t.version(r)
	im.result.Requests = append(im.result.Requests, model.Request{Meta: model.Meta{Name: name}, Path: path, Versions: []model.RequestVersion{v}})
	reportScripts(&rep, r)
	im.result.Items = append(im.result.Items, rep)
}

func reportScripts(rep *importers.Item, r map[string]any) {
	for _, key := range []string{"preRequestScript", "afterResponseScript"} {
		if strings.TrimSpace(str(r[key])) != "" {
			rep.Unmapped("script "+key, "left out", "scripts are JavaScript; use hooks and extract or assert nodes instead")
		}
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type translator struct {
	importers.Translator
}

// templateRef is Insomnia's environment reference, {{ _.name }}.
var templateRef = regexp.MustCompile(`\{\{\s*_\.([A-Za-z0-9_]+)\s*\}\}`)

// tag is a Nunjucks tag ({% … %}) or one of Insomnia's response/faker
// helpers written as an expression; neither has a Flowi equivalent.
var tag = regexp.MustCompile(`\{%.*?%\}`)

// text translates Insomnia's references to Flowi's and reports what it cannot.
func (t *translator) text(s string) string {
	for _, m := range tag.FindAllString(s, -1) {
		t.Item.Unmapped("template tag "+m, "kept as text", "Insomnia's tags have no Flowi equivalent; use a generator or an extract")
	}
	return t.Text(templateRef.ReplaceAllString(s, "{{$1}}"))
}

func (t *translator) credential(where, name, value string) string {
	if importers.SensitiveHeader(name) && value != "" && !importers.HasVar(value) {
		return t.Item.Redact(where, name)
	}
	return t.text(value)
}

func (t *translator) version(r map[string]any) model.RequestVersion {
	v := model.RequestVersion{Label: "1", Method: strings.ToUpper(cmpOr(str(r["method"]), "GET"))}
	raw, hadUserinfo := importers.StripUserinfo(str(r["url"]))
	if hadUserinfo {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Item.Redact("credentials in the URL", "Authorization")})
	}
	base, inline, _ := strings.Cut(raw, "?")
	v.URL = t.text(base)
	for _, part := range strings.Split(inline, "&") {
		if k, val, _ := strings.Cut(part, "="); k != "" {
			v.Query = append(v.Query, model.KeyValue{Key: t.text(k), Value: t.credential("query parameter "+k, k, val)})
		}
	}
	for _, p := range list(r["parameters"]) {
		if !flag(p["disabled"]) && str(p["name"]) != "" {
			v.Query = append(v.Query, model.KeyValue{Key: t.text(str(p["name"])), Value: t.credential("query parameter "+str(p["name"]), str(p["name"]), str(p["value"]))})
		}
	}
	for _, h := range list(r["headers"]) {
		if !flag(h["disabled"]) && str(h["name"]) != "" {
			v.Headers = append(v.Headers, model.KeyValue{Key: str(h["name"]), Value: t.credential("header "+str(h["name"]), str(h["name"]), str(h["value"]))})
		}
	}
	t.auth(asMap(r["authentication"]), &v)
	v.Body = t.body(asMap(r["body"]), &v)
	t.ReportUnknown()
	return v
}

func (t *translator) body(b map[string]any, v *model.RequestVersion) model.Body {
	if b == nil {
		return model.Body{}
	}
	mime, text := str(b["mimeType"]), str(b["text"])
	switch {
	case mime == "application/x-www-form-urlencoded":
		return model.Body{Form: t.formFields(list(b["params"]))}
	case strings.HasPrefix(mime, "multipart/form-data"):
		return model.Body{Multipart: t.parts(list(b["params"]))}
	case mime == "application/octet-stream" || str(b["fileName"]) != "":
		if file := str(b["fileName"]); file != "" {
			t.Item.Unmapped("file body "+file, "kept with its path", "the path is on the machine that exported the collection")
			return model.Body{File: importers.Literal(file)}
		}
		return model.Body{}
	case text == "":
		return model.Body{}
	case mime == "application/json" || strings.HasSuffix(mime, "+json") || mime == "application/graphql":
		return model.Body{JSON: t.Item.RedactBodyText(t.text(text), "the request body")}
	}
	if mime != "" && !slices.ContainsFunc(v.Headers, func(h model.KeyValue) bool { return strings.EqualFold(h.Key, "Content-Type") }) {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Content-Type", Value: mime})
	}
	return model.Body{Text: t.Item.RedactBodyText(t.text(text), "the request body")}
}

func (t *translator) formFields(params []map[string]any) []model.KeyValue {
	var out []model.KeyValue
	for _, p := range params {
		if !flag(p["disabled"]) && str(p["name"]) != "" {
			out = append(out, model.KeyValue{Key: t.text(str(p["name"])), Value: t.credential("form field "+str(p["name"]), str(p["name"]), str(p["value"]))})
		}
	}
	return out
}

func (t *translator) parts(params []map[string]any) []model.Field {
	var out []model.Field
	for _, p := range params {
		name := str(p["name"])
		if flag(p["disabled"]) || name == "" {
			continue
		}
		if str(p["type"]) == "file" {
			t.Item.Unmapped("file part "+name, "kept with its path", "the path is on the machine that exported the collection")
			out = append(out, model.Field{Key: name, Value: []model.Field{{Key: "file", Value: importers.Literal(str(p["fileName"]))}}})
			continue
		}
		out = append(out, model.Field{Key: name, Value: t.credential("form field "+name, name, str(p["value"]))})
	}
	return out
}

// auth maps Insomnia's authentication to a header or an auth block.
func (t *translator) auth(a map[string]any, v *model.RequestVersion) {
	if a == nil || flag(a["disabled"]) {
		return
	}
	switch typ := str(a["type"]); typ {
	case "", "none":
	case "bearer":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Bearer(t.unref(str(a["token"])))})
	case "basic":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Basic(t.unref(str(a["username"])), t.unref(str(a["password"])))})
	case "apikey":
		key := cmpOr(str(a["key"]), "api_key")
		out := t.APIKey(key, t.unref(str(a["value"])))
		if str(a["addTo"]) == "queryParams" {
			v.Query = append(v.Query, model.KeyValue{Key: key, Value: out})
		} else {
			v.Headers = append(v.Headers, model.KeyValue{Key: key, Value: out})
		}
	case "oauth2":
		t.oauth2(a, v)
	case "digest":
		v.Auth = t.AuthBlock(model.AuthDigest, "username", t.unref(str(a["username"])), "password", t.unref(str(a["password"])))
	case "hawk":
		v.Auth = t.AuthBlock(model.AuthHawk, "id", t.unref(str(a["id"])), "key", t.unref(str(a["key"])), "algorithm", strings.ToLower(strings.ReplaceAll(str(a["algorithm"]), "-", "")), "ext", str(a["ext"]))
	case "oauth1":
		v.Auth = t.AuthBlock(model.AuthOAuth1, "consumer_key", t.unref(str(a["consumerKey"])), "consumer_secret", t.unref(str(a["consumerSecret"])),
			"token", t.unref(str(a["tokenKey"])), "token_secret", t.unref(str(a["tokenSecret"])), "signature_method", str(a["signatureMethod"]),
			"realm", str(a["realm"]), "verifier", str(a["verifier"]), "callback", str(a["callback"]))
	case "iam":
		v.Auth = t.AuthBlock(model.AuthAWSSigV4, "access_key_id", t.unref(str(a["accessKeyId"])), "secret_access_key", t.unref(str(a["secretAccessKey"])),
			"session_token", t.unref(str(a["sessionToken"])), "region", str(a["region"]), "service", str(a["service"]))
	default:
		t.Item.Unmapped("auth type "+typ, "left out", "set the credentials up with headers or a pre-hook")
	}
}

// unref turns {{ _.name }} into {{name}} before the helpers look for a
// variable reference (so a token that is a reference is not taken for a literal).
func (t *translator) unref(s string) string { return templateRef.ReplaceAllString(s, "{{$1}}") }

func (t *translator) oauth2(a map[string]any, v *model.RequestVersion) {
	grant := cmpOr(str(a["grantType"]), "authorization_code")
	switch grant {
	case "client_credentials", "password":
		clientAuth := "basic"
		if flag(a["credentialsInBody"]) || str(a["credentialsInBody"]) == "true" {
			clientAuth = "body"
		}
		v.Auth = t.AuthBlock(model.AuthOAuth2, "grant", grant, "token_url", t.unref(str(a["accessTokenUrl"])), "client_id", t.unref(str(a["clientId"])),
			"client_secret", t.unref(str(a["clientSecret"])), "scope", str(a["scope"]), "audience", str(a["audience"]),
			"username", t.unref(str(a["username"])), "password", t.unref(str(a["password"])), "client_auth", clientAuth)
	case "authorization_code":
		pkce := "off"
		if flag(a["usePkce"]) {
			pkce = "on"
		}
		v.Auth = t.AuthBlock(model.AuthOAuth2, "grant", grant, "authorization_url", t.unref(str(a["authorizationUrl"])), "token_url", t.unref(str(a["accessTokenUrl"])),
			"client_id", t.unref(str(a["clientId"])), "client_secret", t.unref(str(a["clientSecret"])), "scope", str(a["scope"]), "pkce", pkce)
		if redirect := str(a["redirectUrl"]); redirect != "" {
			t.Item.Unmapped("OAuth 2.0 redirect address "+redirect, "not used", "Flowi redirects to http://127.0.0.1:<port>/callback; register that address with the provider and set redirect_port to its port")
		}
	default:
		t.Item.Unmapped("OAuth 2.0 grant "+grant, "left out", "Flowi has client credentials, password and authorization code; fetch the token another way and send it as a Bearer header")
	}
}
