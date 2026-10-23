// SPDX-License-Identifier: Apache-2.0

package blueprint

import (
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Resolver turns the text a user gave for a request-ref or flow-ref
// parameter (an id or a path) into a reference. The error says why it did
// not resolve and is shown to the user.
type Resolver interface {
	Request(value string) (model.Ref, error)
	Flow(value string) (model.Ref, error)
}

// Input is everything Instantiate needs besides the blueprint.
type Input struct {
	Values  map[string]string // parameter name → value; defaults fill the gaps
	Resolve Resolver
	NewID   func() model.ID
	Path    string // of the new flow, relative to flows/
	Name    string // of the new flow
}

// Instantiate makes an ordinary flow from bp. Every problem found is
// reported (FLW-BP-005) and then no flow is returned.
func Instantiate(bp model.Blueprint, in Input) (model.Flow, diag.List) {
	// A blueprint that fails its own check (an undeclared parameter, a
	// clash with an input) would otherwise yield a flow with empty
	// references or duplicate inputs.
	if problems := Check(bp); problems.HasErrors() {
		return model.Flow{}, problems
	}
	u := user{bp: bp, in: in, refs: map[string]model.Ref{}}
	inputs := u.bind()
	if len(u.diags) > 0 {
		return model.Flow{}, u.diags
	}
	f := bp.Body
	f.Meta = model.Meta{ID: in.NewID(), Name: in.Name, Description: bp.Description}
	f.Path, f.File = in.Path, ""
	f.Inputs = append(slices.Clone(bp.Body.Inputs), inputs...)
	f.Outputs = slices.Clone(bp.Body.Outputs)
	f.Nodes = cloneNodes(bp.Body.Nodes, u.fillRefs)
	return f, nil
}

type user struct {
	bp    model.Blueprint
	in    Input
	refs  map[string]model.Ref // resolved reference parameters
	diags diag.List
}

func (u *user) fail(detail string) {
	u.diags = append(u.diags, diag.New(diag.CodeBlueprintUse, diag.Location{File: u.bp.File}, map[string]string{"blueprint": u.bp.Name, "detail": detail}))
}

// bind checks the values against the parameters, resolves references and
// returns the inputs the value parameters become.
func (u *user) bind() []model.Input {
	u.rejectUnknown()
	var inputs []model.Input
	for _, p := range u.bp.Params {
		value, ok := u.in.Values[p.Name]
		if !ok {
			value = p.Default
		}
		if value == "" {
			u.fail(fmt.Sprintf("parameter %q needs a value (--set %s=…)", p.Name, p.Name))
			continue
		}
		if input, isInput := u.bindOne(p, value); isInput {
			inputs = append(inputs, input)
		}
	}
	return inputs
}

func (u *user) rejectUnknown() {
	declared := map[string]bool{}
	for _, p := range u.bp.Params {
		declared[p.Name] = true
	}
	for _, name := range slices.Sorted(maps.Keys(u.in.Values)) {
		if !declared[name] {
			u.fail(fmt.Sprintf("there is no parameter %q (the blueprint has: %s)", name, listNames(u.bp.Params)))
		}
	}
}

func (u *user) bindOne(p model.Param, value string) (model.Input, bool) {
	switch p.Type {
	case model.ParamInt:
		if _, err := strconv.Atoi(value); err != nil {
			u.fail(fmt.Sprintf("parameter %q must be a whole number, got %q", p.Name, value))
		}
	case model.ParamBool:
		if value != "true" && value != "false" {
			u.fail(fmt.Sprintf("parameter %q must be true or false, got %q", p.Name, value))
		}
	case model.ParamRequestRef:
		u.resolve(p, value, func(r Resolver) (model.Ref, error) { return r.Request(value) })
		return model.Input{}, false
	case model.ParamFlowRef:
		u.resolve(p, value, func(r Resolver) (model.Ref, error) { return r.Flow(value) })
		return model.Input{}, false
	}
	return model.Input{Name: p.Name, Default: value}, true
}

func (u *user) resolve(p model.Param, _ string, look func(Resolver) (model.Ref, error)) {
	if u.in.Resolve == nil {
		u.fail(fmt.Sprintf("parameter %q names a %s, and there is nothing to look it up in", p.Name, p.Type))
		return
	}
	ref, err := look(u.in.Resolve)
	if err != nil {
		u.fail(fmt.Sprintf("parameter %q: %v", p.Name, err))
		return
	}
	u.refs[p.Name] = ref
}

// fillRefs replaces {param: x} references by the resolved ones.
func (u *user) fillRefs(n *model.Node) {
	if n.Request != nil && n.Request.Request.Param != "" {
		n.Request.Request = u.refs[n.Request.Request.Param]
	}
	if n.Subflow != nil && n.Subflow.Flow.Param != "" {
		n.Subflow.Flow = u.refs[n.Subflow.Flow.Param]
	}
}
