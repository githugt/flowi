// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi-lib/model"
)

func webhookSpec() Spec {
	return Spec{
		Type: "webhook", Group: "Call", Summary: "Send a payload to a URL; wait for the answer or fire and forget.",
		Params: append(httpParams("POST"),
			Param{Name: "payload", Kind: ParamAny, Doc: "A string is sent as text; a mapping or list as JSON."},
			Param{Name: "wait", Kind: ParamBool, Doc: "Default true. With false the run goes on; a failure is only a warning."}),
		Run: runWebhook,
	}
}

func runWebhook(ctx context.Context, n model.Node, s *State) Outcome {
	call, out := buildCall(n, s, "POST", "payload")
	if out.Diags.HasErrors() {
		return out
	}
	if wait, set := param(n, "wait"); set && wait == false {
		s.Runtime.Background(n.ID, call)
		return Outcome{}
	}
	res := s.Runtime.HTTP(ctx, call)
	if res.Response == nil {
		return Outcome{Status: res.Status, Diags: res.Diags}
	}
	return Outcome{Outputs: map[string]any{"status": res.Response.Status, "body": res.Response.Body}, Status: res.Status, Diags: res.Diags}
}
