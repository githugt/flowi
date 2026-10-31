// SPDX-License-Identifier: Apache-2.0

package bruno

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"
)

// requestBuilder turns one .bru file's blocks into a request.
type requestBuilder struct {
	im        *importer
	file      string
	it        *importers.Item
	blocks    []block
	inherited settings
	t         *importers.Translator
}

// at is where an entry is.
func (b requestBuilder) at(e entry) diag.Location { return b.im.loc(b.file, e.line, e.col) }

func methods() []string {
	return []string{"get", "post", "put", "patch", "delete", "options", "head", "connect", "trace"}
}

// build returns the request, or false when the item was skipped.
func (b requestBuilder) build() (model.Request, bool) {
	meta, _ := find(b.blocks, "meta")
	if kind := meta.value("type"); kind != "" && kind != "http" && kind != "graphql" {
		b.it.Skip(kind + " requests are not supported")
		return model.Request{}, false
	}
	i := slices.IndexFunc(b.blocks, func(x block) bool { return slices.Contains(methods(), x.name) })
	if i < 0 {
		b.it.Skip("the file has no method block such as get { url: … }")
		return model.Request{}, false
	}
	call := b.blocks[i]
	if call.value("url") == "" {
		b.it.Skip("the request has no URL")
		return model.Request{}, false
	}
	v := model.RequestVersion{Label: "1", Method: strings.ToUpper(call.name)}
	rawURL, hadUserinfo := importers.StripUserinfo(call.value("url"))
	v.URL, v.Query = b.url(rawURL)
	if hadUserinfo {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: b.it.Redact("credentials in the URL", "Authorization")})
	}
	b.headers(&v)
	b.auth(call.value("auth"), &v)
	v.Body = b.body(call.value("body"), &v)
	v.Assert = b.asserts()
	v.Extract = b.extracts()
	b.report()
	req := model.Request{Meta: model.Meta{Name: cmpOr(meta.value("name"), b.it.Name)}, Versions: []model.RequestVersion{v}}
	if docs, ok := find(b.blocks, "docs"); ok {
		req.Description = strings.TrimSpace(docs.text)
	}
	return req, true
}

var pathParam = regexp.MustCompile(`/:([A-Za-z_][A-Za-z0-9_-]*)`)

// url returns the URL without its query and the query, preferring the
// params blocks Bruno keeps in step with the URL.
func (b requestBuilder) url(raw string) (string, []model.KeyValue) {
	base, rawQuery, _ := strings.Cut(raw, "?")
	pathParams, _ := find(b.blocks, "params:path")
	base = pathParam.ReplaceAllStringFunc(b.t.Text(base), func(m string) string {
		if v := pathParams.value(m[2:]); v != "" {
			return "/" + b.t.Text(v)
		}
		return "/{{" + m[2:] + "}}"
	})
	var query []model.KeyValue
	if q, ok := find(b.blocks, "params:query"); ok {
		for _, e := range q.enabled() {
			query = append(query, model.KeyValue{Key: b.t.Text(e.key), Value: b.t.Credential("query parameter "+e.key, e.key, e.value)})
		}
		return base, query
	}
	for _, part := range strings.Split(rawQuery, "&") {
		if k, v, _ := strings.Cut(part, "="); k != "" {
			query = append(query, model.KeyValue{Key: b.t.Text(k), Value: b.t.Credential("query parameter "+k, k, v)})
		}
	}
	return base, query
}

// headers are the collection's, then the folders', then the request's.
func (b requestBuilder) headers(v *model.RequestVersion) {
	all := slices.Clone(b.inherited.headers)
	if h, ok := find(b.blocks, "headers"); ok {
		all = append(all, h.enabled()...)
	}
	for _, e := range all {
		v.Headers = append(v.Headers, model.KeyValue{Key: e.key, Value: b.t.Credential("header "+e.key, e.key, e.value)})
	}
}

func (b requestBuilder) auth(mode string, v *model.RequestVersion) {
	a := authBlock(b.blocks, mode)
	if mode == "inherit" || mode == "" {
		mode, a = b.inherited.mode, b.inherited.auth
	}
	switch mode {
	case "", "none":
	case "bearer":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: b.t.Bearer(param(a, "token"))})
	case "basic":
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: b.t.Basic(param(a, "username"), param(a, "password"))})
	case "apikey":
		key := cmpOr(param(a, "key"), "api_key")
		value := b.t.APIKey(key, param(a, "value"))
		if param(a, "placement") == "queryparams" {
			v.Query = append(v.Query, model.KeyValue{Key: key, Value: value})
			return
		}
		v.Headers = append(v.Headers, model.KeyValue{Key: key, Value: value})
	default:
		b.it.Unmapped("auth mode "+mode, "left out", "set the credentials up with headers or a pre-hook")
	}
}

func param(a *block, key string) string {
	if a == nil {
		return ""
	}
	return a.value(key)
}

func (b requestBuilder) body(mode string, v *model.RequestVersion) model.Body {
	text := func(name string) string {
		blk, _ := find(b.blocks, name)
		return strings.TrimSpace(blk.text)
	}
	switch mode {
	case "", "none":
	case "json":
		return model.Body{JSON: b.it.RedactBodyText(b.t.Text(text("body:json")), "the request body")}
	case "text", "xml":
		return model.Body{Text: b.it.RedactBodyText(b.t.Text(text("body:"+mode)), "the request body")}
	case "formUrlEncoded":
		blk, _ := find(b.blocks, "body:form-urlencoded")
		var form []model.KeyValue
		for _, e := range blk.enabled() {
			form = append(form, model.KeyValue{Key: e.key, Value: b.t.Credential("form field "+e.key, e.key, e.value)})
		}
		return model.Body{Form: form}
	case "multipartForm":
		return model.Body{Multipart: b.multipart()}
	case "graphql":
		return b.graphQL(text("body:graphql"), text("body:graphql:vars"), v)
	default:
		b.it.Unmapped("body mode "+mode, "left out", "Flowi has no equivalent")
	}
	return model.Body{}
}

var filePart = regexp.MustCompile(`^@file\((.*)\)$`)

func (b requestBuilder) multipart() []model.Field {
	blk, _ := find(b.blocks, "body:multipart-form")
	var parts []model.Field
	for _, e := range blk.enabled() {
		if m := filePart.FindStringSubmatch(e.value); m != nil {
			b.it.Unmapped("file part "+e.key, "kept with its path", "the path is on the machine that exported the collection")
			parts = append(parts, model.Field{Key: e.key, Value: []model.Field{{Key: "file", Value: importers.Literal(m[1])}}})
			continue
		}
		parts = append(parts, model.Field{Key: e.key, Value: b.t.Credential("form field "+e.key, e.key, e.value)})
	}
	return parts
}

func (b requestBuilder) graphQL(query, vars string, v *model.RequestVersion) model.Body {
	var parsed any = map[string]any{}
	if vars != "" {
		if err := json.Unmarshal([]byte(vars), &parsed); err != nil {
			parsed = vars
		}
	}
	data, err := json.Marshal(map[string]any{"query": query, "variables": parsed})
	if err != nil {
		b.it.Unmapped("GraphQL body", "left out", err.Error())
		return model.Body{}
	}
	if !slices.ContainsFunc(v.Headers, func(h model.KeyValue) bool { return strings.EqualFold(h.Key, "Content-Type") }) {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Content-Type", Value: "application/json"})
	}
	return model.Body{JSON: b.t.Text(string(data))}
}

// asserts maps Bruno's assert block ("res.status: eq 200") to checks.
func (b requestBuilder) asserts() []string {
	blk, ok := find(b.blocks, "assert")
	if !ok {
		return nil
	}
	var out []string
	for _, e := range blk.enabled() {
		if check, ok := assertion(e.key, e.value); ok {
			out = append(out, check)
		} else {
			b.it.UnmappedAt(b.at(e), "assert "+e.key+": "+e.value, "left out", "only comparisons and isDefined/isUndefined/isNull map to Flowi checks")
		}
	}
	return out
}

func assertion(lhs, rhs string) (string, bool) {
	subject, ok := responseRef(lhs)
	if !ok {
		return "", false
	}
	op, arg, _ := strings.Cut(strings.TrimSpace(rhs), " ")
	unary := map[string]string{"isDefined": " != nil", "isUndefined": " == nil", "isNull": " == nil"}
	if suffix, ok := unary[op]; ok && arg == "" {
		return subject + suffix, true
	}
	binary := map[string]string{"eq": "==", "neq": "!=", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}
	sym, ok := binary[op]
	value, okValue := literal(strings.TrimSpace(arg))
	if !ok || !okValue {
		return "", false
	}
	return subject + " " + sym + " " + value, true
}

var bodyPath = regexp.MustCompile(`^(\.[A-Za-z_][A-Za-z0-9_]*|\[[0-9]+\])*$`)

// responseRef maps res.status, res.body…, res.headers.x and
// res.responseTime to Flowi's response scope.
func responseRef(s string) (string, bool) {
	switch {
	case s == "res.status":
		return "status", true
	case s == "res.responseTime":
		return "response.time", true
	case strings.HasPrefix(s, "res.headers."):
		return `response.headers[` + strconv.Quote(strings.ToLower(strings.TrimPrefix(s, "res.headers."))) + `]`, true
	case strings.HasPrefix(s, "res.body") && bodyPath.MatchString(strings.TrimPrefix(s, "res.body")):
		return "response.body" + strings.TrimPrefix(s, "res.body"), true
	}
	return "", false
}

// literal accepts numbers, true/false/null and quoted strings as they
// are, quotes bare words, and refuses variables.
func literal(s string) (string, bool) {
	switch {
	case s == "" || strings.Contains(s, "{{"):
		return "", false
	case s == "true" || s == "false" || s == "null":
		return s, true
	case strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) && len(s) > 1, strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") && len(s) > 1:
		return s, true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return s, true
	}
	return strconv.Quote(s), true
}

// extracts maps vars:post-response ("token: res.body.token") to the
// request's extract block.
func (b requestBuilder) extracts() []model.Extraction {
	blk, ok := find(b.blocks, "vars:post-response")
	if !ok {
		return nil
	}
	var out []model.Extraction
	for _, e := range blk.enabled() {
		if x, ok := extraction(e.key, e.value); ok {
			out = append(out, x)
		} else {
			b.it.UnmappedAt(b.at(e), "post-response var "+e.key, "left out", "only res.body paths, res.headers.x and res.status map to extracts")
		}
	}
	return out
}

func extraction(name, expr string) (model.Extraction, bool) {
	switch {
	case expr == "res.status":
		return model.Extraction{Name: name, From: "status"}, true
	case strings.HasPrefix(expr, "res.headers."):
		return model.Extraction{Name: name, From: "header", Header: strings.TrimPrefix(expr, "res.headers.")}, true
	case strings.HasPrefix(expr, "res.body") && bodyPath.MatchString(strings.TrimPrefix(expr, "res.body")):
		return model.Extraction{Name: name, From: "body", Path: "$" + strings.TrimPrefix(expr, "res.body")}, true
	}
	return model.Extraction{}, false
}

// report adds what this request cannot carry: scripts, tests and
// pre-request vars, and dynamic variables without a generator.
func (b requestBuilder) report() {
	reportScripts(b.it, "request", b.blocks)
	if blk, ok := find(b.blocks, "vars:pre-request"); ok && len(blk.enabled()) > 0 {
		b.it.Unmapped("pre-request vars", "left out", "pass values with with, or set them in a flow")
	}
	b.t.ReportUnknown()
}
