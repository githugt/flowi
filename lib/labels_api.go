// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Statuses lists the development statuses of collection: the defaults, then its own (model.Manifest.StatusList).
func (w *Workspace) Statuses(collection model.ID) ([]model.Status, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return c.Manifest.StatusList(), nil
}

// TagsInUse lists the tags collection uses, most used first, for a picker that suggests the ones already there.
func (w *Workspace) TagsInUse(collection model.ID) ([]model.TagUse, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return c.TagsInUse(), nil
}

// SaveStatuses replaces the statuses collection adds to the defaults, written to its flowi.yaml. A definition that
// cannot be used (a bad name, a colour or icon the window cannot draw, a name given twice) is FLW-STATUS-002 and
// nothing is written. A name that matches a default changes that default's colour and icon.
func (w *Workspace) SaveStatuses(collection model.ID, statuses []model.Status, o SaveOptions) error {
	seen := map[string]bool{}
	for _, s := range statuses {
		err := model.CheckStatus(s)
		if err == nil && seen[s.Name] {
			err = fmt.Errorf("it is defined twice")
		}
		if err != nil {
			return diagErr(diag.CodeStatusInvalid, map[string]string{"name": s.Name, "detail": err.Error()})
		}
		seen[s.Name] = true
	}
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	man := c.Manifest
	man.Statuses = slices.Clone(statuses)
	data, err := w.store.EncodeManifest(man)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", man.File, err)
	}
	return w.writeAndReload(collection, man.File, data, o)
}

// SetRequestLabels sets the tags and the development status of request t, in its file. status is "" for none of its
// own (a folder's then applies); a name the collection does not have is FLW-STATUS-001 and nothing is written. Tags
// are cleaned (model.CleanTags). The versions and the working copy are left as they are.
func (w *Workspace) SetRequestLabels(t RequestTarget, tags []string, status string, o SaveOptions) error {
	c, r, err := w.requestFor(t)
	if err != nil {
		return err
	}
	if err := checkStatusUsable(c, status, "request "+r.Path); err != nil {
		return err
	}
	r.Tags, r.Status = model.CleanTags(tags), status
	return w.SaveRequest(t, r, o)
}

// SetFlowLabels sets the tags and the development status of flow t, as SetRequestLabels does for a request.
func (w *Workspace) SetFlowLabels(t FlowTarget, tags []string, status string, o SaveOptions) error {
	c, f, err := w.flowFor(t)
	if err != nil {
		return err
	}
	if err := checkStatusUsable(c, status, "flow "+f.Path); err != nil {
		return err
	}
	f.Tags, f.Status = model.CleanTags(tags), status
	return w.SaveFlow(t, f, o)
}

// SetFolderLabels sets the tags and the development status of the folder dir under tree ("requests" or "flows") of
// collection, in its _folder.yaml, which is created when it is not there yet. They apply to everything under the
// folder. The folder's settings, hooks and order are kept.
func (w *Workspace) SetFolderLabels(collection model.ID, tree, dir string, tags []string, status string, o SaveOptions) error {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	if tree != "requests" && tree != "flows" || dir == "" {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": dir, "detail": "a folder is under requests or flows, and has a name"})
	}
	if detail := requestPathProblem(dir); detail != "" {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": dir, "detail": detail})
	}
	rel := tree + "/" + dir
	if err := checkStatusUsable(c, status, "folder "+rel); err != nil {
		return err
	}
	folder := model.Folder{Dir: rel, File: filepath.Join(c.Root, filepath.FromSlash(rel), "_folder.yaml")}
	if i := slices.IndexFunc(c.Folders, func(f model.Folder) bool { return f.Dir == rel }); i >= 0 {
		folder = c.Folders[i]
	}
	folder.Tags, folder.Status = model.CleanTags(tags), status
	data, err := w.store.EncodeFolder(folder)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", folder.File, err)
	}
	return w.writeAndReload(collection, folder.File, data, o)
}

// checkStatusUsable refuses a status the collection does not have; "" is always fine.
func checkStatusUsable(c model.Collection, status, where string) error {
	if status == "" {
		return nil
	}
	if _, ok := c.StatusNamed(status); ok {
		return nil
	}
	var names []string
	for _, s := range c.Manifest.StatusList() {
		names = append(names, s.Name)
	}
	return diagErr(diag.CodeStatusUnknown, map[string]string{"status": status, "where": where, "known": strings.Join(names, ", ")})
}
