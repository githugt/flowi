// SPDX-License-Identifier: Apache-2.0

package blueprint

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

const maxParamName = 64

var paramName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Check validates a blueprint's own structure: parameter names and types,
// that every {param: x} names a declared parameter of the right type, and
// that reference parameters are used. The flow body is validated as an
// ordinary flow once the blueprint has been used.
func Check(bp model.Blueprint) diag.List {
	c := checker{bp: bp, declared: map[string]model.Param{}, used: map[string]bool{}}
	c.checkParams()
	walkNodes(bp.Body.Nodes, c.checkNode)
	c.reportUnused()
	return c.diags
}

type checker struct {
	bp       model.Blueprint
	declared map[string]model.Param
	used     map[string]bool
	diags    diag.List
}

func (c *checker) add(code diag.Code, args map[string]string) {
	c.diags = append(c.diags, diag.New(code, diag.Location{File: c.bp.File}, args))
}

func (c *checker) checkParams() {
	inputs := map[string]bool{}
	for _, in := range c.bp.Body.Inputs {
		inputs[in.Name] = true
	}
	for _, p := range c.bp.Params {
		switch {
		case len(p.Name) > maxParamName || !paramName.MatchString(p.Name):
			c.add(diag.CodeBlueprintParamName, map[string]string{"name": p.Name, "detail": "a name is letters, digits and underscores, starting with a letter or underscore"})
			continue
		case slices.ContainsFunc(c.diagsNames(), func(n string) bool { return n == p.Name }):
			c.add(diag.CodeBlueprintParamName, map[string]string{"name": p.Name, "detail": "declared twice"})
			continue
		case isValueType(p.Type) && inputs[p.Name]:
			c.add(diag.CodeBlueprintParamName, map[string]string{"name": p.Name, "detail": "the blueprint already declares an input with this name"})
		}
		if !slices.Contains(model.ParamTypes(), p.Type) {
			c.add(diag.CodeBlueprintParamType, map[string]string{"name": p.Name, "detail": fmt.Sprintf("unknown type %q", p.Type)})
		}
		c.declared[p.Name] = p
	}
}

// diagsNames lists the parameter names accepted so far.
func (c *checker) diagsNames() []string {
	names := make([]string, 0, len(c.declared))
	for n := range c.declared {
		names = append(names, n)
	}
	return names
}

func isValueType(t model.ParamType) bool {
	return t == model.ParamString || t == model.ParamInt || t == model.ParamBool
}

func (c *checker) checkNode(n model.Node) {
	if n.Request != nil && n.Request.Request.Param != "" {
		c.checkRef(n, n.Request.Request.Param, model.ParamRequestRef)
	}
	if n.Subflow != nil && n.Subflow.Flow.Param != "" {
		c.checkRef(n, n.Subflow.Flow.Param, model.ParamFlowRef)
	}
}

func (c *checker) checkRef(n model.Node, param string, want model.ParamType) {
	c.used[param] = true
	p, ok := c.declared[param]
	switch {
	case !ok:
		c.add(diag.CodeBlueprintRef, map[string]string{"node": n.ID, "param": param, "detail": "it is not declared under params"})
	case p.Type != want:
		c.add(diag.CodeBlueprintRef, map[string]string{"node": n.ID, "param": param, "detail": fmt.Sprintf("this reference needs a %s parameter, and it is %s", want, p.Type)})
	}
}

func (c *checker) reportUnused() {
	for _, p := range c.bp.Params {
		if (p.Type == model.ParamRequestRef || p.Type == model.ParamFlowRef) && !c.used[p.Name] {
			d := diag.New(diag.CodeBlueprintParamUnused, diag.Location{File: c.bp.File}, map[string]string{"name": p.Name})
			c.diags = append(c.diags, d)
		}
	}
}

// listNames renders parameter names for messages.
func listNames(params []model.Param) string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}
