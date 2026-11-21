// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/base64"
	"fmt"
	"mime"
	"strings"

	"github.com/githugt/flowi/lib/engine"
)

// ResponseKind is what kind of thing a response body is, so the window picks a viewer for it.
type ResponseKind string

// The kinds. Text is any body that is text and is none of the others.
const (
	RespNone   ResponseKind = "none"
	RespJSON   ResponseKind = "json"
	RespXML    ResponseKind = "xml"
	RespHTML   ResponseKind = "html"
	RespImage  ResponseKind = "image"
	RespPDF    ResponseKind = "pdf"
	RespText   ResponseKind = "text"
	RespBinary ResponseKind = "binary"
)

// Limits on what is sent to the window to draw: a bigger body shows as its card (and Save body… writes it out).
const (
	MaxImageBytes = 4 << 20 // an image shown in the response
	MaxHTMLBytes  = 1 << 20 // a page shown in the preview
	HexBytes      = 2048    // the first bytes of a binary body shown as a hex dump
)

// imageTypes are the picture types a browser engine draws from a data address. SVG is among them: loaded as an image
// it runs no script.
var imageTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "image/bmp": true,
	"image/svg+xml": true, "image/x-icon": true, "image/vnd.microsoft.icon": true, "image/avif": true,
}

// ResponseKindOf classifies a body by its content type and what the record kept of it.
// Before: the view model had no content type or kind for the body, so images, PDFs and binary bodies were drawn as
// text like everything else.
func ResponseKindOf(b engine.Body) ResponseKind {
	media := mediaType(b.ContentType)
	switch {
	case b.Size == 0 && b.Text == "" && b.Base64 == "" && b.JSON == nil && b.File == "":
		return RespNone
	case b.JSON != nil:
		return RespJSON
	case media == "application/pdf":
		return RespPDF
	case strings.HasPrefix(media, "image/"):
		return RespImage
	case media == "text/html" || media == "application/xhtml+xml":
		return RespHTML
	case media == "text/xml" || media == "application/xml" || strings.HasSuffix(media, "+xml"):
		return RespXML
	case b.Base64 != "" || b.File != "" && b.Text == "":
		return RespBinary
	case b.Text != "":
		return RespText
	}
	return RespBinary
}

func mediaType(contentType string) string {
	m, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(contentType))
	}
	return m
}

// BodyImage is the picture as a data address for an <img>, or "" when the body is not a picture the window can draw,
// is too big, or was not kept (a streamed download).
func BodyImage(b engine.Body) string {
	media := mediaType(b.ContentType)
	if !imageTypes[media] || b.Size > MaxImageBytes {
		return ""
	}
	switch {
	case b.Base64 != "":
		return "data:" + media + ";base64," + b.Base64
	case b.Text != "":
		return "data:" + media + ";base64," + base64.StdEncoding.EncodeToString([]byte(b.Text)) // SVG is text
	}
	return ""
}

// BodyHTML is the page's source for the sandboxed preview, or "" when it is too big or not text.
func BodyHTML(b engine.Body) string {
	if ResponseKindOf(b) != RespHTML || b.Size > MaxHTMLBytes {
		return ""
	}
	return b.Text
}

// BodyHex is a hex dump of the first bytes of a binary body, sixteen to a line: offset, bytes, then the same bytes as
// text with a dot for anything that is not printable. "" when the bytes were not kept.
func BodyHex(b engine.Body) string {
	if b.Base64 == "" {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(b.Base64)
	if err != nil {
		return ""
	}
	data = data[:min(len(data), HexBytes)]
	var out strings.Builder
	for off := 0; off < len(data); off += 16 {
		end := min(off+16, len(data))
		fmt.Fprintf(&out, "%08x  ", off)
		for i := off; i < off+16; i++ {
			if i < end {
				fmt.Fprintf(&out, "%02x ", data[i])
			} else {
				out.WriteString("   ")
			}
			if i == off+7 {
				out.WriteByte(' ')
			}
		}
		out.WriteString(" |")
		for _, c := range data[off:end] {
			if c < 0x20 || c > 0x7e {
				c = '.'
			}
			out.WriteByte(c)
		}
		out.WriteString("|\n")
	}
	return strings.TrimRight(out.String(), "\n")
}
