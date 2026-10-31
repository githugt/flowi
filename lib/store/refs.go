// SPDX-License-Identifier: Apache-2.0

package store

import "github.com/githugt/flowi/lib/model"

// RefResolver returns the current path of a referenced entity, or false
// when it cannot be resolved (the hint is then left unchanged).
type RefResolver func(model.Ref) (string, bool)

// RepairRefs returns a copy of c with every reference's path hint replaced
// by the resolver's answer. The input is not modified.
func RepairRefs(c model.Collection, resolve RefResolver) model.Collection {
	fix := func(r model.Ref) model.Ref {
		if p, ok := resolve(r); ok {
			r.Path = p
		}
		return r
	}
	out := c
	out.Manifest.Hooks = mapHooks(c.Manifest.Hooks, fix)
	out.Folders = mapSlice(c.Folders, func(f model.Folder) model.Folder {
		f.Hooks = mapHooks(f.Hooks, fix)
		return f
	})
	out.Requests = mapSlice(c.Requests, func(r model.Request) model.Request {
		r.Hooks = mapHooks(r.Hooks, fix)
		r.Versions = mapSlice(r.Versions, func(v model.RequestVersion) model.RequestVersion {
			v.Hooks = mapHooks(v.Hooks, fix)
			return v
		})
		return r
	})
	out.Flows = mapSlice(c.Flows, func(f model.Flow) model.Flow {
		f.Nodes = mapNodes(f.Nodes, fix)
		return f
	})
	return out
}

// LocalResolver resolves references within c itself.
func LocalResolver(c model.Collection) RefResolver {
	paths := map[model.ID]string{}
	for _, r := range c.Requests {
		paths[r.ID] = r.Path
	}
	for _, f := range c.Flows {
		paths[f.ID] = f.Path
	}
	return func(r model.Ref) (string, bool) {
		if r.Collection != "" {
			return "", false
		}
		p, ok := paths[r.ID]
		return p, ok
	}
}

func mapSlice[T any](in []T, fn func(T) T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

func mapHooks(h model.Hooks, fix func(model.Ref) model.Ref) model.Hooks {
	step := func(s model.HookStep) model.HookStep {
		if s.Flow != nil {
			r := fix(*s.Flow)
			s.Flow = &r
		}
		s.Inline = mapNodes(s.Inline, fix)
		return s
	}
	return model.Hooks{Pre: mapSlice(h.Pre, step), Post: mapSlice(h.Post, step), Message: mapSlice(h.Message, step)}
}

func mapNodes(nodes []model.Node, fix func(model.Ref) model.Ref) []model.Node {
	return mapSlice(nodes, func(n model.Node) model.Node {
		if n.Request != nil {
			call := *n.Request
			call.Request = fix(call.Request)
			n.Request = &call
		}
		if n.Subflow != nil {
			call := *n.Subflow
			call.Flow = fix(call.Flow)
			n.Subflow = &call
		}
		n.Body = mapNodes(n.Body, fix)
		n.Then = mapNodes(n.Then, fix)
		n.Else = mapNodes(n.Else, fix)
		n.Branches = mapSlice(n.Branches, func(b model.Branch) model.Branch {
			b.Nodes = mapNodes(b.Nodes, fix)
			return b
		})
		n.Cases = mapSlice(n.Cases, func(cs model.Case) model.Case {
			cs.Nodes = mapNodes(cs.Nodes, fix)
			return cs
		})
		return n
	})
}
