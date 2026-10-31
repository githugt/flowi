// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func parallelSpec() Spec {
	return Spec{
		Type: "parallel", Group: "Control", Summary: "Run branches at once; wait for all, any, or the first success.",
		Params: []Param{
			{Name: "wait", Kind: ParamEnum, Enum: []string{"all", "any", "first-success"}, Doc: "Default all."},
			{Name: "concurrency", Kind: ParamInt, Min: 1, Doc: "Branches in flight (default: all); capped by the run-wide limit."},
		},
		Typed: []string{"branches"},
		Run:   runParallel,
	}
}

// runParallel runs one lane per branch. Branch nodes write to the
// enclosing scope; validate rejects branches writing the same variable.
func runParallel(ctx context.Context, n model.Node, s *State) Outcome {
	wait := stringParam(n, "wait")
	if wait == "" {
		wait = "all"
	}
	lanes := make([]Lane, len(n.Branches))
	for i, b := range n.Branches {
		lanes[i] = Lane{Label: b.ID, Nodes: b.Nodes}
	}
	results := s.Runtime.Lanes(ctx, lanes, LaneOptions{Concurrency: intParam(n, "concurrency", len(lanes)), Wait: wait, LaneJars: true})
	outputs, winner := parallelOutputs(n.Branches, results)
	if wait == "all" {
		return Outcome{Outputs: outputs, Status: combine(results), Diags: diagsOf(results)}
	}
	return settled(n, wait, outputs, winner)
}

// settled is the outcome of an any / first-success wait: the winner's
// status; first-success without a passing branch fails.
func settled(n model.Node, wait string, outputs map[string]any, winner *LaneResult) Outcome {
	if winner == nil || (wait == "first-success" && winner.Status != StatusPassed) {
		return Outcome{Outputs: outputs, Status: StatusFailed, Diags: failed(n, diag.CodeRunNoBranchOK, nil).Diags}
	}
	return Outcome{Outputs: outputs, Status: winner.Status, Diags: winner.Diags}
}

func parallelOutputs(branches []model.Branch, results []LaneResult) (map[string]any, *LaneResult) {
	statuses := map[string]any{}
	outputs := map[string]any{"branches": statuses}
	var winner *LaneResult
	for i, r := range results {
		statuses[branches[i].ID] = map[string]any{"status": string(r.Status)}
		if r.First {
			winner = &results[i]
			outputs["winner"] = branches[i].ID
		}
	}
	return outputs, winner
}
