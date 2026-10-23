// SPDX-License-Identifier: Apache-2.0

package sites

import (
	"maps"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// OwnWrites are the targets one node writes itself.
func OwnWrites(n model.Node) []string {
	switch n.Type {
	case "set":
		if t, _ := ParamValue(n, "target").(string); t != "" {
			return []string{t}
		}
	case "transform":
		if into, _ := ParamValue(n, "into").(string); into != "" {
			return []string{"vars." + into}
		}
	case "extract", "generate", "calc":
		return mapFormWrites(n)
	}
	return nil
}

func mapFormWrites(n model.Node) []string {
	if n.Type == "extract" && SingleExtract(n) {
		if name, _ := ParamValue(n, "name").(string); name != "" {
			return []string{"vars." + name}
		}
		return nil
	}
	var out []string
	for _, f := range n.Params {
		if n.Type == "generate" && f.Key == "seed" {
			continue
		}
		out = append(out, "vars."+f.Key)
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

// singleExtractKeys mark the single form of extract ({name, from, …}); an
// extract without any of them is the map form (var: template).
func singleExtractKeys() []string {
	return []string{"name", "from", "path", "header", "cookie", "regex"}
}

// SingleExtract reports whether an extract node uses the single form
// (name, from, path…) rather than the map form.
func SingleExtract(n model.Node) bool {
	for _, k := range singleExtractKeys() {
		if HasParam(n, k) {
			return true
		}
	}
	return false
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
