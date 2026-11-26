// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/internal/authx"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

// authenticateAsync applies the auth block of a WebSocket or SSE request to the operation just before it connects,
// the way authenticate does for an HTTP request: the credential goes in a connection header or the query of the
// address (what the scheme says), a client certificate in the TLS settings. What it adds is registered with the
// masker, so it shows masked wherever the request is shown. ok is false when the scheme could not be applied (the
// send has failed). A request with no auth block goes through untouched, and so does a protocol that takes its
// credentials in its bindings.
func (x *execution) authenticateAsync(ctx context.Context, b *builtAsync) bool {
	if b.auth == nil {
		return true
	}
	if !slices.Contains(model.AsyncAuthTypes(b.protocol), b.auth.Type) {
		x.failAuth(fmt.Errorf("a %s request can use %s, not %q", b.protocol, strings.Join(model.AsyncAuthTypes(b.protocol), ", "), b.auth.Type))
		return false
	}
	if b.auth.Type == model.AuthMutualTLS {
		return x.presentClientCert(b)
	}
	signer, err := authx.New(*b.auth, x.authEnv())
	if err != nil {
		x.failAuth(err)
		return false
	}
	// The signer works on an HTTP request: give it the address and no headers, and read back what it added.
	target := model.TargetURL(b.protocol, b.op.Server, b.op.Channel)
	req := httpx.Request{Method: http.MethodGet, URL: target, Header: http.Header{}}
	applied, err := signer.Apply(ctx, &req)
	if err != nil {
		x.failAuth(err)
		return false
	}
	for _, a := range applied {
		x.recordApplied(&b.sent, a)
		if a.Where == "header" || a.Where == "cookie" {
			b.op.Headers = setConnectionHeader(b.op.Headers, a)
		}
	}
	if req.URL != target {
		b.op.Server = req.URL
		if b.protocol == model.ProtocolWS {
			b.op.Channel = "" // the channel is a path of the address, and now sits before the query
		}
	}
	return true
}

// presentClientCert adds the certificate a mutual TLS auth block names to the connection's TLS settings.
func (x *execution) presentClientCert(b *builtAsync) bool {
	cert, err := loadClientCert(*b.auth, x.c.collection.Root)
	if err != nil {
		x.failAuth(err)
		return false
	}
	cfg := b.op.TLS
	if cfg == nil {
		cfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	cfg = cfg.Clone()
	cfg.Certificates = append(cfg.Certificates, cert)
	b.op.TLS = cfg
	return true
}

// setConnectionHeader sets one header a scheme added, replacing any of the same name the request wrote. A cookie is
// added to the Cookie header the request has, as the HTTP client does.
func setConnectionHeader(headers []transport.Header, a authx.Applied) []transport.Header {
	if a.Where == "cookie" {
		pair := url.QueryEscape(a.Name) + "=" + a.Value
		i := slices.IndexFunc(headers, func(h transport.Header) bool { return strings.EqualFold(h.Name, "Cookie") })
		if i < 0 {
			return append(slices.Clone(headers), transport.Header{Name: "Cookie", Value: pair})
		}
		out := slices.Clone(headers)
		out[i].Value += "; " + pair
		return out
	}
	out := slices.DeleteFunc(slices.Clone(headers), func(h transport.Header) bool { return strings.EqualFold(h.Name, a.Name) })
	return append(out, transport.Header{Name: a.Name, Value: a.Value})
}
