// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strings"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// NamedAuthView is an authentication a collection keeps under a name.
type NamedAuthView struct {
	Key            string          `json:"key"` // collection id and authentication id
	Collection     string          `json:"collection"`
	CollectionName string          `json:"collectionName"`
	Name           string          `json:"name"` // what a request writes to use it
	Description    string          `json:"description"`
	Scheme         string          `json:"scheme"` // "scheme:<type>", as the request editor's Auth section names it
	SchemeLabel    string          `json:"schemeLabel"`
	Fields         []AuthFieldView `json:"fields"` // the scheme's parameters with their values
	Uses           []string        `json:"uses"`   // the requests that use it
}

// AuthsView is the Authentications screen.
type AuthsView struct {
	Auths       []NamedAuthView `json:"auths"`
	Collections []Choice        `json:"collections"` // where a new one can go
	Schemes     []Choice        `json:"schemes"`
	// Forms are the empty fields of each scheme, by "scheme:<type>": what the
	// form shows when the scheme is changed.
	Forms map[string][]AuthFieldView `json:"forms"`
}

// AuthsService adds, edits and deletes the named authentications of the
// mounted collections.
type AuthsService struct{ app *App }

func newAuthsService(a *App) *AuthsService {
	s := &AuthsService{app: a}
	a.workspace.OnChange(func() { a.emit("auths", s.view()) })
	return s
}

// State is the named authentications of every mounted collection; they
// arrive again as the "auths" event when the workspace reloads.
func (s *AuthsService) State() AuthsView {
	var v AuthsView
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// Create adds an authentication called name to collection, with the scheme
// ("scheme:<type>") and no parameters yet.
func (s *AuthsService) Create(collection, name, scheme string) {
	s.app.actor.Do(func() {
		name = strings.TrimSpace(name)
		if name == "" {
			s.fail(fmt.Errorf("give the authentication a name: requests use it by that name"))
			return
		}
		s.app.workspace.CreateNamedAuth(model.ID(collection), name, "", model.Auth{Type: schemeType(scheme)}, nil, s.fail)
	})
}

// Save writes the description, scheme and parameter values of an
// authentication; an empty value leaves the parameter out.
func (s *AuthsService) Save(key, description, scheme string, values map[string]string) {
	s.app.actor.Do(func() {
		a, ok := s.find(key)
		if !ok {
			s.app.emit("notice", Notice{Level: "error", Text: "that authentication is gone"})
			return
		}
		a.Description = description
		a.Auth = model.Auth{Type: schemeType(scheme), Params: vm.OrderedAuthParams(schemeType(scheme), values)}
		s.app.workspace.SaveNamedAuth(a, func() {
			s.app.emit("notice", Notice{Level: "info", Text: "Saved " + a.Name + "."})
		}, s.fail)
	})
}

// Delete removes an authentication. Requests that use it keep naming it.
func (s *AuthsService) Delete(key string) {
	s.app.actor.Do(func() {
		if a, ok := s.find(key); ok {
			s.app.workspace.DeleteNamedAuth(a, nil, s.fail)
		}
	})
}

func (s *AuthsService) fail(err error) {
	s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)})
}

func (s *AuthsService) find(key string) (vm.NamedAuth, bool) {
	for _, a := range s.app.workspace.NamedAuths() {
		if namedAuthKey(a) == key {
			return a, true
		}
	}
	return vm.NamedAuth{}, false
}

func namedAuthKey(a vm.NamedAuth) string { return fmt.Sprintf("%s/%s", a.Collection, a.ID) }

// schemeType is the auth type of a scheme choice ("scheme:apikey" → "apikey").
func schemeType(scheme string) string { return strings.TrimPrefix(scheme, "scheme:") }

func (s *AuthsService) view() AuthsView {
	v := AuthsView{Auths: []NamedAuthView{}, Collections: []Choice{}, Schemes: authSchemeChoices(), Forms: map[string][]AuthFieldView{}}
	for _, c := range s.app.workspace.Collections() {
		v.Collections = append(v.Collections, Choice{ID: string(c.Manifest.ID), Label: c.Manifest.Name, Path: c.Manifest.Name})
	}
	for _, t := range model.AuthTypes() {
		v.Forms["scheme:"+t] = authFields(t, nil)
	}
	labels := authSchemeLabels()
	for _, a := range s.app.workspace.NamedAuths() {
		v.Auths = append(v.Auths, NamedAuthView{
			Key: namedAuthKey(a), Collection: string(a.Collection), CollectionName: a.CollectionName, Name: a.Name,
			Description: a.Description, Scheme: "scheme:" + a.Auth.Type, SchemeLabel: labels[a.Auth.Type],
			Fields: authFields(a.Auth.Type, a.Auth.Params), Uses: nonNil(a.Uses),
		})
	}
	return v
}
