// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"strings"

	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// SkipReader is a step that reads something a skipped step would have made.
// Required is false when the read has a default (?? or ?.), so a missing value
// is nil rather than an error.
type SkipReader struct {
	Skipped  string
	Reader   string
	Reads    string
	Required bool
}

// SkipReaders lists, in flow order, every step that reads the output of a
// skipped step (nodes.<id>…) or a variable the last earlier writer of which is
// skipped. A step inside a skipped container is skipped too; a skipped step is
// never reported as a reader.
func SkipReaders(f model.Flow, skip []string) []SkipReader {
	flat := flattenNodes(f.Nodes, nil)
	skipped := skippedIDs(f.Nodes, skip, false, map[string]bool{})
	var out []SkipReader
	for i, n := range flat {
		if skipped[n.ID] {
			continue
		}
		out = append(out, readersIn(f.File, n, flat[:i], skipped)...)
	}
	return out
}

// skippedIDs marks the ids in skip and everything nested inside them.
func skippedIDs(list []model.Node, skip []string, inside bool, into map[string]bool) map[string]bool {
	for _, n := range list {
		hit := inside || contains(skip, n.ID)
		if hit {
			into[n.ID] = true
		}
		for _, child := range n.Children() {
			skippedIDs(child, skip, hit, into)
		}
	}
	return into
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// readersIn is what n reads from skipped steps, given the steps before it.
func readersIn(file string, n model.Node, earlier []model.Node, skipped map[string]bool) []SkipReader {
	var out []SkipReader
	for _, site := range sites.NodeTemplates(sites.Located[model.Node]{File: file, Value: n}) {
		refs, err := template.References(site.Src, site.Expression)
		if err != nil {
			continue // the validator reports a template that does not compile
		}
		for _, ref := range refs {
			if from := skippedSource(ref.Path, earlier, skipped); from != "" {
				out = append(out, SkipReader{Skipped: from, Reader: n.ID, Reads: strings.Join(ref.Path, "."), Required: !ref.Guarded})
			}
		}
	}
	return out
}

// skippedSource is the skipped step a read of path comes from, or "".
func skippedSource(path []string, earlier []model.Node, skipped map[string]bool) string {
	switch {
	case len(path) >= 2 && path[0] == "nodes" && skipped[path[1]]:
		return path[1]
	case len(path) >= 2 && path[0] == "vars":
		if writer := lastWriter(path[1], earlier); writer != "" && skipped[writer] {
			return writer
		}
	}
	return ""
}

// lastWriter is the last of earlier that writes variable name, generated
// values included (a skipped generator still leaves the variable unset).
func lastWriter(name string, earlier []model.Node) string {
	for i := len(earlier) - 1; i >= 0; i-- {
		for _, target := range sites.OwnWrites(earlier[i]) {
			if target == "vars."+name {
				return earlier[i].ID
			}
		}
	}
	return ""
}
