// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
	"github.com/githugt/flowi/lib/template"
)

// keepOutputs works out the flow's declared outputs in its final scope, in
// declared order, for the report. An output that cannot be worked out (the run
// failed before the step that makes it) is left out rather than filled with a
// wrong value; when the run passed, that is a warning, because a passed run
// should have all of them. A cancelled run keeps none: its scope is half-built.
func (r *runner) keepOutputs(ctx context.Context, fl model.Flow, sc template.Scope, status Status) {
	if ctx.Err() != nil || len(fl.Outputs) == 0 {
		return
	}
	var kept []report.OutputValue
	for _, o := range fl.Outputs {
		v, err := r.evalTemplate(o.Value, sc)
		if err != nil {
			if status == StatusPassed {
				r.warnAll(templateFailure("output "+o.Name, err))
			}
			continue
		}
		kept = append(kept, report.OutputValue{Name: o.Name, Type: o.Type, Value: v})
	}
	r.mu.Lock()
	r.outputs = kept
	r.mu.Unlock()
}

func (r *runner) warnAll(l diag.List) {
	for _, d := range l {
		d.Severity = diag.SeverityWarning
		r.warn(d)
	}
}

// finalOutputs is the kept outputs with every secret the run read masked.
func (r *runner) finalOutputs() []report.OutputValue {
	r.mu.Lock()
	kept := r.outputs
	r.mu.Unlock()
	m := r.secrets.current()
	out := make([]report.OutputValue, len(kept))
	for i, o := range kept {
		out[i] = report.OutputValue{Name: o.Name, Type: o.Type, Value: maskValue(o.Value, m)}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
