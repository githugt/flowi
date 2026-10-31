// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// deliver writes an answer.
func (h *Handler) deliver(w http.ResponseWriter, r *http.Request, a answer) {
	hd := w.Header()
	hd.Set(HeaderMatch, a.match)
	if a.route != nil {
		hd.Set("X-Flowi-Request", a.route.req.Path)
		hd.Set("X-Flowi-Version", a.route.ver.Label)
	}
	if len(a.allow) > 0 {
		hd.Set("Allow", strings.Join(a.allow, ", "))
	}
	if a.resp == nil {
		writeJSONAs(w, r, a.status, "application/problem+json", a.problem)
		return
	}
	hd.Set("X-Flowi-Example", a.resp.name)
	hd.Set(HeaderSource, a.resp.source)
	writeResponse(w, r, *a.resp)
}

func writeResponse(w http.ResponseWriter, r *http.Request, resp response) {
	hd := w.Header()
	for _, h := range resp.header {
		if safeHeader(h) {
			hd.Add(h.name, h.value)
		}
	}
	if hd.Get("Content-Type") == "" {
		hd.Set("Content-Type", contentTypeOf(resp))
	}
	hasBody := resp.status != http.StatusNoContent && resp.status != http.StatusNotModified
	if hasBody {
		hd.Set("Content-Length", strconv.Itoa(len(resp.body)))
	}
	w.WriteHeader(resp.status)
	if hasBody && r.Method != http.MethodHead {
		_, _ = w.Write(resp.body) // the caller hung up: nothing to do about it
	}
}

func contentTypeOf(resp response) string {
	switch resp.kind {
	case bodyJSON:
		return "application/json"
	case bodyBytes:
		return "application/octet-stream"
	}
	return "text/plain; charset=utf-8"
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	writeJSONAs(w, r, status, "application/json", v)
}

func writeJSONAs(w http.ResponseWriter, r *http.Request, status int, contentType string, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		body = []byte(`{"flowi":true,"error":"could not encode the answer"}`)
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body) // the caller hung up: nothing to do about it
	}
}
