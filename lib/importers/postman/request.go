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

// translator turns one request's parts into Flow-I values.
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
	default:
		t.it.Unmapped("auth type "+a.Type, "left out", "set the credentials up with headers or a pre-hook")
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
		t.it.Unmapped("body mode "+b.Mode, "left out", "Flow-I has no equivalent")
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
