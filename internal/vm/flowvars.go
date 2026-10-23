// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"regexp"
	"slices"

	"github.com/githugt/flowi-lib/model"
)

// FlowVars is what a flow does with variables and secrets, read from the flow
// text: a quick overview for the status bar, not an evaluation. Variables set
// by a request's own hooks or by a subflow are not seen, so Unset is a hint.
type FlowVars struct {
	Set     []string // variables a set, extract or generate step writes
	Read    []string // variables a template or expression reads
	Unset   []string // read but written by no step of this flow
	Secrets []string // secrets referenced as secret.<name>
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
	return v
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
	for _, p := range n.Params {
		value, _ := p.Value.(string)
		switch {
		case n.Type == "set" && p.Key == "target":
			if m := varReadPattern.FindStringSubmatch(value); m != nil {
				out = append(out, m[1])
			}
		case n.Type == "extract" && p.Key == "name" && value != "":
			out = append(out, value)
		case n.Type == "extract" && !isExtractOption(p.Key):
			out = append(out, p.Key) // map form: <variable>: template
		case n.Type == "generate":
			out = append(out, p.Key)
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

// isExtractOption tells an extract step's own options from the <variable>:
// template keys of its map form.
func isExtractOption(key string) bool {
	switch key {
	case "name", "from", "path", "header", "cookie", "regex":
		return true
	}
	return false
}
