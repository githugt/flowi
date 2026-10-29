// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/snippet"

	"github.com/githugt/flowi/internal/vm"
)

// SnippetView is the request rendered as a command, without sending it.
type SnippetView struct {
	Format  string   `json:"format"`
	Formats []string `json:"formats"`
	Exclude bool     `json:"exclude"` // secrets are left out
	Text    string   `json:"text"`
	Error   string   `json:"error,omitempty"`
}

// UsedByRow is a line of the "Used by" tab.
type UsedByRow struct {
	Header string `json:"header,omitempty"`
	Text   string `json:"text"`
	Detail string `json:"detail,omitempty"`
	Opens  bool   `json:"opens"`
	Flow   string `json:"flow,omitempty"` // flow path, with Collection to find the tree row
	Node   string `json:"node,omitempty"`
	// Row is the tree row of the flow to open, "" when it is not found.
	Row string `json:"row,omitempty"`
}

// UsedByView is the "Used by" tab: every consumer of the request.
type UsedByView struct {
	Rows  []UsedByRow `json:"rows"`
	Error string      `json:"error,omitempty"`
}

// CookieView is a cookie of the collection's saved jar.
type CookieView struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  string `json:"expires"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
}

// CookiesView is the "Cookies" tab.
type CookiesView struct {
	Cookies []CookieView `json:"cookies"`
	Error   string       `json:"error,omitempty"`
}

func snippetViewOf(s *vm.Snippet) SnippetView {
	formats := []string{}
	for _, f := range snippet.Formats() {
		formats = append(formats, string(f))
	}
	return SnippetView{Format: string(s.Format()), Formats: formats, Exclude: s.Exclude(), Text: s.Text(), Error: s.Err()}
}

// SetSnippetFormat picks curl, wget, powershell or xh.
func (s *RequestService) SetSnippetFormat(node, format string) {
	s.with(node, func(o *openRequest) { o.snip.SetFormat(snippet.Format(format)) })
}

// SetSnippetExclude leaves secrets out of the snippet (on) or puts their
// values in (off, which may ask for the vault passphrase).
func (s *RequestService) SetSnippetExclude(node string, exclude bool) {
	s.with(node, func(o *openRequest) { o.snip.SetExclude(exclude) })
}

// UsedBy runs impact analysis for the request: where it is called from.
func (s *RequestService) UsedBy(node string) UsedByView {
	ch := make(chan *vm.UsedBy, 1)
	s.with(node, func(o *openRequest) {
		u := vm.NewUsedBy(s.app.backend, s.app.worker, o.req.Target())
		u.OnChange(func() { ch <- u })
		u.Load()
	})
	u, ok := <-ch
	v := UsedByView{Rows: []UsedByRow{}}
	if !ok {
		return v
	}
	s.app.actor.Do(func() {
		v.Error = u.Err()
		for _, r := range u.Rows() {
			row := UsedByRow{Header: r.Header, Text: r.Text, Detail: r.Detail, Opens: r.Opens, Flow: r.Flow, Node: r.Node}
			if r.Opens {
				row.Row = flowRow(s.app.workspace, r.Collection, r.Flow)
			}
			v.Rows = append(v.Rows, row)
		}
	})
	return v
}

// flowRow finds the tree row of the flow at path in a collection.
func flowRow(ws *vm.Workspace, collection model.ID, path string) string {
	c, ok := ws.Collection(collection)
	if !ok {
		return ""
	}
	for _, f := range c.Flows {
		if f.Path == path {
			return "f:" + string(collection) + ":" + string(f.ID)
		}
	}
	return ""
}

// Cookies lists the cookies of the request's collection.
func (s *RequestService) Cookies(node string) CookiesView {
	ch := make(chan *vm.Cookies, 1)
	s.with(node, func(o *openRequest) {
		c := vm.NewCookies(s.app.backend, s.app.worker, o.req.Collection().Manifest.ID)
		c.OnChange(func() {
			select {
			case ch <- c:
			default:
			}
		})
		c.Load()
	})
	c := <-ch
	v := CookiesView{Cookies: []CookieView{}}
	s.app.actor.Do(func() {
		v.Error = c.Err()
		for _, k := range c.Rows() {
			v.Cookies = append(v.Cookies, cookieViewOf(k))
		}
	})
	return v
}

func cookieViewOf(k flowi.CookieInfo) CookieView {
	exp := "session"
	if !k.Expires.IsZero() {
		exp = k.Expires.Local().Format(time.DateTime)
	}
	return CookieView{Key: k.Key, Name: k.Name, Value: k.Value, Domain: k.Domain, Path: k.Path, Expires: exp, Secure: k.Secure, HTTPOnly: k.HTTPOnly}
}

// DeleteCookie removes one cookie from the collection's jar.
func (s *RequestService) DeleteCookie(node, key string) {
	s.with(node, func(o *openRequest) { _ = s.app.backend.DeleteCookie(string(o.req.Collection().Manifest.ID), key) })
}

// ClearCookies empties the collection's jar.
func (s *RequestService) ClearCookies(node string) {
	s.with(node, func(o *openRequest) { _ = s.app.backend.ClearCookies(string(o.req.Collection().Manifest.ID)) })
}

// StoredView is a response kept for a request: a saved response or a pinned example.
type StoredView struct {
	Kind    string `json:"kind"` // saved or example
	Version string `json:"version"`
	Name    string `json:"name"`
	Status  int    `json:"status"`
	At      string `json:"at"`
	Size    int64  `json:"size"`
}

// StoredResponses lists what is kept for the request: pinned examples first,
// then the responses its sends and the recorder saved.
func (s *RequestService) StoredResponses(node string) []StoredView {
	out := []StoredView{}
	s.with(node, func(o *openRequest) {
		list, err := s.app.backend.StoredResponses(o.req.Target())
		if err != nil {
			return
		}
		for _, r := range list {
			out = append(out, StoredView{Kind: r.Kind, Version: r.Version, Name: r.Name, Status: r.Status, At: r.At.Local().Format("2006-01-02 15:04:05"), Size: r.Size})
		}
	})
	return out
}

// DeleteStored removes one stored response. It returns the problem, "" when done.
func (s *RequestService) DeleteStored(node, kind, version, name string) string {
	problem := ""
	s.with(node, func(o *openRequest) {
		if err := s.app.backend.DeleteStoredResponse(o.req.Target(), kind, version, name); err != nil {
			problem = vm.ErrorText(err)
		}
	})
	return problem
}

// ShowStored shows a saved response in the response pane, as the history does.
// It returns the problem, "" when shown.
func (s *RequestService) ShowStored(node, version, name string) string {
	problem := ""
	s.with(node, func(o *openRequest) {
		rec, err := s.app.backend.StoredRecord(o.req.Target(), version, name)
		if err != nil {
			problem = vm.ErrorText(err)
			return
		}
		ex := vm.Exchange{At: rec.At, Title: "Saved " + name + " · v" + version, Target: o.req.Target(), Version: version, Result: engine.SendResult{Status: engine.StatusPassed, Record: &rec}}
		o.shown = &ex
		s.app.emit("request", s.view(node, o))
	})
	return problem
}

// PinStored pins a saved response as an example named example (empty: its
// status) and returns the file written.
func (s *RequestService) PinStored(node, version, name, example string) (string, error) {
	var file string
	var err error
	s.with(node, func(o *openRequest) {
		var rec engine.Record
		if rec, err = s.app.backend.StoredRecord(o.req.Target(), version, name); err != nil {
			return
		}
		file, err = s.app.backend.PinExample(rec, example)
	})
	return file, err
}

// SaveBody writes the body of the response shown to a file the user picks in the
// system's save dialog, and returns the file ("" when the user cancelled).
func (s *RequestService) SaveBody(node string) (string, error) {
	var rec *engine.Record
	s.with(node, func(o *openRequest) {
		if o.shown != nil {
			rec = o.shown.Result.Record
		} else if ex, ok := s.app.send.LastFor(o.req.Target()); ok {
			rec = ex.Result.Record
		}
	})
	if rec == nil {
		return "", errors.New("send the request first")
	}
	body, err := vm.ResponseBodyFile(*rec)
	if err != nil {
		return "", err
	}
	if s.app.pickers.Save == nil {
		return "", errNoDialogs
	}
	file, err := s.app.pickers.Save("Save the response body", body.Name)
	if err != nil || file == "" {
		return "", err
	}
	if err := os.WriteFile(file, body.Data, 0o600); err != nil {
		return "", fmt.Errorf("saving the body: %w", err)
	}
	return file, nil
}

// PinWarning says what is worth knowing before the response shown is pinned:
// a pinned example is committed with the collection, response headers
// included. It names the headers that look like credentials and is "" when
// there is none (or nothing to pin).
func (s *RequestService) PinWarning(node string) string {
	var names []string
	s.with(node, func(o *openRequest) {
		if ex, ok := s.app.send.LastFor(o.req.Target()); ok && ex.Result.Record != nil {
			names = engine.CredentialHeaders(*ex.Result.Record)
		}
	})
	return pinWarningText(names)
}

// PinStoredWarning is PinWarning for a saved response of the request.
func (s *RequestService) PinStoredWarning(node, version, name string) string {
	var names []string
	s.with(node, func(o *openRequest) {
		if rec, err := s.app.backend.StoredRecord(o.req.Target(), version, name); err == nil {
			names = engine.CredentialHeaders(rec)
		}
	})
	return pinWarningText(names)
}

func pinWarningText(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return "This response has " + strings.Join(names, ", ") + ". A pinned example is committed with the collection, headers included: check that no real credential is in it."
}
