// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/authx"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/model"
)

// authenticate applies the version's auth block to the request just before
// it is sent, and records what it added: the secret parts are registered with
// the masker, so they show masked wherever the request is shown. ok is false
// when the scheme could not be applied (the send has failed).
func (x *execution) authenticate(ctx context.Context, b *built) (authx.Signer, bool) {
	if b.auth == nil || b.auth.Type == model.AuthMutualTLS {
		return nil, true
	}
	signer, err := authx.New(*b.auth, x.authEnv())
	if err != nil {
		x.failAuth(err)
		return nil, false
	}
	if err := x.applyAuth(ctx, signer, b); err != nil {
		x.failAuth(err)
		return nil, false
	}
	return signer, true
}

func (x *execution) authEnv() authx.Env {
	return authx.Env{Now: x.r.deps.Clock, Tokens: x.r.deps.AuthTokens, After: x.r.after, Prompt: x.promptPerson}
}

// promptPerson tells whoever watches the run that the request waits for them.
func (x *execution) promptPerson(p authx.Prompt) {
	x.r.emit(AuthPrompted{RunID: x.r.runID, Path: x.c.path, Kind: p.Kind, URL: p.URL, Code: p.Code, Expires: p.Expires})
}

func (x *execution) applyAuth(ctx context.Context, signer authx.Signer, b *built) error {
	applied, err := signer.Apply(ctx, &b.request)
	if err != nil {
		return err
	}
	for _, a := range applied {
		x.recordApplied(&b.sent, a)
	}
	return nil
}

// recordApplied shows what a scheme added in the recorded request, and keeps
// its secret values out of every output.
func (x *execution) recordApplied(sent *SentRequest, a authx.Applied) {
	if a.Secret {
		x.r.secrets.derived(a.Value)
		if a.Where == "query" {
			x.r.secrets.derived(url.QueryEscape(a.Value))
		}
	}
	switch a.Where {
	case "header", "cookie":
		name := a.Name
		if a.Where == "cookie" {
			name = "Cookie"
		}
		sent.Header = replaceHeader(sent.Header, Header{Name: name, Value: a.Value})
	case "query":
		sent.URL = withQueryParam(sent.URL, a.Name, a.Value)
	}
}

func replaceHeader(headers []Header, h Header) []Header {
	out := make([]Header, 0, len(headers)+1)
	for _, e := range headers {
		if !strings.EqualFold(e.Name, h.Name) {
			out = append(out, e)
		}
	}
	return append(out, h)
}

// withQueryParam sets a query parameter in the URL as recorded.
func withQueryParam(raw, name, value string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set(name, value)
	u.RawQuery = q.Encode()
	return u.String()
}

// answerChallenge sends the request again, with credentials, when the
// scheme answers a 401 (Digest) and the response to the first send is one.
func (x *execution) answerChallenge(ctx context.Context, client *httpx.Client, signer authx.Signer, b *built, attempts []httpx.Attempt) ([]httpx.Attempt, error) {
	ch, ok := signer.(authx.Challenger)
	if !ok || len(attempts) == 0 {
		return attempts, nil
	}
	last := attempts[len(attempts)-1]
	if last.Response == nil || !ch.Challenge(last.Response) {
		return attempts, nil
	}
	if err := x.applyAuth(ctx, signer, b); err != nil {
		x.failAuth(err)
		return attempts, nil
	}
	more, err := client.Do(ctx, b.request)
	return append(attempts, more...), err
}

func (x *execution) failAuth(err error) {
	x.fail(StatusError, diag.List{diag.New(diag.CodeAuthInvalid, diag.Location{}, map[string]string{"detail": x.r.secrets.String(err.Error())})})
}

// clientOptionsFor is clientOptions plus what the auth block asks of the
// connection: a client certificate for mutual TLS.
func (x *execution) clientOptionsFor(b built) (httpx.Options, bool) {
	o := x.clientOptions(b)
	if b.auth == nil || b.auth.Type != model.AuthMutualTLS {
		return o, true
	}
	cert, err := loadClientCert(*b.auth, x.c.collection.Root)
	if err != nil {
		x.failAuth(err)
		return o, false
	}
	o.ClientCert = &cert
	return o, true
}

// loadClientCert reads the certificate and key a mutual TLS block names, as
// PEM files (relative paths are the collection's).
// withNamedAuth replaces an auth block that names one of the collection's
// authentications with that authentication, so everything after it sees an
// ordinary auth block. A name the collection does not have is FLW-AUTH-002:
// the request must not go out without the authentication it asked for.
func withNamedAuth(c model.Collection, v model.RequestVersion) (model.RequestVersion, diag.List) {
	if v.Auth == nil || v.Auth.Use == "" {
		return v, nil
	}
	named, ok := c.NamedAuth(v.Auth.Use)
	if !ok {
		return v, diag.List{diag.New(diag.CodeAuthUnknown, diag.Location{}, map[string]string{"name": v.Auth.Use})}
	}
	v.Auth = &model.Auth{Type: named.Auth.Type, Params: named.Auth.Params}
	return v, nil
}

// withoutUnusableDefault drops an auth block a request inherited from auth.default when its protocol cannot use that
// scheme. A collection-wide default is written for HTTP requests; a WebSocket or SSE request in the same collection
// uses it when it can and goes without when it cannot, where a request's own auth block that it cannot use is a
// validation error. Message requests of other protocols never read the auth block.
func withoutUnusableDefault(v model.RequestVersion) model.RequestVersion {
	if v.Async != nil && v.Auth != nil && !slices.Contains(model.AsyncAuthTypes(v.Async.Protocol), v.Auth.Type) {
		v.Auth = nil
	}
	return v
}

func loadClientCert(a model.Auth, root string) (tls.Certificate, error) {
	values := map[string]string{}
	for _, kv := range a.Params {
		values[kv.Key] = kv.Value
	}
	if err := checkParams(a); err != nil {
		return tls.Certificate{}, err
	}
	if values["cert"] == "" || values["key"] == "" {
		return tls.Certificate{}, fmt.Errorf("mtls auth needs cert and key (PEM files)")
	}
	path := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(root, p)
	}
	cert, err := tls.LoadX509KeyPair(path(values["cert"]), path(values["key"]))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("mtls auth: %w", err)
	}
	return cert, nil
}

func checkParams(a model.Auth) error {
	known := model.AuthParams(a.Type)
	for _, kv := range a.Params {
		if !slices.Contains(known, kv.Key) {
			return fmt.Errorf("%s auth has no parameter %q (it has %s)", a.Type, kv.Key, strings.Join(known, ", "))
		}
	}
	return nil
}
