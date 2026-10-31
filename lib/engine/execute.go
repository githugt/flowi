// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"sync"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/internal/names"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/lifecycle"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// execution is one request being executed, with its hooks.
type execution struct {
	r       *runner
	c       call
	version model.RequestVersion
	source  string
	cfg     config
	env     environment
	state   *nodes.State
	with    map[string]any
	memo    *genMemo
	record  *Record
	status  Status

	// mu guards view and diags: nodes of a hook flow run in their own
	// goroutines (and in parallel inside parallel/batch hook nodes).
	mu   sync.Mutex
	view map[string]any // the request as hooks see it
	// async is what an async request's expressions see once it has run
	// (messages, count, outcome, body, headers); nil before and for HTTP.
	async map[string]any
	diags diag.List
	// messageHooks run for each message a message request keeps (see sendAsync).
	messageHooks []HookRef
}

// execute runs one request: version and settings, pre-hooks, render, send,
// extract and assert, post-hooks, save (design §4.5 order).
func (r *runner) execute(ctx context.Context, c call) outcome {
	x, diags := r.prepare(c)
	if diags.HasErrors() {
		return outcome{status: StatusError, diags: diags}
	}
	chain, ok := x.hookChain()
	x.messageHooks = chain.Message
	if ok && x.runPhase(ctx, chain.Pre, nodes.PhasePre) {
		x.sendAndCheck(ctx, chain.Post)
	}
	out := x.outcome()
	// After outcome() has let go of the lock: the scope takes it too.
	out.outcomes = matchOutcomes(x.outcomeList(), out.status, x.scope(true))
	return out
}

// sendAndCheck is everything after the pre-hooks: what the version's family does.
func (x *execution) sendAndCheck(ctx context.Context, post []HookRef) {
	x.exchange().send(ctx, post)
}

// sendHTTP is sendAndCheck for an HTTP request.
func (x *execution) sendHTTP(ctx context.Context, post []HookRef) {
	b, ok := x.build()
	if !ok {
		return
	}
	resp, ok := x.send(ctx, b)
	if !ok {
		return
	}
	x.observe(b, resp)
	x.warnIfDeprecated()
	x.extractOutputs()
	x.checkAssertions()
	x.runPhase(ctx, post, nodes.PhasePost)
	if ctx.Err() == nil {
		x.save()
		x.checkSchemas()
	}
}

// headerOf is the response headers of a record.
func headerOf(rec *Record) http.Header {
	header := http.Header{}
	for _, h := range rec.Header {
		header.Add(h.Name, h.Value)
	}
	return header
}

// warnIfDeprecated adds a warning when the response announces its own end (Deprecation, Sunset): the send is not
// failed, since the endpoint works today.
func (x *execution) warnIfDeprecated() {
	if x.record == nil {
		return
	}
	notice, found := lifecycle.Read(headerOf(x.record))
	if !found {
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.diags = append(x.diags, diag.New(diag.CodeRunDeprecated, diag.Location{}, map[string]string{
		"request": x.c.request.Path, "notice": notice.Describe(x.r.deps.Clock()),
	}))
}

// checkSchemas checks the response (and the request body) against the stored
// schemas, then learns from them. What it finds is added to the send's
// diagnostics without changing its status: a schema mismatch never stops a send.
func (x *execution) checkSchemas() {
	// A flowi overlay is often a deliberately odd request (a bad token, a
	// delay), so its responses must not teach the schema what is normal.
	if x.r.deps.Schemas == nil || !x.cfg.schemas || x.record == nil || x.c.overlay != nil {
		return
	}
	notes := x.r.deps.Schemas.Check(x.c.collection, x.c.request, x.version.Label, *x.record)
	x.mu.Lock()
	defer x.mu.Unlock()
	x.diags = append(x.diags, notes...)
}

// prepare resolves what the request runs with: version, settings,
// environment and with values.
func (r *runner) prepare(c call) (*execution, diag.List) {
	v, source, diags := resolveVersion(c, r.prefer)
	if diags.HasErrors() {
		return nil, diags
	}
	v, overlayNotes := laidOver(c, v)
	cfg := newConfig(r.deps.Lookup.Settings(c.collection.Manifest.ID, folderOf(c.request), v.Settings))
	env, diags := selectEnvironment(c.collection, r.env, cfg.defaultEnv)
	if diags.HasErrors() {
		return nil, diags
	}
	x := &execution{r: r, c: c, version: v, source: source, cfg: cfg, env: env, status: StatusPassed,
		memo: newGenMemo(r.gen), diags: overlayNotes}
	x.state = &nodes.State{Vars: nodes.NewVars(nil), Request: &nodes.MutableRequest{}, Emit: emitter{x}, Masker: r.secrets,
		Scope: func() template.Scope { return x.scope(false) }, Secret: r.secrets.derived}
	with, diags := x.renderWith()
	x.with = with
	return x, diags
}

// renderWith renders the send's --with templates (top level) or takes a
// hook flow's already-rendered values.
func (x *execution) renderWith() (map[string]any, diag.List) {
	if x.c.withValues != nil {
		return x.c.withValues, nil
	}
	out := map[string]any{}
	var diags diag.List
	for k, src := range x.c.with {
		tpl, err := template.Parse(src)
		if err == nil {
			var r template.Result
			r, err = tpl.Eval(x.scope(false))
			out[k] = r.Value
			if r.Tainted {
				x.r.secrets.derived(r.Value)
			}
		}
		if err != nil {
			diags = append(diags, templateFailure("with value "+k+" of "+x.c.request.Path, err)...)
		}
	}
	return out, diags
}

// hookChain is the hooks this execution runs. Requests inside hook flows
// (depth > 0) run their own hooks only with hooks.nested, and no deeper
// than hooks.max_depth.
func (x *execution) hookChain() (HookChain, bool) {
	chain := ChainFor(x.c.collection, x.c.request, x.version).Filter(x.c.hooks)
	if x.c.depth == 0 {
		return chain, true
	}
	if !x.cfg.nested {
		return HookChain{}, true
	}
	if x.c.depth > x.cfg.maxDepth {
		x.fail(StatusFailed, diag.List{diag.New(diag.CodeRunHookDepth, diag.Location{}, map[string]string{"max": itoa(x.cfg.maxDepth)})})
		return HookChain{}, false
	}
	return chain, true
}

// scope is what templates in this execution can read.
func (x *execution) scope(lenient bool) template.Scope {
	values := map[string]any{
		"env": x.env.values, "secret": x.r.secrets.namespace(x.c.collection.Manifest.ID, x.env),
		"with": x.with, "vars": x.state.Vars.Snapshot(), "inputs": map[string]any{}, "nodes": map[string]any{},
		"var": x.requestVariables(), "collection_var": collectionVariables(x.c.collection, x.env.values),
	}
	if view := x.currentView(); view != nil {
		values["request"] = view
	}
	maps.Copy(values, x.asyncValues())
	if resp := x.state.Response; resp != nil {
		values["response"], values["status"] = resp.Values(), resp.Status
	}
	return template.Scope{Values: values, Bare: []string{"with", "vars", "inputs", "env", "var"}, Gen: x.r.gen, Lenient: lenient}
}

// collectionVariables are the collection's own values, each replaced by the
// environment's variable of the same name when the environment has one.
func collectionVariables(c model.Collection, env map[string]any) map[string]any {
	out := make(map[string]any, len(c.Manifest.Variables))
	for _, kv := range c.Manifest.Variables {
		out[kv.Key] = kv.Value
		if v, ok := env[kv.Key]; ok {
			out[kv.Key] = v
		}
	}
	return out
}

// requestVariables are the version's variables as defaults: an environment
// variable of the same name wins, so one request runs against any
// environment and still has values when the environment lacks them.
func (x *execution) requestVariables() map[string]any {
	out := map[string]any{}
	for _, kv := range x.version.Variables {
		out[kv.Key] = kv.Value
		if v, ok := x.env.values[kv.Key]; ok {
			out[kv.Key] = v
		}
	}
	return out
}

// build renders the request strictly, after the pre-hooks, and applies
// their overrides. Missing secrets are reported together.
func (x *execution) build() (built, bool) {
	b, diags := x.render(false)
	diags = append(diags, x.r.secrets.problems(x.env.name, b.missingSecrets)...)
	if diags.HasErrors() {
		x.fail(StatusError, diags)
		return built{}, false
	}
	return b, true
}

func (x *execution) render(lenient bool) (built, diag.List) {
	b, diags := build(buildInput{
		version: x.version, scope: x.scope(lenient), empty: x.cfg.empty, label: x.c.request.Path,
		root: x.c.collection.Root, secretVars: x.env.secrets, missingSecret: x.r.secrets.isMissing(x.env.name), memo: x.memo,
		secret: x.r.secrets.derived, defaults: x.r.defaultHeaders(x.cfg),
	})
	applyNodeHeaders(&b, x.c.headers)
	diags = append(diags, applyOverrides(&b, x.state.Request)...)
	if x.c.bodyOver != nil {
		diags = append(diags, applyBody(&b, &nodes.MutableRequest{Body: x.c.bodyOver})...)
	}
	return b, diags
}

func (x *execution) currentView() map[string]any {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.view
}

func (x *execution) setView(v map[string]any) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.view = v
}

// jar is the cookie jar this request uses: its frame's (a flow lane may
// have its own, or none), else the run's.
func (x *execution) jar() *httpx.Jar {
	if x.c.noJar {
		return nil
	}
	if x.c.jar != nil {
		return x.c.jar
	}
	return x.r.jar
}

// requestView is the request as pre-hooks read it: a lenient render, so
// fields that use values not set yet are empty instead of errors.
func (x *execution) requestView() map[string]any {
	return x.exchange().view()
}

func viewOf(b built) map[string]any {
	headers := map[string]any{}
	for name := range b.request.Header {
		headers[name] = b.request.Header.Get(name)
	}
	return map[string]any{"method": b.request.Method, "url": b.request.URL, "headers": headers, "body": b.bodyText}
}

// send sends the built request. ok is false when nothing usable came back
// (transport failure, or cancellation).
func (x *execution) send(ctx context.Context, b built) (*httpx.Response, bool) {
	x.r.addTraceparent(b.request.Header, x.c.path)
	signer, ok := x.authenticate(ctx, &b)
	if !ok {
		return nil, false
	}
	opts, ok := x.clientOptionsFor(b)
	if !ok {
		return nil, false
	}
	client, err := httpx.NewClient(opts)
	if err != nil {
		x.fail(StatusError, diag.List{diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": err.Error()})})
		return nil, false
	}
	defer client.Close()
	attempts, err := client.Do(ctx, b.request)
	if err == nil {
		attempts, err = x.answerChallenge(ctx, client, signer, &b, attempts)
	}
	if err != nil {
		// The context ended (the run was cancelled or the node timed out):
		// nothing usable came back, so this request did not pass.
		x.fail(StatusCancelled, nil)
		return nil, false
	}
	x.record = x.newRecord(b, attempts)
	last := attempts[len(attempts)-1]
	if last.Err != nil {
		x.fail(StatusError, diag.List{transportDiag(last.Err)})
		return nil, false
	}
	return last.Response, true
}

func transportDiag(err error) diag.Diagnostic {
	var te *httpx.TimeoutError
	if errors.As(err, &te) {
		return diag.New(diag.CodeHTTPTimeout, diag.Location{}, map[string]string{"timeout": te.After.String()})
	}
	return diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": err.Error()})
}

func (x *execution) newRecord(b built, attempts []httpx.Attempt) *Record {
	rec := &Record{
		ID: x.r.deps.NewID(), RunID: x.r.runID, CollectionID: x.c.collection.Manifest.ID, RequestID: x.c.request.ID,
		RequestPath: x.c.request.Path, Version: x.version.Label, VersionSource: x.source, At: x.r.deps.Clock(),
		Request: b.sent, Attempts: attemptRecords(attempts), Overlay: x.c.overlayName(),
	}
	if last := attempts[len(attempts)-1]; last.Response != nil {
		fillResponse(rec, last.Response)
	}
	return rec
}

// observe makes the response visible to extraction, assertions and
// post-hooks; the request can no longer be changed.
func (x *execution) observe(b built, resp *httpx.Response) {
	var body any = x.record.Body.Text
	if x.record.Body.JSON != nil {
		body = scopeValue(x.record.Body.JSON)
	}
	x.state.Response = &nodes.Response{Status: resp.Status, Header: resp.Header, Body: body, Text: string(resp.Body), Time: resp.Timings.Total}
	x.state.Request = nil
	if jar := x.jar(); jar != nil {
		x.state.Cookies = jar.All()
	}
	x.setView(viewOf(b))
}

// extractOutputs runs the version's extract: block.
func (x *execution) extractOutputs() {
	for _, e := range x.version.Extract {
		v, err := nodes.Extract(e, x.state.Response, x.state.Cookies)
		if err != nil {
			x.fail(StatusFailed, diag.List{diag.New(diag.CodeRunExtractEmpty, diag.Location{}, map[string]string{"name": e.Name, "detail": err.Error()})})
			continue
		}
		if x.record.Outputs == nil {
			x.record.Outputs = map[string]any{}
		}
		x.record.Outputs[e.Name] = v
	}
}

// checkAssertions runs the version's assert: list.
func (x *execution) checkAssertions() {
	for _, check := range x.version.Assert {
		ok, _, err := template.Check(check, x.scope(false))
		if err != nil {
			x.fail(StatusFailed, templateFailure("assert "+check+" of "+x.c.request.Path, err))
			continue
		}
		if !ok {
			x.fail(StatusFailed, nodes.Failed("assert", check, "", false, emitter{x}))
		}
	}
}

func (x *execution) save() {
	if !names.AllValid(string(x.c.collection.Manifest.ID), string(x.c.request.ID), x.version.Label) {
		x.r.addSaved(nil, fmt.Errorf("not saving the response of %s: its id or version label is not a plain name", x.c.request.Path))
		return
	}
	files, err := saveRecord(x.record, x.saveDir(), x.savePolicy(), x.r.secrets.current())
	x.r.addSaved(files, err)
	if pinned, err := autoPin(x.record, x.c.request.File, x.cfg.pin, x.r.secrets.current()); err != nil {
		x.r.addSaved(nil, err)
	} else if pinned != "" {
		x.r.addSaved([]string{pinned}, nil)
	}
}

func (x *execution) savePolicy() string {
	if x.c.save != "" {
		return x.c.save
	}
	return x.cfg.save
}

// fail records diagnostics and lowers the status (cancelled > error >
// failed > passed).
func (x *execution) fail(s Status, diags diag.List) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.diags = append(x.diags, diags...)
	if rank(s) > rank(x.status) {
		x.status = s
	}
}

func rank(s Status) int {
	ranks := map[Status]int{StatusPassed: 0, StatusSkipped: 0, StatusFailed: 1, StatusError: 2, StatusCancelled: 3}
	return ranks[s]
}

func (x *execution) outcome() outcome {
	x.mu.Lock()
	defer x.mu.Unlock()
	return outcome{status: x.status, record: x.record, vars: x.state.Vars.Snapshot(), diags: x.diags}
}

// emitter adapts node output to run events.
type emitter struct{ x *execution }

func (e emitter) Log(node, message string) {
	e.x.r.emit(Log{RunID: e.x.r.runID, Node: node, Message: e.x.r.secrets.String(message)})
}

func (e emitter) Digest(node, format, text string) {
	e.x.r.emit(Digest{RunID: e.x.r.runID, Node: node, Format: format, Text: e.x.r.secrets.String(text)})
}

func (e emitter) Warn(d diag.Diagnostic) {
	e.x.mu.Lock()
	e.x.diags = append(e.x.diags, d)
	e.x.mu.Unlock()
	e.x.r.warn(d)
}

// templateFailure maps a template error to diagnostics for field.
func templateFailure(field string, err error) diag.List {
	b := builder{in: buildInput{label: ""}}
	b.templateErrorField(field, err)
	return b.diags
}
