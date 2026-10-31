// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// The engine does work of its own around a send that no hook names: it learns
// a JSON Schema from the bodies and checks later sends against it
// (schemas.infer). The hooks timeline listed only hooks, so a request seemed
// to generate schemas from nowhere; these rows show that work after the
// post-hooks, marked built in.

const schemaSetting = "schemas.infer"

// postHooksTitle is the heading the view-model gives the post-hooks
// (vm.Request.Pipeline); there is none when a request has no post-hooks.
const postHooksTitle = "Post-hooks"

// schemaFindings are the diagnostics the schema step adds to a send: a
// mismatch, or a schema learned anew.
func schemaFindings() map[diag.Code]bool {
	return map[diag.Code]bool{diag.CodeSchemaResponse: true, diag.CodeSchemaRequest: true, diag.CodeSchemaMessage: true}
}

// builtInRows are the timeline rows of the engine's own steps for a request:
// a "Built in" heading and the schema step, in the state the last send left
// it. on says whether schemas.infer is on for the request; last is its latest
// exchange, if any.
func builtInRows(on bool, last *vm.Exchange, hasPost bool) []PipelineRowView {
	step := PipelineRowView{Kind: "builtin", Title: "schema", Detail: "learns the body schema and checks each send against it · " + schemaSetting, State: "idle"}
	switch {
	case !on:
		step.State, step.Reason = "not-run", "Off: the setting "+schemaSetting+" is false."
	case last != nil && last.Result.Record != nil:
		step.State = "passed"
		for _, d := range last.Result.Diags {
			if schemaFindings()[d.Code] {
				msg, _ := diag.English().Message(d) // a code without English text still marks the step failed
				step.State, step.Reason = "failed", msg
				break
			}
		}
	}
	// The engine checks schemas last, after the post-hooks and the saved response (engine.sendAndCheck).
	when := "After the post-hooks"
	if !hasPost {
		when = "After the response"
	}
	return []PipelineRowView{{Kind: "section", Title: when}, step}
}

// SetSchemasInfer turns schema learning on or off for the selected version
// (its own schemas.infer setting). Setting it to what the version would
// inherit removes the version's own value, so the file says only what differs.
func (s *RequestService) SetSchemasInfer(node string, on bool) {
	s.with(node, func(o *openRequest) {
		inherited := schemasOn(s.app.backend, o.req.Target().Collection, model.RequestVersion{})
		o.req.Edit(func(v *model.RequestVersion) {
			if on == inherited {
				delete(v.Settings, schemaSetting)
				return
			}
			if v.Settings == nil {
				v.Settings = model.Settings{}
			}
			v.Settings[schemaSetting] = on
		})
	})
}

// schemasOn says whether schemas.infer is on for a request version: its own
// setting when it has one, else what the collection resolves to.
func schemasOn(b vm.Backend, collection model.ID, v model.RequestVersion) bool {
	if own, ok := v.Settings[schemaSetting].(bool); ok {
		return own
	}
	got, ok := b.Settings(collection).Get(schemaSetting)
	on, isBool := got.Value.(bool)
	return !ok || !isBool || on // unknown means the default, which is on
}
