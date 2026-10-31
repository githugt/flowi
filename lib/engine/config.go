// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"errors"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/template"
	"github.com/githugt/flowi-lib/version"
)

// config is the resolved settings one request runs with.
type config struct {
	timeout         time.Duration
	save            string
	pin             string // off | first | last: which responses to pin as examples
	schemas         bool   // learn and check body schemas (schemas.infer)
	empty           template.EmptyPolicy
	httpVersion     string
	follow          bool
	maxRedirects    int
	redirectCreds   string   // strip | same_site | keep
	redirectKeep    []string // header names forwarded on any redirect
	compression     string
	streamThreshold int64
	proxy           string
	caFiles         []string
	retry           httpx.RetryPolicy
	nested          bool
	maxDepth        int
	defaultEnv      string
	maxConcurrency  int
	grace           time.Duration
	cookies         string // isolated | shared | off (lane jars)
	defaultHeaders  []string
	userAgent       string // "" → the client's own
}

// newConfig reads the settings the engine uses. Every key has a validated
// default, so a missing value is the type's zero only for unknown keys.
func newConfig(r settings.Resolved) config {
	empty, err := template.ParseEmptyPolicy(setting[string](r, "empty"))
	if err != nil {
		empty = template.EmptyOmit // the settings schema only allows valid policies
	}
	return config{
		timeout: setting[time.Duration](r, "timeout"), save: setting[string](r, "responses.save"), pin: setting[string](r, "responses.pin"), schemas: setting[bool](r, "schemas.infer"), empty: empty,
		httpVersion: setting[string](r, "http.version"), follow: setting[bool](r, "http.follow_redirects"),
		maxRedirects:  int(setting[int64](r, "http.max_redirects")),
		redirectCreds: setting[string](r, "http.redirect_credentials"), redirectKeep: setting[[]string](r, "http.redirect_keep_headers"),
		compression:     setting[string](r, "http.request_compression"),
		streamThreshold: setting[int64](r, "downloads.stream_threshold_bytes"),
		proxy:           setting[string](r, "proxy.http"), caFiles: setting[[]string](r, "tls.ca_files"),
		retry: httpx.RetryPolicy{
			Max: int(setting[int64](r, "retry.max")), Backoff: setting[string](r, "retry.backoff"),
			Jitter: setting[bool](r, "retry.jitter"), On: setting[[]string](r, "retry.on"),
		},
		nested: setting[bool](r, "hooks.nested"), maxDepth: int(setting[int64](r, "hooks.max_depth")),
		defaultEnv:     setting[string](r, "defaults.environment"),
		maxConcurrency: int(setting[int64](r, "run.max_concurrency")), grace: setting[time.Duration](r, "run.grace"),
		cookies:        setting[string](r, "http.cookies"),
		defaultHeaders: setting[[]string](r, "http.default_headers"), userAgent: setting[string](r, "http.user_agent"),
	}
}

// defaultHeaders are the headers sent unless a request names them: the
// http.default_headers setting ("Name: value" entries; malformed ones are
// skipped) and the User-Agent, which the http.user_agent setting overrides.
func (r *runner) defaultHeaders(cfg config) []model.KeyValue {
	var out []model.KeyValue
	for _, line := range cfg.defaultHeaders {
		name, value, ok := strings.Cut(line, ":")
		if name = strings.TrimSpace(name); ok && name != "" {
			out = append(out, model.KeyValue{Key: name, Value: strings.TrimSpace(value)})
		}
	}
	agent := cfg.userAgent
	if agent == "" {
		agent = r.deps.UserAgent
	}
	if agent == "" {
		agent = "Flowi"
	}
	return append(out, model.KeyValue{Key: "User-Agent", Value: agent})
}

func setting[T any](r settings.Resolved, name string) T {
	v, _ := r.Get(name)
	t, _ := v.Value.(T)
	return t
}

// folderOf is the collection-relative folder of a request.
func folderOf(r model.Request) string { return path.Join("requests", path.Dir(r.Path)) }

func (x *execution) clientOptions(b built) httpx.Options {
	r := x.r
	o := r.baseOptions(x.cfg, x.jar(), path.Join(r.deps.DataDir(x.c.collection.Manifest.ID), "downloads", string(x.c.request.ID)))
	o.BeforeAttempt, o.AfterAttempt = r.attemptEvents(attemptInfo{
		path: x.c.path, requestID: string(x.c.request.ID), requestPath: x.c.request.Path, version: x.version.Label, source: x.source,
		method: b.request.Method, url: b.sent.URL, depth: x.c.depth,
	})
	return o
}

// baseOptions are the client options every call of the run shares: the
// resolved settings, the cookie jar and the run's HTTP limits.
func (r *runner) baseOptions(cfg config, jar *httpx.Jar, downloadDir string) httpx.Options {
	return httpx.Options{
		Transport: r.deps.Transport, Version: cfg.httpVersion, FollowRedirects: cfg.follow, MaxRedirects: cfg.maxRedirects,
		RedirectCredentials: cfg.redirectCreds, RedirectKeepHeaders: cfg.redirectKeep, Timeout: cfg.timeout,
		Proxy: settings.ProxyConfig(cfg.proxy, r.deps.Environ), CAFiles: cfg.caFiles, RequestCompression: cfg.compression,
		Jar: cookieJarOf(jar), StreamThreshold: cfg.streamThreshold, Retry: cfg.retry, Clock: r.deps.Clock,
		DownloadDir: downloadDir, Gate: r.limits.gate,
	}
}

// attemptInfo names a call in its RequestSent / ResponseReceived events.
type attemptInfo struct {
	path, requestID, requestPath, version, source, method, url string
	depth                                                      int
}

// attemptEvents report each attempt; URLs and errors are masked.
func (r *runner) attemptEvents(a attemptInfo) (func(int), func(int, httpx.Attempt)) {
	before := func(n int) {
		r.emit(RequestSent{RunID: r.runID, Path: a.path, RequestID: a.requestID, RequestPath: a.requestPath, Version: a.version, VersionSource: a.source,
			Method: a.method, URL: r.secrets.String(a.url), Attempt: n + 1, Depth: a.depth})
	}
	after := func(n int, at httpx.Attempt) {
		ev := ResponseReceived{RunID: r.runID, Path: a.path, RequestID: a.requestID, Duration: at.Duration, Wait: at.Wait, Attempt: n + 1, Depth: a.depth}
		if at.Response != nil {
			ev.Status = at.Response.Status
		} else {
			ev.Error = r.secrets.String(at.Err.Error())
		}
		r.emit(ev)
	}
	return before, after
}

// cookieJarOf avoids a typed nil: a nil *httpx.Jar inside the
// http.CookieJar interface would not read as "no jar".
func cookieJarOf(j *httpx.Jar) http.CookieJar {
	if j == nil {
		return nil
	}
	return j
}

// environment is the selected environment of a collection.
type environment struct {
	name    string
	id      model.ID
	scope   model.ID        // vault scope; empty means the collection
	values  map[string]any  // non-secret variables
	secrets map[string]bool // names of secret variables
}

// selectEnvironment picks the environment by name, or the
// defaults.environment setting. No name means no environment.
func selectEnvironment(c model.Collection, name, fallback string) (environment, diag.List) {
	if name == "" {
		name = fallback
	}
	env := environment{values: map[string]any{}, secrets: map[string]bool{}}
	if name == "" {
		return env, nil
	}
	if !slices.ContainsFunc(c.Environments, func(e model.Environment) bool { return e.Name == name }) {
		return env, diag.List{diag.New(diag.CodeRequestEnvUnknown, diag.Location{}, map[string]string{"env": name, "collection": c.Manifest.Name})}
	}
	chain, err := model.EnvironmentChain(c.Environments, name)
	if err != nil {
		return env, diag.List{diag.New(diag.CodeRequestEnvExtends, diag.Location{}, map[string]string{"env": name, "detail": err.Error()})}
	}
	own := chain[len(chain)-1]
	env.name, env.id, env.scope = own.Name, own.ID, own.Scope
	for _, e := range chain {
		if err := applyEnvironment(&env, e, e.Name == own.Name); err != nil {
			return env, diag.List{diag.New(diag.CodeRequestEnvExtends, diag.Location{File: e.File}, map[string]string{"env": name, "detail": err.Error()})}
		}
	}
	return env, nil
}

// applyEnvironment lays e's variables over env. A secret counts only for the selected environment itself.
func applyEnvironment(env *environment, e model.Environment, selected bool) error {
	for _, v := range e.Vars {
		if v.Secret {
			if selected {
				env.secrets[v.Name] = true
				delete(env.values, v.Name)
			}
			continue
		}
		typed, err := v.Typed()
		if err != nil {
			return err
		}
		env.values[v.Name] = typed
		delete(env.secrets, v.Name)
	}
	return nil
}

// resolveVersion applies design §4.4: pin, run preference, flow
// preference, default, highest.
func resolveVersion(c call, runPrefer string) (model.RequestVersion, string, diag.List) {
	req := c.request
	q := version.Query{Pinned: optionalLabel(c.pinned), RunPrefer: optionalLabel(runPrefer), FlowPrefer: optionalLabel(c.flowPrefer), Default: optionalLabel(req.Default)}
	byLabel := map[string]model.RequestVersion{}
	for _, v := range req.Versions {
		if l, err := version.Parse(v.Label); err == nil {
			q.Available = append(q.Available, l)
			byLabel[l.String()] = v
		}
	}
	l, source, err := version.Resolve(q)
	if err != nil {
		return model.RequestVersion{}, "", diag.List{versionDiag(req, c.pinned, err)}
	}
	return byLabel[l.String()], sourceName(source), nil
}

func optionalLabel(s string) *version.Label {
	if s == "" {
		return nil
	}
	l, err := version.Parse(s)
	if err != nil {
		return nil // an invalid label cannot match any version; validation reports it
	}
	return &l
}

func versionDiag(r model.Request, pinned string, err error) diag.Diagnostic {
	switch {
	case errors.Is(err, version.ErrPinnedMissing):
		return diag.New(diag.CodeVersionPinnedAbsent, diag.Location{File: r.File}, map[string]string{"label": pinned, "request": r.Path})
	case errors.Is(err, version.ErrDefaultMissing):
		return diag.New(diag.CodeVersionDefaultAbsent, diag.Location{File: r.File}, map[string]string{"label": r.Default})
	}
	return diag.New(diag.CodeVersionNone, diag.Location{File: r.File}, nil)
}

func sourceName(s version.Source) string {
	names := map[version.Source]string{
		version.SourcePinned: "pinned", version.SourceRunPreference: "preferred", version.SourceFlowPreference: "preferred",
		version.SourceDefault: "default", version.SourceHighest: "highest",
	}
	return names[s]
}

func itoa(n int) string { return strconv.Itoa(n) }
