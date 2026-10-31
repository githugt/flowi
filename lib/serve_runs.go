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
