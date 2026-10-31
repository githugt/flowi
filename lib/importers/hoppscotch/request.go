// SPDX-License-Identifier: Apache-2.0

package hoppscotch

import (
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/internal/loose"
	"github.com/githugt/flowi/lib/model"
)

type translator struct {
	importers.Translator
}

// request imports one request into folders.
func (im *importer) request(r map[string]any, folders []string, in inherited) {
	name := loose.Str(r["name"])
	path := strings.Join(append(slices.Clone(folders), fileSegment(name, "request")), "/")
	rep := importers.Item{Name: path, Source: im.at()}
	if loose.Str(r["endpoint"]) == "" {
		rep.Skip("the request has no URL")
		im.result.Items = append(im.result.Items, rep)
		return
	}
	path = im.taken.Unique(path)
	rep.Name = path
	t := &translator{Translator: importers.Translator{Item: &rep}}
	v := t.version(r, in)
	req := model.Request{Meta: model.Meta{Name: name}, Path: path, Versions: []model.RequestVersion{v}}
	im.result.Requests = append(im.result.Requests, req)
	reportScripts(&rep, r)
	im.result.Items = append(im.result.Items, rep)
}

func reportScripts(rep *importers.Item, r map[string]any) {
	for _, key := range []string{"preRequestScript", "testScript"} {
		if strings.TrimSpace(loose.Str(r[key])) != "" {
			rep.Unmapped("script "+key, "left out", "scripts are JavaScript; use hooks and extract or assert nodes instead")
		}
	}
}

// variableRef is Hoppscotch's <<name>> (and <<$dynamic>>) reference.
var variableRef = regexp.MustCompile(`<<\s*([^<>\s]+)\s*>>`)

// text rewrites <<name>> to {{name}} before the shared translation.
func (t *translator) text(s string) string {
	return t.Text(variableRef.ReplaceAllString(s, "{{$1}}"))
}

func (t *translator) unref(s string) string { return variableRef.ReplaceAllString(s, "{{$1}}") }

func (t *translator) credential(where, name, value string) string {
	value = t.unref(value)
	return t.Credential(where, name, value)
}

func (t *translator) version(r map[string]any, in inherited) model.RequestVersion {
	v := model.RequestVersion{Label: "1", Method: strings.ToUpper(orElse(loose.Str(r["method"]), "GET"))}
	raw, hadUserinfo := importers.StripUserinfo(loose.Str(r["endpoint"]))
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
	v.Query = append(v.Query, t.rows("query parameter", loose.List(r["params"]))...)
	v.Headers = append(v.Headers, t.rows("header", slices.Concat(in.headers, loose.List(r["headers"])))...)
	t.auth(t.effectiveAuth(r, in), &v)
	v.Body = t.body(loose.Map(r["body"]), &v)
	t.ReportUnknown()
	return v
}

func orElse(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// rows are the active key/value rows (Hoppscotch marks the ones in use active).
func (t *translator) rows(what string, rows []map[string]any) []model.KeyValue {
	var out []model.KeyValue
	for _, p := range rows {
		key := loose.Str(p["key"])
		if key == "" || !isActive(p) {
			continue
		}
		out = append(out, model.KeyValue{Key: t.text(key), Value: t.credential(what+" "+key, key, loose.Str(p["value"]))})
	}
	return out
}

// isActive: a row without the field counts as in use, as in older exports.
func isActive(p map[string]any) bool {
	a, ok := p["active"]
	return !ok || loose.Flag(a)
}

// effectiveAuth is the request's own auth, or the enclosing folder's when it
// says "inherit" (or says nothing).
func (t *translator) effectiveAuth(r map[string]any, in inherited) map[string]any {
	a := loose.Map(r["auth"])
	if a == nil || loose.Str(a["authType"]) == "inherit" || loose.Str(a["authType"]) == "" {
		return in.auth
	}
	return a
}

func (t *translator) auth(a map[string]any, v *model.RequestVersion) {
	if a == nil || (a["authActive"] != nil && !loose.Flag(a["authActive"])) {
		return
	}
	switch typ := loose.Str(a["authType"]); typ {
	case "none":
	case "basic":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Basic(t.unref(loose.Str(a["username"])), t.unref(loose.Str(a["password"])))})
	case "bearer":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Bearer(t.unref(loose.Str(a["token"])))})
	case "api-key":
		t.apiKey(a, v)
	case "digest":
		v.Auth = t.AuthBlock(model.AuthDigest, "username", t.unref(loose.Str(a["username"])), "password", t.unref(loose.Str(a["password"])))
	case "aws-signature":
		v.Auth = t.AuthBlock(model.AuthAWSSigV4, "access_key_id", t.unref(loose.Str(a["accessKey"])), "secret_access_key", t.unref(loose.Str(a["secretKey"])),
			"session_token", t.unref(loose.Str(a["serviceToken"])), "region", t.unref(loose.Str(a["region"])), "service", t.unref(loose.Str(a["serviceName"])))
	case "hawk":
		v.Auth = t.AuthBlock(model.AuthHawk, "id", t.unref(loose.Str(a["authId"])), "key", t.unref(loose.Str(a["authKey"])),
			"algorithm", strings.ToLower(strings.ReplaceAll(loose.Str(a["algorithm"]), "-", "")), "ext", loose.Str(a["extraData"]))
	case "oauth-2":
		t.oauth2(loose.Map(a["grantTypeInfo"]), v)
	default:
		t.Item.Unmapped("auth type "+typ, "left out", "set the credentials up with headers or a pre-hook")
	}
}

func (t *translator) apiKey(a map[string]any, v *model.RequestVersion) {
	key := orElse(loose.Str(a["key"]), "api_key")
	out := t.APIKey(key, t.unref(loose.Str(a["value"])))
	if strings.EqualFold(loose.Str(a["addTo"]), "QUERY_PARAMS") {
		v.Query = append(v.Query, model.KeyValue{Key: key, Value: out})
		return
	}
	v.Headers = append(v.Headers, model.KeyValue{Key: key, Value: out})
}

func (t *translator) oauth2(g map[string]any, v *model.RequestVersion) {
	grant := strings.ToLower(loose.Str(g["grantType"]))
	switch grant {
	case "client_credentials", "password":
		v.Auth = t.AuthBlock(model.AuthOAuth2, "grant", grant, "token_url", t.unref(loose.Str(g["authEndpoint"])), "client_id", t.unref(loose.Str(g["clientID"])),
			"client_secret", t.unref(loose.Str(g["clientSecret"])), "scope", loose.Str(g["scopes"]),
			"username", t.unref(loose.Str(g["username"])), "password", t.unref(loose.Str(g["password"])))
	default:
		t.Item.Unmapped("OAuth 2.0 grant "+orElse(grant, "authorization_code"), "left out", "it needs a person to approve it in a browser; fetch the token another way and send it as a Bearer header")
	}
}

func (t *translator) body(b map[string]any, v *model.RequestVersion) model.Body {
	if b == nil || b["contentType"] == nil {
		return model.Body{}
	}
	mime := loose.Str(b["contentType"])
	switch mime {
	case "multipart/form-data":
		return t.multipart(b["body"])
	case "application/x-www-form-urlencoded":
		return model.Body{Form: t.formLines(loose.Str(b["body"]))}
	}
	text := loose.Str(b["body"])
	if strings.TrimSpace(text) == "" {
		return model.Body{}
	}
	if mime == "application/json" || strings.HasSuffix(mime, "+json") {
		return model.Body{JSON: t.Item.RedactBodyText(t.text(text), "the request body")}
	}
	if !slices.ContainsFunc(v.Headers, func(h model.KeyValue) bool { return strings.EqualFold(h.Key, "Content-Type") }) {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Content-Type", Value: mime})
	}
	return model.Body{Text: t.Item.RedactBodyText(t.text(text), "the request body")}
}

// formLines reads the urlencoded body Hoppscotch keeps as "key: value" lines,
// where a line starting with # is switched off.
func (t *translator) formLines(body string) []model.KeyValue {
	var out []model.KeyValue
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, val, _ := strings.Cut(line, ":")
		k = strings.TrimSpace(k)
		if k != "" {
			out = append(out, model.KeyValue{Key: t.text(k), Value: t.credential("form field "+k, k, strings.TrimSpace(val))})
		}
	}
	return out
}

func (t *translator) multipart(body any) model.Body {
	items, _ := body.([]any)
	var parts []model.Field
	for _, it := range items {
		p, _ := it.(map[string]any)
		key := loose.Str(p["key"])
		if key == "" || !isActive(p) {
			continue
		}
		if loose.Flag(p["isFile"]) {
			t.Item.Unmapped("file part "+key, "left out", "the file is on the machine that exported the collection")
			continue
		}
		parts = append(parts, model.Field{Key: t.text(key), Value: t.credential("form field "+key, key, loose.Str(p["value"]))})
	}
	return model.Body{Multipart: parts}
}
