// SPDX-License-Identifier: Apache-2.0

package model

import (
	"path"
	"slices"
	"strings"
)

// FlowTags lists the tags of f: those of the folders it sits under, outermost first, then its own.
// A tag appears once, and the list is sorted.
func (c Collection) FlowTags(f Flow) []string {
	return c.tagsAt(path.Join("flows", path.Dir(f.Path)), f.Tags)
}

// RequestTags lists the tags of r, as FlowTags does for a flow.
func (c Collection) RequestTags(r Request) []string {
	return c.tagsAt(path.Join("requests", path.Dir(r.Path)), r.Tags)
}

// FlowsTagged lists the flows that carry tag, in file order.
func (c Collection) FlowsTagged(tag string) []Flow {
	var out []Flow
	for _, f := range c.Flows {
		if slices.Contains(c.FlowTags(f), tag) {
			out = append(out, f)
		}
	}
	return out
}

func (c Collection) tagsAt(dir string, own []string) []string {
	var tags []string
	for _, f := range c.Folders {
		if dir == f.Dir || strings.HasPrefix(dir, f.Dir+"/") {
			tags = append(tags, f.Tags...)
		}
	}
	tags = append(tags, own...)
	slices.Sort(tags)
	return slices.Compact(tags)
}
