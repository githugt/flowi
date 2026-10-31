// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"maps"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// renderGiven renders input values given as templates (--input k=v).
func (r *runner) renderGiven(given map[string]string, sc template.Scope) (map[string]any, diag.List) {
	out := make(map[string]any, len(given))
	var diags diag.List
	for _, k := range slices.Sorted(maps.Keys(given)) {
		v, err := r.evalTemplate(given[k], sc)
		if err != nil {
			diags = append(diags, templateFailure("input "+k, err)...)
			continue
		}
		out[k] = v
	}
	return out, diags
}

// resolveInputs gives every declared input a value: the given one, else
// its default template, else FLW-RUN-009. Undeclared names are
// FLW-RUN-010 (plan Decision 10).
func (r *runner) resolveInputs(fl model.Flow, given map[string]any, sc template.Scope) (map[string]any, diag.List) {
	var diags diag.List
	for _, name := range slices.Sorted(maps.Keys(given)) {
		if !slices.ContainsFunc(fl.Inputs, func(in model.Input) bool { return in.Name == name }) {
			diags = append(diags, diag.New(diag.CodeRunInputUnknown, diag.Location{}, map[string]string{"name": name, "flow": fl.Path}))
		}
	}
	out := map[string]any{}
	for _, in := range fl.Inputs {
		if v, ok := given[in.Name]; ok {
			out[in.Name] = v
			continue
		}
		if in.Default == "" {
			diags = append(diags, diag.New(diag.CodeRunInputMissing, diag.Location{}, map[string]string{"name": in.Name, "flow": fl.Path}))
			continue
		}
		v, err := r.evalTemplate(in.Default, sc)
		if err != nil {
			diags = append(diags, templateFailure("default of input "+in.Name, err)...)
			continue
		}
		out[in.Name] = v
	}
	return out, diags
}

// renderOutputs renders a subflow's declared outputs in its final scope.
func (r *runner) renderOutputs(outs []model.Output, sc template.Scope) (map[string]any, diag.List) {
	out := make(map[string]any, len(outs))
	var diags diag.List
	for _, o := range outs {
		v, err := r.evalTemplate(o.Value, sc)
		if err != nil {
			diags = append(diags, templateFailure("output "+o.Name, err)...)
			continue
		}
		out[o.Name] = v
	}
	return out, diags
}

// evalTemplate renders src keeping the type of a single expression, and
// registers results built from secrets.
func (r *runner) evalTemplate(src string, sc template.Scope) (any, error) {
	tpl, err := template.Parse(src)
	if err != nil {
		return nil, err
	}
	res, err := tpl.Eval(sc)
	if err != nil {
		return nil, err
	}
	if res.Tainted {
		r.secrets.derived(res.Value)
	}
	return res.Value, nil
}
