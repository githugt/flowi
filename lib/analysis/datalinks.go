// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// DataLink says that node To reads a value node From made: either its output
// (`{{nodes.login.token}}`, Via "login.token") or a variable From wrote
// (`{{vars.token}}`, Via "token"). Links are for drawing and explaining a
// flow; they do not change how it runs.
type DataLink struct {
	From string `json:"from"`
	To   string `json:"to"`
	Via  string `json:"via"`
}

// DataLinks lists the values the nodes of f pass to each other, in the order
// the readers appear. A variable counts as made by the last node before the
// reader that writes it, except a value made by a generator: it is fresh, not
// taken from another step. A node reading its own output is not a link.
func DataLinks(f model.Flow) []DataLink {
	flat := flattenNodes(f.Nodes, nil)
	ids := map[string]bool{}
	for _, n := range flat {
		ids[n.ID] = true
	}
	var out []DataLink
	seen := map[DataLink]bool{}
	add := func(l DataLink) {
		if l.From != "" && l.From != l.To && !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	for i, n := range flat {
		for _, site := range sites.NodeTemplates(sites.Located[model.Node]{File: f.File, Value: n}) {
			refs, err := template.References(site.Src, site.Expression)
			if err != nil {
				continue // the validator reports a template that does not compile
			}
			for _, ref := range refs {
				add(linkOf(ref.Path, n.ID, flat[:i], ids))
			}
		}
	}
	return out
}

// linkOf is the link a read of path makes, or a zero link when the path reads
// neither a node's output nor a variable some earlier node wrote.
func linkOf(path []string, reader string, earlier []model.Node, ids map[string]bool) DataLink {
	switch {
	case len(path) >= 2 && path[0] == "nodes" && ids[path[1]]:
		return DataLink{From: path[1], To: reader, Via: joinFrom(path[1:], 2)}
	case len(path) >= 2 && path[0] == "vars":
		return DataLink{From: writerOf(path[1], earlier), To: reader, Via: path[1]}
	}
	return DataLink{}
}

// joinFrom joins at most n steps of path with dots.
func joinFrom(path []string, n int) string {
	out := ""
	for i, s := range path {
		if i == n {
			break
		}
		if i > 0 {
			out += "."
		}
		out += s
	}
	return out
}

// writerOf is the last node in earlier that writes variable name; generated
// values are skipped on purpose (see DataLinks).
func writerOf(name string, earlier []model.Node) string {
	for i := len(earlier) - 1; i >= 0; i-- {
		n := earlier[i]
		if sites.GeneratedVars(n)[name] {
			continue
		}
		for _, target := range sites.OwnWrites(n) {
			if target == "vars."+name {
				return n.ID
			}
		}
	}
	return ""
}

// flattenNodes lists nodes and everything inside them in document order.
func flattenNodes(list []model.Node, out []model.Node) []model.Node {
	for _, n := range list {
		out = append(out, n)
		for _, child := range n.Children() {
			out = flattenNodes(child, out)
		}
	}
	return out
}
