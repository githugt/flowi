// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"strconv"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// maxBatchTimes bounds times: so a typo cannot start a billion iterations.
const maxBatchTimes = 100000

func batchSpec() Spec {
	return Spec{
		Type: "batch", Group: "Control", Summary: "Run the body for each item of a list, or N times, with bounded concurrency.",
		Params: []Param{
			{Name: "over", Kind: ParamText, Doc: "A list to iterate (item, index in scope). Exactly one of over and times."},
			{Name: "times", Kind: ParamInt, Min: 1, Max: maxBatchTimes, Doc: "Iterate N times (item = index)."},
			{Name: "concurrency", Kind: ParamInt, Min: 1, Doc: "Iterations in flight (default 1); capped by the run-wide limit."},
		},
		Typed: []string{"body"},
		Run:   runBatch,
	}
}

// runBatch runs one isolated lane per item and gathers the results in
// input order.
func runBatch(ctx context.Context, n model.Node, s *State) Outcome {
	items, out := batchItems(n, s)
	if out.Diags.HasErrors() {
		return out
	}
	lanes := make([]Lane, len(items))
	for i, item := range items {
		lanes[i] = Lane{Label: "[" + strconv.Itoa(i) + "]", Nodes: n.Body, OwnVars: true, OwnNodes: true, Item: item, Index: i, HasItem: true}
	}
	results := s.Runtime.Lanes(ctx, lanes, LaneOptions{Concurrency: intParam(n, "concurrency", 1), LaneJars: true})
	return Outcome{Outputs: batchOutputs(items, results), Status: combine(results), Diags: diagsOf(results)}
}

func batchItems(n model.Node, s *State) ([]any, Outcome) {
	if _, hasOver := param(n, "over"); !hasOver {
		count := intParam(n, "times", 0)
		if count < 1 || count > maxBatchTimes {
			// validate refuses this; the engine is public, so refuse it here too
			return nil, failed(n, diag.CodeNodeParam, map[string]string{"field": "times", "detail": "must be between 1 and " + strconv.Itoa(maxBatchTimes)})
		}
		items := make([]any, count)
		for i := range items {
			items[i] = i
		}
		return items, Outcome{}
	}
	raw, _ := param(n, "over")
	v, err := eval(raw, s)
	if err != nil {
		return nil, Outcome{Diags: templateDiags(n, "over", err)}
	}
	list, ok := v.([]any)
	if !ok {
		return nil, failed(n, diag.CodeRunBatchOver, map[string]string{"detail": "got " + describe(v) + " from " + template.String(raw)})
	}
	return list, Outcome{}
}

func batchOutputs(items []any, results []LaneResult) map[string]any {
	out := make([]any, len(results))
	passed, failedCount := 0, 0
	for i, r := range results {
		switch r.Status {
		case StatusPassed:
			passed++
		case StatusFailed, StatusError:
			failedCount++
		}
		out[i] = map[string]any{"index": i, "item": items[i], "status": string(r.Status), "vars": r.Vars, "nodes": r.Nodes}
	}
	return map[string]any{"results": out, "count": len(results), "passed": passed, "failed": failedCount}
}
