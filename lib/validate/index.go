// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

type entityKey struct{ collection, id model.ID }

// index gives every check fast lookup of collections, requests and flows.
type index struct {
	collections []model.Collection
	byID        map[model.ID]int
	requests    map[entityKey]model.Request
	flows       map[entityKey]model.Flow
}

func buildIndex(cs []model.Collection) *index {
	ix := &index{
		collections: cs,
		byID:        map[model.ID]int{},
		requests:    map[entityKey]model.Request{},
		flows:       map[entityKey]model.Flow{},
	}
	for i, c := range cs {
		cid := c.Manifest.ID
		ix.byID[cid] = i
		for _, r := range c.Requests {
			ix.requests[entityKey{cid, r.ID}] = r
		}
		for _, f := range c.Flows {
			ix.flows[entityKey{cid, f.ID}] = f
		}
	}
	return ix
}

// targetCollection resolves a reference's collection from the referencing
// collection. An empty name means the referencing collection itself. On
// failure it returns the diagnostic code and arguments to report.
func (ix *index) targetCollection(from model.Collection, name string) (model.ID, diag.Code, map[string]string) {
	if name == "" {
		return from.Manifest.ID, "", nil
	}
	for _, dep := range from.Manifest.DependsOn {
		if dep.Name != name && string(dep.ID) != name {
			continue
		}
		if _, loaded := ix.byID[dep.ID]; !loaded {
			return "", diag.CodeRefNotMounted, map[string]string{"collection": name, "repo": dep.Repo}
		}
		return dep.ID, "", nil
	}
	return "", diag.CodeRefUndeclared, map[string]string{"collection": name}
}
