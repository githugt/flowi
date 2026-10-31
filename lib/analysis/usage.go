// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"cmp"
	"slices"

	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// Status is how a consumer fares against a contract. Ranked
// Compatible < Unknown < Broken.
type Status string

// Statuses.
const (
	Compatible Status = "compatible"
	Unknown    Status = "unknown"
	Broken     Status = "broken"
)

// Consumer kinds.
const (
	KindNode    = "node"    // a request node in a flow or an inline hook step
	KindHook    = "hook"    // a post-hook step of the request
	KindRequest = "request" // the version's own extract and assert
)

// Consumer is one user of a request version and everything it relies on.
type Consumer struct {
	Kind         string   `json:"kind"`
	Collection   string   `json:"collection"`    // where the consumer is declared
	CollectionID model.ID `json:"collection_id"` //
	File         string   `json:"file"`          // the flow, hook flow or request file
	Flow         string   `json:"flow,omitempty"`
	Node         string   `json:"node,omitempty"`
	Hook         string   `json:"hook,omitempty"` // e.g. "post · request auth/login · inline #0"
	Version      string   `json:"version"`
	Fills        []string `json:"fills,omitempty"` // the node's with keys
	Reads        []Read   `json:"reads,omitempty"`
	Status       Status   `json:"status"`
	Reason       string   `json:"reason,omitempty"` // why the whole consumer is broken, when no single read is
}

// consumers lists every consumer of version v of request r (in
// collection c) across all collections in ix, without statuses beyond
// those known up front (unfilled inputs, unknown extract names).
func consumers(ix *index, c model.Collection, r model.Request, v model.RequestVersion) []Consumer {
	out := nodeConsumers(ix, c, r, v)
	out = append(out, hookConsumers(ix, c, r, v)...)
	if own, ok := requestConsumer(c, r, v); ok {
		out = append(out, own)
	}
	return out
}

// root is one top-level node list: a flow's nodes, or an inline hook
// step's nodes. Node ids are unique within a root, so nodes.<id> in any
// of its templates means that root's node.
type root struct {
	c     model.Collection
	file  string
	path  string // YAML path of the node list
	nodes []model.Node
	flow  *model.Flow // nil for inline hook steps
	hook  string      // label of the inline hook step
}

// key identifies a root: a node list is at one place in one file.
func (rt root) key() string { return rt.file + "\x00" + rt.path }

func (rt root) lists() []sites.NodeList {
	return sites.ExpandLists(sites.NodeList{File: rt.file, Path: rt.path, Value: rt.nodes})
}

// allNodes is every node in the root, nested ones included, with paths.
func (rt root) allNodes() []sites.Located[model.Node] {
	var out []sites.Located[model.Node]
	for _, l := range rt.lists() {
		for i, n := range l.Value {
			out = append(out, sites.Located[model.Node]{File: rt.file, Path: yamlpath.Index(l.Path, i), Value: n})
		}
	}
	return out
}

// sites is every template in the root: its nodes', plus the flow's own
// (input defaults, outputs, run.until).
func (rt root) sites() []sites.Site {
	var out []sites.Site
	for _, n := range rt.allNodes() {
		out = append(out, sites.NodeTemplates(n)...)
	}
	if rt.flow != nil {
		out = append(out, sites.FlowTemplates(*rt.flow)...)
	}
	return out
}

func (rt root) prefer() string {
	if rt.flow == nil {
		return ""
	}
	return rt.flow.PreferVersion
}

// allRoots lists every root of every collection: flows (hook flows
// included), then inline hook steps.
func (ix *index) allRoots() []root {
	var out []root
	for _, c := range ix.collections {
		for i := range c.Flows {
			f := &c.Flows[i]
			out = append(out, root{c: c, file: f.File, path: yamlpath.Key("$", "nodes"), nodes: f.Nodes, flow: f})
		}
		for _, s := range ix.scopes[c.Manifest.ID] {
			out = append(out, s.inlineRoots(c)...)
		}
	}
	return out
}

func nodeConsumers(ix *index, c model.Collection, r model.Request, v model.RequestVersion) []Consumer {
	var out []Consumer
	for _, cl := range ix.callers[r.ID] {
		rt := ix.roots[cl.root]
		if ix.calls(rt, cl.node.Value, c, r, v) {
			out = append(out, nodeConsumer(ix, rt, cl.node, c, r, v))
		}
	}
	return out
}

// calls reports whether node n of root rt runs version v of request r.
func (ix *index) calls(rt root, n model.Node, c model.Collection, r model.Request, v model.RequestVersion) bool {
	if n.Type != "request" || n.Request == nil || n.Request.Request.ID != r.ID {
		return false
	}
	target, ok := ix.collectionFor(rt.c, n.Request.Request.Collection)
	if !ok || target.Manifest.ID != c.Manifest.ID {
		return false
	}
	got, ok := sites.ResolvedVersion(r, n.Request.Version, rt.prefer())
	return ok && got.Label == v.Label
}

func nodeConsumer(ix *index, rt root, n sites.Located[model.Node], c model.Collection, r model.Request, v model.RequestVersion) Consumer {
	out := Consumer{
		Kind: KindNode, Collection: rt.c.Manifest.Name, CollectionID: rt.c.Manifest.ID, File: rt.file,
		Node: n.Value.ID, Hook: rt.hook, Version: v.Label, Fills: withKeys(n.Value.Request),
	}
	if rt.flow != nil {
		out.Flow = rt.flow.Path
	}
	out.Reads = append(outputReads(ix.refsOf(rt), n.Value.ID, v), missingInputs(ix, n, c, r, v)...)
	return out
}

func withKeys(call *model.RequestCall) []string {
	var out []string
	for _, kv := range call.With {
		out = append(out, kv.Key)
	}
	return slices.Sorted(slices.Values(out))
}

func hookConsumers(ix *index, c model.Collection, r model.Request, v model.RequestVersion) []Consumer {
	var out []Consumer
	for _, s := range ix.scopes[c.Manifest.ID] {
		if !s.applies(r, v) {
			continue
		}
		for i, step := range s.hooks.Post {
			rt, ok := s.stepRoot(ix, c, phasePost, i, step)
			if !ok {
				continue // an unknown hook flow is reported by validate
			}
			out = append(out, Consumer{
				Kind: KindHook, Collection: c.Manifest.Name, CollectionID: c.Manifest.ID, File: rt.file,
				Hook: s.stepLabel(phasePost, i, step), Version: v.Label, Reads: ix.hookReads(rt),
			})
		}
	}
	return out
}

// requestConsumer is the version's own extract block and asserts; it is
// a consumer only when it reads something from the response.
func requestConsumer(c model.Collection, r model.Request, v model.RequestVersion) (Consumer, bool) {
	reads := ownReads(r.File, v)
	if len(reads) == 0 {
		return Consumer{}, false
	}
	return Consumer{
		Kind: KindRequest, Collection: c.Manifest.Name, CollectionID: c.Manifest.ID, File: r.File,
		Version: v.Label, Reads: reads,
	}, true
}

// sortConsumers orders consumers for stable output: collection, file,
// then node or hook.
func sortConsumers(cs []Consumer) {
	slices.SortStableFunc(cs, func(a, b Consumer) int {
		return cmp.Or(cmp.Compare(a.Collection, b.Collection), cmp.Compare(a.File, b.File),
			cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Node+a.Hook, b.Node+b.Hook))
	})
}
