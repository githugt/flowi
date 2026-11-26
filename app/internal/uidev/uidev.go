// SPDX-License-Identifier: AGPL-3.0-or-later

// Package uidev is a made-up workspace with a backend that answers without a network, a disk or a vault: what the
// UI-development server (cmd/uidev) serves the front end from.
//
// Before: the front end could be seen only through the full Go app over a real workspace, so a change to a screen
// needed a build, a workspace with the right requests in it, and a server to send them to. This backend is the fake
// the view-model tests already use (vmtest), seeded with a few requests, a flow with a failing step, a flow of 300
// steps and two environments, and with sends and runs that answer with canned results at a visible pace.
//
// Nothing here is saved: edits live in memory until the process ends.
package uidev

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// Flow ids of the fixture, for the tests and for anyone looking for the 300-step flow.
const (
	FlowCheckout model.ID = "F-checkout"
	FlowSoak     model.ID = "F-soak"
	// FailingStep is the step of the checkout flow that fails.
	FailingStep = "pay"
	// SoakSteps is how many steps the long flow has.
	SoakSteps = 300
)

// Backend is the fake backend with canned sends and runs.
type Backend struct {
	*vmtest.Backend
	// Pace is the pause before each event of a run or a send; 0 means 120 ms for the checkout flow and 8 ms for the
	// long one. Tests set it to a nanosecond.
	Pace time.Duration

	sandbox importSandbox // the real importers' collection, see import.go

	varsMu sync.Mutex
	vars   []model.KeyValue // the collection variables the page edits; nothing is saved to disk

	rulesMu sync.Mutex
	rules   map[string]string // the spec rule levels the screen has chosen, by rule; nothing is saved to disk
}

// specRules are the spec linter's rules as the page lists them: the real names, with their defaults.
func specRules() []flowi.SpecRule {
	return []flowi.SpecRule{
		{Name: "info-description", About: "The description says what the API is for.", Default: "warning"},
		{Name: "operation-description", About: "Every operation (or method, or channel operation) has a summary or a description.", Default: "warning"},
		{Name: "operation-id", About: "Every operation has an operationId (OpenAPI, Swagger, AsyncAPI 2).", Default: "warning"},
		{Name: "operation-tags", About: "Every operation has a tag.", Default: "note"},
		{Name: "operation-security", About: "Every operation has a security requirement, or says it is public with security: [].", Default: "warning"},
		{Name: "success-response", About: "Every operation documents a 2xx response.", Default: "warning"},
		{Name: "unused-component", About: "Every component is referred to somewhere.", Default: "warning"},
		{Name: "server-encrypted", About: "Servers are encrypted (https, wss, mqtts, amqps…); plain ones are for this machine only.", Default: "warning"},
		{Name: "path-case", About: "Path segments are lowercase.", Default: "off"},
	}
}

// SpecLintRules lists the rules with the levels chosen so far in this session.
func (b *Backend) SpecLintRules(model.ID) ([]flowi.SpecRule, error) {
	b.rulesMu.Lock()
	defer b.rulesMu.Unlock()
	out := specRules()
	for i, r := range out {
		out[i].Level = r.Default
		if l, ok := b.rules[r.Name]; ok {
			out[i].Level, out[i].Chosen = l, true
		}
	}
	return out, nil
}

// SetSpecLintRule keeps a choice in memory; "" forgets it.
func (b *Backend) SetSpecLintRule(_ model.ID, rule, level string) error {
	b.rulesMu.Lock()
	defer b.rulesMu.Unlock()
	if b.rules == nil {
		b.rules = map[string]string{}
	}
	if level == "" {
		delete(b.rules, rule)
	} else {
		b.rules[rule] = level
	}
	return nil
}

// New returns a backend over the fixture workspace.
func New() *Backend {
	b := &vmtest.Backend{Cols: append([]model.Collection{Collection()}, sidebarCollections()...), Name: "UI development"}
	b.DraftIDs = map[model.ID]bool{vmtest.ScratchID: true}
	// A folder of its own for what the sidebar keeps (stars, recent), so a star can be tried without a notice that it
	// could not be saved. Gone with the temporary folder: nothing of the person's is touched.
	if dir, err := os.MkdirTemp("", "flowi-uidev-session-"); err == nil {
		b.Session = dir
	}
	b.Diags = sidebarProblems()
	// staging's secret holds a value; the payments secrets do not, so the sidebar's Environments show an issue to read.
	b.SecretList = []flowi.SecretInfo{{Environment: "staging", Name: "password", Set: true}}
	b.Values = map[string]any{"defaults.environment": "staging"}
	b.Check.Coverage = madeUpCoverage()
	b.Check.Exports, b.Check.Exported = madeUpExports()
	b.EnvGroups = []string{"data jobs"} // a group made on its own, with no environment in it yet
	// Saved runs, newest first, so the Runs tab has a list to compare, step through and clear.
	started := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	b.RunDetails = map[string]flowi.RunDetail{}
	b.RunStepList = map[string]flowi.RunSteps{}
	for i, status := range []string{"passed", "passed", "failed"} {
		id := fmt.Sprintf("RUN%d", i+1)
		payCode := 200
		if status == "failed" {
			payCode = 500
		}
		ms := time.Millisecond
		b.RunStepList[id] = flowi.RunSteps{ID: id, Status: status, Steps: []flowi.RunStep{
			{ID: "login", Status: "passed", Instances: 1, Code: 200, Start: 0, End: 220 * ms},
			{ID: "create", Status: "passed", Instances: 1, Code: 201, Start: 230 * ms, End: 610 * ms},
			{ID: "wait", Status: "passed", Instances: 1, Start: 620 * ms, End: 1620 * ms},
			{ID: FailingStep, Status: status, Instances: 1, Code: payCode, Start: 1630 * ms, End: 1900 * ms},
		}}
		b.RunDetails[id] = flowi.RunDetail{ID: id, Outputs: map[string]map[string]any{"login": {"status": 200}}, Vars: map[string]any{}}
		b.RunList = append([]flowi.RunSummary{{ID: id, Env: "staging", Status: status, Started: started.Add(time.Duration(i) * time.Hour), Duration: 1500 * time.Millisecond,
			Nodes: []flowi.RunNodeSummary{{ID: "login", Type: "request", Status: "passed", Duration: 200 * time.Millisecond}, {ID: FailingStep, Type: "request", Status: status, Duration: 300 * time.Millisecond}}}}, b.RunList...)
	}
	return &Backend{Backend: b, vars: madeUpVariables()}
}

// Collection is the fixture: a shop with a login, an order and a health request, two environments, a checkout flow
// whose last step fails and a flow of SoakSteps steps.
func Collection() model.Collection {
	get := func(id, path, name, method, url string, body *model.Body) model.Request {
		v := model.RequestVersion{Label: "1", Method: method, URL: url}
		if body != nil {
			v.Body = *body
		}
		return model.Request{Meta: model.Meta{ID: model.ID(id), Name: name}, Path: path, File: "/demo/requests/" + path + ".req.yaml", Versions: []model.RequestVersion{v}}
	}
	login := get("R-login", "auth/login", "Login", "POST", "{{env.base_url}}/login", &model.Body{JSON: `{"email": "ada@example.com", "password": "{{env.password}}"}`})
	login.Versions[0].Extract = []model.Extraction{{Name: "token", From: "body", Path: "$.token"}}
	login.Versions[0].Assert = []string{"status == 200"}
	orders := get("R-orders", "orders/list", "List orders", "GET", "{{env.base_url}}/orders", nil)
	create := get("R-create", "orders/create", "Create an order", "POST", "{{env.base_url}}/orders", &model.Body{JSON: `{"sku": "A-100", "quantity": 2}`})
	pay := get("R-pay", "orders/pay", "Pay for an order", "POST", "{{env.base_url}}/orders/{{order_id}}/pay", &model.Body{JSON: `{"card": "tok_visa"}`})
	health := get("R-health", "health", "Health", "GET", "{{env.base_url}}/health", nil)

	// Development status and tags, so the sidebar, the editors and the search have something to show: a status of its
	// own, one a folder gives (orders/ is in development, Pay overrides it), a status of the collection's own
	// ("needs-design") and a status the collection does not have, which is a diagnostic.
	login.Status, login.Tags = "production", []string{"smoke", "auth"}
	create.Tags = []string{"smoke"}
	orders.Status = "blocked" // not one of the collection's statuses
	pay.Status, pay.Tags = "review", []string{"payments"}
	health.Status, health.Tags = "needs-design", []string{"smoke", "ops"}

	call := func(id, path string) *model.RequestCall {
		return &model.RequestCall{Request: model.Ref{ID: model.ID(id), Path: path}}
	}
	checkout := model.Flow{Meta: model.Meta{ID: FlowCheckout, Name: "Checkout"}, Path: "checkout", File: "/demo/flows/checkout.arazzo.yaml", Nodes: []model.Node{
		{ID: "login", Type: "request", Request: call("R-login", "auth/login")},
		{ID: "create", Type: "request", Request: call("R-create", "orders/create")},
		{ID: "wait", Type: "delay", Params: []model.Field{{Key: "duration", Value: "1s"}}},
		{ID: FailingStep, Type: "request", Request: call("R-pay", "orders/pay")},
		{ID: "done", Type: "log", Params: []model.Field{{Key: "value", Value: "order paid"}}},
	}}
	checkout.Status, checkout.Tags = "testing", []string{"smoke", "payments"}
	soak := model.Flow{Meta: model.Meta{ID: FlowSoak, Name: "Soak: 300 steps"}, Path: "soak", File: "/demo/flows/soak.arazzo.yaml"}
	for i := 0; i < SoakSteps; i++ {
		id := fmt.Sprintf("step%03d", i+1)
		if i%3 == 0 {
			soak.Nodes = append(soak.Nodes, model.Node{ID: id, Type: "request", Request: call("R-health", "health")})
		} else {
			soak.Nodes = append(soak.Nodes, model.Node{ID: id, Type: "log", Params: []model.Field{{Key: "value", Value: "step " + id}}})
		}
	}

	return model.Collection{
		Root:     "/demo",
		Manifest: model.Manifest{Meta: model.Meta{ID: "C-demo", Name: "Demo shop", Description: "The shop API: log in, order, pay."}, Variables: madeUpVariables(), File: "/demo/flowi.yaml", Statuses: []model.Status{{Name: "needs-design", Color: "purple", Icon: "bookmark"}}},
		Folders:  []model.Folder{{Dir: "requests/orders", Status: "in-development", Tags: []string{"orders"}, File: "/demo/requests/orders/_folder.yaml"}},
		Environments: []model.Environment{
			{Meta: model.Meta{Name: "staging"}, File: "/demo/environments/staging.yaml", Vars: []model.EnvVar{{Name: "base_url", Value: "https://staging.shop.example"}, {Name: "password", Secret: true}, {Name: "retries", Value: "3"}, {Name: "region", Value: "eu"}}},
			{Meta: model.Meta{Name: "local"}, File: "/demo/environments/local.yaml", Vars: []model.EnvVar{{Name: "base_url", Value: "http://localhost:8080"}, {Name: "retries", Value: "3", Kind: model.VarNumber}, {Name: "region", Value: "eu"}}},
			// A second set of environments, for the services that take payments: the group Compare and Manage environments sort by.
			{Meta: model.Meta{Name: "payments-staging"}, File: "/demo/environments/payments-staging.yaml", Groups: []string{"checkout", "payments"}, Colour: "blue", Vars: []model.EnvVar{{Name: "timeout", Value: "30", Kind: model.VarNumber}, {Name: "mode", Value: "fast"}, {Name: "api_key", Secret: true}}},
			{Meta: model.Meta{Name: "payments-prod"}, File: "/demo/environments/payments-prod.yaml", Groups: []string{"payments"}, Colour: "red", Protected: true, Extends: "payments-staging", Vars: []model.EnvVar{{Name: "timeout", Value: "30"}, {Name: "api_key", Secret: true}}},
		},
		Requests: []model.Request{login, orders, create, pay, health},
		Flows:    []model.Flow{checkout, soak},
	}
}

// FlowSource writes a flow as the code view shows it, one block for each step, so the view has steps to put a note above
// (the fake's own text is only a count). It is not the real encoding: a step is its id, its type and the request it calls.
func (b *Backend) FlowSource(t flowi.FlowTarget, f model.Flow) (flowi.FlowText, error) {
	text, err := b.Backend.FlowSource(t, f)
	if err != nil {
		return text, err
	}
	var out strings.Builder
	out.WriteString("nodes:\n")
	for _, n := range f.Nodes {
		fmt.Fprintf(&out, "  - id: %s\n    type: %s\n", n.ID, n.Type)
		if n.Request != nil {
			fmt.Fprintf(&out, "    request:\n      id: %s\n      path: %s\n", n.Request.Request.ID, n.Request.Request.Path)
		}
	}
	text.Flow = out.String()
	return text, nil
}

// pace is the pause before each event of a run of the flow.
func (b *Backend) pace(flow model.ID) time.Duration {
	switch {
	case b.Pace != 0:
		return b.Pace
	case flow == FlowSoak:
		return 8 * time.Millisecond
	}
	return 120 * time.Millisecond
}

// Send answers with the canned response of the request: 200 with a body, 404 for the order that does not exist.
func (b *Backend) Send(_ context.Context, t flowi.RequestTarget, _ flowi.SendOptions) (vm.Sending, error) {
	rec, status := responseOf(t.Request)
	rec.Version = "1"
	rec.At = time.Now()
	result := engine.SendResult{RunID: "ui-dev", Status: status, Record: &rec, Seed: 1}
	if t.Request == "M-gql" { // a GraphQL server answers 200 with the failure in the body: the step fails all the same
		result.Status = engine.StatusFailed
		result.Diags = diag.List{diag.New(diag.CodeRunGraphQLError, diag.Location{}, map[string]string{"request": "graphql/get-user", "count": "1 error(s)", "detail": `Cannot query field "broken" on type "Query".`})}
	}
	events := []engine.Event{
		engine.RequestSent{RequestID: string(t.Request), Method: rec.Request.Method, URL: rec.Request.URL, Attempt: 1},
		engine.ResponseReceived{RequestID: string(t.Request), Status: rec.Status, Duration: rec.Timings.Total, Attempt: 1},
	}
	return &pacedSend{events: events, result: result, pause: b.pace("")}, nil
}

// TestConnection answers every kind with a canned verdict, so each way the Test button's result looks can be seen: an
// HTTP request that is refused, a file that reads, and anything else connecting.
func (b *Backend) TestConnection(_ context.Context, _ flowi.RequestTarget, v model.RequestVersion, _ flowi.SendOptions) (engine.Probe, error) {
	took := 42 * time.Millisecond
	switch {
	case v.File != nil:
		return engine.Probe{Status: engine.ProbeOK, Target: v.File.Path, Took: took, Detail: "the file exists and reads as CSV: 1 table (people), 3 rows"}, nil
	case v.GRPC != nil:
		return engine.Probe{Status: engine.ProbeOK, Target: v.GRPC.Server, Took: took, Detail: "connected to the server"}, nil
	case v.Async != nil || v.Dataset != nil:
		return engine.Probe{Status: engine.ProbeOK, Target: "broker", Took: took, Detail: "connected and signed in"}, nil
	case strings.Contains(strings.ToLower(v.URL), "orders"):
		return engine.Probe{Status: engine.ProbeFailed, Target: v.URL, Took: took, Detail: "401 Unauthorized: check the authentication"}, nil
	}
	return engine.Probe{Status: engine.ProbeOK, Target: v.URL, Took: took, Detail: "200 OK"}, nil
}

// RunFlow plays the canned run of the flow: every step passes, except the checkout's FailingStep, and the steps
// after a failed one do not run.
func (b *Backend) RunFlow(_ context.Context, t flowi.FlowTarget, o flowi.RunOptions) (vm.Running, error) {
	var flow model.Flow
	for _, f := range b.Collections()[0].Flows {
		if f.ID == t.Flow {
			flow = f
		}
	}
	events, result := runOf(flow, o.Env)
	return &pacedRun{events: events, result: result, pause: b.pace(t.Flow)}, nil
}

// responseOf is the canned record of a request.
func responseOf(id model.ID) (engine.Record, engine.Status) {
	rec := engine.Record{
		RequestID: id, Status: 200, Proto: "HTTP/2.0",
		Request: engine.SentRequest{Method: "GET", URL: "https://staging.shop.example/health"},
		Header:  []engine.Header{{Name: "Content-Type", Value: "application/json"}, {Name: "X-Request-Id", Value: "9f2c1b"}},
		Body:    engine.Body{JSON: map[string]any{"ok": true}, ContentType: "application/json", Size: 12},
		Timings: engine.Timings{TTFB: 38 * time.Millisecond, Total: 52 * time.Millisecond},
	}
	switch id {
	case "M-gqlsub": // a subscription is a stream: the subscribe message and the events that came back
		at := time.Now()
		rec.Kind, rec.Proto, rec.Outcome = engine.KindAsync, model.ProtocolGraphQLWS, engine.OutcomeOK
		rec.Status = 0
		rec.Request = engine.SentRequest{Method: "GQL", URL: "https://staging.shop.example/graphql"}
		rec.Messages = []engine.RecordMessage{
			{Direction: "sent", At: at, Type: "subscribe", Body: engine.Body{Text: `{"query":"subscription Ticks { tick }","operationName":"Ticks"}`, Size: 58}},
			{Direction: "received", At: at, Type: "next", Body: engine.Body{Text: `{"data":{"tick":{"n":1}}}`, ContentType: "application/json", JSON: map[string]any{"data": map[string]any{"tick": map[string]any{"n": 1}}}, Size: 25}},
			{Direction: "received", At: at, Type: "next", Body: engine.Body{Text: `{"data":{"tick":{"n":2}}}`, ContentType: "application/json", JSON: map[string]any{"data": map[string]any{"tick": map[string]any{"n": 2}}}, Size: 25}},
			{Direction: "received", At: at, Type: "next", Body: engine.Body{Text: `{"data":{"tick":{"n":3}}}`, ContentType: "application/json", JSON: map[string]any{"data": map[string]any{"tick": map[string]any{"n": 3}}}, Size: 25}},
		}
		rec.Body = rec.Messages[1].Body
	case "M-gql":
		rec.Request = engine.SentRequest{Method: "POST", URL: "https://staging.shop.example/graphql"}
		rec.Body = engine.Body{JSON: map[string]any{"errors": []any{map[string]any{"message": `Cannot query field "broken" on type "Query".`}}}, ContentType: "application/graphql-response+json", Size: 80}
	case "R-login":
		rec.Request = engine.SentRequest{Method: "POST", URL: "https://staging.shop.example/login"}
		rec.Body = engine.Body{JSON: map[string]any{"token": "demo-token", "expires_in": 3600}, ContentType: "application/json", Size: 44}
	case "R-orders":
		rec.Request.URL = "https://staging.shop.example/orders"
		rec.Body = engine.Body{JSON: map[string]any{"items": []any{map[string]any{"id": "o-1", "total": 19.9}, map[string]any{"id": "o-2", "total": 4.5}}}, ContentType: "application/json", Size: 80}
	case "R-create":
		rec.Status = 201
		rec.Request = engine.SentRequest{Method: "POST", URL: "https://staging.shop.example/orders"}
		rec.Body = engine.Body{JSON: map[string]any{"id": "o-3", "total": 39.8}, ContentType: "application/json", Size: 30}
	case "R-pay":
		rec.Status = 402
		rec.Request = engine.SentRequest{Method: "POST", URL: "https://staging.shop.example/orders/o-3/pay"}
		rec.Body = engine.Body{JSON: map[string]any{"error": "card_declined", "message": "Your card was declined."}, ContentType: "application/json", Size: 64}
	}
	status := engine.StatusPassed
	if rec.Status >= 400 {
		status = engine.StatusFailed
	}
	return rec, status
}

// runOf is the events and the result of a run of flow: its steps in order, the failing one failed, the rest skipped.
func runOf(flow model.Flow, env string) ([]engine.Event, engine.RunResult) {
	const runID = "ui-dev-run"
	at := time.Now()
	events := []engine.Event{engine.RunStarted{RunID: runID, Kind: "flow", FlowID: string(flow.ID), FlowPath: flow.Path, Env: env, At: at}}
	status := engine.StatusPassed
	for _, n := range flow.Nodes {
		if status != engine.StatusPassed {
			events = append(events, engine.NodeFinished{RunID: runID, Path: n.ID, Node: n.ID, Type: string(n.Type), Status: engine.StatusSkipped})
			continue
		}
		events = append(events, engine.NodeStarted{RunID: runID, Path: n.ID, Node: n.ID, Type: string(n.Type), At: at})
		done := engine.NodeFinished{RunID: runID, Path: n.ID, Node: n.ID, Type: string(n.Type), Status: engine.StatusPassed, Duration: 40 * time.Millisecond}
		if n.Request != nil {
			rec, st := responseOf(n.Request.Request.ID)
			rec.RunID, rec.At = runID, at
			events = append(events, engine.RecordCaptured{RunID: runID, Path: n.ID, Node: n.ID, Record: rec})
			done.Outputs = map[string]any{"status": rec.Status}
			if st != engine.StatusPassed {
				done.Status = engine.StatusFailed
			}
		}
		if done.Status == engine.StatusFailed {
			status = engine.StatusFailed
		}
		events = append(events, done)
	}
	events = append(events, engine.RunFinished{RunID: runID, Status: status, Duration: time.Since(at)})
	return events, engine.RunResult{RunID: runID, Status: status, Seed: 1}
}

// pacedRun replays events one at a time, so the canvas shows the steps going.
type pacedRun struct {
	events []engine.Event
	result engine.RunResult
	pause  time.Duration
}

func (r *pacedRun) Events() <-chan engine.Event { return paced(r.events, r.pause) }
func (r *pacedRun) Wait() engine.RunResult      { return r.result }

type pacedSend struct {
	events []engine.Event
	result engine.SendResult
	pause  time.Duration
}

func (s *pacedSend) Events() <-chan engine.Event { return paced(s.events, s.pause) }
func (s *pacedSend) Wait() engine.SendResult     { return s.result }

func paced(events []engine.Event, pause time.Duration) <-chan engine.Event {
	ch := make(chan engine.Event, 1)
	go func() {
		defer close(ch)
		for _, e := range events {
			time.Sleep(pause)
			ch <- e
		}
	}()
	return ch
}
