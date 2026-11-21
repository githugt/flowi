// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"

	"github.com/githugt/flowi/app/internal/vm"
)

func TestResponseKindOf_Given_each_kind_of_body_Then_the_kind_follows_the_content_type_and_what_was_kept(t *testing.T) {
	// Arrange
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG"))
	tests := []struct {
		name string
		body engine.Body
		want vm.ResponseKind
	}{
		{"empty", engine.Body{}, vm.RespNone},
		{"json", engine.Body{JSON: map[string]any{"a": 1}, ContentType: "application/json", Size: 8}, vm.RespJSON},
		{"image", engine.Body{Base64: png, ContentType: "image/png", Size: 4}, vm.RespImage},
		{"pdf", engine.Body{Base64: png, ContentType: "application/pdf", Size: 4}, vm.RespPDF},
		{"html with a charset", engine.Body{Text: "<p>hi</p>", ContentType: "text/html; charset=utf-8", Size: 9}, vm.RespHTML},
		{"xml", engine.Body{Text: "<a/>", ContentType: "application/atom+xml", Size: 4}, vm.RespXML},
		{"plain text", engine.Body{Text: "hello", ContentType: "text/plain", Size: 5}, vm.RespText},
		{"binary", engine.Body{Base64: png, ContentType: "application/octet-stream", Size: 4}, vm.RespBinary},
		{"a download saved to a file", engine.Body{File: "/tmp/x.zip", ContentType: "application/zip", Size: 9999999}, vm.RespBinary},
		{"text with no content type", engine.Body{Text: "hello", Size: 5}, vm.RespText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := vm.ResponseKindOf(tt.body)

			// Assert
			if got != tt.want {
				t.Fatalf("kind %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBodyImage_Given_a_picture_Then_a_data_address_comes_back_and_big_or_unknown_ones_do_not(t *testing.T) {
	// Arrange
	data := base64.StdEncoding.EncodeToString([]byte("GIF89a"))

	// Act
	gif := vm.BodyImage(engine.Body{Base64: data, ContentType: "image/gif", Size: 6})
	svg := vm.BodyImage(engine.Body{Text: "<svg/>", ContentType: "image/svg+xml", Size: 6})
	big := vm.BodyImage(engine.Body{Base64: data, ContentType: "image/png", Size: vm.MaxImageBytes + 1})
	notImage := vm.BodyImage(engine.Body{Base64: data, ContentType: "application/pdf", Size: 6})
	odd := vm.BodyImage(engine.Body{Base64: data, ContentType: "image/tiff", Size: 6})

	// Assert
	if gif != "data:image/gif;base64,"+data || !strings.HasPrefix(svg, "data:image/svg+xml;base64,") {
		t.Fatalf("gif %q svg %q", gif, svg)
	}
	if big != "" || notImage != "" || odd != "" {
		t.Fatalf("big %q notImage %q odd %q", big, notImage, odd)
	}
}

func TestBodyHTML_Given_a_page_Then_its_source_comes_back_unless_it_is_too_big_or_not_html(t *testing.T) {
	page := engine.Body{Text: "<h1>Hi</h1>", ContentType: "text/html", Size: 11}

	if got := vm.BodyHTML(page); got != "<h1>Hi</h1>" {
		t.Fatalf("page %q", got)
	}
	page.Size = vm.MaxHTMLBytes + 1
	if got := vm.BodyHTML(page); got != "" {
		t.Fatalf("big page %q", got)
	}
	if got := vm.BodyHTML(engine.Body{Text: "x", ContentType: "text/plain", Size: 1}); got != "" {
		t.Fatalf("plain %q", got)
	}
}

func TestBodyHex_Given_bytes_Then_a_dump_with_offsets_and_text_comes_back_cut_at_the_limit(t *testing.T) {
	// Arrange
	raw := []byte("PK\x03\x04hello, world!!0123")
	b := engine.Body{Base64: base64.StdEncoding.EncodeToString(raw), ContentType: "application/zip", Size: int64(len(raw))}

	// Act
	dump := vm.BodyHex(b)
	long := vm.BodyHex(engine.Body{Base64: base64.StdEncoding.EncodeToString(make([]byte, vm.HexBytes*2))})

	// Assert
	lines := strings.Split(dump, "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "00000000  50 4b 03 04 68 65 6c 6c") || !strings.HasSuffix(lines[0], "|PK..hello, world|") {
		t.Fatalf("dump %q", dump)
	}
	if !strings.HasPrefix(lines[1], "00000010  21 21 30 31 32 33") || !strings.HasSuffix(lines[1], "|!!0123|") {
		t.Fatalf("second line %q", lines[1])
	}
	if n := len(strings.Split(long, "\n")); n != vm.HexBytes/16 {
		t.Fatalf("long dump has %d lines", n)
	}
	if vm.BodyHex(engine.Body{Text: "x"}) != "" {
		t.Fatal("text has no bytes to dump")
	}
}
