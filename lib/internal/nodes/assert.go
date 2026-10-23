// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

func assertSpec() Spec {
	return Spec{
		Type: "assert", Group: "Check", Summary: "Check a condition; on_fail: error stops, warn continues.",
		Params: []Param{
			{Name: "check", Kind: ParamExpr, Required: true},
			{Name: "on_fail", Kind: ParamEnum, Enum: []string{"error", "warn"}},
			{Name: "message", Kind: ParamText},
		},
		Run: runAssert,
	}
}

// runAssert checks a condition. on_fail: error (default) fails the node;
// warn reports a warning and carries on.
func runAssert(_ context.Context, n model.Node, s *State) Outcome {
	check := stringParam(n, "check")
	ok, _, err := template.Check(check, s.Scope())
	if err != nil {
		return Outcome{Diags: templateDiags(n, "check", err)}
	}
	if ok {
		return Outcome{}
	}
	return Outcome{Diags: Failed(n.ID, check, stringParam(n, "message"), stringParam(n, "on_fail") == "warn", s.Emit)}
}

// Failed reports a failed assertion: a warning through emit when warn is
// set, otherwise an error diagnostic. Shared with a version's assert: list.
func Failed(node, check, message string, warn bool, emit Emitter) diag.List {
	if message != "" {
		message = " — " + message
	}
	args := map[string]string{"check": check, "message": message, "node": node}
	if warn {
		emit.Warn(diag.New(diag.CodeRunAssertWarning, diag.Location{}, args))
		return nil
	}
	return diag.List{diag.New(diag.CodeRunAssertFailed, diag.Location{}, args)}
}
