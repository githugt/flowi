// SPDX-License-Identifier: Apache-2.0

// Package httpfile imports .http / .rest files (VS Code REST Client, JetBrains
// HTTP Client): requests separated by ###, with file variables (@name = value),
// headers and a body. Credentials become secret placeholders; response
// handlers and scripts are reported and left out.
package httpfile

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/model"
)

var (
	separator   = regexp.MustCompile(`^###`)
	variableDef = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_.-]*)\s*=\s*(.*)$`)
	requestLine = regexp.MustCompile(`^(?i:(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|TRACE|QUERY)\s+)?(\S+)(?:\s+HTTP/[\d.]+)?\s*$`)
	nameNote    = regexp.MustCompile(`^(?:#|//)\s*@name\s+(\S+)`)
	headerLine  = regexp.MustCompile(`^([A-Za-z0-9!#$%&'*+.^_` + "`" + `|~-]+)\s*:\s*(.*)$`)
	nonSlug     = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

// block is one request's lines and where it starts in the file.
type block struct {
	lines []string
	first int // 1-based line number of lines[0]
}

// Import turns every request of src into a request. A file with no request is
// FLW-IMPORT-002; a block that cannot be read is skipped.
func Import(src []byte, file string) (importers.Result, diag.List) {
	blocks, vars := split(string(src))
	var r importers.Result
	var taken importers.Names
	fileItem := importers.Item{Name: "variables", Source: diag.Location{File: file, Pos: diag.Position{Line: 1, Column: 1}}}
	env := environmentOf(&fileItem, file, vars)
	for _, b := range blocks {
		it := importers.Item{Name: fmt.Sprintf("request at line %d", b.first), Source: diag.Location{File: file, Pos: diag.Position{Line: b.first, Column: 1}}}
		if req, ok := requestOf(b, &it); ok {
			req.Path = taken.Unique(req.Path)
			it.Name = req.Path
			r.Requests = append(r.Requests, req)
		}
		r.Items = append(r.Items, it)
	}
	if len(r.Requests) == 0 && len(r.Items) == 0 {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, diag.Location{File: file}, map[string]string{"detail": "no request found"})}
	}
	if len(env.Vars) > 0 {
		r.Environments = append(r.Environments, env)
		r.Items = append(r.Items, fileItem)
	}
	return r, nil
}

type variable struct{ name, value string }

// split cuts the file at ### lines into request blocks and collects the file
// variables, which may be anywhere.
func split(text string) ([]block, []variable) {
	var blocks []block
	var vars []variable
	cur := block{first: 1}
	flush := func() {
		if hasContent(cur.lines) {
			blocks = append(blocks, cur)
		}
	}
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		switch {
		case separator.MatchString(line):
			flush()
			cur = block{first: i + 2}
		case variableDef.MatchString(strings.TrimSpace(line)):
			m := variableDef.FindStringSubmatch(strings.TrimSpace(line))
			vars = append(vars, variable{name: m[1], value: strings.TrimSpace(m[2])})
		default:
			cur.lines = append(cur.lines, line)
		}
	}
	flush()
	return blocks, vars
}

// hasContent reports whether a block holds more than comments and blank lines.
func hasContent(lines []string) bool {
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" && !isComment(t) {
			return true
		}
	}
	return false
}

func isComment(t string) bool { return strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") }

// environmentOf turns the file variables into an environment named after the
// file; credential-like names are declared secret, without their value.
func environmentOf(it *importers.Item, file string, vars []variable) model.Environment {
	name := strings.TrimSuffix(strings.TrimSuffix(lastPart(file), ".http"), ".rest")
	env := model.Environment{Meta: model.Meta{Name: importers.FileName(strings.ToLower(name))}}
	if env.Name == "" {
		env.Name = "http-file"
	}
	for _, v := range vars {
		if importers.SensitiveHeader(v.name) {
			it.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{"where": "variable " + v.name, "placeholder": "a secret declaration"}))
			env.Vars = append(env.Vars, model.EnvVar{Name: v.name, Secret: true})
			continue
		}
		text, _ := importers.Translate(v.value)
		env.Vars = append(env.Vars, model.EnvVar{Name: v.name, Value: text})
	}
	return env
}

func lastPart(p string) string { return p[strings.LastIndexAny(p, `/\`)+1:] }

// requestOf parses one block; false means the item was skipped.
func requestOf(b block, it *importers.Item) (model.Request, bool) {
	tr := &importers.Translator{Item: it}
	lines := b.lines
	name := ""
	i := 0
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if m := nameNote.FindStringSubmatch(t); m != nil {
			name = m[1]
		}
		if t != "" && !isComment(t) {
			break
		}
	}
	m := requestLine.FindStringSubmatch(strings.TrimSpace(lines[i]))
	if m == nil {
		it.Skip("the first line is not a request line (METHOD URL)")
		return model.Request{}, false
	}
	method, target := strings.ToUpper(m[1]), m[2]
	if method == "" {
		method = "GET"
	}
	i++
	for ; i < len(lines) && continuesURL(lines[i]); i++ { // ?a=b and &c=d lines continue the URL
		target += strings.TrimSpace(lines[i])
	}
	v := model.RequestVersion{Label: "1", Method: method, URL: tr.Text(target)}
	if stripped, had := importers.StripUserinfo(v.URL); had {
		v.URL = stripped
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: it.Redact("credentials in the URL", "Authorization")})
	}
	i = headers(lines, i, &v, it, tr)
	body(lines[i:], &v, it, tr)
	tr.ReportUnknown()
	label := method + " " + v.URL
	file := importers.FileName(strings.ToLower(method) + "-" + strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(urlPath(target)), "-"), "-"))
	if name != "" {
		label, file = name, importers.FileName(name)
	}
	return model.Request{Meta: model.Meta{Name: label}, Path: "http/" + file, Versions: []model.RequestVersion{v}}, true
}

func continuesURL(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "?") || strings.HasPrefix(t, "&")
}

// templateAtStart matches a leading {{variable}}, which stands for the origin.
var templateAtStart = regexp.MustCompile(`^\{\{[^}]*\}\}`)

// urlPath is the path part of a URL as written (before translation), for naming.
func urlPath(u string) string {
	u, _, _ = strings.Cut(u, "?")
	u = templateAtStart.ReplaceAllString(u, "")
	if _, rest, ok := strings.Cut(u, "://"); ok {
		_, p, _ := strings.Cut(rest, "/")
		return p
	}
	return strings.TrimPrefix(u, "/")
}

// headers reads Name: value lines until the blank line and returns where the
// body starts.
func headers(lines []string, i int, v *model.RequestVersion, it *importers.Item, tr *importers.Translator) int {
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			return i + 1
		}
		if isComment(t) {
			continue
		}
		m := headerLine.FindStringSubmatch(t)
		if m == nil {
			return i // no blank line: the body follows directly
		}
		value := tr.Text(strings.TrimSpace(m[2]))
		if importers.SensitiveHeader(m[1]) && !importers.HasVar(value) {
			value = it.Redact("header "+m[1], m[1])
		}
		v.Headers = append(v.Headers, model.KeyValue{Key: m[1], Value: value})
	}
	return i
}

// body reads the body lines; response handlers (> …) and response file
// references (<> …) are reported and left out, and so is a body read from a
// file (< path).
func body(lines []string, v *model.RequestVersion, it *importers.Item, tr *importers.Translator) {
	var kept []string
	inScript := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case inScript:
			if strings.Contains(t, "%}") {
				inScript = false
			}
		case strings.HasPrefix(t, "> {%"):
			it.Unmapped("a response handler script", "left out", "Flowi has hooks: add an assert or extract by hand")
			inScript = !strings.Contains(t, "%}")
		case strings.HasPrefix(t, "> ") || strings.HasPrefix(t, "<> "):
			it.Unmapped("a response handler or saved-response reference", "left out", "it has no Flowi equivalent")
		case strings.HasPrefix(t, "< ") && !strings.HasPrefix(t, "<<"):
			it.Unmapped("a body read from "+strings.TrimSpace(t[2:]), "left out", "set the request body by hand")
		default:
			kept = append(kept, l)
		}
	}
	text := strings.TrimRight(strings.Join(kept, "\n"), " \t\n")
	text = strings.TrimLeft(text, "\n")
	if text == "" {
		return
	}
	text = tr.Text(text)
	ct := ""
	for _, h := range v.Headers {
		if strings.EqualFold(h.Key, "Content-Type") {
			ct = strings.ToLower(h.Value)
		}
	}
	switch {
	case strings.Contains(ct, "x-www-form-urlencoded"):
		v.Body.Form = formOf(text)
	case strings.Contains(ct, "json") || strings.HasPrefix(text, "{") || strings.HasPrefix(text, "["):
		v.Body.JSON = it.RedactBodyText(text, "the body")
	default:
		v.Body.Text = it.RedactBodyText(text, "the body")
	}
}

// formOf reads a form body written as a=1&b=2, or one pair per line.
func formOf(text string) []model.KeyValue {
	var out []model.KeyValue
	for _, pair := range strings.FieldsFunc(text, func(r rune) bool { return r == '&' || r == '\n' }) {
		k, val, _ := strings.Cut(strings.TrimSpace(pair), "=")
		if k != "" {
			out = append(out, model.KeyValue{Key: k, Value: val})
		}
	}
	return out
}
