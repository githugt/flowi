// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"net/http"
)

// routeInfo is one line of GET /__flowi/routes.
type routeInfo struct {
	Method    string   `json:"method"`
	Pattern   string   `json:"pattern"`
	Request   string   `json:"request"`
	Version   string   `json:"version"`
	Default   bool     `json:"default"`
	Responses []string `json:"responses"`
}

// control serves the replayer's own endpoints under /__flowi/:
// GET traffic, GET routes, POST reset.
func (h *Handler) control(w http.ResponseWriter, r *http.Request) {
	switch name := r.URL.Path[len(controlPrefix):]; {
	case name == "traffic" && r.Method == http.MethodGet:
		entries := []Entry{}
		if h.opts.Log != nil {
			entries = h.opts.Log.Entries()
		}
		writeJSON(w, r, http.StatusOK, entries)
	case name == "routes" && r.Method == http.MethodGet:
		writeJSON(w, r, http.StatusOK, h.routes())
	case name == "reset" && r.Method == http.MethodPost:
		h.Reset()
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, r, http.StatusNotFound, map[string]any{"flowi": true, "error": "unknown control endpoint " + r.URL.Path})
	}
}

func (h *Handler) routes() []routeInfo {
	out := []routeInfo{}
	for _, rt := range h.idx.Load().routes {
		info := routeInfo{Method: rt.method, Pattern: rt.pattern(), Request: rt.req.Path, Version: rt.ver.Label, Default: rt.isDefault, Responses: []string{}}
		for _, resp := range responsesFor(rt, h.opts.DataDir, func(string, error) {}) {
			info.Responses = append(info.Responses, resp.name)
		}
		out = append(out, info)
	}
	return out
}
