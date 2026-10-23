// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/model"
)

// Target names one request: the collection that holds it and its id.
type Target struct {
	Collection model.ID
	Request    model.ID
}

// index resolves collections, requests and flows across every loaded
// collection, and holds what every request version's analysis shares:
// the roots, who calls each request, hook scopes, and memos of parsed
// templates. Building these once per analysis rather than once per
// request version took a 500-request, 8000-node workspace from 6.8s to
// 0.15s (BenchmarkAnalyze). An index is not safe for
// concurrent use: the memos fill in on first use.
type index struct {
	collections []model.Collection
	byID        map[model.ID]int
	scopes      map[model.ID][]hookScope // per collection
	roots       []root
	callers     map[model.ID][]caller // request id → request nodes naming it

	refsMemo  map[string][]siteRef // root key → its unguarded reads
	hookMemo  map[string][]Read    // root key → its reads as a hook step
	needsMemo map[versionKey][]need
	varsMemo  map[versionKey]map[string]bool // pre-hook vars
}

// caller is a request node and the root it is in.
type caller struct {
	root int // index into index.roots
	node sites.Located[model.Node]
}

// siteRef is one unguarded read and the template it is in.
type siteRef struct {
	site sites.Site
	path []string
}

type versionKey struct {
	collection, request model.ID
	version             string
}

func newIndex(cs []model.Collection) *index {
	ix := &index{collections: cs, byID: map[model.ID]int{}, scopes: map[model.ID][]hookScope{}, callers: map[model.ID][]caller{},
		refsMemo: map[string][]siteRef{}, hookMemo: map[string][]Read{}, needsMemo: map[versionKey][]need{}, varsMemo: map[versionKey]map[string]bool{}}
	for i, c := range cs {
		ix.byID[c.Manifest.ID] = i
		ix.scopes[c.Manifest.ID] = scopesOf(c)
	}
	ix.roots = ix.allRoots()
	for i, rt := range ix.roots {
		for _, n := range rt.allNodes() {
			if n.Value.Type == "request" && n.Value.Request != nil {
				id := n.Value.Request.Request.ID
				ix.callers[id] = append(ix.callers[id], caller{root: i, node: n})
			}
		}
	}
	return ix
}

// refsOf is every unguarded read in rt's templates, parsed once.
func (ix *index) refsOf(rt root) []siteRef {
	if refs, ok := ix.refsMemo[rt.key()]; ok {
		return refs
	}
	refs := parseRefs(rt.sites())
	ix.refsMemo[rt.key()] = refs
	return refs
}

func (ix *index) request(t Target) (model.Collection, model.Request, bool) {
	i, ok := ix.byID[t.Collection]
	if !ok {
		return model.Collection{}, model.Request{}, false
	}
	c := ix.collections[i]
	for _, r := range c.Requests {
		if r.ID == t.Request {
			return c, r, true
		}
	}
	return model.Collection{}, model.Request{}, false
}

// collectionFor resolves a reference's collection as seen from the
// referencing collection: empty is from itself, otherwise a depends_on
// entry by name or id that is loaded.
func (ix *index) collectionFor(from model.Collection, name string) (model.Collection, bool) {
	if name == "" {
		return from, true
	}
	for _, dep := range from.Manifest.DependsOn {
		if dep.Name != name && string(dep.ID) != name {
			continue
		}
		if i, ok := ix.byID[dep.ID]; ok {
			return ix.collections[i], true
		}
	}
	return model.Collection{}, false
}

func (ix *index) flow(from model.Collection, ref model.Ref) (model.Collection, model.Flow, bool) {
	c, ok := ix.collectionFor(from, ref.Collection)
	if !ok {
		return model.Collection{}, model.Flow{}, false
	}
	for _, f := range c.Flows {
		if f.ID == ref.ID {
			return c, f, true
		}
	}
	return model.Collection{}, model.Flow{}, false
}
