// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/workspace"
)

// A flow has no edit slot in its file, as a request has, so the unsaved edits of
// a flow that is kept when the app quits go to a file of their own, in the
// workspace's data folder, as the text the code view shows. The flow's own file
// is not touched.

func (w *Workspace) keptFlowFile(t FlowTarget) string {
	return filepath.Join(workspace.KeptDir(w.opts.Paths, w.opts.Name), "flows", string(t.Flow)+".yaml")
}

// KeepFlowEdits stores f, flow t with unsaved edits, until KeptFlowEdits takes it.
func (w *Workspace) KeepFlowEdits(t FlowTarget, f model.Flow) error {
	text, err := w.FlowSource(t, f)
	if err != nil {
		return fmt.Errorf("writing the edits of flow %s: %w", t.Flow, err)
	}
	file := w.keptFlowFile(t)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(file), err)
	}
	if err := os.WriteFile(file, []byte(text.Flow), 0o600); err != nil {
		return fmt.Errorf("keeping the edits of flow %s: %w", t.Flow, err)
	}
	return nil
}

// KeptFlowEdits returns the text KeepFlowEdits stored for flow t and forgets it.
// ok is false when nothing was kept. Read the text with ParseFlowSource.
func (w *Workspace) KeptFlowEdits(t FlowTarget) (text string, ok bool, err error) {
	file := w.keptFlowFile(t)
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading the kept edits of flow %s: %w", t.Flow, err)
	}
	if err := os.Remove(file); err != nil {
		return "", false, fmt.Errorf("forgetting the kept edits of flow %s: %w", t.Flow, err)
	}
	return string(data), true, nil
}
