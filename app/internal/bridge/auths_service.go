// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"strings"
	"time"

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

// settle runs start on the actor and waits for the outcome it reports, as an error with a message for people.
// Before: the screen sent each of these and moved on at once, so a refusal came as a notice somewhere else and the form
// had already been closed or cleared.
func (s *AuthsService) settle(start func(done func(error))) error {
	out := make(chan error, 1)
	s.app.actor.Do(func() {
		start(func(err error) {
			if err != nil {
				err = errors.New(vm.ErrorText(err))
			}
			out <- err
		})
	})
	return <-out
}

var errNoAuthName = errors.New("give the authentication a name: requests use it by that name")

var errAuthGone = errors.New("that authentication is gone")

// TokenView is an OAuth token the app holds. It never carries the token.
type TokenView struct {
	ID      string `json:"id"`
	Label   string `json:"label"`   // what it is for: the grant, the token endpoint's host, the client id and the scope
	Status  string `json:"status"`  // "Signed in until 14:32", "Expired: renews by itself" or "Expired: sign in again"
	Expired bool   `json:"expired"` // for the colour
	Stored  bool   `json:"stored"`  // kept across restarts
	Expires string `json:"expires"` // RFC 3339
}

// Tokens lists the OAuth tokens the app holds: the sign-ins a person approved (kept in the vault) and the ones got
// since it opened. It reads the vault, so it runs off the actor and may wait for the passphrase.
func (s *AuthsService) Tokens() []TokenView {
	out := []TokenView{}
	for _, t := range s.app.backend.AuthTokens() {
		out = append(out, TokenView{ID: t.ID, Label: t.Label, Status: vm.TokenStatus(t), Expired: t.Expired, Stored: t.Stored, Expires: t.Expires.Format(time.RFC3339)})
	}
	return out
}

// ForgetToken drops a token, so the next send that needs it signs in again. A refusal comes back as the error.
func (s *AuthsService) ForgetToken(id string) error {
	if err := s.app.backend.ForgetAuthToken(id); err != nil {
		return errors.New(vm.ErrorText(err))
	}
	return nil
}

// Create adds an authentication called name to collection, with the scheme
// ("scheme:<type>") and no parameters yet. A refusal comes back as the error.
func (s *AuthsService) Create(collection, name, scheme string) error {
	return s.settle(func(done func(error)) {
		name = strings.TrimSpace(name)
		if name == "" {
			done(errNoAuthName)
			return
		}
		s.app.workspace.CreateNamedAuth(model.ID(collection), name, "", model.Auth{Type: schemeType(scheme)}, func() { done(nil) }, done)
	})
}

// FromRequest keeps the authentication of the open request (a scheme with its values) as a named authentication of
// the request's collection, and makes the request use it by name. The values are copied as they are: templates such
// as {{secret.name}} stay templates.
func (s *AuthsService) FromRequest(node, name string) {
	s.app.actor.Do(func() {
		name = strings.TrimSpace(name)
		o, ok := s.app.requests.reqs[node]
		switch {
		case !ok:
			return
		case name == "":
			s.fail(errNoAuthName)
			return
		}
		a := o.req.Current().Auth
		if a == nil || a.Use != "" || a.Type == "" {
			s.fail(fmt.Errorf("this request has no authentication scheme of its own to keep: pick a scheme in its Auth section first"))
			return
		}
		s.app.workspace.CreateNamedAuth(o.req.Target().Collection, name, "", *a, func() {
			o.req.SetAuthUse(name)
			s.app.emit("notice", Notice{Level: "info", Text: "Saved the authentication as " + name + "; this request now uses it."})
		}, s.fail)
	})
}

// Save writes the description, scheme and parameter values of an
// authentication; an empty value leaves the parameter out. A refusal comes back as the error.
func (s *AuthsService) Save(key, description, scheme string, values map[string]string) error {
	return s.settle(func(done func(error)) {
		a, ok := s.find(key)
		if !ok {
			done(errAuthGone)
			return
		}
		a.Description = description
		a.Auth = model.Auth{Type: schemeType(scheme), Params: vm.OrderedAuthParams(schemeType(scheme), values)}
		s.app.workspace.SaveNamedAuth(a, func() { done(nil) }, done)
	})
}

// Rename gives an authentication another name. The requests that use it are changed to the new name.
func (s *AuthsService) Rename(key, name string) error {
	return s.settle(func(done func(error)) {
		a, ok := s.find(key)
		switch {
		case !ok:
			done(errAuthGone)
		case strings.TrimSpace(name) == "":
			done(errNoAuthName)
		default:
			s.app.workspace.RenameNamedAuth(a, name, func() { done(nil) }, done)
		}
	})
}

// Delete removes an authentication. Requests that use it keep naming it.
func (s *AuthsService) Delete(key string) error {
	return s.settle(func(done func(error)) {
		a, ok := s.find(key)
		if !ok {
			done(nil) // already gone: nothing to do
			return
		}
		s.app.workspace.DeleteNamedAuth(a, func() { done(nil) }, done)
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
	v := AuthsView{Auths: []NamedAuthView{}, Collections: []Choice{}, Schemes: authSchemeChoices(nil), Forms: map[string][]AuthFieldView{}}
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
