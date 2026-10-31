// SPDX-License-Identifier: Apache-2.0

// Package curl turns pasted curl commands (bash or Windows cmd quoting,
// as browsers' "Copy as cURL" writes them) into requests: one item per
// command. Credentials become secret placeholders; flags without meaning
// in a request are ignored, unknown ones are reported.
package curl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"
)

// command is one curl command and where it starts in the source.
type command struct {
	text   string
	offset int
}

// commandStart matches a line that starts a command, after an optional
// shell prompt.
var commandStart = regexp.MustCompile(`(?m)^[ \t]*(?:\$[ \t]+)?(curl(?:\.exe)?)(?:[ \t]|$)`)

// Import turns every curl command in src into a request. A source with no
// command is FLW-IMPORT-002; a command that cannot be read is skipped.
func Import(src []byte, file string) (importers.Result, diag.List) {
	cmds := commands(string(src))
	if len(cmds) == 0 {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, diag.Location{File: file},
			map[string]string{"detail": "no curl command found"})}
	}
	var r importers.Result
	var taken importers.Names
	lines := diag.NewLineIndex(src)
	for _, c := range cmds {
		pos := lines.Position(c.offset)
		it := importers.Item{Name: fmt.Sprintf("command at line %d", pos.Line), Source: diag.Location{File: file, Pos: pos}}
		b := &builder{src: src, lines: lines, file: file, base: c.offset, it: &it}
		if req, ok := b.request(c.text); ok {
			req.Path = taken.Unique(req.Path)
			it.Name = req.Path
			r.Requests = append(r.Requests, req)
		}
		r.Items = append(r.Items, it)
	}
	return r, nil
}

// commands splits src at the lines that start a curl command.
func commands(src string) []command {
	locs := commandStart.FindAllStringSubmatchIndex(src, -1)
	out := make([]command, len(locs))
	for i, loc := range locs {
		end := len(src)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out[i] = command{text: src[loc[2]:end], offset: loc[2]}
	}
	return out
}

// builder turns one command's words into a request, reporting into it.
type builder struct {
	src   []byte
	lines *diag.LineIndex
	file  string
	base  int // offset of the command in src
	it    *importers.Item
	req   parsed
	words []token
	i     int
}

// parsed is what the flags say, before it becomes a request.
type parsed struct {
	method   string
	url      token
	headers  []header
	data     []string
	dataForm []model.KeyValue // --data-urlencode name=value
	json     string
	form     []model.Field
	get      bool
	head     bool
	upload   string
}

type header struct {
	name, value string
	at          token
}

func (b *builder) at(offset int) diag.Location {
	return diag.Location{File: b.file, Pos: b.lines.Position(b.base + offset)}
}

func (b *builder) excerpt(d diag.Diagnostic) diag.Diagnostic {
	d.Excerpt = diag.ExcerptAt(b.src, d.Pos)
	return d
}

// request parses the command; false means the item was skipped.
func (b *builder) request(text string) (model.Request, bool) {
	cmd := strings.Contains(text, "^\n") || strings.Contains(text, "^\r\n") || strings.Contains(text, `^"`)
	words, err := tokenize(text, cmd)
	if err != nil {
		se := err.(*syntaxError) //nolint:errorlint // tokenize returns only *syntaxError
		b.it.Report(b.excerpt(diag.New(diag.CodeImportSyntax, b.at(se.offset), map[string]string{"format": "curl", "detail": se.detail})))
		b.it.Skip("the command cannot be read")
		return model.Request{}, false
	}
	b.words = words[1:] // words[0] is curl
	for b.i = 0; b.i < len(b.words); b.i++ {
		b.flag(b.words[b.i])
	}
	if b.req.url.text == "" {
		b.it.Skip("the command has no URL")
		return model.Request{}, false
	}
	return b.build(), true
}

// next consumes the flag's argument; false when there is none.
func (b *builder) next(flag string) (token, bool) {
	if b.i+1 >= len(b.words) {
		b.it.Unmapped("flag "+flag, "ignored", "it has no value")
		return token{}, false
	}
	b.i++
	return b.words[b.i], true
}

func (b *builder) flag(w token) {
	switch {
	case w.text == "--":
		for b.i++; b.i < len(b.words); b.i++ {
			b.setURL(b.words[b.i])
		}
	case strings.HasPrefix(w.text, "--"):
		name, value, inline := strings.Cut(w.text, "=")
		b.apply(name, w, value, inline)
	case strings.HasPrefix(w.text, "-") && len(w.text) > 1:
		b.short(w)
	default:
		b.setURL(w)
	}
}

// short handles -X POST, -XPOST and clusters such as -sSL.
func (b *builder) short(w token) {
	first := w.text[:2]
	if spec, ok := flags()[first]; ok && spec.arg {
		b.apply(first, w, w.text[2:], len(w.text) > 2)
		return
	}
	for _, c := range w.text[1:] {
		f := "-" + string(c)
		if spec, ok := flags()[f]; !ok || spec.arg {
			b.it.UnmappedAt(b.at(w.offset), "flag "+first, "ignored", "it is not a curl flag Flowi knows")
			return
		}
	}
	for _, c := range w.text[1:] {
		b.apply("-"+string(c), w, "", false)
	}
}

func (b *builder) apply(name string, w token, value string, inline bool) {
	spec, ok := flags()[name]
	if !ok {
		b.it.Unmapped("flag "+name, "ignored", "it is not a curl flag Flowi knows")
		return
	}
	arg := token{text: value, offset: w.offset + len(w.text) - len(value)}
	if spec.arg && !inline {
		var have bool
		if arg, have = b.next(name); !have {
			return
		}
	}
	if spec.set != nil {
		spec.set(b, arg)
	}
}

// setURL takes the command's URL. Values are never echoed in reports:
// a stray word may be an unknown flag's argument, such as a token.
func (b *builder) setURL(w token) {
	switch {
	case !looksLikeURL(w.text):
		b.it.UnmappedAt(b.at(w.offset), "an argument", "ignored", "it is neither a URL nor the value of a flag Flowi knows")
	case b.req.url.text != "":
		b.it.UnmappedAt(b.at(w.offset), "another URL", "ignored", "one request per command; the first URL is used")
	default:
		b.req.url = w
	}
}

// looksLikeURL accepts scheme://…, host.name/… and localhost[:port]/….
func looksLikeURL(s string) bool {
	if strings.ContainsAny(s, " \t\n") || strings.HasPrefix(s, "-") || s == "" {
		return false
	}
	host := s
	if _, rest, ok := strings.Cut(s, "://"); ok {
		return rest != ""
	}
	host, _, _ = strings.Cut(host, "/")
	return strings.Contains(host, ".") || strings.HasPrefix(host, "localhost") || strings.Contains(host, "{{")
}

var bodyFlags = regexp.MustCompile(`^(-d|--data|--data-ascii|--data-binary)$`)

// addData records -d style data; "@file" reads a file, which Flowi does
// not do at import.
func (b *builder) addData(flag string, w token) {
	if bodyFlags.MatchString(flag) && strings.HasPrefix(w.text, "@") {
		b.it.Unmapped("data from "+w.text, "left out", "set the request body by hand")
		return
	}
	b.req.data = append(b.req.data, w.text)
}

// build turns the parsed flags into a request version.
func (b *builder) build() model.Request {
	v := model.RequestVersion{Label: "1", Method: b.method()}
	u, query := b.splitURL()
	v.URL, v.Query = u, query
	if stripped, had := importers.StripUserinfo(u); had {
		v.URL = stripped
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: b.it.RedactAt(b.at(b.req.url.offset), "credentials in the URL", "Authorization")})
	}
	for _, h := range b.req.headers {
		v.Headers = append(v.Headers, b.headerKV(h))
	}
	b.body(&v)
	path := pathOf(u)
	return model.Request{
		Meta:     model.Meta{Name: v.Method + " " + path},
		Path:     "curl/" + importers.FileName(strings.ToLower(v.Method)+"-"+slug(path)),
		Versions: []model.RequestVersion{v},
	}
}

func (b *builder) method() string {
	switch {
	case b.req.method != "":
		return strings.ToUpper(b.req.method)
	case b.req.head:
		return "HEAD"
	case b.req.upload != "":
		return "PUT"
	case (len(b.req.data) > 0 || len(b.req.dataForm) > 0 || b.req.json != "" || len(b.req.form) > 0) && !b.req.get:
		return "POST"
	}
	return "GET"
}

// splitURL separates the query (kept in order, decoded) and adds -G data.
func (b *builder) splitURL() (string, []model.KeyValue) {
	raw := b.req.url.text
	b.checkInvisible("the URL", b.req.url)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw // curl's default scheme
	}
	base, rawQuery, _ := strings.Cut(raw, "?")
	var query []model.KeyValue
	for _, part := range strings.Split(rawQuery, "&") {
		if part != "" {
			query = append(query, decodePair(part))
		}
	}
	if b.req.get {
		for _, d := range b.req.data {
			for _, part := range strings.Split(d, "&") {
				query = append(query, decodePair(part))
			}
		}
		query = append(query, b.req.dataForm...)
	}
	for i, q := range query {
		value := importers.Literal(q.Value)
		if importers.SensitiveHeader(q.Key) && q.Value != "" {
			value = b.it.RedactAt(b.at(b.req.url.offset), "query parameter "+q.Key, q.Key)
		}
		query[i] = model.KeyValue{Key: importers.Literal(q.Key), Value: value}
	}
	return importers.Literal(base), query
}

func decodePair(part string) model.KeyValue {
	k, v, _ := strings.Cut(part, "=")
	if dk, err := url.QueryUnescape(k); err == nil {
		k = dk
	}
	if dv, err := url.QueryUnescape(v); err == nil {
		v = dv
	}
	return model.KeyValue{Key: k, Value: v}
}

func (b *builder) headerKV(h header) model.KeyValue {
	if importers.SensitiveHeader(h.name) {
		return model.KeyValue{Key: h.name, Value: b.it.RedactAt(b.at(h.at.offset), "header "+h.name, h.name)}
	}
	b.checkInvisible("header "+h.name, h.at)
	return model.KeyValue{Key: h.name, Value: importers.Literal(h.value)}
}

// maxInvisibleReports caps the warnings for one word; a pasted word can
// hold thousands of invisible characters.
const maxInvisibleReports = 3

// checkInvisible warns, with an excerpt, about invisible characters in a
// word: they arrive by copy-paste and break requests silently. Each is
// found in the source after the previous one, so the work stays linear.
func (b *builder) checkInvisible(field string, w token) {
	cursor, reported := b.base+w.offset, 0
	for _, r := range w.text {
		if !diag.Invisible(r) {
			continue
		}
		if reported == maxInvisibleReports {
			b.it.UnmappedAt(b.at(w.offset), "more invisible characters in "+field, "kept", "only the first few are shown")
			return
		}
		offset := w.offset
		if idx := bytes.IndexRune(b.src[cursor:], r); idx >= 0 {
			offset = cursor + idx - b.base
			cursor += idx + utf8.RuneLen(r)
		}
		d := diag.New(diag.CodeLintInvisible, b.at(offset), map[string]string{"char": fmt.Sprintf("U+%04X", r), "field": field})
		b.it.Report(b.excerpt(d))
		reported++
	}
}

// body sets the version's body from --json, -F, -d or -T.
func (b *builder) body(v *model.RequestVersion) {
	switch {
	case b.req.json != "":
		v.Body.JSON = importers.Literal(b.it.RedactBodyText(b.req.json, "the request body"))
		addHeaderIfMissing(v, "Content-Type", "application/json")
		addHeaderIfMissing(v, "Accept", "application/json")
	case len(b.req.form) > 0:
		v.Body.Multipart = b.req.form
	case b.req.upload != "":
		v.Body.File = importers.Literal(b.req.upload)
		b.it.Unmapped("upload of "+b.req.upload, "kept as a file body", "the path is relative to where curl ran")
	case b.req.get:
	case len(b.req.data) > 0 || len(b.req.dataForm) > 0:
		b.dataBody(v)
	}
}

func (b *builder) dataBody(v *model.RequestVersion) {
	joined := strings.Join(b.req.data, "&")
	ct := strings.ToLower(headerValue(v, "Content-Type"))
	trimmed := strings.TrimSpace(joined)
	looksJSON := strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
	switch {
	case (strings.Contains(ct, "json") || ct == "" && looksJSON) && json.Valid([]byte(joined)) && len(b.req.dataForm) == 0:
		v.Body.JSON = importers.Literal(b.it.RedactBodyText(joined, "the request body"))
	case (ct == "" || strings.Contains(ct, "x-www-form-urlencoded")) && allPairs(b.req.data):
		for _, d := range b.req.data {
			for _, part := range strings.Split(d, "&") {
				v.Body.Form = append(v.Body.Form, b.formField(decodePair(part)))
			}
		}
		for _, kv := range b.req.dataForm {
			v.Body.Form = append(v.Body.Form, b.formField(kv))
		}
	default:
		v.Body.Text = importers.Literal(b.it.RedactBodyText(joined, "the request body"))
	}
}

// formField keeps a form field, replacing a credential's value.
func (b *builder) formField(kv model.KeyValue) model.KeyValue {
	if importers.SensitiveHeader(kv.Key) && kv.Value != "" {
		return model.KeyValue{Key: importers.Literal(kv.Key), Value: b.it.Redact("form field "+kv.Key, kv.Key)}
	}
	return model.KeyValue{Key: importers.Literal(kv.Key), Value: importers.Literal(kv.Value)}
}

func allPairs(data []string) bool {
	for _, d := range data {
		for _, part := range strings.Split(d, "&") {
			if !strings.Contains(part, "=") {
				return false
			}
		}
	}
	return true
}

func headerValue(v *model.RequestVersion, name string) string {
	for _, h := range v.Headers {
		if strings.EqualFold(h.Key, name) {
			return h.Value
		}
	}
	return ""
}

func addHeaderIfMissing(v *model.RequestVersion, name, value string) {
	if headerValue(v, name) == "" {
		v.Headers = append(v.Headers, model.KeyValue{Key: name, Value: value})
	}
}

func pathOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return "/"
	}
	return u.Path
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(path string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(path), "-"), "-")
}
