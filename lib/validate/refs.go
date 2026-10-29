// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/flowioverlay"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

type refKind int

const (
	refRequest refKind = iota
	refFlow
)

// refSite is one reference with where it lives. For request nodes, pinned
// is the node's version and pinnedPath its YAML path.
type refSite struct {
	file, path string
	ref        model.Ref
	kind       refKind
	pinned     string
	pinnedPath string
}

func refSites(c model.Collection) []refSite {
	var out []refSite
	for _, n := range sites.AllNodes(c) {
		switch {
		case n.Value.Request != nil:
			out = append(out, refSite{
				file: n.File, path: yamlpath.Key(n.Path, "request"), ref: n.Value.Request.Request, kind: refRequest,
				pinned: n.Value.Request.Version, pinnedPath: yamlpath.Key(n.Path, "version"),
			})
		case n.Value.Subflow != nil:
			out = append(out, refSite{file: n.File, path: yamlpath.Key(n.Path, "flow"), ref: n.Value.Subflow.Flow, kind: refFlow})
		}
	}
	for _, s := range sites.HookSteps(c) {
		if s.Value.Flow != nil {
			out = append(out, refSite{file: s.File, path: yamlpath.Key(s.Path, "flow"), ref: *s.Value.Flow, kind: refFlow})
		}
	}
	return out
}

// checkOverlayNames reports request steps that name a flowi overlay their
// request's collection does not have (FLW-OVL-002).
func checkOverlayNames(ix *index, c model.Collection) diag.List {
	var out diag.List
	for _, n := range sites.AllNodes(c) {
		call := n.Value.Request
		if call == nil || call.Overlay == "" || call.Request.ID == "" {
			continue
		}
		target, code, _ := ix.targetCollection(c, call.Request.Collection)
		if code != "" {
			continue // the reference is reported by checkRef
		}
		if _, missing := flowioverlay.Resolve(ix.collections[ix.byID[target]].Overlays, call.Overlay, ""); missing != "" {
			out = append(out, diag.New(diag.CodeOverlayNotFound, diag.Location{File: n.File, Path: yamlpath.Key(n.Path, "overlay")}, map[string]string{"name": missing}))
		}
	}
	return out
}

func checkRefs(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		out = append(out, checkOverlayNames(ix, c)...)
		out = append(out, checkConnections(ix, c)...)
		for _, site := range refSites(c) {
			if site.ref.ID == "" {
				continue // the codec already reported the missing id
			}
			out = append(out, ix.checkRef(c, site)...)
		}
	}
	return out
}

func (ix *index) checkRef(from model.Collection, site refSite) diag.List {
	at := func(key string) diag.Location {
		return diag.Location{File: site.file, Path: yamlpath.Key(site.path, key)}
	}
	target, code, args := ix.targetCollection(from, site.ref.Collection)
	if code != "" {
		return diag.List{diag.New(code, at("collection"), args)}
	}
	key := entityKey{target, site.ref.ID}
	if site.kind == refFlow {
		f, ok := ix.flows[key]
		if !ok {
			return diag.List{diag.New(diag.CodeRefUnknownFlow, at("id"), map[string]string{"id": string(site.ref.ID)})}
		}
		return stalePath(site, f.Path, at("path"))
	}
	r, ok := ix.requests[key]
	if !ok {
		return diag.List{diag.New(diag.CodeRefUnknownRequest, at("id"), map[string]string{"id": string(site.ref.ID)})}
	}
	out := stalePath(site, r.Path, at("path"))
	if site.pinned != "" && !hasVersion(r, site.pinned) {
		loc := diag.Location{File: site.file, Path: site.pinnedPath}
		out = append(out, diag.New(diag.CodeVersionPinnedAbsent, loc, map[string]string{"label": site.pinned, "request": r.Path}))
	}
	return out
}

func stalePath(site refSite, want string, loc diag.Location) diag.List {
	if site.ref.Path == "" || site.ref.Path == want {
		return nil
	}
	return diag.List{diag.New(diag.CodeRefStalePath, loc, map[string]string{"got": site.ref.Path, "want": want})}
}
