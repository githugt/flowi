// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// NamedAuth is an authentication a collection keeps under a name, with the
// request versions that use it.
type NamedAuth struct {
	Collection     model.ID
	CollectionName string
	ID             model.ID
	Name           string
	Description    string
	Auth           model.Auth
	Uses           []string // "path" or "path (version)" of each request version that uses it
}

// Target is the library's handle for the authentication.
func (a NamedAuth) Target() flowi.AuthTarget {
	return flowi.AuthTarget{Collection: a.Collection, Auth: a.ID}
}

// NamedAuths lists the named authentications of the mounted collections, as
// of the last load.
func (ws *Workspace) NamedAuths() []NamedAuth {
	var out []NamedAuth
	for _, c := range ws.Collections() {
		for _, a := range c.Auths {
			out = append(out, NamedAuth{
				Collection: c.Manifest.ID, CollectionName: c.Manifest.Name, ID: a.ID, Name: a.Name,
				Description: a.Description, Auth: a.Auth, Uses: authUses(c, a.Name),
			})
		}
	}
	return out
}

// authUses names the request versions of c that use the authentication
// called name; the version is said only when the request has several.
func authUses(c model.Collection, name string) []string {
	var out []string
	for _, r := range c.Requests {
		for _, v := range r.Versions {
			if v.Auth == nil || v.Auth.Use != name {
				continue
			}
			use := r.Path
			if len(r.Versions) > 1 {
				use += " (" + v.Label + ")"
			}
			out = append(out, use)
		}
	}
	return out
}

// CreateNamedAuth adds an authentication called name to collection, then
// refreshes the lists.
func (ws *Workspace) CreateNamedAuth(collection model.ID, name, description string, auth model.Auth, done func(), fail func(error)) {
	ws.changeAuths(func() error {
		_, err := ws.b.NewNamedAuth(collection, name, description, auth)
		return err
	}, done, fail)
}

// SaveNamedAuth writes a's description, scheme and parameters, then
// refreshes the lists. Its name stays: requests use it by name.
func (ws *Workspace) SaveNamedAuth(a NamedAuth, done func(), fail func(error)) {
	ws.changeAuths(func() error {
		return ws.b.SaveNamedAuth(a.Target(), model.NamedAuth{Meta: model.Meta{Description: a.Description}, Auth: a.Auth})
	}, done, fail)
}

// DeleteNamedAuth deletes a, then refreshes the lists. The requests in
// a.Uses keep naming it and stop validating until they pick another.
func (ws *Workspace) DeleteNamedAuth(a NamedAuth, done func(), fail func(error)) {
	ws.changeAuths(func() error { return ws.b.DeleteNamedAuth(a.Target()) }, done, fail)
}

func (ws *Workspace) changeAuths(change func() error, done func(), fail func(error)) {
	ws.create(func() (string, error) { return "", change() }, func(string) {
		if done != nil {
			done()
		}
	}, fail)
}

// authLabel is what a card shows for an auth block: the named authentication
// it uses, or its scheme.
func authLabel(a *model.Auth) string {
	if a.Use != "" {
		return a.Use
	}
	return a.Type
}

// OrderedAuthParams puts values in the order a file keeps them: the scheme's
// own parameters first, in form order, then any others by name. Empty values
// are left out.
func OrderedAuthParams(scheme string, values map[string]string) []model.KeyValue {
	var out []model.KeyValue
	known := model.AuthParams(scheme)
	for _, name := range known {
		if values[name] != "" {
			out = append(out, model.KeyValue{Key: name, Value: values[name]})
		}
	}
	var others []string
	for name, value := range values {
		if value != "" && !slices.Contains(known, name) {
			others = append(others, name)
		}
	}
	slices.Sort(others)
	for _, name := range others {
		out = append(out, model.KeyValue{Key: name, Value: values[name]})
	}
	return out
}
