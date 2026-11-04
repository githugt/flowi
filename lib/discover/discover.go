// SPDX-License-Identifier: Apache-2.0

// Package discover finds out what an API with no description offers, by asking it politely: it reads a few well-known
// places and follows the links the answers contain, on the same host only, and never sends anything but GET. What it
// finds becomes a draft of requests with the shape of each response inferred from what came back.
package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const userAgent = "flowi"

// Policy bounds what a discovery may do to somebody's server.
type Policy struct {
	// Start are the paths tried first, before any link is followed.
	Start []string
	// MaxRequests caps every request made, followed links included.
	MaxRequests int
	// Interval is the pause between two requests.
	Interval time.Duration
	// MaxBody caps how much of one response is read.
	MaxBody int64
}

// DefaultPolicy tries where an API usually describes itself or says it is alive, at four requests a second.
func DefaultPolicy() Policy {
	return Policy{
		Start: []string{
			"/", "/openapi.json", "/swagger.json", "/v3/api-docs", "/openapi.yaml", "/api-docs", "/.well-known/openid-configuration",
			"/health", "/healthz", "/status", "/api", "/v1", "/api/v1",
		},
		MaxRequests: 40,
		Interval:    250 * time.Millisecond,
		MaxBody:     1 << 20,
	}
}

// Auth is a header sent with every request, to the base host only.
type Auth struct{ Header, Value string }

// Probe is one request a discovery makes. It is always a GET.
type Probe struct {
	Path string `json:"path"`
	Why  string `json:"why"`
}

// Finding is what one probed path answered.
type Finding struct {
	Path        string `json:"path"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	// Kind is "spec" for an OpenAPI or Swagger description (import it instead of guessing), "json" for a JSON body
	// a request can be drafted from, and "other" for anything else.
	Kind  string   `json:"kind"`
	Links []string `json:"links"` // paths on the same host the body points to
	// Sample is the parsed JSON body, nil unless Kind is "json" or "spec".
	Sample any `json:"-"`
}

// Report is everything a discovery learned.
type Report struct {
	Findings []Finding `json:"findings"`
	// Stopped says why it ended before running out of paths: the server limited it, or the cap was reached. "" when
	// every path was tried.
	Stopped string `json:"stopped,omitempty"`
}

// Plan lists the requests a discovery starts with, so they can be shown and ticked before any is sent. Links found in
// answers are not known yet and are only followed from paths that were allowed.
func Plan(base string, p Policy) ([]Probe, error) {
	if _, err := parseBase(base); err != nil {
		return nil, err
	}
	out := make([]Probe, 0, len(p.Start))
	for _, path := range p.Start {
		out = append(out, Probe{Path: path, Why: whyStart(path)})
	}
	return out, nil
}

func whyStart(path string) string {
	switch path {
	case "/":
		return "the root often lists what the API offers"
	case "/health", "/healthz", "/status":
		return "a liveness endpoint"
	case "/.well-known/openid-configuration":
		return "the sign-in provider's description"
	}
	if strings.Contains(path, "api-docs") || strings.Contains(path, "openapi") || strings.Contains(path, "swagger") {
		return "where an API usually serves its own description"
	}
	return "a common place for an API's entry point"
}

func parseBase(base string) (*url.URL, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%q is not an http or https URL", base)
	}
	return u, nil
}

// Run probes the allowed paths of base, then the same-host paths their answers point to, one at a time and at the
// policy's pace, and reports what answered. paths narrows the start (the ones a person ticked); nil tries every
// start path. doer is the HTTP client, which must not follow a redirect to another host.
func Run(ctx context.Context, doer *http.Client, base string, auth Auth, p Policy, paths []string) (Report, error) {
	u, err := parseBase(base)
	if err != nil {
		return Report{}, err
	}
	if paths == nil {
		paths = p.Start
	}
	pr := prober{doer: sameHostOnly(doer, u.Host), base: u, auth: auth, p: p, seen: map[string]bool{}}
	queue := slices.Clone(paths)
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return pr.report, err
		}
		if pr.sent >= p.MaxRequests {
			pr.report.Stopped = fmt.Sprintf("stopped after %d requests, the limit", p.MaxRequests)
			break
		}
		path := queue[0]
		queue = queue[1:]
		if pr.seen[path] {
			continue
		}
		pr.seen[path] = true
		f, err := pr.probe(ctx, path)
		if errors.Is(err, errLimited) {
			pr.report.Stopped = "the server said to slow down (429): stopped"
			break
		}
		if err != nil {
			return pr.report, err
		}
		pr.report.Findings = append(pr.report.Findings, f)
		queue = append(queue, f.Links...)
		if err := pause(ctx, p.Interval); err != nil {
			return pr.report, err
		}
	}
	return pr.report, nil
}

var errLimited = errors.New("rate limited")

type prober struct {
	doer   *http.Client
	base   *url.URL
	auth   Auth
	p      Policy
	seen   map[string]bool
	sent   int
	report Report
}

func (pr *prober) probe(ctx context.Context, path string) (Finding, error) {
	pr.sent++
	target := pr.base.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), http.NoBody)
	if err != nil {
		return Finding{}, fmt.Errorf("building the request for %s: %w", path, err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, */*;q=0.5")
	if pr.auth.Header != "" {
		req.Header.Set(pr.auth.Header, pr.auth.Value)
	}
	resp, err := pr.doer.Do(req) //nolint:gosec // the host is the one the person asked to explore
	if err != nil {
		// A path that cannot be reached is a finding of its own kind, not a reason to give up on the rest.
		return Finding{Path: path, Kind: "other"}, nil //nolint:nilerr // reported as a finding with status 0
	}
	defer resp.Body.Close() //nolint:errcheck // a body already read
	if resp.StatusCode == http.StatusTooManyRequests {
		return Finding{}, errLimited
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, pr.p.MaxBody))
	if err != nil {
		return Finding{Path: path, Status: resp.StatusCode, Kind: "other"}, nil //nolint:nilerr // a body that broke off is "other"
	}
	return describe(path, resp, body, pr.base), nil
}

func describe(path string, resp *http.Response, body []byte, base *url.URL) Finding {
	f := Finding{Path: path, Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"), Kind: "other", Links: []string{}}
	if resp.StatusCode < 200 || resp.StatusCode > 299 || !strings.Contains(f.ContentType, "json") {
		return f
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return f
	}
	f.Sample, f.Kind = v, "json"
	if m, ok := v.(map[string]any); ok && (m["openapi"] != nil || m["swagger"] != nil) {
		f.Kind = "spec"
		return f
	}
	f.Links = linksIn(v, base)
	return f
}

// avoidWords mark a path that, although it is only a GET, tends to do something when asked.
func avoidWords() []string {
	return []string{"logout", "signout", "sign-out", "delete", "remove", "destroy", "unsubscribe", "reset", "revoke"}
}

// linksIn lists the same-host paths a JSON body mentions as URLs or as "/rooted" strings, without queries, once each.
func linksIn(v any, base *url.URL) []string {
	found := []string{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			if p, ok := pathOnHost(x, base); ok && !slices.Contains(found, p) {
				found = append(found, p)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, k := range sortedKeys(x) {
				walk(x[k])
			}
		}
	}
	walk(v)
	return found
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func pathOnHost(s string, base *url.URL) (string, bool) {
	var u *url.URL
	switch {
	case strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//"):
		u = &url.URL{Path: s}
	case strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://"):
		parsed, err := url.Parse(s)
		if err != nil || parsed.Host != base.Host {
			return "", false
		}
		u = parsed
	default:
		return "", false
	}
	path := u.Path
	if path == "" || path == "/" || strings.ContainsAny(path, " {}<>") {
		return "", false
	}
	lower := strings.ToLower(path)
	for _, w := range avoidWords() {
		if strings.Contains(lower, w) {
			return "", false
		}
	}
	return path, true
}

// sameHostOnly is c with a redirect policy that refuses to leave host, so a credential never travels elsewhere.
func sameHostOnly(c *http.Client, host string) *http.Client {
	out := *c
	out.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Host != host {
			return fmt.Errorf("the server redirected to %s, another host: not followed", req.URL.Host)
		}
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return nil
	}
	return &out
}

func pause(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
