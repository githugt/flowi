// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/webdav"
)

// A real WebDAV handler answers every verb a person sends with a custom method: the library only has to put the
// method, the headers and the body on the wire as given. CalDAV (MKCALENDAR, REPORT) is not in this handler; see the
// compose file for a CalDAV server.
func TestClient_Given_a_WebDAV_server_Then_each_verb_gets_the_answer_its_standard_gives(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.NewMemFS(), LockSystem: webdav.NewMemLS()})
	defer srv.Close()
	c := newClient(t, defaultOptions())
	send := func(method, path string, header map[string]string, body string) (int, string, http.Header) {
		t.Helper()
		h := http.Header{}
		for k, v := range header {
			h.Set(k, v)
		}
		var b BodySource
		if body != "" {
			b = BytesBody([]byte(body), "application/xml")
		}
		attempts, err := c.Do(context.Background(), Request{Method: method, URL: srv.URL + path, Header: h, Body: b})
		if err != nil || len(attempts) != 1 || attempts[0].Response == nil {
			t.Fatalf("%s %s: %v (%d attempts)", method, path, err, len(attempts))
		}
		res := attempts[0].Response
		return res.Status, string(res.Body), res.Header
	}
	propfind := `<?xml version="1.0" encoding="utf-8"?><D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`

	steps := []struct {
		name, method, path string
		header             map[string]string
		body               string
		wantStatus         int
		wantBody           string // a fragment
	}{
		{name: "make a collection", method: "MKCOL", path: "/docs/", wantStatus: http.StatusCreated},
		{name: "put a file in it", method: "PUT", path: "/docs/a.txt", body: "hello", wantStatus: http.StatusCreated},
		{name: "list it at depth 1", method: "PROPFIND", path: "/docs/", header: map[string]string{"Depth": "1"}, body: propfind, wantStatus: http.StatusMultiStatus, wantBody: "/docs/a.txt"},
		{name: "ask only about itself at depth 0", method: "PROPFIND", path: "/docs/", header: map[string]string{"Depth": "0"}, body: propfind, wantStatus: http.StatusMultiStatus},
		{name: "copy it", method: "COPY", path: "/docs/a.txt", header: map[string]string{"Destination": srv.URL + "/docs/b.txt", "Overwrite": "F"}, wantStatus: http.StatusCreated},
		{name: "refuse to copy over it", method: "COPY", path: "/docs/a.txt", header: map[string]string{"Destination": srv.URL + "/docs/b.txt", "Overwrite": "F"}, wantStatus: http.StatusPreconditionFailed},
		{name: "move it", method: "MOVE", path: "/docs/b.txt", header: map[string]string{"Destination": srv.URL + "/docs/c.txt"}, wantStatus: http.StatusCreated},
		{name: "set a property", method: "PROPPATCH", path: "/docs/c.txt", body: `<D:propertyupdate xmlns:D="DAV:"><D:set><D:prop><Z:note xmlns:Z="urn:z">x</Z:note></D:prop></D:set></D:propertyupdate>`, wantStatus: http.StatusMultiStatus, wantBody: "200 OK"},
	}

	// Act and Assert, in order: each step builds on the one before.
	for _, s := range steps {
		status, body, _ := send(s.method, s.path, s.header, s.body)
		if status != s.wantStatus || !strings.Contains(body, s.wantBody) {
			t.Fatalf("%s: %s %s gave %d %q, want %d containing %q", s.name, s.method, s.path, status, body, s.wantStatus, s.wantBody)
		}
	}

	lockBody := `<?xml version="1.0"?><D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype><D:owner>flowi</D:owner></D:lockinfo>`
	status, _, header := send("LOCK", "/docs/a.txt", map[string]string{"Timeout": "Second-3600"}, lockBody)
	token := header.Get("Lock-Token")
	if status != http.StatusOK || token == "" {
		t.Fatalf("LOCK gave %d with Lock-Token %q", status, token)
	}
	if status, _, _ := send("UNLOCK", "/docs/a.txt", map[string]string{"Lock-Token": token}, ""); status != http.StatusNoContent {
		t.Fatalf("UNLOCK with the token gave %d", status)
	}
}
