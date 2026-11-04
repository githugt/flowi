// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/serve/runs"
)

// maxOutcomeMessages bounds the diagnostics kept on a finished run.
const maxOutcomeMessages = 20

// startRun is the runs.Starter of flowi serve: it resolves the flow a
// trigger or schedule names and starts it. Problems the caller can fix
// come back as *runs.StartError with a message written for them.
func (w *Workspace) startRun(ctx context.Context, r runs.Request) (runs.Handle, error) {
	target, err := w.FindFlow(r.Flow, r.Collection)
	if err != nil {
		return nil, asStartError(err)
	}
	if r.Watch {
		return w.startWatch(ctx, target, r)
	}
	run, err := w.RunFlow(ctx, target, RunOptions{Env: r.Env, PreferVersion: r.PreferVersion, Inputs: r.Inputs, Seed: r.Seed})
	if err != nil {
		return nil, asStartError(err)
	}
	// Events must be drained or the run blocks. Goroutine ownership: this
	// one ends when the run closes its event stream.
	go func() {
		for range run.Events() { //nolint:revive // draining
		}
	}()
	return engineHandle{run}, nil
}

// startWatch starts a watch pass. A flow that may not be watched is refused here, where the caller can still be told;
// whatever goes wrong once the pass is under way ends it with an "error" outcome instead, since the pass runs on its
// own goroutine (it ends with the pass, which the context cancels).
func (w *Workspace) startWatch(ctx context.Context, target FlowTarget, r runs.Request) (runs.Handle, error) {
	problems, err := w.WatchProblems(target)
	if err != nil {
		return nil, asStartError(err)
	}
	if len(problems) > 0 {
		return nil, &runs.StartError{Kind: runs.KindInvalid, Message: "this flow cannot be watched: " + strings.Join(problems, "; ")}
	}
	h := &watchHandle{done: make(chan struct{})}
	go func() {
		defer close(h.done)
		res, err := w.WatchFlow(ctx, target, RunOptions{Env: r.Env, PreferVersion: r.PreferVersion, Inputs: r.Inputs, Seed: r.Seed})
		h.out = watchOutcome(res, err)
	}()
	return h, nil
}

type watchHandle struct {
	done chan struct{}
	out  runs.Outcome
}

func (h *watchHandle) Wait() runs.Outcome {
	<-h.done
	return h.out
}

// watchOutcome says how a watch pass ended: the run's own status, or "changed" when the run passed but its answers
// moved, with each change in a line.
func watchOutcome(res WatchResult, err error) runs.Outcome {
	if err != nil {
		return runs.Outcome{Status: string(engine.StatusError), Messages: []string{err.Error()}}
	}
	out := runs.Outcome{Status: string(res.Status)}
	if res.Status != engine.StatusPassed {
		out.Messages = []string{"the run " + string(res.Status) + ", so nothing was compared and the last baseline is kept"}
		return out
	}
	for _, c := range res.Change {
		if len(out.Messages) == maxOutcomeMessages {
			break
		}
		out.Messages = append(out.Messages, c.Kind+" "+c.Request+": "+c.Detail)
	}
	if len(out.Messages) > 0 {
		out.Status = runs.StatusChanged
	}
	return out
}

type engineHandle struct{ run *engine.Run }

func (h engineHandle) Wait() runs.Outcome {
	res := h.run.Wait()
	out := runs.Outcome{EngineRunID: res.RunID, Status: string(res.Status), Seed: res.Seed}
	for _, d := range res.Diags {
		if len(out.Messages) == maxOutcomeMessages {
			break
		}
		msg, _ := diag.English().Message(d)
		out.Messages = append(out.Messages, msg)
	}
	if res.Err != nil && len(out.Messages) < maxOutcomeMessages {
		out.Messages = append(out.Messages, res.Err.Error())
	}
	return out
}

// asStartError sorts a refusal by who can fix it. Diagnostics are written
// for users; anything else is a system failure whose details stay in logs.
func asStartError(err error) error {
	var list *diag.ListError
	if !errors.As(err, &list) || len(list.List) == 0 {
		return &runs.StartError{Kind: runs.KindInternal, Message: "the run could not be started", Err: err}
	}
	kind := runs.KindInvalid
	switch list.List[0].Code {
	case diag.CodeFlowNotFound, diag.CodeWSUnknownCollection:
		kind = runs.KindNotFound
	case diag.CodeFlowAmbiguous:
		kind = runs.KindAmbiguous
	}
	var msgs []string
	for i, d := range list.List {
		if i == 5 {
			msgs = append(msgs, "…")
			break
		}
		msg, _ := diag.English().Message(d)
		msgs = append(msgs, msg)
	}
	return &runs.StartError{Kind: kind, Message: strings.Join(msgs, "; "), Err: err}
}
