// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/publicsuffix"
)

type hopsKey struct{}

// withHops gives the redirect policy somewhere to record this attempt's
// hops; the policy is shared by every request of the client.
func withHops(ctx context.Context, hops *[]Hop) context.Context {
	return context.WithValue(ctx, hopsKey{}, hops)
}

func redirectPolicy(o Options) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if !o.FollowRedirects {
			return http.ErrUseLastResponse
		}
		if len(via) > o.MaxRedirects {
			return fmt.Errorf("stopped after %d redirects (http.max_redirects)", o.MaxRedirects)
		}
		forwarded := applyCredentialPolicy(o, via[0], req)
		if hops, ok := req.Context().Value(hopsKey{}).(*[]Hop); ok && req.Response != nil {
			*hops = append(*hops, Hop{
				Status:    req.Response.StatusCode,
				URL:       via[len(via)-1].URL.String(),
				Location:  req.Response.Header.Get("Location"),
				Header:    req.Response.Header.Clone(),
				Forwarded: forwarded,
			})
		}
		return nil
	}
}

// Values of Options.RedirectCredentials.
const (
	RedirectStrip    = "strip"
	RedirectSameSite = "same_site"
	RedirectKeep     = "keep"
)

// applyCredentialPolicy decides which credential headers of the first
// request go with a redirect to a new origin, and returns the names it let
// through. It always measures from the first request: net/http copies the
// first request's headers again on every hop, comparing only with the hop
// before, so a chain A -> B -> B would hand B the headers on its second hop.
func applyCredentialPolicy(o Options, first, req *http.Request) []string {
	if !leavesOrigin(first.URL, req.URL) {
		return nil
	}
	strip := stripsCredentials(o.RedirectCredentials, first.URL, req.URL)
	var forwarded []string
	for name := range first.Header {
		if !isCredentialHeader(name) {
			continue
		}
		if strip && !slices.ContainsFunc(o.RedirectKeepHeaders, func(k string) bool { return strings.EqualFold(k, name) }) {
			req.Header.Del(name)
			continue
		}
		// net/http has already dropped Authorization and Cookie on a host
		// change; put back what the settings say to keep.
		if _, ok := req.Header[http.CanonicalHeaderKey(name)]; !ok {
			req.Header[http.CanonicalHeaderKey(name)] = slices.Clone(first.Header[name])
		}
		forwarded = append(forwarded, http.CanonicalHeaderKey(name))
	}
	slices.Sort(forwarded)
	return forwarded
}

// stripsCredentials applies the mode to a redirect that leaves the origin.
func stripsCredentials(mode string, from, to *url.URL) bool {
	switch mode {
	case RedirectKeep:
		return false
	case RedirectSameSite:
		return !sameSite(from, to) || downgrades(from, to)
	}
	return true
}

func downgrades(from, to *url.URL) bool { return from.Scheme == "https" && to.Scheme != "https" }

// sameSite reports whether both URLs belong to one registrable domain
// (api.example.com and www.example.com do). Hosts without one, such as IP
// addresses and localhost, are the same site only when they are equal.
func sameSite(a, b *url.URL) bool {
	ha, hb := a.Hostname(), b.Hostname()
	if ha == hb {
		return true
	}
	sa, errA := publicsuffix.EffectiveTLDPlusOne(ha)
	sb, errB := publicsuffix.EffectiveTLDPlusOne(hb)
	return errA == nil && errB == nil && sa == sb
}

// leavesOrigin reports whether following a redirect from one URL to the
// next sends the request somewhere the credentials were not meant for: a
// different host or port, or a downgrade from https to http. net/http
// compares the host name only and strips only Authorization and Cookie, so
// it would let a different port through.
func leavesOrigin(from, to *url.URL) bool {
	if from.Hostname() != to.Hostname() || effectivePort(from) != effectivePort(to) {
		return true
	}
	return downgrades(from, to)
}

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// credentialHeaderParts are name fragments that mark a header as carrying
// a credential. Matching on fragments catches custom headers (X-Api-Key,
// X-Auth-Token) that a fixed list would miss. The cost is that a harmless
// header such as Idempotency-Key is also dropped on a cross-origin
// redirect, which is the safe side to err on; http.redirect_keep_headers
// lists the ones to keep.
func credentialHeaderParts() []string {
	return []string{"auth", "token", "key", "secret", "passw", "credential", "signature", "session", "cookie"}
}

func isCredentialHeader(name string) bool {
	lower := strings.ToLower(name)
	return slices.ContainsFunc(credentialHeaderParts(), func(part string) bool { return strings.Contains(lower, part) })
}
