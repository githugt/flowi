// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/hex"
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

// minEncodedLen is the shortest extra encoding worth masking. A 3-character
// secret in hex is 6 characters that can turn up in a timestamp or an id, and
// masking those would change text that is not secret.
const minEncodedLen = 8

// encodedForms lists how a value can appear: raw, inside a JSON string
// (with and without HTML escaping), URL query and path escaped, and
// base64 (Basic auth headers carry base64 of "user:password", which
// taint tracking catches; this covers a bare base64 secret). Base64 also
// comes URL-safe and unpadded (JWT segments, signed URLs), and as hex
// (digests, signatures); those longer-than-minEncodedLen forms are added too.
//
// Before: only raw, escaped and padded standard base64 were masked, so a
// secret echoed back as base64url or hex was shown in the response.
func encodedForms(v string) []string {
	raw := []byte(v)
	forms := []string{v, url.QueryEscape(v), url.PathEscape(v), base64.StdEncoding.EncodeToString(raw)}
	for _, f := range []string{
		base64.RawStdEncoding.EncodeToString(raw),
		base64.URLEncoding.EncodeToString(raw),
		base64.RawURLEncoding.EncodeToString(raw),
		hex.EncodeToString(raw),
		strings.ToUpper(hex.EncodeToString(raw)),
	} {
		if len(f) >= minEncodedLen {
			forms = append(forms, f)
		}
	}
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
