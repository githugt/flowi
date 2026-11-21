// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"cmp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/model"
)

// FlowWrite is a step of a flow that changes data where it sends.
type FlowWrite struct {
	// Step is the id of the node, inside the flow it is in.
	Step string
	// Request is the path of the request the step sends.
	Request string
	// What is how it writes: "POST", "gRPC call", "MQTT publish", "SQL query that may change data".
	What string
	// Via is the path of the subflow the step is in; empty for the flow itself.
	Via string
}

// FlowWrites lists the steps of flow t (draft, when given, instead of the saved one) that send something that changes
// data: an HTTP request other than GET, HEAD and OPTIONS, a gRPC call, a message sent, a query allowed to write. The
// flows it calls as subflows are followed. It reads the files and never sends: it is for a question before a run in a
// protected environment, where a flow that only reads need not ask. A request it cannot find is not counted, and the
// requests that the hooks of a request send are not followed.
//
// Before: a flow run in a protected environment asked "A flow can send requests that change data", whether or not it
// did, and never said which.
func (w *Workspace) FlowWrites(t FlowTarget, draft *model.Flow) ([]FlowWrite, error) {
	c, f, err := w.flowFor(t)
	if err != nil {
		return nil, err
	}
	if draft != nil {
		f = *draft
	}
	all := w.Collections()
	var out []FlowWrite
	visiting := map[string]bool{}
	var walk func(c model.Collection, f model.Flow, nodes []model.Node, via string)
	walk = func(c model.Collection, f model.Flow, nodes []model.Node, via string) {
		for _, n := range nodes {
			switch {
			case n.Request != nil:
				if what, ok := requestWrites(all, c, f, n.Request); ok {
					out = append(out, FlowWrite{Step: n.ID, Request: requestLabel(all, c, n.Request.Request), What: what, Via: via})
				}
			case n.Subflow != nil:
				sc, sf, ok := findFlowRef(all, c, n.Subflow.Flow)
				if ok && !visiting[sf.File] {
					visiting[sf.File] = true
					walk(sc, sf, sf.Nodes, sf.Path)
					delete(visiting, sf.File)
				}
			}
			for _, children := range n.Children() {
				walk(c, f, children, via)
			}
		}
	}
	visiting[f.File] = true
	walk(c, f, f.Nodes, "")
	return out, nil
}

// requestWrites says how the request a node sends writes, if it does.
func requestWrites(all []model.Collection, c model.Collection, f model.Flow, call *model.RequestCall) (string, bool) {
	_, r, ok := findRequestRef(all, c, call.Request)
	if !ok {
		return "", false
	}
	v, ok := sites.ResolvedVersion(r, call.Version, f.PreferVersion)
	if !ok {
		return "", false
	}
	switch v.Family() {
	case model.FamilyHTTP:
		switch m := strings.ToUpper(v.Method); m {
		case "", "GET", "HEAD", "OPTIONS":
			return "", false
		default:
			return m, true
		}
	case model.FamilyRPC:
		return "gRPC call", true
	case model.FamilyMessaging:
		if v.Exchange().Sends() {
			return strings.ToUpper(v.Async.Protocol) + " message sent", true
		}
	case model.FamilyDataset:
		if v.Dataset != nil && v.Dataset.Writes {
			return "SQL query that may change data", true
		}
	case model.FamilyFile:
		if v.File != nil && v.File.Writes {
			return "data file query that may change the file", true
		}
	}
	return "", false
}

// requestLabel is the path of the request ref names, or the reference as written when it is not found.
func requestLabel(all []model.Collection, c model.Collection, ref model.Ref) string {
	if _, r, ok := findRequestRef(all, c, ref); ok {
		return r.Path
	}
	return cmp.Or(ref.Path, string(ref.ID))
}

// findRequestRef is findRequest that also finds a request by the path a reference gives.
func findRequestRef(all []model.Collection, from model.Collection, ref model.Ref) (model.Collection, model.Request, bool) {
	if c, r, ok := findRequest(all, from, ref); ok {
		return c, r, true
	}
	if ref.Collection == "" && ref.Path != "" {
		if i := slices.IndexFunc(from.Requests, func(r model.Request) bool { return r.Path == ref.Path }); i >= 0 {
			return from, from.Requests[i], true
		}
	}
	return model.Collection{}, model.Request{}, false
}

// findFlowRef resolves a flow reference as seen from collection from, by id or by path.
func findFlowRef(all []model.Collection, from model.Collection, ref model.Ref) (model.Collection, model.Flow, bool) {
	target := from
	if ref.Collection != "" {
		i := slices.IndexFunc(from.Manifest.DependsOn, func(d model.Dependency) bool {
			return d.Name == ref.Collection || string(d.ID) == ref.Collection
		})
		if i < 0 {
			return model.Collection{}, model.Flow{}, false
		}
		j := slices.IndexFunc(all, func(c model.Collection) bool { return c.Manifest.ID == from.Manifest.DependsOn[i].ID })
		if j < 0 {
			return model.Collection{}, model.Flow{}, false
		}
		target = all[j]
	}
	for _, f := range target.Flows {
		if refersTo(ref, f) {
			return target, f, true
		}
	}
	return model.Collection{}, model.Flow{}, false
}
