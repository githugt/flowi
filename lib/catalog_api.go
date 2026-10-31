// SPDX-License-Identifier: Apache-2.0

package flowi

import "github.com/githugt/flowi/lib/internal/nodes"

// NodeSpec describes a flow node type for editors: its catalogue entry
// and parameters. Typed fields (request, flow, with, body, branches, …)
// are not listed as parameters.
type NodeSpec struct {
	Type    string
	Group   string // Call, Data, Check, Control, Output
	Summary string
	MapForm bool // other keys are "<variable>: template"
	Params  []NodeParam
	Typed   []string
}

// NodeParam is one parameter of a node type. Kind is string, text
// (a {{…}} template), expr (a bare expression), duration, int, bool,
// enum, name (a plain file name), any or map.
type NodeParam struct {
	Name     string
	Kind     string
	Doc      string
	Required bool
	Enum     []string
	Min, Max int64
}

// NodeCatalog lists every node type in catalogue order.
func NodeCatalog() []NodeSpec {
	var out []NodeSpec
	for _, s := range nodes.Specs() {
		spec := NodeSpec{Type: string(s.Type), Group: s.Group, Summary: s.Summary, MapForm: s.MapForm, Typed: s.Typed}
		for _, p := range s.Params {
			spec.Params = append(spec.Params, NodeParam{Name: p.Name, Kind: kindName(p.Kind), Doc: p.Doc, Required: p.Required, Enum: p.Enum, Min: p.Min, Max: p.Max})
		}
		out = append(out, spec)
	}
	return out
}

func kindName(k nodes.ParamKind) string {
	names := map[nodes.ParamKind]string{
		nodes.ParamString: "string", nodes.ParamText: "text", nodes.ParamExpr: "expr", nodes.ParamDuration: "duration",
		nodes.ParamInt: "int", nodes.ParamBool: "bool", nodes.ParamEnum: "enum", nodes.ParamName: "name",
		nodes.ParamAny: "any", nodes.ParamMap: "map",
	}
	return names[k]
}

// Param returns the parameter named name.
func (s NodeSpec) Param(name string) (NodeParam, bool) {
	for _, p := range s.Params {
		if p.Name == name {
			return p, true
		}
	}
	return NodeParam{}, false
}
