// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/jsonrpc"
	"github.com/githugt/flowi/lib/model"
)

// BodyKind is what the Body section's first picker chooses.
type BodyKind string

// Body kinds.
const (
	BodyNone BodyKind = "none"
	BodyText BodyKind = "text"
	BodyFile BodyKind = "file"
	BodyForm BodyKind = "form"
)

// BodyFormat is the text kind's second picker.
type BodyFormat string

// Body formats.
const (
	FormatJSON BodyFormat = "json"
	FormatXML  BodyFormat = "xml"
	FormatRaw  BodyFormat = "raw"
	FormatYAML BodyFormat = "yaml"
)

const contentTypeHeader = "Content-Type"

// formatTypes are the Content-Type values the format picker manages. JSON
// needs none: the engine defaults to application/json.
func formatType(f BodyFormat) string {
	switch f {
	case FormatXML:
		return "application/xml"
	case FormatYAML:
		return "application/yaml"
	case FormatRaw:
		return "text/plain"
	}
	return ""
}

func bodyHasContent(b model.Body) bool {
	return b.JSON != "" || b.Text != "" || len(b.Form) > 0 || len(b.Multipart) > 0 || b.File != ""
}

// BodyKind is the selected version's body kind.
func (r *Request) BodyKind() BodyKind {
	b := r.Current().Body
	switch {
	case b.JSON != "" || b.Text != "":
		return BodyText
	case len(b.Form) > 0 || len(b.Multipart) > 0:
		return BodyForm
	case b.File != "":
		return BodyFile
	}
	if r.chosenKind != "" {
		return r.chosenKind
	}
	return BodyNone
}

// BodyFormat is the format of a text body: JSON, or by its Content-Type
// header XML, YAML or raw.
func (r *Request) BodyFormat() BodyFormat {
	v := r.Current()
	switch {
	case v.Body.JSON != "":
		return FormatJSON
	case v.Body.Text != "":
		return formatOfType(headerOf(v.Headers, contentTypeHeader))
	}
	if r.chosenFormat != "" {
		return r.chosenFormat
	}
	return FormatJSON
}

func formatOfType(contentType string) BodyFormat {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "xml"):
		return FormatXML
	case strings.Contains(ct, "yaml"):
		return FormatYAML
	}
	return FormatRaw
}

func headerOf(hs []model.KeyValue, name string) string {
	if i := slices.IndexFunc(hs, func(kv model.KeyValue) bool { return strings.EqualFold(kv.Key, name) }); i >= 0 {
		return hs[i].Value
	}
	return ""
}

// JSONRPC reads the selected version's JSON body as a JSON-RPC 2.0 call. ok
// is false for any other body (or one that has a {{template}} where JSON
// needs a value, which is not JSON until it is rendered).
func (r *Request) JSONRPC() (call jsonrpc.Call, ok bool) {
	body := r.Current().Body
	if body.JSON == "" {
		return jsonrpc.Call{}, false
	}
	return jsonrpc.Parse(body.JSON)
}

// SetJSONRPCField changes the method or the id of the JSON-RPC call the JSON body holds.
// An empty id makes it a notification. The parameters are kept as they are.
func (r *Request) SetJSONRPCField(field, value string) {
	call, ok := r.JSONRPC()
	if !ok {
		return
	}
	switch field {
	case "method":
		call.Method = value
	case "id":
		call.ID = value
	default:
		return
	}
	r.Edit(func(v *model.RequestVersion) { v.Body.JSON = jsonrpc.Envelope(call) })
}

// MakeJSONRPC turns the JSON body into a JSON-RPC call: an empty body becomes
// a blank call, and a body that holds JSON becomes the call's parameters.
func (r *Request) MakeJSONRPC() {
	if _, already := r.JSONRPC(); already {
		return
	}
	r.Edit(func(v *model.RequestVersion) {
		call := jsonrpc.Call{Method: "method", ID: "1", Params: strings.TrimSpace(v.Body.JSON)}
		v.Body = model.Body{JSON: jsonrpc.Envelope(call)}
	})
}

// Multipart reports whether a form body is sent as multipart/form-data.
func (r *Request) Multipart() bool {
	return len(r.Current().Body.Multipart) > 0 || (r.chosenMultipart && r.BodyKind() == BodyForm)
}

// MultipartPart is one part of a multipart body as the editor shows it: text, or a
// file (its path, relative to the collection, and an optional content type).
type MultipartPart struct {
	Name        string
	File        bool
	Value       string // the text, or the file's path
	ContentType string // of a file; empty lets the engine pick one
}

// Parts lists the parts of the selected version's multipart body. A part whose
// shape is neither text nor a file mapping is shown as its text.
func (r *Request) Parts() []MultipartPart {
	out := []MultipartPart{}
	for _, f := range r.Current().Body.Multipart {
		switch v := f.Value.(type) {
		case string:
			out = append(out, MultipartPart{Name: f.Key, Value: v})
		case map[string]any:
			path, _ := v["file"].(string)
			ctype, _ := v["content_type"].(string)
			out = append(out, MultipartPart{Name: f.Key, File: true, Value: path, ContentType: ctype})
		case []model.Field: // how a request file decodes a mapping
			part := MultipartPart{Name: f.Key, File: true}
			for _, field := range v {
				text, _ := field.Value.(string)
				switch field.Key {
				case "file":
					part.Value = text
				case "content_type":
					part.ContentType = text
				}
			}
			out = append(out, part)
		default:
			out = append(out, MultipartPart{Name: f.Key, Value: fmt.Sprint(f.Value)})
		}
	}
	return out
}

// SetParts replaces the parts of the multipart body. A part with no name is
// not kept; the body stays multipart however many parts it has.
func (r *Request) SetParts(parts []MultipartPart) {
	r.chosenMultipart = true
	r.Edit(func(v *model.RequestVersion) {
		fields := make([]model.Field, 0, len(parts))
		for _, p := range parts {
			if p.Name == "" {
				continue
			}
			if !p.File {
				fields = append(fields, model.Field{Key: p.Name, Value: p.Value})
				continue
			}
			// Ordered fields, as a request file decodes them, so the file is written the same way every time.
			file := []model.Field{{Key: "file", Value: p.Value}}
			if p.ContentType != "" {
				file = append(file, model.Field{Key: "content_type", Value: p.ContentType})
			}
			fields = append(fields, model.Field{Key: p.Name, Value: file})
		}
		v.Body.Form, v.Body.Multipart = nil, fields
	})
}

// SetBodyKind picks the body kind. Typed text is kept when moving to text;
// the other kinds start empty (a body has at most one).
func (r *Request) SetBodyKind(k BodyKind) {
	r.chosenKind = k
	r.editBody(func(v *model.RequestVersion) {
		text := v.Body.JSON + v.Body.Text
		v.Body = model.Body{}
		if k == BodyText {
			setText(&v.Body, text, r.BodyFormat())
			return
		}
		// Only text bodies use the picker's Content-Type: a form or file
		// keeps the engine's own (and multipart needs its boundary).
		v.Headers = withManagedType(v.Headers, FormatJSON)
	})
}

// SetBodyFormat picks the text body's format: the text moves to the field
// of that format and the Content-Type header follows, unless the user set
// a Content-Type of their own.
func (r *Request) SetBodyFormat(f BodyFormat) {
	r.chosenFormat = f
	r.editBody(func(v *model.RequestVersion) {
		text := v.Body.JSON + v.Body.Text
		v.Body.JSON, v.Body.Text = "", ""
		setText(&v.Body, text, f)
		v.Headers = withManagedType(v.Headers, f)
	})
}

// SetMultipart sends a form as multipart/form-data (on) or urlencoded (off).
// String fields convert both ways; a multipart file field cannot be a form
// row and is dropped when switching off.
func (r *Request) SetMultipart(on bool) {
	r.chosenMultipart = on
	r.editBody(func(v *model.RequestVersion) {
		if on {
			v.Body.Multipart = slices.Concat(v.Body.Multipart, fieldsOf(v.Body.Form))
			v.Body.Form = nil
			return
		}
		v.Body.Form = slices.Concat(v.Body.Form, rowsOf(v.Body.Multipart))
		v.Body.Multipart = nil
	})
}

// editBody applies f as an edit only when it changes the body or headers,
// so picking what is already picked does not make the request dirty.
func (r *Request) editBody(f func(v *model.RequestVersion)) {
	before := cloneRequest(model.Request{Versions: []model.RequestVersion{r.Current()}}).Versions[0]
	probe := cloneRequest(model.Request{Versions: []model.RequestVersion{before}}).Versions[0]
	f(&probe)
	if sameBodyAndHeaders(before, probe) {
		r.changed()
		return
	}
	r.Edit(f)
}

func sameBodyAndHeaders(a, b model.RequestVersion) bool {
	return a.Body.JSON == b.Body.JSON && a.Body.Text == b.Body.Text && a.Body.File == b.Body.File &&
		slices.Equal(a.Body.Form, b.Body.Form) && len(a.Body.Multipart) == len(b.Body.Multipart) &&
		slices.Equal(a.Headers, b.Headers) && slices.EqualFunc(a.Body.Multipart, b.Body.Multipart, func(x, y model.Field) bool { return x.Key == y.Key })
}

func setText(b *model.Body, text string, f BodyFormat) {
	if f == FormatJSON {
		b.JSON = text
		return
	}
	b.Text = text
}

// withManagedType sets (or, for JSON, removes) the Content-Type the picker
// manages, leaving any other value the user wrote alone.
func withManagedType(hs []model.KeyValue, f BodyFormat) []model.KeyValue {
	out := slices.Clone(hs)
	i := slices.IndexFunc(out, func(kv model.KeyValue) bool { return strings.EqualFold(kv.Key, contentTypeHeader) })
	want := formatType(f)
	switch {
	case i >= 0 && !isManagedType(out[i].Value):
		return out // the user's own
	case i >= 0 && want == "":
		return slices.Delete(out, i, i+1)
	case i >= 0:
		out[i].Value = want
	case want != "":
		out = append(out, model.KeyValue{Key: contentTypeHeader, Value: want})
	}
	return out
}

func isManagedType(v string) bool {
	return slices.Contains([]string{"application/xml", "application/yaml", "text/plain"}, strings.ToLower(strings.TrimSpace(v)))
}

func fieldsOf(rows []model.KeyValue) []model.Field {
	out := make([]model.Field, len(rows))
	for i, kv := range rows {
		out[i] = model.Field{Key: kv.Key, Value: kv.Value}
	}
	return out
}

func rowsOf(fields []model.Field) []model.KeyValue {
	var out []model.KeyValue
	for _, f := range fields {
		if s, ok := f.Value.(string); ok {
			out = append(out, model.KeyValue{Key: f.Key, Value: s})
		}
	}
	return out
}
