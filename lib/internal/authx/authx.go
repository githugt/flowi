// SPDX-License-Identifier: Apache-2.0

// Package authx applies the authentication schemes of a request's auth block:
// it adds what a scheme needs (a key, a token, a signature) to the request
// just before it is sent, and reports what it added so the record can show it
// with the secret parts masked.
//
// Every parameter reaches this package already rendered, so credentials come
// from the vault by the templates that named them; nothing here reads a
// secret or a file of its own accord.
package authx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/model"
)

// Applied is one thing a scheme added to a request.
type Applied struct {
	Where  string // header, query or cookie
	Name   string
	Value  string
	Secret bool // the value is a credential or derived from one: show it masked
}

// Signer adds a scheme's credentials to a request before it is sent. It may
// change r.Header and r.URL.
type Signer interface {
	Apply(ctx context.Context, r *httpx.Request) ([]Applied, error)
}

// Challenger is a Signer for a scheme that answers a 401: Challenge reads the
// response to the request sent without credentials and reports whether Apply
// can now authenticate it, in which case the caller sends it again.
type Challenger interface {
	Signer
	Challenge(resp *httpx.Response) bool
}

// Env is what the schemes need from outside.
type Env struct {
	// HTTP calls token endpoints (OAuth 2.0); nil uses a client with a
	// timeout of its own.
	HTTP *http.Client
	// Now is the clock for timestamps and token expiry; nil is time.Now.
	Now func() time.Time
	// Tokens caches OAuth 2.0 tokens between sends; nil keeps none.
	Tokens *TokenCache
	// Nonce returns a random string of n characters; nil uses crypto/rand.
	Nonce func(n int) string
	// Prompt tells the person what to do for a grant that needs one (open a
	// page, enter a code). It must return at once; nil means nobody is
	// there, and such a grant waits until its context ends.
	Prompt func(Prompt)
	// After is a timer on the scheme's clock, for polling; nil is time.After.
	After func(time.Duration) <-chan time.Time
}

// Prompt kinds.
const (
	// PromptOpen asks the person to open URL in a browser, sign in and approve.
	PromptOpen = "open"
	// PromptDevice asks the person to open URL on any device and enter Code.
	PromptDevice = "device"
)

// Prompt is what a grant that needs a person asks of them.
type Prompt struct {
	Kind    string
	URL     string
	Code    string    // PromptDevice: the code to enter
	Expires time.Time // when the request stops being valid
}

func (e Env) prompt(p Prompt) {
	if e.Prompt != nil {
		e.Prompt(p)
	}
}

func (e Env) after(d time.Duration) <-chan time.Time {
	if e.After != nil {
		return e.After(d)
	}
	return time.After(d)
}

func (e Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e Env) nonce(n int) string {
	if e.Nonce != nil {
		return e.Nonce(n)
	}
	return randomString(n)
}

// New builds the signer for a scheme from its rendered parameters. A mutual
// TLS auth block has no signer: it configures the connection instead.
func New(a model.Auth, env Env) (Signer, error) {
	p := paramsOf(a)
	switch a.Type {
	case model.AuthAPIKey:
		return newAPIKey(p)
	case model.AuthDigest:
		return newDigest(p, env)
	case model.AuthOAuth2:
		return newOAuth2(p, env)
	case model.AuthOAuth1:
		return newOAuth1(p, env)
	case model.AuthAWSSigV4:
		return newSigV4(p, env)
	case model.AuthJWT:
		return newJWT(p, env)
	case model.AuthHawk:
		return newHawk(p, env)
	case model.AuthNTLM:
		return newNTLM(p, env)
	case "":
		return nil, fmt.Errorf("the auth block has no type (one of %s)", strings.Join(model.AuthTypes(), ", "))
	}
	return nil, fmt.Errorf("auth type %q is not one of %s", a.Type, strings.Join(model.AuthTypes(), ", "))
}

// params are a scheme's rendered parameters.
type params struct {
	typ    string
	values map[string]string
}

func paramsOf(a model.Auth) params {
	p := params{typ: a.Type, values: make(map[string]string, len(a.Params))}
	for _, kv := range a.Params {
		p.values[kv.Key] = kv.Value
	}
	return p
}

// get is a parameter's value, or def when it is empty.
func (p params) get(name, def string) string {
	if v := p.values[name]; v != "" {
		return v
	}
	return def
}

// need is a required parameter, or an error naming it.
func (p params) need(names ...string) error {
	var missing []string
	for _, n := range names {
		if p.values[n] == "" {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s auth needs %s", p.typ, strings.Join(missing, ", "))
	}
	return nil
}

// only refuses a parameter the scheme does not know (a typo would otherwise
// leave a credential out silently).
func (p params) only() error {
	known := model.AuthParams(p.typ)
	names := make([]string, 0, len(p.values))
	for name := range p.values {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if !slices.Contains(known, name) {
			return fmt.Errorf("%s auth has no parameter %q (it has %s)", p.typ, name, strings.Join(known, ", "))
		}
	}
	return nil
}

// bodyBytes reads the whole body of r, for the schemes that hash it.
func bodyBytes(r *httpx.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	rc, _, _, err := r.Body.Open()
	if err != nil {
		return nil, fmt.Errorf("reading the body to sign it: %w", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("reading the body to sign it: %w", err)
	}
	return data, nil
}
