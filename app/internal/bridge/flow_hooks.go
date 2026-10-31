// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// HookTick is one hook that runs around a request step of a flow: before the
// request (pre) or after its response (post). State is "" until a run reaches
// it, then how it went: passed, failed and so on.
type HookTick struct {
	Phase string `json:"phase"`
	Level string `json:"level"` // collection, folder, request or version
	Label string `json:"label"` // the hook flow's path, or "inline"
	State string `json:"state"`
}

// hookTicksOf lists the hooks that run around request step n of the flow, from
// every level (collection, folders, request, version) in the order they run, with
// how each went in the last run. A step whose request cannot be found has none.
func (s *FlowService) hookTicksOf(o *openFlow, n model.Node) []HookTick {
	if n.Type != "request" || n.Request == nil {
		return nil
	}
	coll, req, ok := s.requestOf(o.doc.Collection(), n.Request.Request)
	if !ok {
		return nil
	}
	version, ok := versionOf(req, n.Request.Version)
	if !ok {
		return nil
	}
	chain := engine.ChainFor(coll, req, version)
	var out []HookTick
	for _, phase := range []struct {
		name string
		refs []engine.HookRef
	}{{"pre", chain.Pre}, {"post", chain.Post}} {
		position := map[string]int{} // the index of a hook within its level
		for _, h := range phase.refs {
			i := position[h.Level]
			position[h.Level]++
			out = append(out, HookTick{Phase: phase.name, Level: h.Level, Label: h.Label(), State: o.run.HookState(n.ID, vm.HookKey(phase.name, h.Level, i))})
		}
	}
	return out
}

// requestOf finds the request a flow step calls: in the flow's own collection, or
// in the one the reference names.
func (s *FlowService) requestOf(own model.Collection, ref model.Ref) (model.Collection, model.Request, bool) {
	for _, c := range s.app.workspace.Collections() {
		if ref.Collection != "" && c.Manifest.Name != ref.Collection && string(c.Manifest.ID) != ref.Collection {
			continue
		}
		if ref.Collection == "" && c.Manifest.ID != own.Manifest.ID {
			continue
		}
		i := slices.IndexFunc(c.Requests, func(r model.Request) bool {
			return (ref.ID != "" && r.ID == ref.ID) || (ref.ID == "" && ref.Path != "" && r.Path == ref.Path)
		})
		if i >= 0 {
			return c, c.Requests[i], true
		}
	}
	return model.Collection{}, model.Request{}, false
}

// versionOf is the version of req a step runs: the one it pins, else the request's
// default, else the first.
func versionOf(req model.Request, pinned string) (model.RequestVersion, bool) {
	for _, label := range []string{pinned, req.Default} {
		if i := slices.IndexFunc(req.Versions, func(v model.RequestVersion) bool { return label != "" && v.Label == label }); i >= 0 {
			return req.Versions[i], true
		}
	}
	if len(req.Versions) == 0 {
		return model.RequestVersion{}, false
	}
	return req.Versions[0], true
}
