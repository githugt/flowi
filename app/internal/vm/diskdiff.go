// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "github.com/githugt/flowi/lib/modeldiff"

// DiskDiff is what differs between the file as it is on disk and the edits in the editor, for the banner that says
// the file changed under unsaved edits. Before: the banner offered "Load from disk" and "Keep mine" with nothing to
// choose by, and either choice could throw someone's work away unseen.
//
// The slot of an autosaved edit is not a difference: it is the editor's own working copy.
func (r *Request) DiskDiff() modeldiff.Result {
	return modeldiff.Compare(r.disk, r.draft, "Edit", "EditBase", "EditOverlay")
}

// DiskDiff is what differs between the flow file on disk and the flow being edited.
func (d *FlowDoc) DiskDiff() modeldiff.Result {
	return modeldiff.Compare(d.disk, d.draft)
}
