// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

// checkHookCycles reports hook flows that call each other in a cycle.
//
// A hook flow F leads to hook flow G when F runs a request R whose hooks
// include G and R runs its hooks inside F — which only happens with
// hooks.nested. Without nested hooks a "cycle" (a collection pre-hook
// that calls the collection's own token request) cannot recurse, so only
// edges through nested requests count. hooks.nested is read from the
// files (collection, folders, version); turning it on through user
// settings or FLOWI_HOOKS_NESTED is caught at run time by max_depth.
func checkHookCycles(ix *index) diag.List {
	g := newFlowGraph(ix)
	for ci, c := range ix.collections {
		for _, f := range c.Flows {
			from := g.byKey[entityKey{c.Manifest.ID, f.ID}].name()
			for _, target := range ix.hookFlowsReached(ci, f) {
				if v, ok := g.byKey[target]; ok {
					g.edges[from] = append(g.edges[from], v.name())
				}
			}
		}
	}
	var out diag.List
	for _, cycle := range findCycles(g.edges) {
		start := g.byName[cycle[0]]
		labels := make([]string, 0, len(cycle))
		for _, n := range cycle {
			labels = append(labels, ix.flowLabel(g.byName[n], start.collection))
		}
		out = append(out, diag.New(diag.CodeCycleHooks, ix.hookReference(start), map[string]string{"cycle": describeCycle(labels)}))
	}
	return out
}

// flowGraph holds every flow as a vertex, keyed both ways.
type flowGraph struct {
	byKey  map[entityKey]flowVertex
	byName map[string]flowVertex
	edges  map[string][]string
}

func newFlowGraph(ix *index) flowGraph {
	g := flowGraph{byKey: map[entityKey]flowVertex{}, byName: map[string]flowVertex{}, edges: map[string][]string{}}
	for ci, c := range ix.collections {
		for _, f := range c.Flows {
			v := flowVertex{ci, f.Path, entityKey{c.Manifest.ID, f.ID}}
			g.byKey[v.key], g.byName[v.name()] = v, v
			g.edges[v.name()] = nil
		}
	}
	return g
}

// hookFlowsReached lists the hook flows of every nested request flow f
// runs, at any depth.
func (ix *index) hookFlowsReached(ci int, f model.Flow) []entityKey {
	c := ix.collections[ci]
	var out []entityKey
	var walk func(nodes []model.Node)
	walk = func(nodes []model.Node) {
		for _, n := range nodes {
			if n.Request != nil {
				out = append(out, ix.nestedHookFlows(c, n.Request.Request)...)
			}
			for _, child := range n.Children() {
				walk(child)
			}
		}
	}
	walk(f.Nodes)
	return out
}

// nestedHookFlows is the hook flows of the referenced request, if it runs
// its hooks when called from a hook.
func (ix *index) nestedHookFlows(from model.Collection, ref model.Ref) []entityKey {
	target, code, _ := ix.targetCollection(from, ref.Collection)
	r, ok := ix.requests[entityKey{target, ref.ID}]
	if code != "" || !ok {
		return nil
	}
	c := ix.collections[ix.byID[target]]
	if !nestedHooks(c, r) {
		return nil
	}
	var out []entityKey
	for _, h := range requestHooks(c, r) {
		for _, step := range append(h.Pre, h.Post...) {
			if step.Flow == nil {
				continue
			}
			if fc, fcode, _ := ix.targetCollection(c, step.Flow.Collection); fcode == "" {
				out = append(out, entityKey{fc, step.Flow.ID})
			}
		}
	}
	return out
}

// requestHooks is every hooks block that applies to a request: the
// collection, its folders, the request and each of its versions.
func requestHooks(c model.Collection, r model.Request) []model.Hooks {
	out := []model.Hooks{c.Manifest.Hooks}
	for _, f := range sites.AncestorFolders(c, r) {
		out = append(out, f.Hooks)
	}
	out = append(out, r.Hooks)
	for _, v := range r.Versions {
		out = append(out, v.Hooks)
	}
	return out
}

// nestedHooks reports whether any version of r resolves hooks.nested to
// true from the collection, folder and version settings.
func nestedHooks(c model.Collection, r model.Request) bool {
	schema := settings.Schema()
	base := []settings.LayerValues{fileLayer(schema, settings.LayerCollection, c.Manifest.Settings)}
	for _, f := range sites.AncestorFolders(c, r) {
		base = append(base, fileLayer(schema, settings.LayerFolder, f.Settings))
	}
	for _, v := range r.Versions {
		layers := slices.Concat(base, []settings.LayerValues{fileLayer(schema, settings.LayerRequest, v.Settings)})
		if got, ok := settings.Resolve(schema, layers...).Get("hooks.nested"); ok && got.Value == true {
			return true
		}
	}
	return false
}

// fileLayer reads a settings block; invalid values are reported by the
// settings checks, so here they simply do not count.
func fileLayer(schema []settings.Key, layer settings.Layer, s model.Settings) settings.LayerValues {
	lv, _ := settings.FromBlock(schema, layer, "", s)
	return lv
}

// hookReference locates the first hook step that uses flow v, so the
// cycle is reported where it is wired up.
func (ix *index) hookReference(v flowVertex) diag.Location {
	for _, c := range ix.collections {
		for _, s := range sites.HookSteps(c) {
			if s.Value.Flow != nil && s.Value.Flow.ID == v.key.id {
				return diag.Location{File: s.File, Path: s.Path}
			}
		}
	}
	return diag.Location{File: ix.flows[v.key].File, Path: "$"}
}
