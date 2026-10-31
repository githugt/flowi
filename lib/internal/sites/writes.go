// SPDX-License-Identifier: Apache-2.0

package sites

import (
	"maps"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// OwnWrites are the targets one node writes itself.
func OwnWrites(n model.Node) []string {
	if n.Type != "set" {
		return nil
	}
	if len(n.Values) > 0 {
		out := make([]string, 0, len(n.Values))
		for _, v := range n.Values {
			if v.Name != "" {
				out = append(out, "vars."+v.Name)
			}
		}
		return out
	}
	if t, _ := ParamValue(n, "target").(string); t != "" {
		return []string{t}
	}
	return nil
}

// GeneratedVars are the variables a node fills with generated values: fresh
// data, not taken from another step.
func GeneratedVars(n model.Node) map[string]bool {
	out := map[string]bool{}
	for _, v := range n.Values {
		if v.From == "generate" {
			out[v.Name] = true
		}
	}
	return out
}

// WrittenVars is every variable name written anywhere in a node list,
// batch bodies included.
func WrittenVars(list []model.Node) map[string]bool {
	out := map[string]bool{}
	for _, n := range list {
		for _, t := range OwnWrites(n) {
			if name, ok := strings.CutPrefix(t, "vars."); ok {
				out[name] = true
			}
		}
		for _, child := range n.Children() {
			maps.Copy(out, WrittenVars(child))
		}
	}
	return out
}

// HasParam reports whether n sets the parameter key.
func HasParam(n model.Node, key string) bool {
	for _, p := range n.Params {
		if p.Key == key {
			return true
		}
	}
	return false
}

// ParamValue is the value of parameter key, or nil.
func ParamValue(n model.Node, key string) any {
	for _, p := range n.Params {
		if p.Key == key {
			return p.Value
		}
	}
	return nil
}
