// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/fuzzcases"
	"github.com/githugt/flowi/lib/model"
)

const (
	defaultFuzzCases = 20
	maxFuzzCases     = 200
	maxFailuresShown = 3
)

func fuzzSpec() Spec {
	return Spec{
		Type: "fuzz", Group: "Check", Summary: "Send a request with bad bodies made from its good one, and fail when the server breaks.",
		Typed: []string{"request", "version", "overlay", "hooks", "with", "headers"},
		Params: []Param{
			{Name: "count", Kind: ParamInt, Min: 1, Max: maxFuzzCases, Doc: "How many bad bodies to send (default 20). They are spread over the fields, in a fixed order."},
			{Name: "expect", Kind: ParamEnum, Enum: []string{"no_server_error", "rejected"},
				Doc: "What passes: no_server_error (default) fails on a 5xx or no answer; rejected also fails when the server accepts a bad body (a 2xx or 3xx)."},
		},
		Run: runFuzz,
	}
}

// runFuzz sends the request once for each bad body made from its good one and fails when the server answers any
// of them with a server error or not at all, or, with expect: rejected, accepts one. The step's outputs hold
// the number of cases, the failures (each with its case name and status) and how many cases got each status.
func runFuzz(ctx context.Context, n model.Node, s *State) Outcome {
	call := n.Request
	if call == nil {
		return failed(n, diag.CodeNodeMissingField, map[string]string{"type": "fuzz", "field": "request"})
	}
	with, diags := renderValues(n, "with ", call.With, s)
	headers, headerDiags := renderTexts(n, "header ", call.Headers, s)
	if diags = append(diags, headerDiags...); diags.HasErrors() {
		return Outcome{Diags: diags}
	}
	base := RequestCall{Ref: call.Request, Version: call.Version, Overlay: call.Overlay, Hooks: call.Hooks, With: with, Headers: headers}
	subject, err := s.Runtime.FuzzSubject(ctx, base)
	if err != nil {
		return Outcome{Status: StatusError, Diags: diag.List{diag.New(diag.CodeRunFuzzFailed, diag.Location{}, map[string]string{
			"node": n.ID, "failed": "0", "cases": "0", "first": err.Error()})}}
	}
	count := intParam(n, "count", defaultFuzzCases)
	rejected := stringParam(n, "expect") == "rejected"
	var failures []any
	statuses := map[string]int{}
	cases := fuzzcases.Cases(subject.Body, subject.Schema, count)
	for _, c := range cases {
		if ctx.Err() != nil {
			return Outcome{Status: StatusCancelled}
		}
		body := c.Body
		attempt := base
		attempt.Body = &body
		got := s.Runtime.Request(ctx, attempt)
		status, why := judgeFuzz(got, rejected)
		statuses[strconv.Itoa(status)]++
		if why != "" {
			failures = append(failures, map[string]any{"case": c.Name, "status": status, "why": why})
		}
	}
	outputs := map[string]any{"cases": len(cases), "failed": len(failures), "failures": failures, "statuses": statuses}
	if len(failures) == 0 {
		return Outcome{Outputs: outputs}
	}
	return Outcome{Outputs: outputs, Status: StatusFailed, Diags: diag.List{fuzzDiag(n.ID, len(cases), failures)}}
}

// judgeFuzz is the status a case got and, when it breaks the expectation, why.
func judgeFuzz(o Outcome, rejected bool) (status int, why string) {
	status = statusOfOutputs(o.Outputs)
	switch {
	case status == 0:
		return 0, "no answer"
	case status >= 500:
		return status, fmt.Sprintf("the server failed (%d)", status)
	case rejected && status >= 200 && status < 400:
		return status, fmt.Sprintf("the server accepted the bad body (%d)", status)
	}
	return status, ""
}

func statusOfOutputs(out map[string]any) int {
	switch v := out["status"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func fuzzDiag(node string, cases int, failures []any) diag.Diagnostic {
	var first []string
	for _, f := range failures[:min(len(failures), maxFailuresShown)] {
		m := f.(map[string]any)
		first = append(first, fmt.Sprintf("%v → %v", m["case"], m["why"]))
	}
	return diag.New(diag.CodeRunFuzzFailed, diag.Location{}, map[string]string{
		"node": node, "failed": strconv.Itoa(len(failures)), "cases": strconv.Itoa(cases), "first": strings.Join(first, "; ")})
}
