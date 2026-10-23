// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
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

// Multipart reports whether a form body is sent as multipart/form-data.
func (r *Request) Multipart() bool {
	return len(r.Current().Body.Multipart) > 0 || (r.chosenMultipart && r.BodyKind() == BodyForm)
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
