// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/http/httpguts"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
	"github.com/githugt/flowi-lib/vault"
)

// buildInput is everything needed to turn a request version into an HTTP
// request.
type buildInput struct {
	version model.RequestVersion
	scope   template.Scope
	empty   template.EmptyPolicy
	label   string // request path, named in diagnostics
	root    string // collection root, for file bodies
	// secretVars are the environment's secret variable names, for the
	// "use secret.x" hint.
	secretVars map[string]bool
	// missingSecret reports a declared secret that has no value; those are
	// reported by the caller (FLW-SEC-001), all together.
	missingSecret func(name string) bool
	// secret, when set, registers rendered values that read a secret, so
	// they are masked wherever they appear (a server echoing a header).
	secret func(v any)
	// memo, when set, makes generators return the same values each time
	// the request is rendered, so what pre-hooks read (a body they sign)
	// is what is sent.
	memo *genMemo
	// verbatim keeps a field's template text when it cannot be rendered
	// (an unknown variable or generator) instead of reporting an error: for
	// previews, which show what is known and leave the rest as written.
	verbatim bool
	// defaults are headers added when the request does not name them (even
	// with an empty value, which is how a request opts out of one).
	defaults []model.KeyValue
}

// genMemo remembers generator results per field and call position.
type genMemo struct {
	gen    template.GeneratorFunc
	values map[string][]any
}

func newGenMemo(gen template.GeneratorFunc) *genMemo {
	return &genMemo{gen: gen, values: map[string][]any{}}
}

// forField returns a generator for one render of one field: its n-th call
// returns the n-th value generated for that field in any earlier render.
func (m *genMemo) forField(field string) template.GeneratorFunc {
	n := 0
	return func(name string, args []any) (any, error) {
		defer func() { n++ }()
		if prev := m.values[field]; n < len(prev) {
			return prev[n], nil
		}
		v, err := m.gen(name, args)
		if err == nil {
			m.values[field] = append(m.values[field], v)
		}
		return v, err
	}
}

// built is a request ready to send, and how it is recorded.
type built struct {
	request httpx.Request
	// sent is the request as recorded: fields that read a secret are
	// already replaced by vault.Mask; substring masking happens when the
	// record is made.
	sent    SentRequest
	tainted map[string]bool // "url", "query:<k>", "header:<Name>", "body"
	// bodyText is the unmasked text of an in-memory body (JSON, text,
	// form), so pre-hook overrides can edit it.
	bodyText string
	// missingSecrets are declared secrets this render needed but which have
	// no value; the caller reports them (FLW-SEC-001).
	missingSecrets []string
	// parts are a multipart body's rendered fields, kept for previews.
	parts []httpx.Part
}

// build renders a request version. Diagnostics are user errors (undefined
// variables, bad headers, files outside the collection); with any of them
// the request must not be sent.
func build(in buildInput) (built, diag.List) {
	b := &builder{in: in, tainted: map[string]bool{}}
	requestURL, shownURL := b.url()
	header, shown := b.headers()
	body, sentBody := b.body()
	if sentBody != nil && sentBody.ContentType != "" && header.Get("Content-Type") == "" {
		header.Set("Content-Type", sentBody.ContentType)
		shown = append(shown, Header{Name: "Content-Type", Value: sentBody.ContentType})
	}
	shown = b.addDefaultHeaders(header, shown)
	req := httpx.Request{Method: in.version.Method, URL: requestURL, Header: header, Body: body}
	sent := SentRequest{Method: in.version.Method, URL: shownURL, Header: shown, Body: sentBody}
	return built{request: req, sent: sent, tainted: b.tainted, bodyText: b.bodyText, missingSecrets: b.missingSecrets, parts: b.parts}, b.diags
}

type builder struct {
	in             buildInput
	tainted        map[string]bool
	diags          diag.List
	bodyText       string
	missingSecrets []string
	parts          []httpx.Part
}

func (b *builder) derived(r template.Result) {
	if r.Tainted && b.in.secret != nil {
		b.in.secret(r.Value)
	}
}

// scope is the template scope for rendering one field.
func (b *builder) scope(field string) template.Scope {
	s := b.in.scope
	if b.in.memo != nil {
		s.Gen = b.in.memo.forField(field)
	}
	return s
}

func (b *builder) field(desc string) string { return desc + " of " + b.in.label }

// eval renders one field's template; ok is false when it failed (and a
// diagnostic was added).
func (b *builder) eval(desc, key, src string) (any, bool) {
	tpl, err := template.Parse(src)
	if err != nil {
		return b.unrendered(desc, src, err)
	}
	r, err := tpl.Eval(b.scope(desc))
	if err != nil {
		return b.unrendered(desc, src, err)
	}
	b.tainted[key] = b.tainted[key] || r.Tainted
	b.derived(r)
	return r.Value, true
}

// unrendered handles a field that failed to render: an error, or in
// verbatim mode (previews) the template text as written.
func (b *builder) unrendered(desc, src string, err error) (any, bool) {
	if b.in.verbatim {
		return src, true
	}
	b.templateError(desc, err)
	return nil, false
}

func (b *builder) text(desc, key, src string) (string, bool) {
	v, ok := b.eval(desc, key, src)
	if !ok {
		return "", false
	}
	return template.String(v), true
}

// url renders the URL and query; the second result is the recorded form.
func (b *builder) url() (string, string) {
	base, _ := b.text("URL", "url", b.in.version.URL)
	var sendable, shown []string
	for _, q := range b.in.version.Query {
		key := "query:" + q.Key
		v, ok := b.eval(b.queryDesc(q.Key), key, q.Value)
		if !ok {
			continue
		}
		value, keep, err := template.ApplyEmpty(v, b.in.empty)
		if err != nil {
			b.emptyError(b.queryDesc(q.Key))
		}
		if !keep {
			continue
		}
		pair := url.QueryEscape(q.Key) + "="
		sendable = append(sendable, pair+url.QueryEscape(value))
		shown = append(shown, pair+maskIf(b.tainted[key], url.QueryEscape(value)))
	}
	if b.tainted["url"] {
		return joinQuery(base, sendable), vault.Mask
	}
	return joinQuery(base, sendable), joinQuery(base, shown)
}

func (b *builder) queryDesc(k string) string { return "query parameter " + k }

func joinQuery(base string, pairs []string) string {
	if len(pairs) == 0 {
		return base
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + strings.Join(pairs, "&")
}

func maskIf(tainted bool, s string) string {
	if tainted {
		return vault.Mask
	}
	return s
}

// headers renders headers, applying the empty policy and refusing values
// that would split the header (CR/LF) or are otherwise invalid.
func (b *builder) headers() (http.Header, []Header) {
	h := http.Header{}
	var shown []Header
	for _, kv := range b.in.version.Headers {
		desc, key := "header "+kv.Key, "header:"+kv.Key
		v, ok := b.eval(desc, key, kv.Value)
		if !ok {
			continue
		}
		value, keep, err := template.ApplyEmpty(v, b.in.empty)
		if err != nil {
			b.emptyError(desc)
		}
		if !keep || !b.validHeader(kv.Key, value) {
			continue
		}
		h.Add(kv.Key, value)
		shown = append(shown, Header{Name: kv.Key, Value: maskIf(b.tainted[key], value)})
	}
	return h, shown
}

// addDefaultHeaders adds the default headers the request does not name.
func (b *builder) addDefaultHeaders(h http.Header, shown []Header) []Header {
	named := map[string]bool{}
	for _, kv := range b.in.version.Headers {
		named[http.CanonicalHeaderKey(kv.Key)] = true
	}
	for _, d := range b.in.defaults {
		if named[http.CanonicalHeaderKey(d.Key)] {
			continue
		}
		h.Set(d.Key, d.Value)
		shown = append(shown, Header{Name: d.Key, Value: d.Value})
	}
	return shown
}

func (b *builder) validHeader(name, value string) bool {
	detail := ""
	switch {
	case name == "":
		detail = "the name is empty"
	case !httpguts.ValidHeaderFieldName(name):
		detail = "the name is not a valid header name"
	case !httpguts.ValidHeaderFieldValue(value):
		detail = "the value contains a line break or control character"
	}
	if detail != "" {
		b.add(diag.CodeRequestHeader, map[string]string{"name": name, "detail": detail})
	}
	return detail == ""
}

// body renders whichever body kind the version has. The recorded body's
// ContentType is the default for its kind; a Content-Type header the
// request sets wins over it.
func (b *builder) body() (httpx.BodySource, *Body) {
	body := b.in.version.Body
	switch {
	case body.JSON != "":
		return b.jsonBody(body.JSON)
	case body.Text != "":
		return b.textBody(body.Text)
	case len(body.Form) > 0:
		return b.formBody(body.Form)
	case len(body.Multipart) > 0:
		return b.multipartBody(body.Multipart)
	case body.File != "":
		return b.fileBody(body.File)
	}
	return nil, nil
}

func (b *builder) jsonBody(src string) (httpx.BodySource, *Body) {
	r, err := template.RenderJSON(src, b.scope("body"))
	if err != nil && b.in.verbatim {
		return b.inMemory(src, "application/json")
	}
	if err != nil {
		b.templateError("body", err)
		return nil, nil
	}
	b.tainted["body"] = r.Tainted
	b.derived(r)
	return b.inMemory(r.Value.(string), "application/json")
}

func (b *builder) textBody(src string) (httpx.BodySource, *Body) {
	text, ok := b.text("body", "body", src)
	if !ok {
		return nil, nil
	}
	return b.inMemory(text, "text/plain; charset=utf-8")
}

func (b *builder) inMemory(text, ctype string) (httpx.BodySource, *Body) {
	b.bodyText = text
	sent := &Body{Text: maskIf(b.tainted["body"], text), Size: int64(len(text)), ContentType: ctype}
	return httpx.BytesBody([]byte(text), ctype), sent
}

func (b *builder) formBody(fields []model.KeyValue) (httpx.BodySource, *Body) {
	var out []httpx.FormField
	for _, f := range fields {
		v, ok := b.text("form field "+f.Key, "body", f.Value)
		if ok {
			out = append(out, httpx.FormField{Name: f.Key, Value: v})
		}
	}
	text := formText(out)
	b.bodyText = text
	return httpx.FormBody(out), &Body{Text: maskIf(b.tainted["body"], text), Size: int64(len(text)), ContentType: "application/x-www-form-urlencoded"}
}

// formText is the encoded form, as httpx.FormBody sends it.
func formText(fields []httpx.FormField) string {
	pairs := make([]string, len(fields))
	for i, f := range fields {
		pairs[i] = url.QueryEscape(f.Name) + "=" + url.QueryEscape(f.Value)
	}
	return strings.Join(pairs, "&")
}

func (b *builder) multipartBody(fields []model.Field) (httpx.BodySource, *Body) {
	var parts []httpx.Part
	var summary []string
	for _, f := range fields {
		p, ok := b.part(f)
		if !ok {
			continue
		}
		parts = append(parts, p)
		summary = append(summary, partSummary(p))
	}
	b.parts = parts
	src, err := httpx.MultipartBody(b.in.root, parts)
	if err != nil {
		b.fileError("multipart file", err)
		return nil, nil
	}
	return src, &Body{Text: maskIf(b.tainted["body"], strings.Join(summary, "\n")), ContentType: "multipart/form-data"}
}

func partSummary(p httpx.Part) string {
	if p.File != "" {
		return p.Name + ": file " + p.File
	}
	return p.Name + ": " + p.Value
}

// part reads one multipart field: a template string, or {file, content_type}.
func (b *builder) part(f model.Field) (httpx.Part, bool) {
	desc := "multipart field " + f.Key
	switch v := f.Value.(type) {
	case string:
		text, ok := b.text(desc, "body", v)
		return httpx.Part{Name: f.Key, Value: text}, ok
	case map[string]any:
		file, _ := v["file"].(string)
		ctype, _ := v["content_type"].(string)
		path, ok := b.text(desc, "body", file)
		return httpx.Part{Name: f.Key, File: path, ContentType: ctype}, ok && path != ""
	}
	b.add(diag.CodeTemplateSyntax, map[string]string{"field": b.field(desc), "detail": "must be text or {file, content_type}"})
	return httpx.Part{}, false
}

func (b *builder) fileBody(src string) (httpx.BodySource, *Body) {
	path, ok := b.text("body file", "body", src)
	if !ok {
		return nil, nil
	}
	body, err := httpx.FileBody(b.in.root, path)
	if err != nil {
		b.fileError(path, err)
		return nil, nil
	}
	return body, &Body{File: path}
}

func (b *builder) fileError(path string, err error) {
	var outside *httpx.OutsideRootError
	if errors.As(err, &outside) {
		b.add(diag.CodeRequestFileOutside, map[string]string{"path": outside.Path})
		return
	}
	detail := "cannot be read"
	if errors.Is(err, fs.ErrNotExist) {
		detail = "no such file"
	}
	b.add(diag.CodeRequestFileRead, map[string]string{"path": path, "detail": detail})
}

func (b *builder) emptyError(desc string) {
	b.add(diag.CodeTemplateEmpty, map[string]string{"field": b.field(desc)})
}

// templateError turns a template failure in a field into diagnostics,
// one per undefined name.
func (b *builder) templateError(desc string, err error) {
	b.templateErrorField(b.field(desc), err)
}

// templateErrorField is templateError for a field already named in full.
func (b *builder) templateErrorField(field string, err error) {
	if names := template.Undefined(err); len(names) > 0 {
		for _, name := range names {
			b.undefined(field, name)
		}
		return
	}
	var se *template.SyntaxError
	var ge *template.UnknownGeneratorError
	switch {
	case errors.As(err, &ge):
		b.add(diag.CodeTemplateGenerator, map[string]string{"name": ge.Name})
	case errors.As(err, &se):
		b.add(diag.CodeTemplateSyntax, map[string]string{"field": field, "detail": se.Detail})
	default:
		b.add(diag.CodeTemplateSyntax, map[string]string{"field": field, "detail": err.Error()})
	}
}

func (b *builder) undefined(field, name string) {
	hint := ""
	if rest, ok := strings.CutPrefix(name, "secret."); ok {
		secret, _, _ := strings.Cut(rest, ".")
		if b.in.missingSecret != nil && b.in.missingSecret(secret) {
			if !slices.Contains(b.missingSecrets, secret) {
				b.missingSecrets = append(b.missingSecrets, secret) // the caller reports FLW-SEC-001
			}
			return
		}
		hint = " (no secret variable " + secret + " in this environment)"
	}
	if rest, ok := strings.CutPrefix(name, "env."); ok {
		if v, _, _ := strings.Cut(rest, "."); b.in.secretVars[v] {
			hint = " (it is a secret: use secret." + v + ")"
		}
	}
	b.add(diag.CodeTemplateUndefined, map[string]string{"name": name, "field": field, "hint": hint})
}

func (b *builder) add(c diag.Code, args map[string]string) {
	b.diags = append(b.diags, diag.New(c, diag.Location{}, args))
}
