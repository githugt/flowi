// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"cmp"
	"slices"

	"github.com/githugt/flowi/lib/model"
)

// Caller is one step that runs a request: a request node of a flow, or of an inline hook step. It is what would be left
// pointing at nothing if the request were deleted.
type Caller struct {
	Collection   string   `json:"collection"` // where the step is declared
	CollectionID model.ID `json:"collection_id"`
	File         string   `json:"file"`
	Flow         string   `json:"flow,omitempty"` // the flow's path; empty for an inline hook step
	Node         string   `json:"node"`
	Hook         string   `json:"hook,omitempty"` // the label of the inline hook step, e.g. "post · request auth/login · inline #0"
}

// Callers lists, for each of the requests ts, the steps across every collection in cs that run it. A request nothing
// runs has no entry. It reads no examples and compares no contracts, so it is cheap enough to ask while a menu opens;
// Impact says whether the callers still fit.
func Callers(cs []model.Collection, ts []Target) map[Target][]Caller {
	ix := newIndex(cs)
	out := map[Target][]Caller{}
	for _, t := range ts {
		c, r, ok := ix.request(t)
		if !ok {
			continue
		}
		for _, cl := range ix.callers[r.ID] {
			rt := ix.roots[cl.root]
			n := cl.node.Value
			if n.Request == nil {
				continue
			}
			if target, ok := ix.collectionFor(rt.c, n.Request.Request.Collection); !ok || target.Manifest.ID != c.Manifest.ID {
				continue
			}
			caller := Caller{Collection: rt.c.Manifest.Name, CollectionID: rt.c.Manifest.ID, File: rt.file, Node: n.ID, Hook: rt.hook}
			if rt.flow != nil {
				caller.Flow = rt.flow.Path
			}
			out[t] = append(out[t], caller)
		}
		slices.SortStableFunc(out[t], func(a, b Caller) int {
			return cmp.Or(cmp.Compare(a.Collection, b.Collection), cmp.Compare(a.File, b.File), cmp.Compare(a.Node, b.Node))
		})
	}
	return out
}
