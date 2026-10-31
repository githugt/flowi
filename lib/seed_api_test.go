// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

func newSeedRequest(t *testing.T, w *Workspace, path string) RequestTarget {
	t.Helper()
	flow, _ := onboardingFlow(t, w)
	rt, err := w.NewRequest(flow.Collection, path)
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

func postHooks(t *testing.T, w *Workspace, rt RequestTarget) []model.HookStep {
	t.Helper()
	_, r, err := w.requestFor(rt)
	if err != nil {
		t.Fatal(err)
	}
	return r.Hooks.Post
}

func TestSeedRequestHooks_Given_the_default_setting_Then_the_request_logs_and_checks_the_response(t *testing.T) {
	w := demoWorkspace(t)
	rt := newSeedRequest(t, w, "seeded/ping")

	err := w.SeedRequestHooks(rt)

	if err != nil {
		t.Fatal(err)
	}
	post := postHooks(t, w, rt)
	if len(post) != 1 || len(post[0].Inline) != 2 || post[0].Inline[0].Type != "log" || post[0].Inline[1].Type != "assert" {
		t.Fatalf("post hooks %+v", post)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors after seeding: %v", diags)
	}
}

func TestSeedRequestHooks_Given_off_Then_nothing_is_written(t *testing.T) {
	w := demoWorkspace(t)
	rt := newSeedRequest(t, w, "seeded/quiet")
	if err := w.SetSetting(settings.LayerWorkspace, "requests.seed_blueprint", "off"); err != nil {
		t.Fatal(err)
	}

	err := w.SeedRequestHooks(rt)

	if err != nil || len(postHooks(t, w, rt)) != 0 {
		t.Fatalf("err %v, hooks %+v", err, postHooks(t, w, rt))
	}
}

func TestSeedRequestHooks_Given_post_hooks_already_Then_they_are_left_alone(t *testing.T) {
	w := demoWorkspace(t)
	rt := newSeedRequest(t, w, "seeded/own")
	_, r, _ := w.requestFor(rt)
	r.Hooks.Post = []model.HookStep{{Inline: []model.Node{{ID: "mine", Type: "log", Params: []model.Field{{Key: "value", Value: "x"}}}}}}
	if err := w.SaveRequest(rt, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	err := w.SeedRequestHooks(rt)

	post := postHooks(t, w, rt)
	if err != nil || len(post) != 1 || post[0].Inline[0].ID != "mine" {
		t.Fatalf("err %v, hooks %+v", err, post)
	}
}

func TestSeedRequestHooks_Given_a_named_blueprint_Then_its_nodes_are_the_hook(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	writeBlueprint(t, w, flow.Collection, "just-log", model.Blueprint{
		Meta: model.Meta{Name: "Just log"},
		Body: model.Flow{Nodes: []model.Node{{ID: "only", Type: "log", Params: []model.Field{{Key: "value", Value: "done"}}}}},
	})
	if err := w.SetSetting(settings.LayerWorkspace, "requests.seed_blueprint", "just-log"); err != nil {
		t.Fatal(err)
	}
	rt := newSeedRequest(t, w, "seeded/named")

	err := w.SeedRequestHooks(rt)

	post := postHooks(t, w, rt)
	if err != nil || len(post) != 1 || len(post[0].Inline) != 1 || post[0].Inline[0].ID != "only" {
		t.Fatalf("err %v, hooks %+v", err, post)
	}
}

func TestSeedRequestHooks_Given_an_unknown_blueprint_or_one_with_parameters_Then_it_says_so_and_writes_nothing(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	writeBlueprint(t, w, flow.Collection, "needs-a-request", model.Blueprint{
		Params: []model.Param{{Name: "target", Type: model.ParamRequestRef}},
		Body:   model.Flow{Nodes: []model.Node{{ID: "only", Type: "request", Request: &model.RequestCall{Request: model.Ref{Param: "target"}}}}},
	})
	for _, which := range []string{"no-such", "needs-a-request"} {
		if err := w.SetSetting(settings.LayerWorkspace, "requests.seed_blueprint", which); err != nil {
			t.Fatal(err)
		}
		rt := newSeedRequest(t, w, "seeded/"+which)

		err := w.SeedRequestHooks(rt)

		if err == nil || len(postHooks(t, w, rt)) != 0 {
			t.Fatalf("%s: err %v, hooks %+v", which, err, postHooks(t, w, rt))
		}
	}
}

// writeBlueprint stores bp as a blueprint file of the collection.
func writeBlueprint(t *testing.T, w *Workspace, collection model.ID, path string, bp model.Blueprint) {
	t.Helper()
	w.mu.RLock()
	c, _ := w.collection(collection)
	w.mu.RUnlock()
	bp.ID, bp.Path = w.newID(), path
	bp.File = filepath.Join(c.Root, "blueprints", path+blueprintSuffix)
	data, err := w.store.EncodeBlueprint(bp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(bp.File), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := w.writeAndReload(collection, bp.File, data, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
}

// seededSend seeds a new GET request for url and sends it.
func seededSend(t *testing.T, w *Workspace, path, url string) engine.SendResult {
	t.Helper()
	rt := newSeedRequest(t, w, path)
	_, r, _ := w.requestFor(rt)
	r.Versions[0].URL = url
	if err := w.SaveRequest(rt, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := w.SeedRequestHooks(rt); err != nil {
		t.Fatal(err)
	}
	s, err := w.SendRequest(context.Background(), rt, SendOptions{Env: "local"})
	if err != nil {
		t.Fatal(err)
	}
	return drain(s)
}

func TestSeedRequestHooks_Given_a_2xx_response_Then_the_seeded_request_passes(t *testing.T) {
	w := demoWorkspace(t)

	res := seededSend(t, w, "seeded/ok", "{{env.base_url}}/v1/config")

	if res.Status != engine.StatusPassed {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

func TestSeedRequestHooks_Given_a_non_2xx_response_Then_the_seeded_request_fails(t *testing.T) {
	w := demoWorkspace(t)

	res := seededSend(t, w, "seeded/missing", "{{env.base_url}}/v1/no-such-thing")

	if res.Status == engine.StatusPassed {
		t.Fatalf("a 404 passed: %+v", res.Record)
	}
}

func TestSeedRequestHooks_Given_a_pre_blueprint_and_the_default_Then_each_goes_to_its_own_phase(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	writeBlueprint(t, w, flow.Collection, "stamp", model.Blueprint{
		Meta: model.Meta{Name: "Stamp"}, Phase: "pre",
		Body: model.Flow{Nodes: []model.Node{{ID: "stamp", Type: "log", Params: []model.Field{{Key: "value", Value: "go"}}}}},
	})
	if err := w.SetSetting(settings.LayerWorkspace, "requests.seed_blueprint", "default, stamp"); err != nil {
		t.Fatal(err)
	}
	rt := newSeedRequest(t, w, "seeded/both")

	err := w.SeedRequestHooks(rt)

	_, r, _ := w.requestFor(rt)
	if err != nil || len(r.Hooks.Pre) != 1 || r.Hooks.Pre[0].Inline[0].ID != "stamp" || len(r.Hooks.Post) != 1 || r.Hooks.Post[0].Inline[0].ID != "response" {
		t.Fatalf("err %v hooks %+v", err, r.Hooks)
	}
}

func TestCreateHookBlueprint_should_store_the_nodes_with_their_phase_and_be_usable_for_seeding(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	nodes := []model.Node{{ID: "n1", Type: "log", Params: []model.Field{{Key: "value", Value: "hi"}}}}

	bt, err := w.CreateHookBlueprint(flow.Collection, "hooks/hello", "Hello", "pre", nodes)

	if err != nil {
		t.Fatal(err)
	}
	bp, _ := w.Blueprint(bt)
	if bp.Phase != "pre" || bp.Name != "Hello" || len(bp.Body.Nodes) != 1 || bp.Path != "hooks/hello" {
		t.Fatalf("blueprint %+v", bp)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors: %v", diags)
	}
}

func TestCreateHookBlueprint_Given_a_bad_phase_or_no_nodes_Then_refuses(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	node := []model.Node{{ID: "n1", Type: "log"}}

	_, badPhase := w.CreateHookBlueprint(flow.Collection, "x", "X", "middle", node)
	_, noNodes := w.CreateHookBlueprint(flow.Collection, "y", "Y", "post", nil)

	if badPhase == nil || noNodes == nil {
		t.Fatalf("bad phase %v, no nodes %v", badPhase, noNodes)
	}
}

func TestSeedRequestHooks_Given_off_inside_a_list_Then_the_other_entries_still_seed(t *testing.T) {
	w := demoWorkspace(t)
	rt := newSeedRequest(t, w, "seeded/mixed")
	if err := w.SetSetting(settings.LayerWorkspace, "requests.seed_blueprint", "off, default"); err != nil {
		t.Fatal(err)
	}

	err := w.SeedRequestHooks(rt)

	if err != nil || len(postHooks(t, w, rt)) != 1 {
		t.Fatalf("err %v hooks %+v", err, postHooks(t, w, rt))
	}
}

func TestSeedRequestHooks_Given_an_async_request_Then_it_logs_the_request_and_checks_the_outcome(t *testing.T) {
	w := demoWorkspace(t)
	rt := saveAsync(t, w, "seeded/events", &model.Async{Protocol: "nats", Server: "nats://h", Channel: "c", Send: &model.AsyncSend{}})

	err := w.SeedRequestHooks(rt)

	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(rt)
	pre, post := r.Hooks.Pre, r.Hooks.Post
	if len(pre) != 1 || len(pre[0].Inline) != 1 || pre[0].Inline[0].Type != "log" {
		t.Fatalf("pre hooks %+v", pre)
	}
	if len(post) != 1 || len(post[0].Inline) != 2 || post[0].Inline[0].Type != "log" || post[0].Inline[1].Type != "assert" {
		t.Fatalf("post hooks %+v", post)
	}
	check, _ := post[0].Inline[1].Params[0].Value.(string)
	if strings.Contains(check, "status") {
		t.Fatalf("an async request has no status, but the assert reads it: %q", check)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors after seeding: %v", diags)
	}
}

func TestSeedRequestHooks_Given_an_async_request_Then_the_seeded_hooks_pass_when_it_is_sent(t *testing.T) {
	w := demoWorkspace(t)
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(rw, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_, data, err := c.Read(r.Context())
		if err == nil {
			_ = c.Write(r.Context(), websocket.MessageText, data)
		}
		_, _, _ = c.Read(r.Context())
	}))
	t.Cleanup(srv.Close)
	rt := saveAsync(t, w, "seeded/echo", &model.Async{Protocol: "ws", Server: "ws" + strings.TrimPrefix(srv.URL, "http"),
		Send: &model.AsyncSend{Body: model.Body{Text: "hi"}}, Receive: &model.AsyncReceive{Timeout: 5 * time.Second}})
	if err := w.SeedRequestHooks(rt); err != nil {
		t.Fatal(err)
	}

	s, err := w.SendRequest(t.Context(), rt, SendOptions{Env: "local", Hooks: model.HookControl{Mode: "all"}})
	if err != nil {
		t.Fatal(err)
	}
	var logs []string
	for ev := range s.Events() {
		if l, ok := ev.(engine.Log); ok {
			logs = append(logs, l.Message)
		}
	}
	res := s.Wait()

	if res.Status != engine.StatusPassed || len(logs) != 2 {
		t.Fatalf("status %s diags %v logs %v", res.Status, res.Diags, logs)
	}
	if !strings.Contains(logs[0], "WS") || !strings.Contains(logs[1], "ok") {
		t.Fatalf("logs %v", logs)
	}
}

func TestSeedRequestHooks_Given_a_grpc_request_Then_it_logs_the_call_and_checks_the_code(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.NewRequest(w.Collections()[0].Manifest.ID, "seeded/grpc")
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	r.Versions = []model.RequestVersion{{Label: "1", GRPC: &model.GRPC{Server: "grpc://h:1", Method: "a.B/C"}}}
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := w.SeedRequestHooks(target); err != nil {
		t.Fatal(err)
	}

	_, r, _ = w.requestFor(target)
	pre, post := r.Hooks.Pre, r.Hooks.Post
	if len(pre) != 1 || len(pre[0].Inline) != 1 || pre[0].Inline[0].Type != "log" {
		t.Fatalf("pre hooks %+v", pre)
	}
	if len(post) != 1 || len(post[0].Inline) != 2 || post[0].Inline[1].Type != "assert" {
		t.Fatalf("post hooks %+v", post)
	}
	check, _ := post[0].Inline[1].Params[0].Value.(string)
	if !strings.Contains(check, "code") || strings.Contains(check, "status") {
		t.Fatalf("the assert must read the gRPC code, not an HTTP status: %q", check)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors after seeding: %v", diags)
	}
}

func TestSeedRequestHooks_Given_a_dataset_request_Then_it_logs_the_query_and_checks_the_outcome(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.NewRequest(w.Collections()[0].Manifest.ID, "seeded/query")
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	r.Versions = []model.RequestVersion{{Label: "1", Dataset: &model.Dataset{Driver: "sqlite", Server: "shop.db", Query: "SELECT 1"}}}
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := w.SeedRequestHooks(target); err != nil {
		t.Fatal(err)
	}

	_, r, _ = w.requestFor(target)
	post := r.Hooks.Post
	if len(post) != 1 || len(post[0].Inline) != 2 || post[0].Inline[1].Type != "assert" {
		t.Fatalf("post hooks %+v", post)
	}
	check, _ := post[0].Inline[1].Params[0].Value.(string)
	if !strings.Contains(check, "outcome") || strings.Contains(check, "status") {
		t.Fatalf("the assert must read the outcome, not an HTTP status: %q", check)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors after seeding: %v", diags)
	}
}

func openingOutcomes(t *testing.T, w *Workspace, rt RequestTarget) []model.Outcome {
	t.Helper()
	_, r, err := w.requestFor(rt)
	if err != nil {
		t.Fatal(err)
	}
	return r.Versions[0].Outcomes
}

// New requests used to start with no outcomes written, so success and failure were implied and could not be
// described or recoloured until the person wrote the list by hand.
func TestSeedRequestHooks_Given_the_default_setting_Then_the_request_starts_with_success_and_failure_described(t *testing.T) {
	w := demoWorkspace(t)
	rt := newSeedRequest(t, w, "seeded/outcomes")
	dataset := saveDataset(t, w, "seeded/rows")

	err := w.SeedRequestHooks(rt)
	derr := w.SeedRequestHooks(dataset)

	got := openingOutcomes(t, w, rt)
	rows := openingOutcomes(t, w, dataset)
	if err != nil || derr != nil || len(got) != 2 || got[0].Name != "success" || got[1].Name != "failure" || got[0].Description == "" || got[1].Description == "" {
		t.Fatalf("err %v/%v, http outcomes %+v", err, derr, got)
	}
	if len(rows) != 2 || !strings.Contains(rows[0].Description, "query") {
		t.Fatalf("dataset outcomes %+v: a query has its own words for ending well", rows)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors after seeding: %v", diags)
	}
}

func TestSeedRequestHooks_Given_a_blueprint_with_outcomes_Then_the_request_starts_with_them(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	writeBlueprint(t, w, flow.Collection, "with-outcomes", model.Blueprint{
		Meta:     model.Meta{Name: "With outcomes"},
		Outcomes: []model.Outcome{{Name: "success"}, {Name: "gone", When: "response.status == 410", Color: "amber", Description: "Removed"}, {Name: "failure"}},
		Body:     model.Flow{Nodes: []model.Node{{ID: "only", Type: "log", Params: []model.Field{{Key: "value", Value: "done"}}}}},
	})
	if err := w.SetSetting(settings.LayerWorkspace, "requests.seed_blueprint", "with-outcomes"); err != nil {
		t.Fatal(err)
	}
	rt := newSeedRequest(t, w, "seeded/blueprint-outcomes")

	err := w.SeedRequestHooks(rt)

	got := openingOutcomes(t, w, rt)
	if err != nil || len(got) != 3 || got[1].Name != "gone" || got[1].Color != "amber" || got[1].Description != "Removed" {
		t.Fatalf("err %v, outcomes %+v", err, got)
	}
}

func TestSeedRequestHooks_Given_outcomes_already_written_Then_they_are_left_alone(t *testing.T) {
	w := demoWorkspace(t)
	rt := newSeedRequest(t, w, "seeded/kept-outcomes")
	_, r, _ := w.requestFor(rt)
	r.Versions[0].Outcomes = []model.Outcome{{Name: "failure"}, {Name: "mine", When: "response.status == 1"}}
	if err := w.SaveRequest(rt, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	err := w.SeedRequestHooks(rt)

	got := openingOutcomes(t, w, rt)
	if err != nil || len(got) != 2 || got[1].Name != "mine" {
		t.Fatalf("err %v, outcomes %+v", err, got)
	}
}

func saveDataset(t *testing.T, w *Workspace, path string) RequestTarget {
	t.Helper()
	target, err := w.NewRequest(w.Collections()[0].Manifest.ID, path)
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	r.Versions = []model.RequestVersion{{Label: "1", Dataset: &model.Dataset{Driver: "sqlite", Server: "shop.db", Query: "SELECT 1"}}}
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return target
}
