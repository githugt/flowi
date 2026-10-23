// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"

	"github.com/githugt/flowi-lib/model"
)

// An extract node has two forms: read one value from the response (name, from,
// and the one field that says where in that source), or set several variables
// from templates. The form follows the node's "from"; an inspector shows only
// what applies, instead of every field of both.

// ParamValue is the value of parameter key, nil when the node has none.
func ParamValue(n model.Node, key string) any {
	for _, f := range n.Params {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

// SetParam sets key, keeping the parameter's place (or appending it).
func SetParam(fs []model.Field, key string, v any) []model.Field {
	out := slices.Clone(fs)
	if i := slices.IndexFunc(out, func(f model.Field) bool { return f.Key == key }); i >= 0 {
		out[i].Value = v
		return out
	}
	return append(out, model.Field{Key: key, Value: v})
}

// DropParam removes key.
func DropParam(fs []model.Field, key string) []model.Field {
	return slices.DeleteFunc(slices.Clone(fs), func(f model.Field) bool { return f.Key == key })
}

// ExtractSource is the node's "from", "" until one is chosen.
func ExtractSource(n model.Node) string { return ParamText(n, "from") }

// ParamShown says whether parameter name of node n applies to what is chosen.
func ParamShown(n model.Node, name string) bool {
	if n.Type == "condition" && name == "check" {
		return len(n.Cases) == 0 // a condition with cases tests in its cases; its own check is not used
	}
	if n.Type != "extract" {
		return true
	}
	from := ExtractSource(n)
	switch name {
	case "name":
		return from != ""
	case "path":
		return from == "body"
	case "header":
		return from == "header"
	case "cookie":
		return from == "cookie"
	case "regex":
		return from == "body" || from == "header" || from == "cookie"
	}
	return true
}

// VariablesShown says whether the variables rows apply: an extract that reads
// one value ignores them.
func VariablesShown(n model.Node) bool { return n.Type != "extract" || ExtractSource(n) == "" }
