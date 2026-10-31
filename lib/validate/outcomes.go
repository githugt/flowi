// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// checkOutcomes reports what is wrong with the outcomes: list of every request version.
func checkOutcomes(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
				out = append(out, outcomeDiags(r.File, yamlpath.Key(base, "outcomes"), v.Outcomes)...)
			}
		}
	}
	return out
}

// checkStepOutcomes reports what is wrong with the outcomes request steps carry in place of their request's.
func checkStepOutcomes(l sites.NodeList) diag.List {
	var out diag.List
	for i, n := range l.Value {
		if n.Request != nil && len(n.Request.Outcomes) > 0 {
			out = append(out, outcomeDiags(l.File, yamlpath.Key(yamlpath.Index(l.Path, i), "outcomes"), n.Request.Outcomes)...)
		}
	}
	return out
}

// outcomeDiags checks one list. path is the path of the list itself.
func outcomeDiags(file, path string, outcomes []model.Outcome) diag.List {
	var out diag.List
	seen := map[string]bool{}
	for i, o := range outcomes {
		item := yamlpath.Index(path, i)
		problem := func(key, detail string) {
			out = append(out, diag.New(diag.CodeOutcome, diag.Location{File: file, Path: yamlpath.Key(item, key)}, map[string]string{"detail": detail}))
		}
		switch {
		case !model.ValidOutcomeName(o.Name):
			problem("name", "an outcome name is letters, digits, - and _ (got \""+o.Name+"\")")
		case seen[o.Name]:
			problem("name", "the outcome \""+o.Name+"\" is listed twice")
		}
		seen[o.Name] = true
		if o.Color != "" && !slices.Contains(model.NodeColors(), o.Color) {
			problem("color", "the colour is one of "+strings.Join(model.NodeColors(), ", ")+" (got \""+o.Color+"\")")
		}
		switch {
		case o.Builtin() && o.When != "":
			problem("when", o.Name+" is built in: it means the request passed (or failed) and takes no rule; give a named outcome the rule instead")
		case !o.Builtin() && strings.TrimSpace(o.When) == "":
			problem("when", "the outcome \""+o.Name+"\" needs a rule (when), for example response.status == 404")
		case o.When != "":
			if err := template.CheckSyntax(o.When); err != nil {
				problem("when", "the rule does not parse: "+err.Error())
			}
		}
	}
	return out
}

// afterOutcomeDiags checks one after_outcome entry, "step:outcome", of node i of list l: both parts are
// there, the outcome name is valid and the step is a sibling.
func afterOutcomeDiags(l sites.NodeList, i, k int, entry string, siblings map[string]bool) diag.List {
	path := yamlpath.Index(yamlpath.Key(yamlpath.Index(l.Path, i), "after_outcome"), k)
	step, outcome, ok := strings.Cut(entry, ":")
	switch {
	case !ok || step == "" || !model.ValidOutcomeName(outcome):
		return diag.List{diag.New(diag.CodeOutcome, diag.Location{File: l.File, Path: path}, map[string]string{"detail": "after_outcome entries are written step:outcome, for example login:not-found (got \"" + entry + "\")"})}
	case !siblings[step]:
		return diag.List{diag.New(diag.CodeNodeNeedsSibling, diag.Location{File: l.File, Path: path}, map[string]string{"target": step})}
	}
	return nil
}
