// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"errors"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// Runner runs one node.
type Runner func(ctx context.Context, n model.Node, s *State) Outcome

// ParamKind is what a node parameter holds.
type ParamKind int

// Parameter kinds.
const (
	ParamString   ParamKind = iota + 1 // a literal string (a set target, a var name)
	ParamText                          // a {{…}} template
	ParamExpr                          // a bare expression (check, until)
	ParamDuration                      // a duration literal: 500ms, 2s
	ParamInt
	ParamBool
	ParamEnum
	ParamName // a plain file name (names.Valid)
	ParamAny  // any value; strings inside are templates
	ParamMap  // a mapping of names to templates
)

// Param describes one node parameter.
type Param struct {
	Name     string
	Kind     ParamKind
	Required bool
	Enum     []string
	Min, Max int64 // ParamInt bounds; 0 and 0 mean unbounded
	Doc      string
}

// Spec describes a node type: its catalogue entry, parameters and runner.
type Spec struct {
	Type    model.NodeType
	Group   string // Call, Data, Check, Control, Output
	Summary string
	Params  []Param
	// MapForm means every key that is not a Param is "<var>: template".
	MapForm bool
	// Typed lists the keys the codec reads into typed fields (request,
	// flow, with, body, branches, …); they are not Params.
	Typed []string
	Run   Runner
}

// Param returns the spec's parameter named name.
func (s Spec) Param(name string) (Param, bool) {
	i := slices.IndexFunc(s.Params, func(p Param) bool { return p.Name == name })
	if i < 0 {
		return Param{}, false
	}
	return s.Params[i], true
}

// Specs returns every registered node type in catalogue order.
func Specs() []Spec {
	byType := map[model.NodeType]Spec{}
	for _, s := range specList() {
		byType[s.Type] = s
	}
	var out []Spec
	for _, t := range model.NodeTypes() {
		if s, ok := byType[t]; ok {
			out = append(out, s)
		}
	}
	return out
}

func specList() []Spec {
	return []Spec{
		setSpec(), assertSpec(), fuzzSpec(), logSpec(), noteSpec(),
		delaySpec(), conditionSpec(), batchSpec(), runTillSpec(), parallelSpec(), stopSpec(),
		requestSpec(), subflowSpec(), fetchSpec(), webhookSpec(), digestSpec(), saveSpec(),
	}
}

// Lookup returns the runner for a node type.
func Lookup(t model.NodeType) (Runner, bool) {
	for _, s := range specList() {
		if s.Type == t {
			return s.Run, true
		}
	}
	return nil, false
}

// param returns a node parameter by key.
func param(n model.Node, key string) (any, bool) {
	for _, f := range n.Params {
		if f.Key == key {
			return f.Value, true
		}
	}
	return nil, false
}

func stringParam(n model.Node, key string) string {
	v, _ := param(n, key)
	s, _ := v.(string)
	return s
}

// eval renders a parameter value: strings are templates (keeping their
// type when they are one expression), lists and maps are rendered leaf by
// leaf, anything else is used as is.
func eval(raw any, s *State) (any, error) {
	switch v := raw.(type) {
	case string:
		tpl, err := template.Parse(v)
		if err != nil {
			return nil, err
		}
		r, err := tpl.Eval(s.Scope())
		if err == nil {
			s.derived(r)
		}
		return r.Value, err
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			rendered, err := eval(item, s)
			if err != nil {
				return nil, err
			}
			out[i] = rendered
		}
		return out, nil
	case []model.Field:
		out := make(map[string]any, len(v))
		for _, f := range v {
			rendered, err := eval(f.Value, s)
			if err != nil {
				return nil, err
			}
			out[f.Key] = rendered
		}
		return out, nil
	}
	return raw, nil
}

func newDiag(n model.Node, c diag.Code, args map[string]string) diag.Diagnostic {
	if args == nil {
		args = map[string]string{}
	}
	args["node"] = n.ID
	return diag.New(c, diag.Location{}, args)
}

// failed is an outcome with one error diagnostic.
func failed(n model.Node, c diag.Code, args map[string]string) Outcome {
	return Outcome{Diags: diag.List{newDiag(n, c, args)}}
}

// templateDiags turns a template failure in a node field into diagnostics.
func templateDiags(n model.Node, field string, err error) diag.List {
	desc := field + " of node " + n.ID
	if names := template.Undefined(err); len(names) > 0 {
		var out diag.List
		for _, name := range names {
			out = append(out, newDiag(n, diag.CodeTemplateUndefined, map[string]string{"name": name, "field": desc, "hint": ""}))
		}
		return out
	}
	var ge *template.UnknownGeneratorError
	if errors.As(err, &ge) {
		return diag.List{newDiag(n, diag.CodeTemplateGenerator, map[string]string{"name": ge.Name})}
	}
	detail := err.Error()
	var se *template.SyntaxError
	if errors.As(err, &se) {
		detail = se.Detail
	}
	return diag.List{newDiag(n, diag.CodeTemplateSyntax, map[string]string{"field": desc, "detail": detail})}
}
