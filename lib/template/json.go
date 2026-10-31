// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"strings"
)

// RenderJSON renders a JSON body template so the result stays valid JSON
// whatever the values contain. Each placeholder is encoded by where it
// sits in the JSON text:
//
//   - the whole content of a string literal ("{{x}}"): a string becomes a
//     JSON string, nil becomes null, anything else a quoted string;
//   - inside a longer string literal ("Bearer {{x}}"): the text is escaped;
//   - outside any string ({"n": {{x}}}): the value is JSON-encoded.
func RenderJSON(src string, s Scope) (Result, error) {
	t, err := Parse(src)
	if err != nil {
		return Result{}, err
	}
	w := jsonWriter{}
	var errs []error
	for i, seg := range t.segments {
		if seg.expr == nil {
			w.text(seg.text)
			continue
		}
		r, err := seg.expr.run(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		w.tainted = w.tainted || r.Tainted
		w.value(r.Value, closesStringNext(t.segments, i))
	}
	if len(errs) > 0 {
		return Result{}, errors.Join(errs...)
	}
	return Result{Value: w.out.String(), Tainted: w.tainted}, nil
}

// closesStringNext reports whether the segment after i is text starting
// with the quote that would close a string opened just before i.
func closesStringNext(segs []segment, i int) bool {
	return i+1 < len(segs) && segs[i+1].expr == nil && strings.HasPrefix(segs[i+1].text, `"`)
}

// jsonWriter tracks whether the output is inside a JSON string literal.
type jsonWriter struct {
	out         strings.Builder
	inString    bool
	stringEmpty bool // inside a string that has no content yet
	skipQuote   bool // the next text's leading quote was already written
	tainted     bool
}

func (w *jsonWriter) text(s string) {
	if w.skipQuote {
		s = s[1:]
		w.skipQuote, w.inString = false, false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		w.out.WriteByte(c)
		switch {
		case w.inString && c == '\\' && i+1 < len(s):
			i++
			w.out.WriteByte(s[i])
			w.stringEmpty = false
		case c == '"':
			w.inString = !w.inString
			w.stringEmpty = w.inString
		case w.inString:
			w.stringEmpty = false
		}
	}
}

func (w *jsonWriter) value(v any, closesNext bool) {
	switch {
	case w.inString && w.stringEmpty && closesNext:
		w.wholeLiteral(v)
	case w.inString:
		quoted := marshalJSON(stringify(v))
		w.out.Write(quoted[1 : len(quoted)-1])
		w.stringEmpty = false
	default:
		w.out.Write(marshalJSON(v))
	}
}

// wholeLiteral replaces "{{x}}" (quotes included) with the encoded value.
func (w *jsonWriter) wholeLiteral(v any) {
	written := w.out.String()
	w.out.Reset()
	w.out.WriteString(written[:len(written)-1]) // drop the opening quote
	switch x := v.(type) {
	case nil:
		w.out.WriteString("null")
	case string:
		w.out.Write(marshalJSON(x))
	default:
		w.out.Write(marshalJSON(stringify(x)))
	}
	w.skipQuote = true
}
