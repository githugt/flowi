// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Before: a request, a flow and a folder had tags and nothing that said where the work stood.
func TestStatus_should_round_trip_on_a_request_a_flow_and_a_folder_beside_the_tags(t *testing.T) {
	c := newCodec()
	req := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXY0\nname: login\ntags:\n  - smoke\nstatus: review\nversions:\n  \"1\":\n    method: GET\n    url: https://example.test\n")
	flow := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXYZ\nname: login\ntags:\n  - smoke\nstatus: testing\nnodes: []\n")
	folder := []byte("tags:\n  - slow\nstatus: in-development\n")

	r, rd := c.DecodeRequest("login.req.yaml", "login", req)
	f, fd := c.DecodeFlow("login.flow.yaml", "login", flow)
	fo, od := c.DecodeFolder("_folder.yaml", "requests/auth", folder)
	rout, rerr := c.EncodeRequest(r)
	fout, ferr := c.EncodeFlow(f)
	oout, oerr := c.EncodeFolder(fo)

	requireNoDiagnostics(t, rd)
	requireNoDiagnostics(t, fd)
	requireNoDiagnostics(t, od)
	if r.Status != "review" || f.Status != "testing" || fo.Status != "in-development" {
		t.Fatalf("request %q flow %q folder %q", r.Status, f.Status, fo.Status)
	}
	if rerr != nil || ferr != nil || oerr != nil {
		t.Fatal(rerr, ferr, oerr)
	}
	if !strings.Contains(string(rout), "tags:\n  - smoke\nstatus: review\n") || !strings.Contains(string(fout), "tags:\n  - smoke\nstatus: testing\n") {
		t.Fatalf("request:\n%s\nflow:\n%s", rout, fout)
	}
	if string(oout) != string(folder) {
		t.Fatalf("folder round trip: %q", oout)
	}
}

func TestStatus_Given_none_Then_no_status_line_is_written(t *testing.T) {
	c := newCodec()

	out, err := c.EncodeFolder(model.Folder{Tags: []string{"slow"}})

	if err != nil || strings.Contains(string(out), "status") {
		t.Fatalf("%q %v", out, err)
	}
}

func TestManifest_should_read_and_write_its_statuses_in_order(t *testing.T) {
	c := newCodec()
	src := []byte("format: 1\nid: 01J9Z8K3M4N5P6Q7R8S9T0VWXY\nstatuses:\n  - name: needs-design\n    color: pink\n    icon: bookmark\n  - name: testing\n    color: red\n")

	m, diags := c.DecodeManifest("flowi.yaml", src)
	out, err := c.EncodeManifest(m)

	requireNoDiagnostics(t, diags)
	want := []model.Status{{Name: "needs-design", Color: "pink", Icon: "bookmark"}, {Name: "testing", Color: "red"}}
	if !slices.Equal(m.Statuses, want) {
		t.Fatalf("got %+v", m.Statuses)
	}
	if err != nil || string(out) != string(src) {
		t.Fatalf("round trip:\n%s\n%v", out, err)
	}
}

func TestManifest_Given_no_statuses_Then_none_are_written(t *testing.T) {
	c := newCodec()

	out, err := c.EncodeManifest(model.Manifest{Format: 1, Meta: model.Meta{ID: "01J9Z8K3M4N5P6Q7R8S9T0VWXY"}})

	if err != nil || strings.Contains(string(out), "statuses") {
		t.Fatalf("%q %v", out, err)
	}
}

func TestManifest_Given_a_status_with_an_unknown_key_Then_it_is_reported(t *testing.T) {
	c := newCodec()

	_, diags := c.DecodeManifest("flowi.yaml", []byte("format: 1\nid: 01J9Z8K3M4N5P6Q7R8S9T0VWXY\nstatuses:\n  - name: a\n    colour: red\n"))

	if len(diags) == 0 || diags[0].Code != diag.CodeYAMLUnknownKey {
		t.Fatalf("got %v", diags)
	}
}
