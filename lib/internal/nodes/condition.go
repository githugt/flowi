// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"strconv"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

func conditionSpec() Spec {
	return Spec{
		Type: "condition", Group: "Check", Summary: "Branch: check with then/else, or cases (first true when) with else as default.",
		Params: []Param{{Name: "check", Kind: ParamExpr, Doc: "True or false, written without braces, e.g. nodes.login.status == 200. then runs when it is true, else when it is not; leave it empty when you use cases."}},
		Typed:  []string{"then", "else", "cases"},
		Run:    runCondition,
	}
}

// runCondition runs the chosen branch in the enclosing scope, so its
// nodes are visible to later siblings.
func runCondition(ctx context.Context, n model.Node, s *State) Outcome {
	label, list, out := chooseBranch(n, s)
	if out.Diags.HasErrors() {
		return out
	}
	if len(list) == 0 {
		return Outcome{Outputs: map[string]any{"branch": "none"}}
	}
	results := s.Runtime.Lanes(ctx, []Lane{{Label: label, Nodes: list}}, LaneOptions{Concurrency: 1})
	return Outcome{Outputs: map[string]any{"branch": label}, Status: combine(results), Diags: diagsOf(results)}
}

// chooseBranch picks then/else, or the first case whose when holds.
func chooseBranch(n model.Node, s *State) (string, []model.Node, Outcome) {
	if len(n.Cases) == 0 {
		ok, _, err := template.Check(stringParam(n, "check"), s.Scope())
		if err != nil {
			return "", nil, Outcome{Diags: templateDiags(n, "check", err)}
		}
		if ok {
			return "then", n.Then, Outcome{}
		}
		return "else", n.Else, Outcome{}
	}
	for i, c := range n.Cases {
		ok, _, err := template.Check(c.When, s.Scope())
		if err != nil {
			return "", nil, Outcome{Diags: templateDiags(n, "cases["+strconv.Itoa(i)+"].when", err)}
		}
		if ok {
			return "case:" + strconv.Itoa(i), c.Nodes, Outcome{}
		}
	}
	return "else", n.Else, Outcome{}
}
