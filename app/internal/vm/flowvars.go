// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"regexp"
	"slices"

	"github.com/githugt/flowi/lib/model"
)

// FlowVars is what a flow does with variables and secrets, read from the flow
// text: a quick overview for the status bar, not an evaluation. Variables set
// by a request's own hooks or by a subflow are not seen, so Unset is a hint.
type FlowVars struct {
	Set     []string // variables a set step writes
	Read    []string // variables a template or expression reads
	Unset   []string // read but written by no step of this flow
	Secrets []string // secrets referenced as secret.<name>
	// Writers and Readers say which steps write or read each variable, by step id: click a variable and its steps light
	// up. Before: the bar listed names only, so finding who changed a variable meant reading every step.
	Writers map[string][]string
	Readers map[string][]string
}

var (
	varReadPattern = regexp.MustCompile(`\bvars\.([A-Za-z_][A-Za-z0-9_]*)`)
	secretPattern  = regexp.MustCompile(`\bsecret\.([A-Za-z_][A-Za-z0-9_]*)`)
)

// ScanFlow summarises the variables and secrets of f.
func ScanFlow(f model.Flow) FlowVars {
	text, _ := json.Marshal(f) // a flow always marshals; on failure the scan is just empty
	set := map[string]bool{}
	collectWrites(f.Nodes, set)
	read := matches(varReadPattern, string(text))
	v := FlowVars{
		Set:     sortedKeys(set),
		Read:    read,
		Secrets: matches(secretPattern, string(text)),
	}
	for _, name := range read {
		if !set[name] {
			v.Unset = append(v.Unset, name)
		}
	}
	v.Writers, v.Readers = map[string][]string{}, map[string][]string{}
	collectUses(f.Nodes, v.Writers, v.Readers)
	return v
}

// collectUses records, for every step at any depth, the variables it writes and the ones its own text reads. A
// container's own text leaves out its children (they are listed on their own), and a set step's target is a write, not a read.
func collectUses(list []model.Node, writers, readers map[string][]string) {
	for _, n := range list {
		for _, name := range writesOf(n) {
			writers[name] = appendOnce(writers[name], n.ID)
		}
		for _, name := range matches(varReadPattern, ownText(n)) {
			readers[name] = appendOnce(readers[name], n.ID)
		}
		for _, child := range n.Children() {
			collectUses(child, writers, readers)
		}
	}
}

func appendOnce(list []string, id string) []string {
	if slices.Contains(list, id) {
		return list
	}
	return append(list, id)
}

// ownText is the step's text without its children and, for a set step, without the variable it writes.
func ownText(n model.Node) string {
	n.Body, n.Branches, n.Then, n.Else, n.Cases = nil, nil, nil, nil, nil
	if n.Type == "set" {
		n.Params = slices.DeleteFunc(slices.Clone(n.Params), func(p model.Field) bool { return p.Key == "target" })
	}
	text, _ := json.Marshal(n)
	return string(text)
}

func collectWrites(list []model.Node, set map[string]bool) {
	for _, n := range list {
		for _, name := range writesOf(n) {
			set[name] = true
		}
		for _, child := range n.Children() {
			collectWrites(child, set)
		}
	}
}

// writesOf are the variables one step writes.
func writesOf(n model.Node) []string {
	var out []string
	for _, v := range n.Values {
		if v.Name != "" {
			out = append(out, v.Name)
		}
	}
	for _, p := range n.Params {
		value, _ := p.Value.(string)
		if n.Type == "set" && p.Key == "target" {
			if m := varReadPattern.FindStringSubmatch(value); m != nil {
				out = append(out, m[1])
			}
		}
	}
	return out
}

func matches(re *regexp.Regexp, text string) []string {
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		seen[m[1]] = true
	}
	return sortedKeys(seen)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
