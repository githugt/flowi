// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"net/http"
	"testing"

	"github.com/githugt/flowi/lib/internal/httpx"
)

func bodyOf(ctype string, raw []byte) Body {
	h := http.Header{}
	if ctype != "" {
		h.Set("Content-Type", ctype)
	}
	return responseBody(&httpx.Response{Header: h, Body: raw, Size: int64(len(raw))})
}

// Before: only valid UTF-8 counted as text, so a Latin-1 or Shift-JIS page showed as binary, and a BOM made a JSON
// body fail to parse.
func TestResponseBody_Given_a_declared_charset_Then_text_is_decoded(t *testing.T) {
	cases := []struct {
		name    string
		ctype   string
		raw     []byte
		text    string
		charset string
	}{
		{"latin-1", "text/plain; charset=iso-8859-1", []byte{'c', 'a', 'f', 0xE9}, "café", "windows-1252"},
		{"windows-1252 quoted", `text/html; charset="Windows-1252"`, []byte{0x93, 'h', 'i', 0x94}, "“hi”", "windows-1252"},
		{"shift-jis", "text/plain; charset=shift_jis", []byte{0x82, 0xA0}, "あ", "shift_jis"},
		{"utf-8 declared", "text/plain; charset=UTF-8", []byte("héllo"), "héllo", ""},
		{"utf-8 bom", "text/plain", append([]byte{0xEF, 0xBB, 0xBF}, "hi"...), "hi", ""},
		{"utf-16le bom", "text/plain", []byte{0xFF, 0xFE, 'h', 0, 'i', 0}, "hi", "utf-16le"},
		{"utf-16be bom", "text/plain", []byte{0xFE, 0xFF, 0, 'h', 0, 'i'}, "hi", "utf-16be"},
		{"unknown charset falls back to utf-8", "text/plain; charset=x-nope", []byte("ok"), "ok", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := bodyOf(c.ctype, c.raw)
			if b.Text != c.text || b.Charset != c.charset || b.Base64 != "" {
				t.Fatalf("Text=%q Charset=%q Base64=%q, want %q %q", b.Text, b.Charset, b.Base64, c.text, c.charset)
			}
		})
	}
}

func TestResponseBody_Given_a_json_body_with_a_bom_Then_it_is_parsed(t *testing.T) {
	raw := append([]byte{0xEF, 0xBB, 0xBF}, `{"a":1}`...)

	b := bodyOf("application/json", raw)

	if b.JSON == nil || b.ParseErr != "" {
		t.Fatalf("JSON=%v ParseErr=%q, want it parsed", b.JSON, b.ParseErr)
	}
}

func TestResponseBody_Given_binary_with_no_charset_Then_it_stays_base64(t *testing.T) {
	b := bodyOf("application/octet-stream", []byte{0xFF, 0x00, 0xC3})

	if b.Base64 == "" || b.Text != "" {
		t.Fatalf("Text=%q Base64=%q, want base64 only", b.Text, b.Base64)
	}
}

func TestResponseBody_Given_a_binary_type_that_starts_with_a_utf16_mark_Then_it_is_not_decoded(t *testing.T) {
	// Before (this change's first draft): the bytes FF FE 00 80 in an octet-stream became "耀".
	b := bodyOf("application/octet-stream", []byte{0xFF, 0xFE, 0x00, 0x80})

	if b.Charset != "" || b.Base64 == "" {
		t.Fatalf("Charset=%q Base64=%q, want binary", b.Charset, b.Base64)
	}
}

// Before: the Content-Encoding the transport decoded was dropped, so a 4 KB response on the wire showed as 340 KB.
func TestResponseBody_Given_a_decoded_content_encoding_Then_it_is_recorded(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "text/plain")

	b := responseBody(&httpx.Response{Header: h, Body: []byte("hello"), Size: 5, RawSize: 25, Encoding: "gzip"})

	if b.Encoding != "gzip" || b.RawSize != 25 || b.Size != 5 {
		t.Fatalf("%+v", b)
	}
}
