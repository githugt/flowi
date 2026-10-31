// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/arazzo"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/store"
	"github.com/githugt/flowi-lib/validate"
)

// layoutFile is the layout next to a flow: user-creation.arazzo.yaml →
// user-creation.layout.yaml.
func layoutFile(flowFile string) string {
	return strings.TrimSuffix(flowFile, ".arazzo.yaml") + ".layout.yaml"
}

// FlowLayout reads the flow's canvas layout. A missing file is an empty
// layout; a broken one gives diagnostics and whatever could be read.
func (w *Workspace) FlowLayout(t FlowTarget) (model.Layout, diag.List, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return model.Layout{}, nil, err
	}
	file := layoutFile(f.File)
	empty := model.Layout{Flow: f.ID, Nodes: map[string]model.Point{}, File: file}
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return empty, nil, nil
	}
	if err != nil {
		return empty, nil, fmt.Errorf("reading %s: %w", file, err)
	}
	l, diags := w.codec.DecodeLayout(file, data)
	if len(diags) > 0 {
		l = empty // a half-read layout would misplace nodes; start fresh
	}
	l.Flow, l.File = f.ID, file
	return l, w.positionedIn(file, data, diags), nil
}

// positionedIn positions diagnostics of a file the store does not load.
func (w *Workspace) positionedIn(file string, data []byte, l diag.List) diag.List {
	for i, d := range l {
		if pos, ok := w.codec.Locate(data, d.Path); ok && d.File == file {
			l[i].Pos = pos
		}
	}
	return l
}

// SaveFlowLayout writes the flow's layout. Positions and collapsed strips
// of nodes the flow no longer has are dropped. Layout files belong to the
// tool, so they are always rewritten in canonical form.
func (w *Workspace) SaveFlowLayout(t FlowTarget, l model.Layout) error {
	_, f, err := w.flowFor(t)
	if err != nil {
		return err
	}
	ids := flowNodeIDs(f.Nodes)
	out := model.Layout{Flow: f.ID, Nodes: map[string]model.Point{}}
	for id, p := range l.Nodes {
		if ids[id] {
			out.Nodes[id] = p
		}
	}
	for _, id := range l.Collapsed {
		if ids[id] {
			out.Collapsed = append(out.Collapsed, id)
		}
	}
	data, err := w.codec.EncodeLayout(out)
	if err != nil {
		return fmt.Errorf("encoding layout: %w", err)
	}
	file := layoutFile(f.File)
	old, _ := os.ReadFile(file) // only for the change record; a missing file is a new one
	return store.Apply([]store.FileChange{{File: file, Old: old, New: data}})
}

func flowNodeIDs(list []model.Node) map[string]bool {
	out := map[string]bool{}
	for _, n := range list {
		out[n.ID] = true
		for _, child := range n.Children() {
			for id := range flowNodeIDs(child) {
				out[id] = true
			}
		}
	}
	return out
}

// SaveFlow writes f, an edited copy of flow t, in canonical form and
// reloads it. Problems are as for SaveRequest (FLW-WRITE-001 when the
// file has comments and o.DropComments is false).
func (w *Workspace) SaveFlow(t FlowTarget, f model.Flow, o SaveOptions) error {
	c, current, err := w.flowFor(t)
	if err != nil {
		return err
	}
	f.ID, f.File, f.Path = current.ID, current.File, current.Path // identity and location are not editable here
	data, err := w.store.EncodeFlow(c, f)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", f.File, err)
	}
	return w.writeAndReload(t.Collection, f.File, data, o)
}

// NewUntitledFlow creates an empty flow in collection named untitled,
// untitled-2, … (the first not taken), so a flow can be made without asking
// for a path; it can be renamed after.
func (w *Workspace) NewUntitledFlow(collection model.ID) (FlowTarget, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return FlowTarget{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	for n := 1; n <= untitledLimit; n++ {
		path := "untitled"
		if n > 1 {
			path = fmt.Sprintf("untitled-%d", n)
		}
		if _, err := os.Stat(filepath.Join(c.Root, "flows", path+".arazzo.yaml")); errors.Is(err, fs.ErrNotExist) {
			return w.NewFlow(collection, path)
		}
	}
	return FlowTarget{}, fmt.Errorf("no free untitled flow name in %s", c.Manifest.Name)
}

// NewFlow creates flows/<path>.arazzo.yaml in collection with a fresh id and
// no nodes, and loads it. Path rules are as for NewRequest (FLW-WRITE-002).
func (w *Workspace) NewFlow(collection model.ID, path string) (FlowTarget, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return FlowTarget{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	if detail := requestPathProblem(path); detail != "" {
		return FlowTarget{}, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": detail})
	}
	file := filepath.Join(c.Root, "flows", filepath.FromSlash(path)+flowSuffix)
	if _, err := os.Stat(file); err == nil {
		return FlowTarget{}, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": "it already exists"})
	} else if !errors.Is(err, fs.ErrNotExist) {
		return FlowTarget{}, fmt.Errorf("checking %s: %w", file, err)
	}
	f := model.Flow{Meta: model.Meta{ID: w.newID(), Name: filepath.Base(path)}, Path: path, File: file}
	data, err := w.store.EncodeFlow(c, f)
	if err != nil {
		return FlowTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeAndReload(collection, file, data, SaveOptions{}); err != nil {
		return FlowTarget{}, err
	}
	return FlowTarget{Collection: collection, Flow: f.ID}, nil
}

// FlowCompatibility lists the steps of flow t that only Flowi can run
// (FLW-ARAZZO-002): in the Arazzo file they call the built-in operation, and
// other Arazzo tools cannot run them; and the steps whose reply or match
// filter Arazzo cannot say (FLW-ARAZZO-003). It is for the person editing the flow;
// Validate does not raise it.
func (w *Workspace) FlowCompatibility(t FlowTarget) diag.List {
	c, f, err := w.flowFor(t)
	if err != nil {
		return nil
	}
	var out diag.List
	for _, step := range arazzo.CustomSteps(f, arazzo.NewResolver(c)) {
		d := diag.New(diag.CodeArazzoCustom, diag.Location{File: f.File, Path: fmt.Sprintf("$.nodes[%d]", step.Index)},
			map[string]string{"step": step.Node, "flow": f.Path, "type": step.Type})
		out = append(out, d)
	}
	for _, l := range arazzo.Limits(f, arazzo.NewResolver(c)) {
		out = append(out, diag.New(diag.CodeArazzoLimit, diag.Location{File: f.File, Path: nodePath(f, l.Node)},
			map[string]string{"step": l.Node, "flow": f.Path, "what": l.What}))
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.positioned(out)
}

// nodePath is the path of the top-level node with the given id.
func nodePath(f model.Flow, id string) string {
	for i, n := range f.Nodes {
		if n.ID == id {
			return fmt.Sprintf("$.nodes[%d]", i)
		}
	}
	return "$.nodes"
}

// FlowText is a flow shown as text: Flow is its own data (what the code view
// edits), File the Arazzo document Flowi would save for it.
type FlowText struct {
	Flow string
	File string
}

// FlowSource shows f, an edited copy of flow t (unsaved edits included), as
// text.
func (w *Workspace) FlowSource(t FlowTarget, f model.Flow) (FlowText, error) {
	c, current, err := w.flowFor(t)
	if err != nil {
		return FlowText{}, err
	}
	f.ID, f.File, f.Path = current.ID, current.File, current.Path
	body, err := w.store.EncodeFlowBody(f)
	if err != nil {
		return FlowText{}, fmt.Errorf("encoding %s: %w", f.Path, err)
	}
	file, err := w.store.EncodeFlow(c, f)
	if err != nil {
		return FlowText{}, fmt.Errorf("encoding %s: %w", f.File, err)
	}
	return FlowText{Flow: string(body), File: string(file)}, nil
}

// ParseFlowSource reads text in the form FlowSource shows into flow t. What
// is wrong with the text comes back as diagnostics (positioned in text),
// not as an error; the flow's identity and location stay those of t.
func (w *Workspace) ParseFlowSource(t FlowTarget, text string) (model.Flow, diag.List, error) {
	_, current, err := w.flowFor(t)
	if err != nil {
		return model.Flow{}, nil, err
	}
	f, diags := w.store.DecodeFlowBody(current.File, current.Path, []byte(text))
	f.ID, f.File, f.Path = current.ID, current.File, current.Path
	return f, diags, nil
}

// CheckFlowDraft validates f in place of the flow t names, with the rest of
// the workspace as it is, and returns the problems in that flow's own file.
// It is what a draft not yet saved would be reported for; nothing is changed.
func (w *Workspace) CheckFlowDraft(t FlowTarget, f model.Flow) diag.List {
	c, current, err := w.flowFor(t)
	if err != nil {
		return nil
	}
	f.ID, f.File, f.Path = current.ID, current.File, current.Path
	c.Flows = slices.Clone(c.Flows)
	for i := range c.Flows {
		if c.Flows[i].ID == f.ID {
			c.Flows[i] = f
		}
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	all := validate.Validate(validate.Target{Collections: withCollection(w.collections, c)})
	return w.positioned(slices.DeleteFunc(all, func(d diag.Diagnostic) bool { return d.File != f.File }))
}
