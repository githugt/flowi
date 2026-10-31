// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
)

// Mask replaces secret values wherever they are shown or saved.
const Mask = "••••••"

// Masker replaces known secret values, in the forms they commonly take on
// the wire, with Mask. It is immutable; the zero value masks nothing.
type Masker struct {
	values   []string
	replacer *strings.Replacer
}

// NewMasker returns a masker for values; empty values are ignored.
func NewMasker(values ...string) Masker {
	var m Masker
	return m.With(values...)
}

// With returns a masker that also masks values.
func (m Masker) With(values ...string) Masker {
	all := slices.Clone(m.values)
	for _, v := range values {
		if v != "" && !slices.Contains(all, v) {
			all = append(all, v)
		}
	}
	return Masker{values: all, replacer: replacerFor(all)}
}

// String masks every known secret in s.
func (m Masker) String(s string) string {
	if m.replacer == nil {
		return s
	}
	return m.replacer.Replace(s)
}

// Bytes masks every known secret in b.
func (m Masker) Bytes(b []byte) []byte {
	if m.replacer == nil {
		return b
	}
	return []byte(m.replacer.Replace(string(b)))
}

// replacerFor builds a replacer over every form of every value, longest
// first: a secret that contains another must be masked whole, not around
// the shorter one.
func replacerFor(values []string) *strings.Replacer {
	if len(values) == 0 {
		return nil
	}
	var forms []string
	for _, v := range values {
		for _, f := range encodedForms(v) {
			if !slices.Contains(forms, f) {
				forms = append(forms, f)
			}
		}
	}
	slices.SortStableFunc(forms, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	pairs := make([]string, 0, 2*len(forms))
	for _, f := range forms {
		pairs = append(pairs, f, Mask)
	}
	return strings.NewReplacer(pairs...)
}

// encodedForms lists how a value can appear: raw, inside a JSON string
// (with and without HTML escaping), URL query and path escaped, and
// base64 (Basic auth headers carry base64 of "user:password", which
// taint tracking catches; this covers a bare base64 secret).
func encodedForms(v string) []string {
	forms := []string{v, url.QueryEscape(v), url.PathEscape(v), base64.StdEncoding.EncodeToString([]byte(v))}
	for _, escapeHTML := range []bool{false, true} {
		forms = append(forms, jsonEscaped(v, escapeHTML))
	}
	return forms
}

func jsonEscaped(v string, escapeHTML bool) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(escapeHTML)
	_ = enc.Encode(v) // encoding a string cannot fail
	s := strings.TrimSuffix(buf.String(), "\n")
	return s[1 : len(s)-1]
}
