// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/names"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/store"
	"github.com/githugt/flowi/lib/vault"
	"github.com/githugt/flowi/lib/workspace"
)

// Environments are global: they belong to the workspace, live in its
// environments directory (not in any collection, so never in git) and are
// the same in every collection.

// Environments returns the workspace's environments.
func (w *Workspace) Environments() []model.Environment {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return slices.Clone(w.globalEnvs)
}

// SaveEnvironment writes e, an edited environment, and reloads it. Secret
// variables are written as declarations only: their values belong in the
// vault (SetSecret). The environment is matched by file; problems are as
// for SaveRequest.
func (w *Workspace) SaveEnvironment(e model.Environment, o SaveOptions) error {
	w.mu.RLock()
	known := slices.ContainsFunc(w.globalEnvs, func(x model.Environment) bool { return x.File == e.File })
	w.mu.RUnlock()
	if !known {
		return diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": e.Name})
	}
	for i := range e.Vars {
		if e.Vars[i].Secret {
			e.Vars[i].Value = ""
		}
	}
	data, err := w.store.EncodeEnvironment(e)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", e.File, err)
	}
	return w.writeEnvironment(e.File, data, o)
}

// NewEnvironment creates <environments dir>/<name>.yaml with a fresh id and
// no variables, and loads it; it returns the file. The name must be a plain
// name; a bad name or an existing file is FLW-WRITE-002.
func (w *Workspace) NewEnvironment(name string) (string, error) {
	if !names.Valid(name) {
		return "", diagErr(diag.CodeWriteRequestPath, map[string]string{"path": "environments/" + name, "detail": "the name must use letters, digits, '.', '_' or '-' and not start with '.'"})
	}
	file := filepath.Join(workspace.EnvironmentsDir(w.opts.Paths, w.opts.Name), name+".yaml")
	if _, err := os.Stat(file); err == nil {
		return "", diagErr(diag.CodeWriteRequestPath, map[string]string{"path": "environments/" + name, "detail": "it already exists"})
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("checking %s: %w", file, err)
	}
	env := model.Environment{Meta: model.Meta{ID: w.newID(), Name: name}, File: file, Colour: model.DefaultEnvColour(name)}
	data, err := w.store.EncodeEnvironment(env)
	if err != nil {
		return "", fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeEnvironment(file, data, SaveOptions{}); err != nil {
		return "", err
	}
	return file, nil
}

// CollectionEnvironments reads the environment files a collection ships in
// its own environments directory. The workspace does not use them (its
// environments are global); they are offered for copying into it, for
// example from an example or a colleague's collection. A collection
// without the directory has none. Problems with single files are
// diagnostics; an unknown collection is FLW-WS-005.
func (w *Workspace) CollectionEnvironments(collection model.ID) ([]model.Environment, diag.List, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return nil, nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return w.store.LoadEnvironments(filepath.Join(c.Root, "environments"), "")
}

// ShareEnvironment writes the workspace's environment name into the collection's own environments folder
// (<collection>/environments/<name>.yaml), so it goes with the collection, in git, to a colleague. It keeps the
// colour, the base it builds on, and each variable with its kind; a secret is written as a declaration only, with no
// value, because the values live in the vault. The copy gets a fresh id and no vault scope, as a collection's own
// environment has.
//
// A file of that name already there is not touched unless overwrite is set: existed then says so and nothing is
// written. A base the collection does not also hold is named in the returned notes, because the file would build on an
// environment the colleague lacks.
//
// Before: environments could be imported from a collection but never exported to one, so a team shared them by hand.
func (w *Workspace) ShareEnvironment(collection model.ID, name string, overwrite bool) (file string, existed bool, notes []string, err error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	i := slices.IndexFunc(w.globalEnvs, func(x model.Environment) bool { return x.Name == name })
	var env model.Environment
	if i >= 0 {
		env = w.globalEnvs[i]
	}
	w.mu.RUnlock()
	switch {
	case !ok:
		return "", false, nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	case i < 0:
		return "", false, nil, diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": name})
	case !names.Valid(name):
		return "", false, nil, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": "environments/" + name, "detail": "the name must use letters, digits, '.', '_' or '-' and not start with '.'"})
	}
	file = filepath.Join(c.Root, "environments", name+".yaml")
	old, err := os.ReadFile(file)
	switch {
	case err == nil && !overwrite:
		return file, true, nil, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return "", false, nil, fmt.Errorf("reading %s: %w", file, err)
	}
	shared := env
	shared.ID, shared.Scope, shared.File = w.newID(), "", file
	shared.Vars = slices.Clone(env.Vars)
	for j := range shared.Vars {
		if shared.Vars[j].Secret {
			shared.Vars[j].Value = ""
		}
	}
	if shared.Extends != "" && !w.collectionHasEnvironment(c, shared.Extends, name) {
		notes = append(notes, fmt.Sprintf("%s builds on %s, which is not shared with the collection: share it too, or the file will name an environment a colleague lacks.", name, shared.Extends))
	}
	data, err := w.store.EncodeEnvironment(shared)
	if err != nil {
		return "", false, nil, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return "", false, nil, fmt.Errorf("creating %s: %w", filepath.Dir(file), err)
	}
	if err := store.Apply([]store.FileChange{{File: file, Old: old, New: data}}); err != nil {
		return "", false, nil, err
	}
	return file, false, notes, nil
}

// collectionHasEnvironment says whether the collection's environments folder holds an environment called base.
func (w *Workspace) collectionHasEnvironment(c model.Collection, base, except string) bool {
	if base == except {
		return false
	}
	_, err := os.Stat(filepath.Join(c.Root, "environments", base+".yaml"))
	return err == nil
}

// DeleteEnvironment removes an environment file and the secret values it declared from the vault: they are for good
// with it, and left behind they would sit in the vault with nothing to show or remove them. An environment that
// declares no secret does not touch the vault, so deleting it never asks to unlock it. When the file is gone but the
// vault could not be cleaned (locked, no way to ask), the error says so, and `flowi secret prune` finishes the job.
// An unknown file is FLW-REQ-004.
func (w *Workspace) DeleteEnvironment(file string) error {
	w.mu.RLock()
	i := slices.IndexFunc(w.globalEnvs, func(x model.Environment) bool { return x.File == file })
	var gone model.Environment
	if i >= 0 {
		gone = w.globalEnvs[i]
	}
	w.mu.RUnlock()
	if i < 0 {
		return diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": filepath.Base(file)})
	}
	if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", file, err)
	}
	if err := w.reloadEnvironments(); err != nil {
		return err
	}
	if !slices.ContainsFunc(gone.Vars, func(v model.EnvVar) bool { return v.Secret }) {
		return nil
	}
	_, err := w.secrets().Prune(func(e vault.Entry) bool { return e.Collection != vault.WorkspaceScope || e.Environment != gone.ID })
	if err != nil {
		return fmt.Errorf("%s was deleted, but its stored secrets could not be removed (%w); flowi secret prune removes them later", gone.Name, vaultErr(err))
	}
	return nil
}

// writeEnvironment writes file atomically, refusing to drop comments unless
// allowed, then reloads the workspace's environments.
func (w *Workspace) writeEnvironment(file string, data []byte, o SaveOptions) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(file), err)
	}
	old, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", file, err)
	}
	if !o.DropComments && old != nil && w.store.HasComments(old) {
		return diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{File: file}, map[string]string{"file": filepath.Base(file)})}.Err()
	}
	if err := store.Apply([]store.FileChange{{File: file, Old: old, New: data}}); err != nil {
		return err
	}
	return w.reloadEnvironments()
}

// reloadEnvironments re-reads the environments directory and hands the
// result to every collection.
func (w *Workspace) reloadEnvironments() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	diags, err := w.loadGlobalEnvironments()
	if err != nil {
		return fmt.Errorf("reloading environments: %w", err)
	}
	for i := range w.collections {
		w.collections[i].Environments = withShipped(w.globalEnvs, w.collections[i].Environments)
	}
	dir := workspace.EnvironmentsDir(w.opts.Paths, w.opts.Name)
	w.replaceLoadDiags(func(file string) bool { return within(dir, file) }, diags)
	return nil
}
