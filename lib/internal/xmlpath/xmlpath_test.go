// SPDX-License-Identifier: Apache-2.0

package xmlpath

import (
	"slices"
	"strings"
	"testing"
)

const multistatus = `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/files/</D:href>
    <D:propstat>
      <D:prop><D:displayname>files</D:displayname><D:resourcetype><D:collection/></D:resourcetype></D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response id="b">
    <D:href>/files/a.txt</D:href>
    <D:propstat>
      <D:prop><D:displayname>a &amp; b.txt</D:displayname></D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

func TestSelect_should_read_values_by_path(t *testing.T) {
	tests := []struct {
		name, path string
		want       []string
	}{
		{"an absolute path", "/multistatus/response/href", []string{"/files/", "/files/a.txt"}},
		{"a prefix on the path is ignored, as the server's prefix is its own choice", "/D:multistatus/D:response[2]/D:href", []string{"/files/a.txt"}},
		{"the first of several", "/multistatus/response[1]/href", []string{"/files/"}},
		{"the last", "/multistatus/response[last()]/href", []string{"/files/a.txt"}},
		{"anywhere", "//displayname", []string{"files", "a & b.txt"}},
		{"anywhere, second under its parent", "//response[2]/propstat/prop/displayname", []string{"a & b.txt"}},
		{"by attribute", "//response[@id='b']/href", []string{"/files/a.txt"}},
		{"an attribute's value", "/multistatus/response[2]/@id", []string{"b"}},
		{"an attribute anywhere", "//@id", []string{"b"}},
		{"any element", "/multistatus/*/href", []string{"/files/", "/files/a.txt"}},
		{"the text of an element", "/multistatus/response[1]/href/text()", []string{"/files/"}},
		{"all the text under an element", "/multistatus/response[1]/propstat/prop/displayname", []string{"files"}},
		{"an empty element", "//collection", []string{""}},
		{"nothing", "//nothere", nil},
		{"the wrong root", "/response", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Select(multistatus, tt.path)

			if err != nil || !slices.Equal(got, tt.want) {
				t.Fatalf("%s: got %q err %v, want %q", tt.path, got, err, tt.want)
			}
		})
	}
}

func TestSelect_should_refuse_what_it_cannot_read(t *testing.T) {
	tests := []struct{ name, doc, path, want string }{
		{"a path that is not one", multistatus, "href", "start with /"},
		{"an unclosed predicate", multistatus, "//response[2", "["},
		{"an empty step", multistatus, "//response//", "empty"},
		{"an unsupported predicate", multistatus, "//response[href='x']", "predicate"},
		{"an attribute that is not last", multistatus, "//@id/x", "last"},
		{"a document that is not XML", "{not xml", "//a", "XML"},
		{"an index of zero", multistatus, "//response[0]", "from 1"},
		{"a quote that is not closed", multistatus, "//response[@id='b]", "quote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Select(tt.doc, tt.path)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestSelect_should_be_safe_on_a_hostile_document(t *testing.T) {
	deep := strings.Repeat("<a>", 20000) + strings.Repeat("</a>", 20000)

	_, err := Select(deep, "//a")

	if err == nil || !strings.Contains(err.Error(), "deep") {
		t.Fatalf("a document nested 20000 deep must be refused, err %v", err)
	}
}
