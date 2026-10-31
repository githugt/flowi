// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

func stopSpec() Spec {
	return Spec{
		Type: "stop", Group: "Control", Summary: "End the run here: nothing after it starts. Useful in a branch of a condition.",
		Params: []Param{
			{Name: "status", Kind: ParamEnum, Enum: []string{"passed", "failed"}, Doc: "How the run ends: passed (default) is a normal end that skips the rest, failed ends it as a failure."},
			{Name: "message", Kind: ParamText, Doc: "Why it stops; written to the run log, or into the failure."},
		},
		Run: runStop,
	}
}

// runStop ends the run through the runtime: steps already running finish,
// every step that has not started is skipped, in the enclosing lists too. A
// stop as failed also fails this node, so the run's status says so.
func runStop(_ context.Context, n model.Node, s *State) Outcome {
	message, err := stopMessage(n, s)
	if err != nil {
		return Outcome{Diags: templateDiags(n, "message", err)}
	}
	s.Runtime.StopRun()
	if stringParam(n, "status") == "failed" {
		return failed(n, diag.CodeRunStopped, map[string]string{"node": n.ID, "message": optionalReason(message)})
	}
	s.Emit.Log(n.ID, "stopped"+optionalReason(message))
	return Outcome{Outputs: map[string]any{"message": message}}
}

// stopMessage renders the message field; an empty one stays empty.
func stopMessage(n model.Node, s *State) (string, error) {
	raw := stringParam(n, "message")
	if raw == "" {
		return "", nil
	}
	tpl, err := template.Parse(raw)
	if err != nil {
		return "", err
	}
	r, err := tpl.Render(s.Scope())
	if err != nil {
		return "", err
	}
	s.derived(r)
	return s.Masker.String(template.String(r.Value)), nil
}

// optionalReason is " — message", or nothing when there is no message.
func optionalReason(message string) string {
	if message == "" {
		return ""
	}
	return " — " + message
}
