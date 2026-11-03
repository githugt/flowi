// SPDX-License-Identifier: Apache-2.0

package flowi

import "github.com/githugt/flowi/lib/model"

// flowFilesNotRun are the files of the flows a send of r does not run. A send builds one request and runs its
// hooks, so a flow only matters to it when a hook calls it (directly, or through a subflow). A flow that merely
// uses the request, and whatever it adds around it, is the flow's own business: its errors are shown in the
// problems list but must not stop the request being sent.
func flowFilesNotRun(all []model.Collection, c model.Collection, r model.Request) map[string]bool {
	run := hookFlowsOf(all, c, r)
	skipped := map[string]bool{}
	for _, coll := range all {
		for _, f := range coll.Flows {
			if !run[f.File] {
				skipped[f.File] = true
			}
		}
	}
	return skipped
}

// hookFlowsOf returns the files of the flows the hooks of r call, including the flows those call as subflows.
// A reference is matched by id or path in every collection, so an unclear one counts as run: a send is blocked
// rather than let through when it might use a broken flow.
func hookFlowsOf(all []model.Collection, c model.Collection, r model.Request) map[string]bool {
	var refs []model.Ref
	for _, h := range hookSets(c, r) {
		refs = append(refs, hookRefs(h)...)
	}
	run := map[string]bool{}
	for len(refs) > 0 {
		ref := refs[0]
		refs = refs[1:]
		for _, coll := range all {
			for _, f := range coll.Flows {
				if run[f.File] || !refersTo(ref, f) {
					continue
				}
				run[f.File] = true
				refs = append(refs, subflowRefs(f.Nodes)...)
			}
		}
	}
	return run
}

// hookSets are every hook list that can apply to a send of r: the collection's, its folders', the request's
// and its versions'.
func hookSets(c model.Collection, r model.Request) []model.Hooks {
	sets := []model.Hooks{c.Manifest.Hooks, r.Hooks}
	for _, f := range c.Folders {
		sets = append(sets, f.Hooks)
	}
	for _, v := range r.Versions {
		sets = append(sets, v.Hooks)
	}
	return sets
}

// hookRefs are the flows a hook list calls and the flows called from inside its inline steps.
func hookRefs(h model.Hooks) []model.Ref {
	var refs []model.Ref
	for _, steps := range [][]model.HookStep{h.Pre, h.Post, h.Message} {
		for _, s := range steps {
			if s.Flow != nil {
				refs = append(refs, *s.Flow)
			}
			refs = append(refs, subflowRefs(s.Inline)...)
		}
	}
	return refs
}

// subflowRefs are the flows the nodes call as subflows, at any depth.
func subflowRefs(nodes []model.Node) []model.Ref {
	var refs []model.Ref
	for _, n := range nodes {
		if n.Subflow != nil {
			refs = append(refs, n.Subflow.Flow)
		}
		for _, children := range n.Children() {
			refs = append(refs, subflowRefs(children)...)
		}
	}
	return refs
}

func refersTo(ref model.Ref, f model.Flow) bool {
	return (ref.ID != "" && ref.ID == f.ID) || (ref.Path != "" && ref.Path == f.Path)
}
