// SPDX-License-Identifier: Apache-2.0

package thunderclient

import (
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/internal/loose"
	"github.com/githugt/flowi/lib/model"
)

type translator struct {
	importers.Translator
}

// request imports one request into folders.
func (im *importer) request(r map[string]any, folders []string) {
	name := loose.Str(r["name"])
	path := strings.Join(append(slices.Clone(folders), fileSegment(name, "request")), "/")
	rep := importers.Item{Name: path, Source: im.at()}
	if loose.Str(r["url"]) == "" {
		rep.Skip("the request has no URL")
		im.result.Items = append(im.result.Items, rep)
		return
	}
	path = im.taken.Unique(path)
	rep.Name = path
	t := &translator{Translator: importers.Translator{Item: &rep}}
	v := t.version(r)
	im.result.Requests = append(im.result.Requests, model.Request{Meta: model.Meta{Name: name}, Path: path, Versions: []model.RequestVersion{v}})
	im.result.Items = append(im.result.Items, rep)
}

func (t *translator) version(r map[string]any) model.RequestVersion {
	v := model.RequestVersion{Label: "1", Method: strings.ToUpper(orElse(loose.Str(r["method"]), "GET"))}
	raw, hadUserinfo := importers.StripUserinfo(loose.Str(r["url"]))
	if hadUserinfo {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Item.Redact("credentials in the URL", "Authorization")})
	}
	base, inline, _ := strings.Cut(raw, "?")
	v.URL = t.Text(base)
	for _, part := range strings.Split(inline, "&") {
		if k, val, _ := strings.Cut(part, "="); k != "" {
			v.Query = append(v.Query, model.KeyValue{Key: t.Text(k), Value: t.Credential("query parameter "+k, k, val)})
		}
	}
	v.Headers = append(v.Headers, t.pairs("header", r["headers"], false)...)
	v.Query = append(v.Query, t.queryParams(inline, r["params"])...)
	t.auth(loose.Map(r["auth"]), &v)
	v.Body = t.body(loose.Map(r["body"]), &v)
	v.Assert = t.tests(r["tests"])
	t.ReportUnknown()
	return v
}

func orElse(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// pairs reads enabled name/value rows (Thunder Client marks the off ones isDisabled).
func (t *translator) pairs(what string, rows any, form bool) []model.KeyValue {
	var out []model.KeyValue
	for _, p := range loose.List(rows) {
		name := loose.Str(p["name"])
		if name == "" || loose.Flag(p["isDisabled"]) {
			continue
		}
		out = append(out, model.KeyValue{Key: t.pairKey(name, form), Value: t.Credential(what+" "+name, name, loose.Str(p["value"]))})
	}
	return out
}

func (t *translator) pairKey(name string, form bool) string {
	if form {
		return t.Text(name)
	}
	return name
}

// queryParams are the params rows that are not path parameters and are not
// already in the URL's own query. A path parameter (isPath) fills :name.
func (t *translator) queryParams(inline string, rows any) []model.KeyValue {
	var out []model.KeyValue
	for _, p := range loose.List(rows) {
		name := loose.Str(p["name"])
		if name == "" || loose.Flag(p["isDisabled"]) || loose.Flag(p["isPath"]) || strings.Contains("&"+inline, "&"+name+"=") {
			continue
		}
		out = append(out, model.KeyValue{Key: t.Text(name), Value: t.Credential("query parameter "+name, name, loose.Str(p["value"]))})
	}
	return out
}

// auth maps Thunder Client's auth block ({type, <type>: {...}}).
func (t *translator) auth(a map[string]any, v *model.RequestVersion) {
	if a == nil {
		return
	}
	switch typ := loose.Str(a["type"]); typ {
	case "", "none":
	case "bearer":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Bearer(loose.Str(a["bearer"]))})
	case "basic":
		b := loose.Map(a["basic"])
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: t.Basic(loose.Str(b["username"]), loose.Str(b["password"]))})
	case "apikey":
		k := loose.Map(a["apikey"])
		key := orElse(loose.Str(k["key"]), "api_key")
		out := t.APIKey(key, loose.Str(k["value"]))
		if strings.EqualFold(loose.Str(k["addTo"]), "query") {
			v.Query = append(v.Query, model.KeyValue{Key: key, Value: out})
		} else {
			v.Headers = append(v.Headers, model.KeyValue{Key: key, Value: out})
		}
	case "oauth2":
		t.oauth2(loose.Map(a["oauth2"]), v)
	case "aws":
		x := loose.Map(a["aws"])
		v.Auth = t.AuthBlock(model.AuthAWSSigV4, "access_key_id", loose.Str(x["accessKeyId"]), "secret_access_key", loose.Str(x["secretAccessKey"]),
			"session_token", loose.Str(x["sessionToken"]), "region", loose.Str(x["region"]), "service", loose.Str(x["service"]))
	default:
		t.Item.Unmapped("auth type "+typ, "left out", "set the credentials up with headers or a pre-hook")
	}
}

func (t *translator) oauth2(o map[string]any, v *model.RequestVersion) {
	grant := orElse(loose.Str(o["grantType"]), "authorization_code")
	switch grant {
	case "client_credentials", "password":
		v.Auth = t.AuthBlock(model.AuthOAuth2, "grant", grant, "token_url", loose.Str(o["tokenUrl"]), "client_id", loose.Str(o["clientId"]),
			"client_secret", loose.Str(o["clientSecret"]), "scope", loose.Str(o["scope"]),
			"username", loose.Str(o["username"]), "password", loose.Str(o["password"]))
	default:
		t.Item.Unmapped("OAuth 2.0 grant "+grant, "left out", "it needs a person to approve it in a browser; fetch the token another way and send it as a Bearer header")
	}
}

func (t *translator) body(b map[string]any, v *model.RequestVersion) model.Body {
	if b == nil {
		return model.Body{}
	}
	raw := loose.Str(b["raw"])
	switch typ := loose.Str(b["type"]); typ {
	case "json":
		return model.Body{JSON: t.jsonText(raw)}
	case "text", "xml":
		return t.textBody(typ, raw, v)
	case "formencoded":
		return model.Body{Form: t.pairs("form field", b["form"], true)}
	case "formdata":
		return t.multipart(b)
	case "graphql":
		return t.graphql(loose.Map(b["graphql"]))
	case "binary":
		if file := loose.Str(b["binary"]); file != "" {
			t.Item.Unmapped("file body "+file, "kept with its path", "the path is on the machine that exported the collection")
			return model.Body{File: importers.Literal(file)}
		}
	}
	return model.Body{}
}

func (t *translator) jsonText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return t.Item.RedactBodyText(t.Text(raw), "the request body")
}

func (t *translator) textBody(typ, raw string, v *model.RequestVersion) model.Body {
	if strings.TrimSpace(raw) == "" {
		return model.Body{}
	}
	mime := map[string]string{"text": "text/plain", "xml": "application/xml"}[typ]
	if !slices.ContainsFunc(v.Headers, func(h model.KeyValue) bool { return strings.EqualFold(h.Key, "Content-Type") }) {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Content-Type", Value: mime})
	}
	return model.Body{Text: t.Item.RedactBodyText(t.Text(raw), "the request body")}
}

func (t *translator) multipart(b map[string]any) model.Body {
	var parts []model.Field
	for _, p := range loose.List(b["form"]) {
		name := loose.Str(p["name"])
		if name != "" && !loose.Flag(p["isDisabled"]) {
			parts = append(parts, model.Field{Key: t.Text(name), Value: t.Credential("form field "+name, name, loose.Str(p["value"]))})
		}
	}
	for _, f := range loose.List(b["files"]) {
		name := loose.Str(f["name"])
		if name != "" {
			t.Item.Unmapped("file part "+name, "left out", "the file is on the machine that exported the collection")
		}
	}
	return model.Body{Multipart: parts}
}

// graphql sends Thunder Client's {query, variables} as a JSON body.
func (t *translator) graphql(g map[string]any) model.Body {
	query := loose.Str(g["query"])
	if query == "" {
		return model.Body{}
	}
	doc := `{"query":` + strconv.Quote(query)
	if vars := strings.TrimSpace(loose.Str(g["variables"])); vars != "" {
		doc += `,"variables":` + vars
	}
	return model.Body{JSON: t.Item.RedactBodyText(t.Text(doc+"}"), "the request body")}
}

// tests maps Thunder Client's response checks to Flowi's assert lines.
func (t *translator) tests(rows any) []string {
	var out []string
	for _, c := range loose.List(rows) {
		if line, ok := check(c); ok {
			out = append(out, line)
		} else {
			t.Item.Unmapped("test "+loose.Str(c["type"])+" "+loose.Str(c["action"]), "left out", "only comparisons of the status, a header, the time or a JSON value map to Flowi checks")
		}
	}
	return out
}

func check(c map[string]any) (string, bool) {
	subject, ok := subjectOf(loose.Str(c["type"]), loose.Str(c["custom"]))
	if !ok {
		return "", false
	}
	value := loose.Str(c["value"])
	switch action := loose.Str(c["action"]); action {
	case "equal":
		return subject + " == " + literal(value), true
	case "notequal":
		return subject + " != " + literal(value), true
	case ">", "<", ">=", "<=":
		return subject + " " + action + " " + literal(value), true
	}
	return "", false
}

func subjectOf(typ, custom string) (string, bool) {
	switch typ {
	case "res-code":
		return "status", true
	case "res-time":
		return "response.time", true
	case "res-header":
		return `response.headers[` + strconv.Quote(strings.ToLower(custom)) + `]`, true
	case "json-query":
		if path, ok := strings.CutPrefix(custom, "json."); ok && !strings.ContainsAny(path, " ()[]\"'") {
			return "response.body." + path, true
		}
	}
	return "", false
}

func literal(s string) string {
	if _, err := strconv.ParseFloat(s, 64); err == nil || s == "true" || s == "false" || s == "null" {
		return s
	}
	return strconv.Quote(s)
}
