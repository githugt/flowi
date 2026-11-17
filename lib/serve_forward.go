// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/serve/replay"
)

// forward builds what the replayer does with a call no saved response can answer, from ServeOptions.Forward (and
// the older PassThrough, which is Forward "pass" with ForwardTo).
func (b *serveBuild) forward() (*replay.Forward, error) {
	mode, to := b.o.Forward, b.o.ForwardTo
	if b.o.PassThrough != "" {
		if mode != "" || to != "" {
			return nil, fmt.Errorf("%w: pass-through is the older way to say forward \"pass\" to one API; use one of them", ErrInvalidServeOptions)
		}
		mode, to = ForwardPass, b.o.PassThrough
	}
	if mode == "" {
		if to != "" || len(b.o.ForwardOnly) > 0 {
			return nil, fmt.Errorf("%w: an API to forward to is given but forwarding is off: choose pass or save", ErrInvalidServeOptions)
		}
		return nil, nil
	}
	if mode != ForwardPass && mode != ForwardSave {
		return nil, fmt.Errorf("%w: forward is %q or %q, not %q", ErrInvalidServeOptions, ForwardPass, ForwardSave, mode)
	}
	if b.o.Record != "" {
		return nil, fmt.Errorf("%w: record already forwards every call; forward and record cannot be used together", ErrInvalidServeOptions)
	}
	f := &replay.Forward{Save: mode == ForwardSave, Self: b.s.replayAddr, BaseURL: b.w.forwardBaseURL(b.o.ForwardEnv)}
	if to != "" {
		up, err := parseForwardURL(to)
		if err != nil {
			return nil, err
		}
		if up.Host == b.s.replayAddr {
			return nil, fmt.Errorf("%w: the API to forward to is the replayer itself; give the real API's URL", ErrInvalidServeOptions)
		}
		f.Upstream = up
	} else {
		for _, raw := range b.o.ForwardOnly {
			up, err := parseForwardURL(raw)
			if err != nil {
				return nil, err
			}
			f.Only = append(f.Only, up)
		}
	}
	if b.o.RecordHAR != "" {
		f.Recorder.Tape = b.tape()
	}
	if f.Save {
		target, err := b.w.recordCollection(b.o.RecordTo)
		if err != nil {
			return nil, err
		}
		f.Recorder.Capture = b.w.captureTo(target)
	}
	b.s.notes = append(b.s.notes, forwardNote(mode, to, b.o.ForwardOnly, b.o.RecordTo))
	return f, nil
}

func forwardNote(mode, to string, only []string, recordTo string) string {
	where := "calls no saved response can answer go to the API of the saved request they match"
	switch {
	case to != "":
		where = "calls no saved response can answer go to " + to
	case len(only) > 0:
		where += ", only when that API is one of " + strings.Join(only, ", ")
	}
	if mode == ForwardSave {
		return "forwarding: " + where + "; each real answer is saved as a response of that request (a call that matches none becomes a request in " + recordTarget(recordTo) + ")"
	}
	return "forwarding: " + where + "; nothing is saved"
}

func parseForwardURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: an API to forward to needs an http:// or https:// URL, got %q", ErrInvalidServeOptions, raw)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: put credentials in the calls' headers, not in the URL", ErrInvalidServeOptions)
	}
	return u, nil
}

// ForwardTarget is one API the saved requests call, as the replayer could forward to it.
type ForwardTarget struct {
	// Base is the API's base URL with the environment's variables filled in; when it cannot be worked out, the
	// base as the requests write it ({{env.base_url}}).
	Base string
	// Problem says why Base cannot be worked out ("" when it can).
	Problem string
	// Requests is how many saved requests (versions) call it.
	Requests int
}

// ForwardTargets lists the APIs the saved requests call, in the environment env ("" uses defaults.environment), so a
// caller can show where forwarded calls would go.
func (w *Workspace) ForwardTargets(env string) []ForwardTarget {
	base := w.forwardBaseURL(env)
	var out []ForwardTarget
	for _, c := range w.Collections() {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				if v.Family() != model.FamilyHTTP {
					continue
				}
				written := replay.BaseOf(v.URL)
				if written == "" {
					continue
				}
				shown, problem := written, ""
				if u, err := base(c, r, v); err != nil {
					problem = err.Error()
				} else {
					shown = u.String()
				}
				i := slices.IndexFunc(out, func(t ForwardTarget) bool { return t.Base == shown && t.Problem == problem })
				if i < 0 {
					out = append(out, ForwardTarget{Base: shown, Problem: problem})
					i = len(out) - 1
				}
				out[i].Requests++
			}
		}
	}
	return out
}

// forwardBaseURL works out the base URL of a saved request in env: the part of its URL before the path, with the
// environment's variables filled in. It is cached per collection and written base, since many requests share one.
func (w *Workspace) forwardBaseURL(env string) func(c model.Collection, r model.Request, v model.RequestVersion) (*url.URL, error) {
	type key struct {
		coll    model.ID
		written string
	}
	type result struct {
		u   *url.URL
		err error
	}
	var cache typedMap[key, result]
	return func(c model.Collection, r model.Request, v model.RequestVersion) (*url.URL, error) {
		written := replay.BaseOf(v.URL)
		if written == "" {
			return nil, errors.New("its URL has no base URL")
		}
		k := key{c.Manifest.ID, written}
		if got, ok := cache.Load(k); ok {
			return got.u, got.err
		}
		u, err := w.renderBase(c, r, v, written, env)
		cache.Store(k, result{u, err})
		return u, err
	}
}

func (w *Workspace) renderBase(c model.Collection, r model.Request, v model.RequestVersion, written, env string) (*url.URL, error) {
	probe := model.RequestVersion{Label: v.Label, Method: "GET", URL: written}
	p, err := w.PreviewRequest(RequestTarget{Collection: c.Manifest.ID, Request: r.ID}, probe, SendOptions{Env: env}, false)
	if err != nil {
		return nil, err
	}
	rendered := strings.TrimSpace(p.Masked.URL)
	if strings.Contains(rendered, "{{") {
		return nil, errors.New("a variable in " + written + " has no value in this environment")
	}
	u, err := url.Parse(rendered)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%q is not an http:// or https:// URL", rendered)
	}
	u.RawQuery, u.Fragment = "", ""
	return u, nil
}

// typedMap is sync.Map with types.
type typedMap[K comparable, V any] struct{ m sync.Map }

func (s *typedMap[K, V]) Load(k K) (V, bool) {
	v, ok := s.m.Load(k)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

func (s *typedMap[K, V]) Store(k K, v V) { s.m.Store(k, v) }
