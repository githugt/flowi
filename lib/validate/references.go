// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"maps"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// requestBuiltins are the outputs every request node has besides its
// extracted values.
func requestBuiltins() []string { return []string{"status", "headers", "body", "vars", "time"} }

// checkReferences checks what templates read (design §7.1): nodes.<id>
// must be a node that can have finished by then, inputs.<name> a declared
// input, vars.<name> a variable some node writes (FLW-NODE-009); and
// nodes.<request>.<name> an output in that request's contract
// (FLW-NODE-010, a warning — plan Decision 16).
func checkReferences(ix *index) diag.List {
	hookFlows := ix.flowsUsedAsHooks()
	var out diag.List
	for _, c := range ix.collections {
		for _, f := range c.Flows {
			w := newRefWalk(ix, c, f.File, &f, f.Nodes, hookFlows[entityKey{c.Manifest.ID, f.ID}])
			out = append(out, w.walkFlow()...)
		}
		for _, s := range sites.HookSteps(c) {
			if len(s.Value.Inline) > 0 {
				w := newRefWalk(ix, c, s.File, nil, s.Value.Inline, true)
				out = append(out, w.walk(s.Value.Inline, yamlpath.Key(s.Path, "inline"), map[string]bool{})...)
			}
		}
	}
	return out
}

// refWalk checks one root node list (a flow, or an inline hook step).
type refWalk struct {
	ix     *index
	c      model.Collection
	file   string
	flow   *model.Flow
	inputs map[string]bool
	vars   map[string]bool // nil: not checked (hooks read the request's vars)
	nodes  map[string]nodeInfo
}

// nodeInfo is where a node sits: repeated (inside a batch or run_till
// body, so not readable from outside) and its container.
type nodeInfo struct {
	node      model.Node
	repeated  bool
	container string
}

func newRefWalk(ix *index, c model.Collection, file string, f *model.Flow, list []model.Node, hook bool) *refWalk {
	w := &refWalk{ix: ix, c: c, file: file, flow: f, inputs: map[string]bool{}, nodes: map[string]nodeInfo{}}
	if f != nil {
		for _, in := range f.Inputs {
			w.inputs[in.Name] = true
		}
	}
	if !hook {
		w.vars = sites.WrittenVars(list)
	}
	indexNodes(list, false, "", w.nodes)
	return w
}

func indexNodes(list []model.Node, repeated bool, container string, out map[string]nodeInfo) {
	for _, n := range list {
		out[n.ID] = nodeInfo{node: n, repeated: repeated, container: container}
		inner, innerContainer := repeated, container
		if n.Type == "batch" || n.Type == "run_till" {
			inner, innerContainer = true, n.ID
		}
		for _, child := range n.Children() {
			indexNodes(child, inner, innerContainer, out)
		}
	}
}

// walkFlow checks the flow's nodes, then its outputs and run.until, which
// see every top-level node.
func (w *refWalk) walkFlow() diag.List {
	out := w.walk(w.flow.Nodes, yamlpath.Key("$", "nodes"), map[string]bool{})
	all := map[string]bool{}
	for _, n := range w.flow.Nodes {
		all[n.ID] = true
		maps.Copy(all, sites.VisibleDescendants(n))
	}
	for _, s := range sites.FlowTemplates(*w.flow) {
		out = append(out, w.checkSite(s, all, nil, "")...)
	}
	return out
}

// walk checks a node list where avail are the nodes finished before it.
func (w *refWalk) walk(list []model.Node, path string, avail map[string]bool) diag.List {
	available := maps.Clone(avail)
	var out diag.List
	for i, n := range list {
		npath := yamlpath.Index(path, i)
		visible, pending := readable(list, i, avail, available)
		for _, s := range sites.NodeTemplates(sites.Located[model.Node]{File: w.file, Path: npath, Value: n}) {
			out = append(out, w.checkSite(s, siteAvailable(n, npath, s, visible), pending, n.ID)...)
		}
		for _, child := range sites.ChildLists(n, npath) {
			out = append(out, w.walk(child.Value, child.Path, visible)...)
		}
		available[n.ID] = true
		maps.Copy(available, sites.VisibleDescendants(n))
	}
	return out
}

// readable splits what node i of list may read. A serial node waits for
// every earlier sibling, so all of them are readable. A concurrent node
// waits only for its needs (transitively): other earlier siblings may still
// be running, so they are pending, not readable.
func readable(list []model.Node, i int, outer, available map[string]bool) (visible, pending map[string]bool) {
	if !list[i].Concurrent {
		return available, nil
	}
	visible = maps.Clone(outer)
	needed := dependencyClosure(list, i)
	pending = map[string]bool{}
	for j := range i {
		ids := sites.VisibleDescendants(list[j])
		ids[list[j].ID] = true
		target := pending
		if needed[j] {
			target = visible
		}
		maps.Copy(target, ids)
	}
	return visible, pending
}

// siteAvailable: a run_till's until also sees its body's nodes.
func siteAvailable(n model.Node, npath string, s sites.Site, available map[string]bool) map[string]bool {
	if n.Type != "run_till" || s.Path != yamlpath.Key(npath, "until") {
		return available
	}
	out := maps.Clone(available)
	for _, b := range n.Body {
		out[b.ID] = true
		maps.Copy(out, sites.VisibleDescendants(b))
	}
	return out
}

func (w *refWalk) checkSite(s sites.Site, available, pending map[string]bool, reader string) diag.List {
	refs, err := template.References(s.Src, s.Expression)
	if err != nil {
		return nil // checkTemplates reports syntax errors
	}
	var out diag.List
	for _, r := range refs {
		if d, bad := w.checkRef(s, r.Path, available, pending, reader); bad {
			out = append(out, d)
		}
	}
	return out
}

func (w *refWalk) checkRef(s sites.Site, path []string, available, pending map[string]bool, reader string) (diag.Diagnostic, bool) {
	if len(path) < 2 {
		return diag.Diagnostic{}, false
	}
	at := diag.Location{File: s.File, Path: s.Path}
	unresolved := func(hint string) (diag.Diagnostic, bool) {
		return diag.New(diag.CodeNodeUnresolved, at, map[string]string{"name": path[0] + "." + path[1], "field": s.Field, "hint": hint}), true
	}
	switch path[0] {
	case "nodes":
		if available[path[1]] {
			return w.contract(at, path, reader)
		}
		if pending[path[1]] {
			return unresolved(" (it may still be running when this concurrent node starts; add it to needs)")
		}
		return unresolved(w.nodeHint(path[1]))
	case "inputs":
		if w.flow != nil && !w.inputs[path[1]] {
			return unresolved(" (declare it under inputs:)")
		}
	case "vars":
		if w.vars != nil && !w.vars[path[1]] {
			return unresolved(" (set it with a set node)")
		}
	}
	return diag.Diagnostic{}, false
}

func (w *refWalk) nodeHint(id string) string {
	info, exists := w.nodes[id]
	switch {
	case !exists:
		return ""
	case info.repeated:
		return " (it runs inside " + info.container + "; read nodes." + info.container + ".results instead)"
	}
	return " (it runs later; add it to needs or move it earlier)"
}

// contract warns when a request node's output is neither built in nor
// extracted by the version the node runs.
func (w *refWalk) contract(at diag.Location, path []string, reader string) (diag.Diagnostic, bool) {
	info := w.nodes[path[1]]
	call := info.node.Request
	if len(path) < 3 || info.node.Type != "request" || call == nil || slices.Contains(requestBuiltins(), path[2]) {
		return diag.Diagnostic{}, false
	}
	target, code, _ := w.ix.targetCollection(w.c, call.Request.Collection)
	req, ok := w.ix.requests[entityKey{target, call.Request.ID}]
	if code != "" || !ok {
		return diag.Diagnostic{}, false // reported by checkRefs
	}
	prefer := ""
	if w.flow != nil {
		prefer = w.flow.PreferVersion
	}
	v, ok := sites.ResolvedVersion(req, call.Version, prefer)
	if !ok || slices.ContainsFunc(v.Extract, func(e model.Extraction) bool { return e.Name == path[2] }) {
		return diag.Diagnostic{}, false
	}
	d := diag.New(diag.CodeNodeContract, at, map[string]string{"node": reader, "name": path[2], "request": req.Path, "version": v.Label})
	d.Severity = diag.SeverityWarning
	return d, true
}

// flowsUsedAsHooks are flows some hook step runs: their vars come from
// the request, so vars references in them are not checked.
func (ix *index) flowsUsedAsHooks() map[entityKey]bool {
	out := map[entityKey]bool{}
	for _, c := range ix.collections {
		for _, s := range sites.HookSteps(c) {
			if s.Value.Flow == nil {
				continue
			}
			if target, code, _ := ix.targetCollection(c, s.Value.Flow.Collection); code == "" {
				out[entityKey{target, s.Value.Flow.ID}] = true
			}
		}
	}
	return out
}
