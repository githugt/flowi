// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"strconv"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// defaultRunIterations bounds a flow-level run: without max_iterations it
// must not repeat forever.
const defaultRunIterations = 10

// runIterations repeats a whole flow (flow-level run:, design §7.5): each
// iteration gets fresh nodes.<id> under "[i]/", variables carry over;
// until is checked after each iteration, which runs at least once.
func (r *runner) runIterations(ctx context.Context, fs *flowScope, vars *nodes.Vars, fl model.Flow, prior map[string]map[string]any) listOutcome {
	till := fl.Run
	limit := cmpOrInt(till.MaxIterations, defaultRunIterations)
	start := r.deps.Clock()
	for i := 0; ; i++ {
		fs.iteration = i + 1 // iterations run one after another; nothing reads fs concurrently here
		label := "[" + strconv.Itoa(i) + "]"
		f := &frame{fs: fs, vars: vars, outputs: newOutputs(nil), jar: r.jar, prefix: label + "/", lane: label}
		seedPrior(f.outputs, prior, fl.Nodes)
		out := r.runList(ctx, f, fl.Nodes)
		if out.status != StatusPassed || r.stop.Load() {
			return out
		}
		met, _, err := template.Check(till.Until, f.scope(r))
		if err != nil {
			return listOutcome{status: StatusFailed, diags: templateFailure("run.until", err)}
		}
		if met {
			return out
		}
		if reached := runLimitReached(i+1, limit, r.deps.Clock().Sub(start), till); reached != "" {
			return listOutcome{status: StatusFailed, diags: diag.List{diag.New(diag.CodeRunTillLimit, diag.Location{}, map[string]string{"node": "run", "limit": reached})}}
		}
		if err := r.wait(ctx, till.Interval); err != nil {
			return listOutcome{status: StatusCancelled}
		}
	}
}

func runLimitReached(done, limit int, elapsed time.Duration, till *model.RunTill) string {
	if done >= limit {
		return "max_iterations " + strconv.Itoa(limit)
	}
	if till.MaxDuration > 0 && elapsed >= till.MaxDuration {
		return "max_duration " + till.MaxDuration.String()
	}
	return ""
}

// wait waits d on the run's clock, or until ctx is done.
func (r *runner) wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	select {
	case <-r.after(d):
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}
