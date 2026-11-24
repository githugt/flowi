// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Loop control actions.
const (
	actionNext  = "next"
	actionLeave = "leave"
)

func loopControlSpec() Spec {
	return Spec{
		Type: "loop_control", Group: "Control", Summary: "Inside a loop: end this item or round here and go on with the next, or leave the loop. Useful in a branch of a condition.",
		Params: []Param{
			{Name: "action", Kind: ParamEnum, Enum: []string{actionNext, actionLeave}, Default: actionNext, Doc: "What it does: next (default) ends the current item (batch) or round (run_till) here, counts it as passed and goes on with the next one; leave ends the loop: a run_till counts as met, a batch starts no more items and lets the ones already running finish. It acts on the nearest loop around it, and only works inside a batch or run_till body."},
			{Name: "message", Kind: ParamText, Doc: "Why; written to the run log."},
		},
		Run: runLoopControl,
	}
}

// runLoopControl ends the iteration it sits in, or the loop around it,
// through the runtime: steps already running finish, every step of the
// iteration that has not started is skipped, in the enclosing lists too. The
// loop node reads what happened from its lanes. Outside a loop the node
// fails (validate refuses it first; the engine is public, so it checks too).
func runLoopControl(_ context.Context, n model.Node, s *State) Outcome {
	message, err := stopMessage(n, s)
	if err != nil {
		return Outcome{Diags: templateDiags(n, "message", err)}
	}
	action := stringParam(n, "action")
	if action == "" {
		action = actionNext
	}
	done := s.Runtime.EndIteration
	what := "next"
	if action == actionLeave {
		done, what = s.Runtime.LeaveLoop, "left the loop"
	}
	if !done() {
		return failed(n, diag.CodeNodeLoopControl, nil)
	}
	s.Emit.Log(n.ID, what+optionalReason(message))
	return Outcome{Outputs: map[string]any{"action": action, "message": message}}
}
