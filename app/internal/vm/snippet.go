// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/snippet"
	"github.com/githugt/flowi/lib/vault"
)

// secretPlaceholder stands for a secret value in a snippet that excludes
// secrets.
const secretPlaceholder = "<secret>"

// Snippet is the code-snippet panel: the edited request rendered by the
// library without sending, as one of the snippet formats. With secrets
// excluded (the default) their values never leave the library.
type Snippet struct {
	b        Backend
	w        *Worker
	ws       *Workspace
	req      *Request
	format   snippet.Format
	exclude  bool
	masked   engine.PreviewRequest
	actual   *engine.PreviewRequest
	text     string
	err      string
	seq      int
	env      string // the environment the preview was made for
	closed   bool
	onChange []func()
}

// NewSnippet returns the snippet view-model of req. excludeSecrets is the
// starting state of the exclude toggle (ui.snippet_exclude_secrets).
func NewSnippet(b Backend, w *Worker, ws *Workspace, req *Request, excludeSecrets bool) *Snippet {
	s := &Snippet{b: b, w: w, ws: ws, req: req, format: snippet.Curl, exclude: excludeSecrets}
	ws.OnChange(s.EnvironmentChanged)
	return s
}

// OnChange registers f to run after the snippet changes.
func (s *Snippet) OnChange(f func()) { s.onChange = append(s.onChange, f) }

func (s *Snippet) changed() {
	for _, f := range s.onChange {
		f()
	}
}

// Format is the selected format.
func (s *Snippet) Format() snippet.Format { return s.format }

// Exclude reports whether secrets are left out.
func (s *Snippet) Exclude() bool { return s.exclude }

// Text is the snippet, or "" before the first preview or after a problem.
func (s *Snippet) Text() string { return s.text }

// Err is why there is no snippet, or "".
func (s *Snippet) Err() string { return s.err }

// Request is the rendered request with secrets excluded: what the
// exchange panel's Request tab shows.
func (s *Snippet) Request() engine.PreviewRequest { return s.masked }

// SetFormat picks a format; the preview is reused.
func (s *Snippet) SetFormat(f snippet.Format) {
	s.format = f
	s.render()
	s.changed()
}

// SetExclude toggles secrets; including them needs the real values, which
// are fetched (and may ask for the vault passphrase).
func (s *Snippet) SetExclude(exclude bool) {
	s.exclude = exclude
	if exclude || s.actual != nil {
		s.render()
		s.changed()
		return
	}
	s.Refresh()
}

// Close stops the view-model: a closed request tab refreshes nothing.
func (s *Snippet) Close() { s.closed = true }

// EnvironmentChanged refreshes the preview when the workspace's selected
// environment is not the one it was made for.
func (s *Snippet) EnvironmentChanged() {
	if s.ws.SelectedEnv() == s.env {
		return
	}
	s.Refresh()
}

// Refresh previews the request as currently edited. It is cheap enough to
// call after every edit: nothing is written and nothing is sent. The
// request preview always uses the masked rendering; the real secret values
// are fetched only while secrets are included, and when that fails (a
// cancelled passphrase prompt) secrets go back to excluded instead of
// asking again on every edit.
func (s *Snippet) Refresh() {
	if s.closed {
		return
	}
	t, v := s.req.Target(), s.req.Current()
	env := s.ws.SelectedEnv()
	s.env = env
	o := flowi.SendOptions{Env: env, Version: v.Label}
	s.seq++
	seq := s.seq
	type outcome struct {
		p   engine.Preview
		err error
	}
	Run(s.w, func() outcome {
		p, err := s.b.PreviewRequest(t, v, o, false)
		return outcome{p, err}
	}, func(out outcome) {
		if seq != s.seq || s.closed {
			return // a newer refresh is in flight
		}
		if out.err != nil {
			s.text, s.err = "", ErrorText(out.err)
			s.changed()
			return
		}
		s.masked, s.err = placeholders(out.p.Masked), ""
		s.actual = nil
		s.render()
		s.changed()
		if !s.exclude {
			s.fetchActual(t, v, o, seq)
		}
	})
}

// fetchActual asks for the real values (it may prompt for the vault).
func (s *Snippet) fetchActual(t flowi.RequestTarget, v model.RequestVersion, o flowi.SendOptions, seq int) {
	type outcome struct {
		p   engine.Preview
		err error
	}
	Run(s.w, func() outcome {
		p, err := s.b.PreviewRequest(t, v, o, true)
		return outcome{p, err}
	}, func(out outcome) {
		if seq != s.seq || s.closed {
			return
		}
		if out.err != nil {
			s.exclude, s.actual, s.err = true, nil, ErrorText(out.err)
			s.render()
			s.changed()
			return
		}
		s.actual = out.p.Real
		s.render()
		s.changed()
	})
}

func (s *Snippet) render() {
	req := s.masked
	if !s.exclude && s.actual != nil {
		req = *s.actual
	}
	text, err := snippet.Generate(s.format, req)
	if err != nil {
		s.text, s.err = "", err.Error()
		return
	}
	s.text = text
}

// placeholders replaces the library's mask with the snippet placeholder.
func placeholders(p engine.PreviewRequest) engine.PreviewRequest {
	swap := func(x string) string { return strings.ReplaceAll(x, vault.Mask, secretPlaceholder) }
	p.URL, p.Body, p.File = swap(p.URL), swap(p.Body), swap(p.File)
	p.Headers = append([]engine.Header(nil), p.Headers...)
	for i := range p.Headers {
		p.Headers[i].Value = swap(p.Headers[i].Value)
	}
	p.Parts = append([]engine.PreviewPart(nil), p.Parts...)
	for i := range p.Parts {
		p.Parts[i].Value, p.Parts[i].File = swap(p.Parts[i].Value), swap(p.Parts[i].File)
	}
	return p
}
