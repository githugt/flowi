// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"path"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/openapi"
)

// Where an import lands. Every importer names its requests and flows by a path
// below requests/ and flows/ (a Postman folder, an OpenAPI tag, a gRPC service);
// ImportSource.Into puts all of them in one folder of the collection, nested as
// deep as the person likes ("demo/websocket"). The folder is made by the import
// when it is not there yet. Without Into the paths are the importer's own, as before.

// cleanInto checks a target folder and returns it as slash-separated segments with no
// leading or trailing slash. "" is the top of the collection.
func cleanInto(into string) (string, error) {
	into = strings.Trim(strings.ReplaceAll(strings.TrimSpace(into), `\`, "/"), "/")
	if into == "" {
		return "", nil
	}
	if detail := requestPathProblem(into); detail != "" {
		return "", diagErr(diag.CodeWriteRequestPath, map[string]string{"path": into, "detail": detail})
	}
	return into, nil
}

// joinInto is p inside the folder into ("" for the top).
func joinInto(into, p string) string {
	if into == "" {
		return p
	}
	return path.Join(into, p)
}

func intoRequests(reqs []model.Request, into string) {
	if into == "" {
		return
	}
	for i := range reqs {
		reqs[i].Path = joinInto(into, reqs[i].Path)
	}
}

// intoFlows moves flows into the folder. refs says the requests the flows call are
// new ones moved the same way, so their paths follow; the requests of Mermaid and
// Arazzo flows are the collection's own, which stay where they are.
func intoFlows(flows []model.Flow, into string, refs bool) {
	if into == "" {
		return
	}
	for i := range flows {
		flows[i].Path = joinInto(into, flows[i].Path)
		if refs {
			flows[i].Nodes = intoRefs(flows[i].Nodes, into)
		}
	}
}

// intoRefs prefixes the path of every request call of this collection that has one.
// It returns a copy: the importer's nodes are not changed.
func intoRefs(nodes []model.Node, into string) []model.Node {
	out := make([]model.Node, len(nodes))
	copy(out, nodes)
	for i, n := range out {
		if n.Request != nil && n.Request.Request.Collection == "" && n.Request.Request.Path != "" {
			call := *n.Request
			call.Request.Path = joinInto(into, call.Request.Path)
			out[i].Request = &call
		}
	}
	return out
}

// intoResult moves everything an importer produced into the folder.
func intoResult(r importers.Result, into string, kind ImportKind) importers.Result {
	if into == "" {
		return r
	}
	r.Requests = append([]model.Request(nil), r.Requests...)
	intoRequests(r.Requests, into)
	r.Flows = append([]model.Flow(nil), r.Flows...)
	intoFlows(r.Flows, into, kind != ImportMermaid)
	r.Examples = append([]importers.Example(nil), r.Examples...)
	for i := range r.Examples {
		r.Examples[i].RequestPath = joinInto(into, r.Examples[i].RequestPath)
	}
	return r
}

// intoOpenAPI moves an OpenAPI import into the folder; the response schemas are kept under the request's new path.
func intoOpenAPI(r *openapi.Result, into string) {
	if into == "" {
		return
	}
	intoRequests(r.Requests, into)
	schemas := make(map[string]map[string]map[string]any, len(r.Schemas))
	for p, s := range r.Schemas {
		schemas[joinInto(into, p)] = s
	}
	r.Schemas = schemas
	intoFlows(r.Flows, into, true)
}

// intoBase re-keys the previous import's requests the way this one is moved, so a re-import into
// the same folder still merges against what the first one made.
func intoBase(base map[string]model.Request, into string) map[string]model.Request {
	if into == "" || base == nil {
		return base
	}
	out := make(map[string]model.Request, len(base))
	for p, r := range base {
		out[joinInto(into, p)] = r
	}
	return out
}
