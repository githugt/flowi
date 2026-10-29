// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"strconv"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

func assertSpec() Spec {
	return Spec{
		Type: "assert", Group: "Check", Summary: "Check a condition, or that an earlier request's response matches its kept schema; on_fail: error stops, warn continues.",
		Params: []Param{
			{Name: "check", Kind: ParamExpr, Doc: "True or false, written without braces. Give check, schema or both."},
			{Name: "schema", Kind: ParamString, Doc: "The id of an earlier request step: passes when what it received matches the response schema kept for its request."},
			{Name: "on_fail", Kind: ParamEnum, Enum: []string{"error", "warn"}},
			{Name: "message", Kind: ParamText},
		},
		Run: runAssert,
	}
}

// runAssert checks a condition and/or a response schema; both must hold.
// on_fail: error (default) fails the node; warn reports a warning and carries
// on.
func runAssert(_ context.Context, n model.Node, s *State) Outcome {
	check, step := stringParam(n, "check"), stringParam(n, "schema")
	if check == "" && step == "" {
		return Outcome{Diags: diag.List{diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "(nothing to check: give check or schema)", "message": "", "node": n.ID})}}
	}
	warn := stringParam(n, "on_fail") == "warn"
	if check != "" {
		if d := runCheck(n, s, check, warn); len(d) > 0 {
			return Outcome{Diags: d}
		}
	}
	if step != "" {
		return Outcome{Diags: runSchemaCheck(n, s, step, warn)}
	}
	return Outcome{}
}

func runCheck(n model.Node, s *State, check string, warn bool) diag.List {
	ok, _, err := template.Check(check, s.Scope())
	if err != nil {
		return templateDiags(n, "check", err)
	}
	if ok {
		return nil
	}
	return Failed(n.ID, check, stringParam(n, "message"), warn, s.Emit)
}

// runSchemaCheck fails when the step has no kept schema or its response does
// not match it; the first mismatches are reported by path.
func runSchemaCheck(n model.Node, s *State, step string, warn bool) diag.List {
	var res SchemaCheck
	if s.Runtime != nil {
		res = s.Runtime.CheckSchema(step)
	}
	if !res.Found {
		return diag.List{diag.New(diag.CodeRunAssertNoSchema, diag.Location{}, map[string]string{"node": n.ID, "step": step})}
	}
	var out diag.List
	for _, v := range res.Violations[:min(len(res.Violations), maxSchemaDiags)] {
		d := diag.New(diag.CodeSchemaResponse, diag.Location{}, map[string]string{
			"request": res.Request, "path": v.Path, "message": v.Message, "status": strconv.Itoa(res.Status),
		})
		if warn {
			d.Severity = diag.SeverityWarning
			s.Emit.Warn(d)
			continue
		}
		out = append(out, d)
	}
	return out
}

// maxSchemaDiags caps the mismatches one check reports, as a send's note does.
const maxSchemaDiags = 3

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
