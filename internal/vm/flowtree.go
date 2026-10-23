// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"regexp"
	"slices"
	"strconv"

	"github.com/githugt/flowi-lib/model"
)

// Node lists are addressed by YAML path, the way flowi-lib reports them:
// "$.nodes", "$.nodes[2].body", "$.nodes[4].branches[1].nodes",
// "$.nodes[6].then", "$.nodes[6].cases[0].nodes".

var pathStep = regexp.MustCompile(`\.([a-z_]+)|\[(\d+)\]`)

// listAt returns the node list at path in f, for changing it in place.
func listAt(f *model.Flow, path string) (*[]model.Node, bool) {
	steps := pathStep.FindAllStringSubmatch(path, -1)
	if len(steps) == 0 || steps[0][1] != "nodes" || "$"+joinSteps(steps) != path {
		return nil, false
	}
	list := &f.Nodes
	for i := 1; i < len(steps); i++ {
		node, ok := at(list, steps[i])
		if !ok || i+1 >= len(steps) {
			return nil, false
		}
		i++
		next, consumed, ok := childList(node, steps[i:])
		if !ok {
			return nil, false
		}
		list = next
		i += consumed - 1
	}
	return list, true
}

func joinSteps(steps [][]string) string {
	out := ""
	for _, s := range steps {
		out += s[0]
	}
	return out
}

func at(list *[]model.Node, step []string) (*model.Node, bool) {
	if step[2] == "" {
		return nil, false
	}
	i, err := strconv.Atoi(step[2])
	if err != nil || i >= len(*list) {
		return nil, false
	}
	return &(*list)[i], true
}

// childList follows a container's list steps (.body, .then, .else,
// .branches[j].nodes, .cases[j].nodes) and says how many it used.
func childList(n *model.Node, steps [][]string) (*[]model.Node, int, bool) {
	switch steps[0][1] {
	case "body":
		return &n.Body, 1, true
	case "then":
		return &n.Then, 1, true
	case "else":
		return &n.Else, 1, true
	case "branches", "cases":
		if len(steps) < 3 || steps[1][2] == "" || steps[2][1] != "nodes" {
			return nil, 0, false
		}
		j, _ := strconv.Atoi(steps[1][2])
		if steps[0][1] == "branches" && j < len(n.Branches) {
			return &n.Branches[j].Nodes, 3, true
		}
		if steps[0][1] == "cases" && j < len(n.Cases) {
			return &n.Cases[j].Nodes, 3, true
		}
	}
	return nil, 0, false
}

// childPaths are the YAML paths of a node's lists, with the lists.
func childPaths(n model.Node, p string) []struct {
	path  string
	nodes []model.Node
} {
	type lp = struct {
		path  string
		nodes []model.Node
	}
	out := []lp{{p + ".body", n.Body}, {p + ".then", n.Then}, {p + ".else", n.Else}}
	for i, b := range n.Branches {
		out = append(out, lp{p + ".branches[" + strconv.Itoa(i) + "].nodes", b.Nodes})
	}
	for i, c := range n.Cases {
		out = append(out, lp{p + ".cases[" + strconv.Itoa(i) + "].nodes", c.Nodes})
	}
	return out
}

// locate finds node id: the path of its list and its index there.
func locate(list []model.Node, path, id string) (string, int, bool) {
	for i, n := range list {
		if n.ID == id {
			return path, i, true
		}
		for _, c := range childPaths(n, path+"["+strconv.Itoa(i)+"]") {
			if p, j, ok := locate(c.nodes, c.path, id); ok {
				return p, j, true
			}
		}
	}
	return "", 0, false
}

// allIDs collects every node and branch id in list.
func allIDs(list []model.Node, out map[string]bool) {
	for _, n := range list {
		out[n.ID] = true
		for _, b := range n.Branches {
			out[b.ID] = true
		}
		for _, child := range n.Children() {
			allIDs(child, out)
		}
	}
}

// cloneFlow deep-copies everything an edit can change.
func cloneFlow(f model.Flow) model.Flow {
	f.Inputs, f.Outputs = slices.Clone(f.Inputs), slices.Clone(f.Outputs)
	f.Execution.HostRateLimits = slices.Clone(f.Execution.HostRateLimits)
	if f.Run != nil {
		run := *f.Run
		f.Run = &run
	}
	f.Nodes = cloneNodes(f.Nodes)
	return f
}

func cloneNodes(list []model.Node) []model.Node {
	if list == nil {
		return nil
	}
	out := make([]model.Node, len(list))
	for i, n := range list {
		n.Needs = slices.Clone(n.Needs)
		if n.Request != nil {
			r := *n.Request
			r.With, r.Headers, r.Hooks.Skip = slices.Clone(r.With), slices.Clone(r.Headers), slices.Clone(r.Hooks.Skip)
			n.Request = &r
		}
		if n.Subflow != nil {
			s := *n.Subflow
			s.With = slices.Clone(s.With)
			n.Subflow = &s
		}
		n.Body, n.Then, n.Else = cloneNodes(n.Body), cloneNodes(n.Then), cloneNodes(n.Else)
		n.Branches = slices.Clone(n.Branches)
		for j := range n.Branches {
			n.Branches[j].Nodes = cloneNodes(n.Branches[j].Nodes)
		}
		n.Cases = slices.Clone(n.Cases)
		for j := range n.Cases {
			n.Cases[j].Nodes = cloneNodes(n.Cases[j].Nodes)
		}
		n.Params = cloneFields(n.Params)
		out[i] = n
	}
	return out
}

func cloneFields(fs []model.Field) []model.Field {
	if fs == nil {
		return nil
	}
	out := make([]model.Field, len(fs))
	for i, f := range fs {
		out[i] = model.Field{Key: f.Key, Value: cloneValue(f.Value)}
	}
	return out
}

func cloneValue(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = cloneValue(item)
		}
		return out
	case []model.Field:
		return cloneFields(x)
	}
	return v // scalars are immutable
}

// dropNeeds removes ids from every needs list.
func dropNeeds(list []model.Node, gone map[string]bool) {
	for i := range list {
		list[i].Needs = slices.DeleteFunc(list[i].Needs, func(id string) bool { return gone[id] })
		if len(list[i].Needs) == 0 {
			list[i].Needs = nil
		}
		for _, child := range listsOf(&list[i]) {
			dropNeeds(*child, gone)
		}
	}
}

// listsOf are pointers to a node's child lists.
func listsOf(n *model.Node) []*[]model.Node {
	out := []*[]model.Node{&n.Body, &n.Then, &n.Else}
	for j := range n.Branches {
		out = append(out, &n.Branches[j].Nodes)
	}
	for j := range n.Cases {
		out = append(out, &n.Cases[j].Nodes)
	}
	return out
}
