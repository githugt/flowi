// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/flowioverlay"
	"github.com/githugt/flowi/lib/model"
)

func (c call) overlayName() string {
	if c.overlay == nil {
		return ""
	}
	return c.overlay.Name
}

// laidOver applies the call's flowi overlay to the resolved version and
// returns what did not fit as warnings (FLW-OVL-003) or drifted (FLW-OVL-004).
// The send goes on with whatever applied: an overlay that fits only in part
// still shows the user what it can.
func laidOver(c call, v model.RequestVersion) (model.RequestVersion, diag.List) {
	if c.overlay == nil {
		return v, nil
	}
	out, problems := flowioverlay.Apply(v, c.overlay.Ops)
	var notes diag.List
	for _, p := range problems {
		code := diag.CodeOverlayNotFitting
		if p.Drift {
			code = diag.CodeOverlayDrift
		}
		notes = append(notes, diag.New(code, diag.Location{File: c.request.File}, map[string]string{
			"overlay": c.overlay.Name, "detail": p.Describe(), "reason": p.Reason,
		}))
	}
	return out, notes
}

// overlayOfCall finds the flowi overlay name in the collection of the request
// a flow step runs, as the operations that apply to that request. "" is none;
// an unknown name fails the step (FLW-OVL-002).
func overlayOfCall(c model.Collection, r model.Request, name string) (*Overlay, diag.List) {
	if name == "" {
		return nil, nil
	}
	ops, missing := flowioverlay.Resolve(c.Overlays, name, r.ID)
	if missing != "" {
		return nil, diag.List{diag.New(diag.CodeOverlayNotFound, diag.Location{File: r.File}, map[string]string{"name": missing})}
	}
	return &Overlay{Name: name, Ops: ops}, nil
}
