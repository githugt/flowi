// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/template"
)

// Verdicts of EvalChecks.
const (
	CheckPassed  = "passed"
	CheckFailed  = "failed"
	CheckError   = "error"   // the send could not evaluate the check (it names something that is not there)
	CheckUnknown = "unknown" // EvalChecks only: the check names something only a send has (a variable, the environment)
)

// CheckEval is what one check said about a response: from a send, or from EvalChecks on one already received.
type CheckEval = nodes.Check

// EvalChecks tries each check of a request on the response in rec, without sending anything: a check being
// written is judged on the response already on screen, so trying it does not need a second request (which
// would create another record, for a POST that makes one). A check sees what a request's own checks see of a
// response (status, response.body, response.headers, response.time); one that reads a variable, the
// environment or a secret cannot be judged without a send and is unknown.
func EvalChecks(rec Record, lines []string) []CheckEval {
	scope := responseScope(rec)
	out := make([]CheckEval, 0, len(lines))
	for _, line := range lines {
		ok, _, err := template.Check(line, scope)
		switch {
		case err != nil:
			out = append(out, CheckEval{Line: line, Verdict: CheckUnknown, Note: err.Error()})
		case ok:
			out = append(out, CheckEval{Line: line, Verdict: CheckPassed})
		default:
			out = append(out, CheckEval{Line: line, Verdict: CheckFailed, Note: nodes.Saw(line, scope)})
		}
	}
	return out
}

// responseScope is the scope a request's checks run in, from a stored record.
func responseScope(rec Record) template.Scope {
	var body any = rec.Body.Text
	if rec.Body.JSON != nil {
		body = scopeValue(rec.Body.JSON)
	}
	resp := nodes.Response{Status: rec.Status, Header: headerOf(&rec), Body: body, Text: rec.Body.Text, Time: rec.Timings.Total}
	return template.Scope{Values: map[string]any{"response": resp.Values(), "status": rec.Status}}
}
