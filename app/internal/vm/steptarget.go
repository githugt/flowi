// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "github.com/githugt/flowi/lib/model"

// StepTargetID is the tree node id of the request or flow a step calls, or "" for a step that calls neither (or
// calls through a blueprint parameter). own is the collection of the flow the step is in: a reference with no
// collection means that one.
func StepTargetID(own model.ID, n model.Node) string {
	ref := func(prefix string, r model.Ref) string {
		if r.ID == "" || r.Param != "" {
			return ""
		}
		coll := model.ID(r.Collection)
		if coll == "" {
			coll = own
		}
		return prefix + string(coll) + ":" + string(r.ID)
	}
	switch {
	case n.Request != nil:
		return ref("r:", n.Request.Request)
	case n.Subflow != nil:
		return ref("f:", n.Subflow.Flow)
	}
	return ""
}

// TargetOf is the tree node id of the request or flow step id calls (see StepTargetID).
func (d *FlowDoc) TargetOf(id string) string {
	n, ok := d.Node(id)
	if !ok {
		return ""
	}
	return StepTargetID(d.coll.Manifest.ID, n)
}
