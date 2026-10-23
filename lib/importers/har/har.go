// SPDX-License-Identifier: Apache-2.0

// Package har turns an HTTP Archive (a browser's "Save all as HAR") into
// requests, pinned examples and one flow that replays the recording in
// order. Calls to the same endpoint (ids in the path replaced by
// {{id}}) become one request; static resources and non-HTTP traffic are
// left out with a summary; credentials never reach a file.
package har

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/importers/jsonpos"
	"github.com/githugt/flowi-lib/model"
)

type archive struct {
	Log *struct {
		Entries []entry `json:"entries"`
	} `json:"log"`
}

type entry struct {
	ResourceType string    `json:"_resourceType"`
	Request      *request  `json:"request"`
	Response     *response `json:"response"`
}

type request struct {
	Method      string    `json:"method"`
	URL         string    `json:"url"`
	Headers     []pair    `json:"headers"`
	QueryString []pair    `json:"queryString"`
	PostData    *postData `json:"postData"`
}

type pair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type postData struct {
	MimeType string  `json:"mimeType"`
	Text     string  `json:"text"`
	Params   []param `json:"params"`
}

type param struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	FileName string `json:"fileName"`
}

type response struct {
	Status  int    `json:"status"`
	Headers []pair `json:"headers"`
	Content struct {
		MimeType string `json:"mimeType"`
		Text     string `json:"text"`
		Encoding string `json:"encoding"`
	} `json:"content"`
}

// Import reads a HAR file. A file that is not JSON is FLW-IMPORT-004 at
// the error; JSON without log.entries is FLW-IMPORT-002.
func Import(src []byte, file string) (importers.Result, diag.List) {
	offsets, err := jsonpos.Scan(src, func(p []string) bool { return len(p) == 3 && p[0] == "log" && p[1] == "entries" })
	if err != nil {
		return importers.Result{}, diag.List{syntaxError(src, file, err)}
	}
	var a archive
	if err := json.Unmarshal(src, &a); err != nil || a.Log == nil || a.Log.Entries == nil {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, diag.Location{File: file},
			map[string]string{"detail": "a HAR file has log.entries"})}
	}
	im := &importer{src: src, file: file, groups: map[string]*group{}}
	lines := diag.NewLineIndex(src)
	for i, e := range a.Log.Entries {
		im.entry(e, lines.Position(offsets["log/entries/"+strconv.Itoa(i)]))
	}
	return im.result(), nil
}

func syntaxError(src []byte, file string, err error) diag.Diagnostic {
	offset, _ := jsonpos.ErrorOffset(err, len(src))
	pos := diag.PositionAt(src, offset)
	d := diag.New(diag.CodeImportSyntax, diag.Location{File: file, Pos: pos}, map[string]string{"format": "HAR", "detail": err.Error()})
	d.Excerpt = diag.ExcerptAt(src, pos)
	return d
}

// group is one endpoint: the calls with the same method and path shape.
type group struct {
	item    *importers.Item
	req     model.Request
	example *importers.Example
}

type importer struct {
	src     []byte
	file    string
	groups  map[string]*group
	order   []*group
	skipped []importers.Item
	nodes   []model.Node
	taken   importers.Names
	nodeIDs importers.Names
	left    map[string]int // left-out reason → count
}

// staticTypes are browser resource types that are not API calls.
func staticTypes() []string {
	return []string{"document", "stylesheet", "script", "image", "font", "media", "manifest", "texttrack", "ping", "preflight", "other"}
}

func (im *importer) leaveOut(reason string) {
	if im.left == nil {
		im.left = map[string]int{}
	}
	im.left[reason]++
}

func (im *importer) entry(e entry, pos diag.Position) {
	loc := diag.Location{File: im.file, Pos: pos}
	if e.Request == nil || e.Request.Method == "" || e.Request.URL == "" {
		im.skip(loc, "entry at line "+strconv.Itoa(pos.Line), "the entry has no request method or URL")
		return
	}
	u, err := url.Parse(e.Request.URL)
	switch {
	case err != nil:
		im.skip(loc, e.Request.URL, "the URL cannot be read: "+err.Error())
		return
	case u.Scheme != "http" && u.Scheme != "https":
		im.leaveOut(u.Scheme + " traffic")
		return
	case slices.Contains(staticTypes(), e.ResourceType) || e.ResourceType == "" && static(e):
		im.leaveOut("static resources")
		return
	}
	tpl, ids := templatePath(u.EscapedPath())
	key := e.Request.Method + " " + u.Scheme + "://" + u.Host + tpl
	g, seen := im.groups[key]
	if !seen {
		g = im.newGroup(e, u, tpl, loc)
		im.groups[key] = g
		im.order = append(im.order, g)
	}
	if g.example == nil {
		g.example = im.example(e, g)
	}
	im.addNode(e.Request.Method, g.req.Path, tpl, ids)
}

func (im *importer) skip(loc diag.Location, name, detail string) {
	it := importers.Item{Name: name, Source: loc}
	it.Skip(detail)
	im.skipped = append(im.skipped, it)
}

// static recognises non-API responses when the recorder did not say.
func static(e entry) bool {
	if e.Response == nil {
		return false
	}
	mime := strings.ToLower(e.Response.Content.MimeType)
	for _, prefix := range []string{"image/", "font/", "audio/", "video/", "text/css", "text/html", "application/javascript", "text/javascript"} {
		if strings.HasPrefix(mime, prefix) {
			return true
		}
	}
	return false
}

var (
	digits = regexp.MustCompile(`^[0-9]+$`)
	uuid   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	hexID  = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
)

// templatePath replaces id-like segments (numbers, UUIDs, long hex) with
// {{id}}, {{id_2}}…, returning the template and the values in order.
// Other segments are kept literally.
func templatePath(p string) (string, []string) {
	segs := strings.Split(p, "/")
	var values []string
	for i, s := range segs {
		if !digits.MatchString(s) && !uuid.MatchString(s) && !hexID.MatchString(s) {
			segs[i] = importers.Literal(s)
			continue
		}
		values = append(values, s)
		segs[i] = "{{" + idName(len(values)) + "}}"
	}
	return strings.Join(segs, "/"), values
}

func idName(n int) string {
	if n == 1 {
		return "id"
	}
	return fmt.Sprintf("id_%d", n)
}

func (im *importer) newGroup(e entry, u *url.URL, tpl string, loc diag.Location) *group {
	method := strings.ToUpper(e.Request.Method)
	readable := strings.NewReplacer("{{", "", "}}", "").Replace(tpl)
	reqPath := im.taken.Unique("har/" + importers.FileName(u.Host) + "/" + importers.FileName(strings.ToLower(method)+"-"+slug(readable)))
	g := &group{item: &importers.Item{Name: reqPath, Source: loc}}
	v := model.RequestVersion{Label: "1", Method: method, URL: u.Scheme + "://" + u.Host + tpl}
	for _, q := range e.Request.QueryString {
		value := importers.Literal(q.Value)
		if importers.SensitiveHeader(q.Name) && q.Value != "" {
			value = g.item.Redact("query parameter "+q.Name, q.Name)
		}
		v.Query = append(v.Query, model.KeyValue{Key: importers.Literal(q.Name), Value: value})
	}
	v.Headers = im.headers(g.item, e.Request.Headers)
	if u.User != nil {
		v.Headers = append(v.Headers, model.KeyValue{Key: "Authorization", Value: g.item.Redact("credentials in the URL", "Authorization")})
	}
	v.Body = im.body(g.item, e.Request.PostData)
	shown := strings.NewReplacer("{{", "{", "}}", "}").Replace(tpl)
	g.req = model.Request{Meta: model.Meta{Name: method + " " + shown}, Path: reqPath, Versions: []model.RequestVersion{v}}
	return g
}

// headers keeps the recorded headers except HTTP/2 pseudo-headers and
// those computed when sending; credentials become secret placeholders.
func (im *importer) headers(it *importers.Item, hs []pair) []model.KeyValue {
	var out []model.KeyValue
	for _, h := range hs {
		switch {
		case strings.HasPrefix(h.Name, ":"), slices.Contains([]string{"content-length", "host", "connection"}, strings.ToLower(h.Name)):
		case importers.SensitiveHeader(h.Name):
			out = append(out, model.KeyValue{Key: h.Name, Value: it.Redact("header "+h.Name, h.Name)})
		default:
			out = append(out, model.KeyValue{Key: h.Name, Value: importers.Literal(h.Value)})
		}
	}
	return out
}

func (im *importer) body(it *importers.Item, pd *postData) model.Body {
	if pd == nil {
		return model.Body{}
	}
	mime := strings.ToLower(pd.MimeType)
	switch {
	case strings.Contains(mime, "json") && json.Valid([]byte(pd.Text)):
		redacted := redactedJSONText(it, pd.Text, "the recorded request body")
		return model.Body{JSON: importers.Literal(redacted)}
	case strings.HasPrefix(mime, "multipart/form-data"):
		var parts []model.Field
		for _, p := range pd.Params {
			if p.FileName != "" {
				parts = append(parts, model.Field{Key: p.Name, Value: []model.Field{{Key: "file", Value: importers.Literal(p.FileName)}}})
				it.Unmapped("file part "+p.Name, "kept with its file name", "the recording does not hold the file")
				continue
			}
			value := importers.Literal(p.Value)
			if importers.SensitiveHeader(p.Name) && p.Value != "" {
				value = it.Redact("form field "+p.Name, p.Name)
			}
			parts = append(parts, model.Field{Key: p.Name, Value: value})
		}
		return model.Body{Multipart: parts}
	case strings.HasPrefix(mime, "application/x-www-form-urlencoded") && len(pd.Params) > 0:
		var form []model.KeyValue
		for _, p := range pd.Params {
			value := importers.Literal(p.Value)
			if importers.SensitiveHeader(p.Name) {
				value = it.Redact("form field "+p.Name, p.Name)
			}
			form = append(form, model.KeyValue{Key: importers.Literal(p.Name), Value: value})
		}
		return model.Body{Form: form}
	}
	return model.Body{Text: importers.Literal(it.RedactBodyText(pd.Text, "the recorded request body"))}
}

// redactedJSONText re-encodes a JSON body with credential values replaced,
// reporting when any were.
func redactedJSONText(it *importers.Item, text, where string) string {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return text
	}
	clean, n := importers.RedactJSON(v)
	if n == 0 {
		return text
	}
	it.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
		"where": fmt.Sprintf("%s (%d values)", where, n), "placeholder": importers.Redacted}))
	data, err := json.Marshal(clean)
	if err != nil {
		return text
	}
	return string(data)
}

// example keeps a 2xx JSON response as the request's pinned example.
func (im *importer) example(e entry, g *group) *importers.Example {
	r := e.Response
	if r == nil || r.Status < 200 || r.Status > 299 || !strings.Contains(strings.ToLower(r.Content.MimeType), "json") {
		return nil
	}
	text := r.Content.Text
	if r.Content.Encoding == "base64" {
		data, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return nil
		}
		text = string(data)
	}
	var body any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		return nil
	}
	clean, n := importers.RedactJSON(body)
	if n > 0 {
		g.item.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
			"where": fmt.Sprintf("the recorded response (%d values)", n), "placeholder": importers.Redacted}))
	}
	ex := &importers.Example{RequestPath: g.req.Path, Name: fmt.Sprintf("har-%d", r.Status), Version: "1", Status: r.Status, Body: clean}
	for _, h := range r.Headers {
		value := h.Value
		if importers.SensitiveHeader(h.Name) {
			value = importers.Redacted
			g.item.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
				"where": "response header " + h.Name, "placeholder": importers.Redacted}))
		}
		ex.Headers = append(ex.Headers, importers.Header{Name: h.Name, Value: value})
	}
	return ex
}

// addNode adds the call to the recording's flow, passing the path ids.
// The node is named after the method and the last path word that is not
// an id (get-users for GET /v1/users/{{id}}).
func (im *importer) addNode(method, reqPath, tpl string, ids []string) {
	last := "root"
	for _, seg := range strings.Split(tpl, "/") {
		if seg != "" && !strings.HasPrefix(seg, "{{") {
			last = seg
		}
	}
	id := im.nodeIDs.Unique(importers.FileName(strings.ToLower(method) + "-" + last))
	call := &model.RequestCall{Request: model.Ref{Path: reqPath}}
	for i, v := range ids {
		call.With = append(call.With, model.KeyValue{Key: idName(i + 1), Value: importers.Literal(v)})
	}
	im.nodes = append(im.nodes, model.Node{ID: id, Type: "request", Request: call})
}

func (im *importer) result() importers.Result {
	var r importers.Result
	for _, g := range im.order {
		r.Requests = append(r.Requests, g.req)
		if g.example != nil {
			r.Examples = append(r.Examples, *g.example)
		}
		r.Items = append(r.Items, *g.item)
	}
	r.Items = append(r.Items, im.skipped...)
	if len(im.left) > 0 {
		summary := importers.Item{Name: "left out of the recording", Source: diag.Location{File: im.file}}
		for _, reason := range slices.Sorted(maps.Keys(im.left)) {
			summary.Unmapped(fmt.Sprintf("%s (%d)", reason, im.left[reason]), "left out", "only HTTP API calls are imported")
		}
		r.Items = append(r.Items, summary)
	}
	if len(im.nodes) > 0 {
		stem := strings.TrimSuffix(path.Base(im.file), path.Ext(im.file))
		name := importers.FileName(cmp.Or(stem, "recording"))
		r.Flows = []model.Flow{{Meta: model.Meta{Name: name}, Path: "har/" + name, Nodes: im.nodes}}
	}
	return r
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(p string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(p), "-"), "-")
}
