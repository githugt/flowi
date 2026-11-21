// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/flowdiff"
)

// DiffFlowAgainst compares flow t as it is now with older, the text of an earlier version of its file (what
// `git show <rev>:<file>` gives), and lists what changed step by step: steps added, removed, moved, renamed,
// retyped or rewired, and the flow's own inputs, outputs and limits. Nothing is written. The diagnostics are the
// problems found reading older; when it could not be read as a flow, there are no changes to list and the
// diagnostics say why.
func (w *Workspace) DiffFlowAgainst(t FlowTarget, older []byte) ([]flowdiff.Change, diag.List, error) {
	c, now, err := w.flowFor(t)
	if err != nil {
		return nil, nil, err
	}
	before, diags := w.store.DecodeFlowFile(c, now.File, now.Path, older)
	if before.ID == "" {
		return nil, diags, nil
	}
	return flowdiff.Diff(before, now), diags, nil
}

// FlowFile is the path of the file flow t is kept in, for a caller that needs an earlier version of it from git.
func (w *Workspace) FlowFile(t FlowTarget) (string, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return "", err
	}
	return f.File, nil
}
