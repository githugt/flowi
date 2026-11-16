// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
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
