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

// FolderTags lists the tags that apply at the folder dir ("requests/auth"): those of the folder and of the folders above
// it. A folder with no _folder.yaml of its own still has those of the folders above it.
func (c Collection) FolderTags(dir string) []string {
	return c.tagsAt(dir, nil)
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

// CleanTag is tag as it is kept: runs of white space and commas become one '-', so a tag is one word that works as a
// `flowi run --tag` argument and as a list item in a file. "" for a tag with nothing in it.
func CleanTag(tag string) string {
	words := strings.Fields(strings.ReplaceAll(tag, ",", " "))
	return strings.Trim(strings.Join(words, "-"), "-")
}

// CleanTags cleans each tag (CleanTag), drops the empty ones and repeats, and keeps the order the person gave.
func CleanTags(tags []string) []string {
	var out []string
	for _, t := range tags {
		if t = CleanTag(t); t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// TagUse is a tag and how many requests, flows and folders carry it themselves (not through a folder).
type TagUse struct {
	Tag   string
	Count int
}

// TagsInUse lists the tags the collection uses, the most used first and by name among equals, so an editor can suggest
// the ones already in use instead of a person re-typing "smoke" as "smokes".
func (c Collection) TagsInUse() []TagUse {
	counts := map[string]int{}
	add := func(tags []string) {
		for _, t := range CleanTags(tags) {
			counts[t]++
		}
	}
	for _, r := range c.Requests {
		add(r.Tags)
	}
	for _, f := range c.Flows {
		add(f.Tags)
	}
	for _, f := range c.Folders {
		add(f.Tags)
	}
	out := make([]TagUse, 0, len(counts))
	for t, n := range counts {
		out = append(out, TagUse{Tag: t, Count: n})
	}
	slices.SortFunc(out, func(a, b TagUse) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Tag, b.Tag)
	})
	return out
}
