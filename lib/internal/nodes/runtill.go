// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"maps"
	"strconv"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// Run-till limits: a missing max must not mean "forever".
const (
	defaultRunTillMax = 10
	maxRunTillMax     = 10000
)

func runTillSpec() Spec {
	return Spec{
		Type: "run_till", Group: "Control", Summary: "Repeat the body until a condition holds or a limit is reached.",
		Params: []Param{
			{Name: "until", Kind: ParamExpr, Required: true, Doc: "Checked after each iteration; sees that iteration's nodes."},
			{Name: "max", Kind: ParamInt, Min: 1, Max: maxRunTillMax, Doc: "Most iterations (default 10)."},
			{Name: "interval", Kind: ParamDuration, Doc: "Wait between iterations."},
			{Name: "max_duration", Kind: ParamDuration, Doc: "Stop once this much time has passed."},
		},
		Typed: []string{"body"},
		Run:   runRunTill,
	}
}

// runRunTill repeats the body in a fresh nodes scope each time; variables
// carry over, so counters work.
func runRunTill(ctx context.Context, n model.Node, s *State) Outcome {
	limit, maxDuration, interval := intParam(n, "max", defaultRunTillMax), durationParam(n, "max_duration"), durationParam(n, "interval")
	start := s.Runtime.Now()
	for i := 0; ; i++ {
		lane := Lane{Label: "[" + strconv.Itoa(i) + "]", Nodes: n.Body, OwnNodes: true}
		r := s.Runtime.Lanes(ctx, []Lane{lane}, LaneOptions{Concurrency: 1})[0]
		outputs := map[string]any{"iterations": i + 1, "met": false, "last": r.Nodes}
		if r.Status != StatusPassed {
			return Outcome{Outputs: outputs, Status: r.Status, Diags: r.Diags}
		}
		if s.Runtime.Stopped() {
			return Outcome{Outputs: outputs} // a stop node ended the run: the condition is not asked
		}
		met, err := untilHolds(n, s, r.Nodes)
		if err != nil {
			return Outcome{Outputs: outputs, Diags: templateDiags(n, "until", err)}
		}
		if met {
			outputs["met"] = true
			return Outcome{Outputs: outputs}
		}
		if reached := tillLimit(i+1, limit, s.Runtime.Now().Sub(start), maxDuration); reached != "" {
			return Outcome{Outputs: outputs, Diags: diag.List{newDiag(n, diag.CodeRunTillLimit, map[string]string{"limit": reached})}}
		}
		if interval > 0 && s.Runtime.Sleep(ctx, interval) != nil {
			return Outcome{Outputs: outputs, Status: StatusCancelled}
		}
	}
}

// untilHolds evaluates until with the iteration's nodes over the scope.
func untilHolds(n model.Node, s *State, laneNodes map[string]any) (bool, error) {
	sc := s.Scope()
	values := maps.Clone(sc.Values)
	merged := map[string]any{}
	if outer, ok := values["nodes"].(map[string]any); ok {
		maps.Copy(merged, outer)
	}
	maps.Copy(merged, laneNodes)
	values["nodes"] = merged
	sc.Values = values
	ok, _, err := template.Check(stringParam(n, "until"), sc)
	return ok, err
}

// tillLimit names the limit reached after done iterations, or "".
func tillLimit(done, limit int, elapsed, maxDuration time.Duration) string {
	if done >= limit {
		return "max " + strconv.Itoa(limit)
	}
	if maxDuration > 0 && elapsed >= maxDuration {
		return "max_duration " + maxDuration.String()
	}
	return ""
}
