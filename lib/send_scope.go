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

// filesNotRun are the files a send of r does not use: the flows its hooks do not call (flowFilesNotRun) and the
// other requests those flows do not send. A mistake in one request, such as a query a database request may not
// run, is that request's own business and must not stop a different request being sent.
func filesNotRun(all []model.Collection, c model.Collection, r model.Request) map[string]bool {
	skipped := flowFilesNotRun(all, c, r)
	var refs []model.Ref
	for _, h := range hookSets(c, r) {
		for _, steps := range [][]model.HookStep{h.Pre, h.Post, h.Message} {
			for _, s := range steps {
				refs = append(refs, requestRefs(s.Inline)...)
			}
		}
	}
	for _, coll := range all {
		for _, f := range coll.Flows {
			if !skipped[f.File] {
				refs = append(refs, requestRefs(f.Nodes)...)
			}
		}
	}
	for _, coll := range all {
		for _, other := range coll.Requests {
			sent := other.File == r.File
			for _, ref := range refs {
				// As for flows, an unclear reference counts as sent: it is matched by id or path in every collection.
				sent = sent || (ref.ID != "" && ref.ID == other.ID) || (ref.Path != "" && ref.Path == other.Path)
			}
			if !sent {
				skipped[other.File] = true
			}
		}
	}
	return skipped
}

// requestRefs are the requests the nodes send, at any depth.
func requestRefs(nodes []model.Node) []model.Ref {
	var refs []model.Ref
	for _, n := range nodes {
		if n.Request != nil {
			refs = append(refs, n.Request.Request)
		}
		for _, children := range n.Children() {
			refs = append(refs, requestRefs(children)...)
		}
	}
	return refs
}

// filesNotRunByFlow are the files a run of flow f does not use: the flows it does not call (as subflows, or from the
// hooks of the requests it sends) and the requests it does not send. Like a send, a run must not be stopped by a
// mistake in a file it never touches. Unclear references count as used, as in filesNotRun.
func filesNotRunByFlow(all []model.Collection, f model.Flow) map[string]bool {
	flowsRun := map[string]bool{f.File: true}
	reqsRun := map[string]bool{}
	flowQueue := []model.Flow{f}
	reqQueue := []model.Ref{}
	hookQueue := []model.Hooks{}

	for len(flowQueue)+len(reqQueue)+len(hookQueue) > 0 {
		var flowRefs, reqRefs []model.Ref
		switch {
		case len(flowQueue) > 0:
			g := flowQueue[0]
			flowQueue = flowQueue[1:]
			flowRefs, reqRefs = subflowRefs(g.Nodes), requestRefs(g.Nodes)
		case len(hookQueue) > 0:
			h := hookQueue[0]
			hookQueue = hookQueue[1:]
			flowRefs = hookRefs(h)
			for _, steps := range [][]model.HookStep{h.Pre, h.Post, h.Message} {
				for _, s := range steps {
					reqRefs = append(reqRefs, requestRefs(s.Inline)...)
				}
			}
		default:
			reqRefs = []model.Ref{reqQueue[0]}
			reqQueue = reqQueue[1:]
		}
		for _, ref := range flowRefs {
			for _, coll := range all {
				for _, g := range coll.Flows {
					if !flowsRun[g.File] && refersTo(ref, g) {
						flowsRun[g.File] = true
						flowQueue = append(flowQueue, g)
					}
				}
			}
		}
		for _, ref := range reqRefs {
			for _, coll := range all {
				for _, r := range coll.Requests {
					if reqsRun[r.File] || ((ref.ID == "" || ref.ID != r.ID) && (ref.Path == "" || ref.Path != r.Path)) {
						continue
					}
					reqsRun[r.File] = true
					hookQueue = append(hookQueue, hookSets(coll, r)...) // what a send of it runs around it
				}
			}
		}
	}
	skipped := map[string]bool{}
	for _, coll := range all {
		for _, g := range coll.Flows {
			if !flowsRun[g.File] {
				skipped[g.File] = true
			}
		}
		for _, r := range coll.Requests {
			if !reqsRun[r.File] {
				skipped[r.File] = true
			}
		}
	}
	return skipped
}
