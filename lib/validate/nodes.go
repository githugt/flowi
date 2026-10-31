// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// requiredField names the field a node type cannot do without, and how to
// tell it is missing. Encoding the rules as data keeps them in one table.
type requiredField struct {
	field   string
	missing func(model.Node) bool
}

func requiredFields() map[model.NodeType]requiredField {
	noBody := requiredField{"body", func(n model.Node) bool { return len(n.Body) == 0 }}
	return map[model.NodeType]requiredField{
		"request":   {"request", func(n model.Node) bool { return n.Request == nil || n.Request.Request == (model.Ref{}) }},
		"fuzz":      {"request", func(n model.Node) bool { return n.Request == nil || n.Request.Request == (model.Ref{}) }},
		"subflow":   {"flow", func(n model.Node) bool { return n.Subflow == nil || n.Subflow.Flow == (model.Ref{}) }},
		"batch":     noBody,
		"run_till":  noBody,
		"parallel":  {"branches", func(n model.Node) bool { return len(n.Branches) < 2 }},
		"condition": {"then", func(n model.Node) bool { return len(n.Then) == 0 && len(n.Cases) == 0 }},
	}
}

func checkNodes(ix *index) diag.List {
	known := model.NodeTypes()
	required := requiredFields()
	var out diag.List
	for _, c := range ix.collections {
		for _, n := range sites.AllNodes(c) {
			out = append(out, checkNodeShape(n, known, required)...)
		}
		for _, l := range sites.NodeLists(c) {
			out = append(out, checkSiblingNeeds(l)...)
			out = append(out, checkStepOutcomes(l)...)
		}
		for _, root := range sites.RootLists(c) {
			out = append(out, checkUniqueNodeIDs(root)...)
		}
	}
	return out
}

// checkNodeShape reports the first missing required field of a node.
func checkNodeShape(n sites.Located[model.Node], known []model.NodeType, required map[model.NodeType]requiredField) diag.List {
	t := n.Value.Type
	if use, removed := model.RemovedNodeTypes()[t]; removed {
		loc := diag.Location{File: n.File, Path: yamlpath.Key(n.Path, "type")}
		return diag.List{diag.New(diag.CodeNodeRemovedType, loc, map[string]string{"type": string(t), "use": use})}
	}
	if !slices.Contains(known, t) {
		loc := diag.Location{File: n.File, Path: yamlpath.Key(n.Path, "type")}
		return diag.List{diag.New(diag.CodeNodeUnknownType, loc, map[string]string{"type": string(t)})}
	}
	rules := paramRequired()[t]
	if rule, ok := required[t]; ok {
		rules = append(rules, rule)
	}
	for _, rule := range rules {
		if rule.missing(n.Value) {
			loc := diag.Location{File: n.File, Path: n.Path}
			return diag.List{diag.New(diag.CodeNodeMissingField, loc, map[string]string{"type": string(t), "field": rule.field})}
		}
	}
	return nil
}

// checkSiblingNeeds requires every needs entry to name a node in the same
// list: dependencies never cross into or out of a container.
func checkSiblingNeeds(l sites.NodeList) diag.List {
	siblings := map[string]bool{}
	for _, n := range l.Value {
		siblings[n.ID] = true
	}
	var out diag.List
	for i, n := range l.Value {
		for _, arrows := range []struct {
			key     string
			targets []string
		}{{"needs", n.Needs}, {"after_failure_of", n.AfterFailureOf}, {"after_outcome", n.AfterOutcome}} {
			for k, target := range arrows.targets {
				if arrows.key == "after_outcome" {
					out = append(out, afterOutcomeDiags(l, i, k, target, siblings)...)
					continue
				}
				if siblings[target] {
					continue
				}
				path := yamlpath.Index(yamlpath.Key(yamlpath.Index(l.Path, i), arrows.key), k)
				out = append(out, diag.New(diag.CodeNodeNeedsSibling, diag.Location{File: l.File, Path: path}, map[string]string{"target": target}))
			}
		}
	}
	return out
}

func checkUniqueNodeIDs(root sites.NodeList) diag.List {
	seen := map[string]bool{}
	var out diag.List
	for _, l := range sites.ExpandLists(root) {
		for i, n := range l.Value {
			if n.ID == "" {
				continue
			}
			if seen[n.ID] {
				loc := diag.Location{File: l.File, Path: yamlpath.Key(yamlpath.Index(l.Path, i), "id")}
				out = append(out, diag.New(diag.CodeNodeDuplicateID, loc, map[string]string{"node": n.ID}))
			}
			seen[n.ID] = true
		}
	}
	return out
}
