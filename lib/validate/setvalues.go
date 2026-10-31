// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// setSources are the places a set value may come from; "" means expr.
func setSources() []string { return []string{"", "expr", "literal", "response", "generate"} }

// responsePlaces are the parts of a response a value may read.
func responsePlaces() []string { return []string{"body", "header", "cookie", "status"} }

// checkSetValues reports the values of set nodes that cannot be run: no
// name, an unknown source, a response value that does not say which part, or
// two values that write one name.
func checkSetValues(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, n := range sites.AllNodes(c) {
			out = append(out, checkValues(n)...)
		}
	}
	return out
}

func checkValues(n sites.Located[model.Node]) diag.List {
	var out diag.List
	seen := map[string]bool{}
	for i, v := range n.Value.Values {
		at := yamlpath.Index(yamlpath.Key(n.Path, "values"), i)
		problem := func(key, detail string) {
			loc := diag.Location{File: n.File, Path: yamlpath.Key(at, key)}
			out = append(out, diag.New(diag.CodeNodeParam, loc, map[string]string{"field": "value " + v.Name, "node": n.Value.ID, "detail": detail}))
		}
		switch {
		case v.Name == "":
			problem("name", "every value needs a name")
		case seen[v.Name]:
			problem("name", v.Name+" is written twice by this step")
		}
		seen[v.Name] = true
		checkSource(v, problem)
	}
	return out
}

func checkSource(v model.SetValue, problem func(key, detail string)) {
	if !slices.Contains(setSources(), v.From) {
		problem("from", "unknown source "+v.From+" (use expr, literal, response or generate)")
		return
	}
	if v.From != "response" {
		return
	}
	switch {
	case !slices.Contains(responsePlaces(), v.Where):
		problem("in", "say which part of the response: body, header, cookie or status")
	case v.Where == "header" && v.Header == "":
		problem("header", "name the header to read")
	case v.Where == "cookie" && v.Cookie == "":
		problem("cookie", "name the cookie to read")
	case v.Where == "body" && v.Path == "" && v.Regex == "":
		problem("path", "give a JSONPath or a regex to read from the body")
	}
}
