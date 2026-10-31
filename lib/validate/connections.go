// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkConnections reports request steps that misuse a connection kept open:
// open and over together, close without over, a request that is not an async
// request, and an over that no step of the same flow opens (a warning: a flow
// that calls this one may have opened it).
func checkConnections(ix *index, c model.Collection) diag.List {
	var out diag.List
	opened := openedByFlow(c)
	for _, n := range sites.AllNodes(c) {
		call := n.Value.Request
		if call == nil || (call.Open == "" && call.Over == "" && !call.Close) {
			continue
		}
		at := func(key string) diag.Location { return diag.Location{File: n.File, Path: yamlpath.Key(n.Path, key)} }
		fail := func(key, detail string) {
			out = append(out, diag.New(diag.CodeAsyncConn, at(key), map[string]string{"detail": detail}))
		}
		switch {
		case call.Open != "" && call.Over != "":
			fail("over", "a step either opens a connection (open) or runs over one (over), not both")
		case call.Close && call.Over == "":
			fail("close", "close ends the connection that over names; this step has no over")
		}
		if !requestIsAsync(ix, c, call) {
			fail(connectionKey(call), "only an async request (WebSocket, MQTT 5 or NATS) can be kept open between steps")
		}
		if call.Over != "" && !opened[n.File][call.Over] {
			out = append(out, diag.New(diag.CodeAsyncConnName, at("over"), map[string]string{"name": call.Over}))
		}
	}
	return out
}

func connectionKey(call *model.RequestCall) string {
	if call.Over != "" {
		return "over"
	}
	if call.Open != "" {
		return "open"
	}
	return "close"
}

// openedByFlow lists, for each file, the connection names its steps open.
func openedByFlow(c model.Collection) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, n := range sites.AllNodes(c) {
		if call := n.Value.Request; call != nil && call.Open != "" {
			if out[n.File] == nil {
				out[n.File] = map[string]bool{}
			}
			out[n.File][call.Open] = true
		}
	}
	return out
}

// requestIsAsync reports whether every version of the request a step calls
// is an async request; a request that cannot be found is left to checkRef.
func requestIsAsync(ix *index, from model.Collection, call *model.RequestCall) bool {
	target, code, _ := ix.targetCollection(from, call.Request.Collection)
	if code != "" {
		return true
	}
	r, ok := ix.requests[entityKey{target, call.Request.ID}]
	if !ok {
		return true
	}
	for _, v := range r.Versions {
		if v.Async == nil {
			return false
		}
	}
	return true
}
