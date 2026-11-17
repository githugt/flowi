// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/store"
)

const authSuffix = ".auth.yaml"

// AuthTarget identifies a named authentication of a mounted collection.
type AuthTarget struct {
	Collection model.ID
	Auth       model.ID
}

// AuthUse is one request version that uses a named authentication.
type AuthUse struct {
	Request RequestTarget
	Path    string // the request's path
	Version string // the version's label
}

// NamedAuths lists the collection's named authentications in file order. An
// unknown collection is FLW-WS-005.
func (w *Workspace) NamedAuths(collection model.ID) ([]model.NamedAuth, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return slices.Clone(c.Auths), nil
}

// NamedAuth returns one named authentication; an unknown one is FLW-AUTH-003.
func (w *Workspace) NamedAuth(t AuthTarget) (model.NamedAuth, error) {
	_, a, err := w.authFor(t)
	return a, err
}

func (w *Workspace) authFor(t AuthTarget) (model.Collection, model.NamedAuth, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(t.Collection)
	if !ok {
		return c, model.NamedAuth{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(t.Collection)})
	}
	i := slices.IndexFunc(c.Auths, func(a model.NamedAuth) bool { return a.ID == t.Auth })
	if i < 0 {
		return c, model.NamedAuth{}, diagErr(diag.CodeAuthNotFound, map[string]string{"name": string(t.Auth)})
	}
	return c, c.Auths[i], nil
}

// NewNamedAuth creates auths/<name>.auth.yaml in collection. The name is
// what requests write in auth: {use: <name>}, so it must be a plain file
// name that no other authentication of the collection has (FLW-WRITE-002).
func (w *Workspace) NewNamedAuth(collection model.ID, name, description string, auth model.Auth) (AuthTarget, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return AuthTarget{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	if _, taken := c.NamedAuth(name); taken {
		return AuthTarget{}, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": name, "detail": "it already exists"})
	}
	file, err := newEntityFile(c, "auths", name, authSuffix)
	if err != nil {
		return AuthTarget{}, err
	}
	if base := filepath.Base(file); base != name+authSuffix {
		return AuthTarget{}, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": name, "detail": "an authentication's name has no folders"})
	}
	a := model.NamedAuth{Meta: model.Meta{ID: w.newID(), Name: name, Description: description}, Auth: plainAuth(auth), File: file}
	data, err := w.store.EncodeAuth(a)
	if err != nil {
		return AuthTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeAndReload(collection, file, data, SaveOptions{}); err != nil {
		return AuthTarget{}, err
	}
	return AuthTarget{Collection: collection, Auth: a.ID}, nil
}

// SaveNamedAuth writes the description, scheme and parameters of a named
// authentication. Its id, file and name stay as they are: requests use it by
// name, so a rename would silently detach them.
func (w *Workspace) SaveNamedAuth(t AuthTarget, a model.NamedAuth, o SaveOptions) error {
	_, current, err := w.authFor(t)
	if err != nil {
		return err
	}
	a.ID, a.Name, a.File, a.Auth = current.ID, current.Name, current.File, plainAuth(a.Auth)
	data, err := w.store.EncodeAuth(a)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", a.File, err)
	}
	return w.writeAndReload(t.Collection, a.File, data, o)
}

// DeleteNamedAuth removes the authentication's file. Requests that use it
// are not touched: they fail validation (FLW-AUTH-002) until they name
// another one, so ask NamedAuthUses first.
func (w *Workspace) DeleteNamedAuth(t AuthTarget) error {
	_, a, err := w.authFor(t)
	if err != nil {
		return err
	}
	return w.removeFile(t.Collection, a.File)
}

// NamedAuthUses lists the request versions of the collection that use the
// named authentication, in request order.
func (w *Workspace) NamedAuthUses(t AuthTarget) ([]AuthUse, error) {
	c, a, err := w.authFor(t)
	if err != nil {
		return nil, err
	}
	var out []AuthUse
	for _, r := range c.Requests {
		for _, v := range r.Versions {
			if v.Auth != nil && v.Auth.Use == a.Name {
				out = append(out, AuthUse{Request: RequestTarget{Collection: t.Collection, Request: r.ID}, Path: r.Path, Version: v.Label})
			}
		}
	}
	return out, nil
}

// plainAuth is a's scheme and parameters: a named authentication never uses
// another one.
func plainAuth(a model.Auth) model.Auth { return model.Auth{Type: a.Type, Params: a.Params} }

// RenameNamedAuth gives the authentication another name: its file moves to auths/<name>.auth.yaml and every request
// version that used it by the old name now uses the new one, so nothing is detached. The id stays. A name that is
// taken or not a plain file name is FLW-WRITE-002; a request file with comments needs o.DropComments, as a save does.
// The auth.default settings of the collection and its folders are not rewritten: check them after a rename.
func (w *Workspace) RenameNamedAuth(t AuthTarget, name string, o SaveOptions) error {
	c, a, err := w.authFor(t)
	if err != nil {
		return err
	}
	if name == a.Name {
		return nil
	}
	if _, taken := c.NamedAuth(name); taken {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": name, "detail": "it already exists"})
	}
	file, err := newEntityFile(c, "auths", name, authSuffix)
	if err != nil {
		return err
	}
	if base := filepath.Base(file); base != name+authSuffix {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": name, "detail": "an authentication's name has no folders"})
	}
	renamed := a
	renamed.Name, renamed.File = name, file
	data, err := w.store.EncodeAuth(renamed)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", file, err)
	}
	changes := []store.FileChange{{File: file, New: data}}
	touched := []string{file, a.File}
	for _, r := range c.Requests {
		change, used, err := w.renameAuthUse(r, a.Name, name, o)
		if err != nil {
			return err
		}
		if used {
			changes = append(changes, change)
			touched = append(touched, r.File)
		}
	}
	if err := store.Apply(changes); err != nil {
		return err
	}
	if err := os.Remove(a.File); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing the old %s: %w", a.File, err)
	}
	for _, f := range touched {
		if err := w.reloadFile(t.Collection, f); err != nil {
			return err
		}
	}
	return nil
}

// renameAuthUse is the change that makes the versions of r which use the authentication oldName use newName; used is
// false when none does.
func (w *Workspace) renameAuthUse(r model.Request, oldName, newName string, o SaveOptions) (change store.FileChange, used bool, err error) {
	usesOld := func(v model.RequestVersion) bool { return v.Auth != nil && v.Auth.Use == oldName }
	if !slices.ContainsFunc(r.Versions, usesOld) {
		return store.FileChange{}, false, nil
	}
	r.Versions = slices.Clone(r.Versions)
	for i, v := range r.Versions {
		if usesOld(v) {
			use := *v.Auth
			use.Use = newName
			r.Versions[i].Auth = &use
		}
	}
	old, err := os.ReadFile(r.File)
	if err != nil {
		return store.FileChange{}, false, fmt.Errorf("reading %s: %w", r.File, err)
	}
	if !o.DropComments && w.store.HasComments(old) {
		return store.FileChange{}, false, diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{File: r.File}, map[string]string{"file": filepath.Base(r.File)})}.Err()
	}
	enc, err := w.store.EncodeRequest(r)
	if err != nil {
		return store.FileChange{}, false, fmt.Errorf("encoding %s: %w", r.File, err)
	}
	return store.FileChange{File: r.File, Old: old, New: enc}, true, nil
}
