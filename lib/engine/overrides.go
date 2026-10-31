// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/template"
	"github.com/githugt/flowi/lib/vault"
)

// applyOverrides puts what pre-hook set nodes changed on top of the
// rendered request; overrides win over the request's own values.
func applyOverrides(b *built, m *nodes.MutableRequest) diag.List {
	if m == nil {
		return nil
	}
	m.Mu.Lock()
	defer m.Mu.Unlock()
	if m.Method != nil {
		b.request.Method, b.sent.Method = *m.Method, *m.Method
	}
	if m.URL != nil {
		b.request.URL, b.sent.URL = *m.URL, *m.URL
	}
	diags := applyQuery(b, m.Query)
	for _, name := range slices.Sorted(maps.Keys(m.Headers)) {
		setHeader(b, name, m.Headers[name])
	}
	applyCookies(b, m.Cookies)
	return append(diags, applyBody(b, m)...)
}

// applyNodeHeaders sets a request node's headers: over the version's own,
// under what pre-hooks set (applied after this).
func applyNodeHeaders(b *built, headers map[string]string) {
	for _, name := range slices.Sorted(maps.Keys(headers)) {
		setHeader(b, name, headers[name])
	}
}

func applyQuery(b *built, q map[string]string) diag.List {
	if len(q) == 0 {
		return nil
	}
	u, err := url.Parse(b.request.URL)
	if err != nil {
		return diag.List{setFailure("request.query", "the URL is not valid")}
	}
	values := u.Query()
	for k, v := range q {
		values.Set(k, v)
	}
	u.RawQuery = values.Encode()
	b.request.URL = u.String()
	if b.sent.URL != vault.Mask {
		b.sent.URL = b.request.URL // secrets in it are masked with the record
	}
	return nil
}

func setHeader(b *built, name, value string) {
	b.request.Header.Set(name, value)
	i := slices.IndexFunc(b.sent.Header, func(h Header) bool { return strings.EqualFold(h.Name, name) })
	if i >= 0 {
		b.sent.Header[i].Value = value
		return
	}
	b.sent.Header = append(b.sent.Header, Header{Name: name, Value: value})
}

func applyCookies(b *built, cookies map[string]string) {
	if len(cookies) == 0 {
		return
	}
	var pairs []string
	if existing := b.request.Header.Get("Cookie"); existing != "" {
		pairs = append(pairs, existing)
	}
	for _, name := range slices.Sorted(maps.Keys(cookies)) {
		pairs = append(pairs, name+"="+cookies[name])
	}
	setHeader(b, "Cookie", strings.Join(pairs, "; "))
}

// applyBody replaces the body, or sets dot paths inside a JSON body.
func applyBody(b *built, m *nodes.MutableRequest) diag.List {
	text, ctype := b.bodyText, "application/json"
	if m.Body != nil {
		if s, isText := (*m.Body).(string); isText {
			text, ctype = s, "text/plain; charset=utf-8"
		} else {
			text = string(mustJSON(*m.Body))
		}
	}
	if len(m.BodyPaths) > 0 {
		edited, err := setJSONPaths(text, m.BodyPaths)
		if err != nil {
			return diag.List{setFailure("request.body", "the body is not a JSON object")}
		}
		text = edited
	}
	if m.Body == nil && len(m.BodyPaths) == 0 {
		return nil
	}
	if ct := b.request.Header.Get("Content-Type"); ct != "" && m.Body == nil {
		ctype = ct
	}
	b.bodyText = text
	b.request.Body = httpx.BytesBody([]byte(text), ctype)
	b.sent.Body = &Body{Text: text, Size: int64(len(text)), ContentType: ctype}
	return nil
}

// setJSONPaths sets "a.b.c" paths in a JSON object, creating objects on
// the way.
func setJSONPaths(text string, paths map[string]any) (string, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return "", err
	}
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		parts := strings.Split(p, ".")
		cur := doc
		for _, key := range parts[:len(parts)-1] {
			next, ok := cur[key].(map[string]any)
			if !ok {
				next = map[string]any{}
				cur[key] = next
			}
			cur = next
		}
		cur[parts[len(parts)-1]] = paths[p]
	}
	return string(mustJSON(doc)), nil
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(`"` + template.String(v) + `"`) // values from set nodes are JSON-encodable; keep text if not
	}
	return data
}

func setFailure(target, detail string) diag.Diagnostic {
	return diag.New(diag.CodeRunSetTarget, diag.Location{}, map[string]string{"target": target, "detail": detail})
}
