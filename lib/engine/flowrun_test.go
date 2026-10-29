// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/examples"
	"github.com/githugt/flowi-lib/examples/demoserver"
	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/store"
	"github.com/githugt/flowi-lib/vault"
)

// exampleCollection loads the embedded user-creation example with its
// local environment pointed at baseURL.
func exampleCollection(t *testing.T, baseURL string) model.Collection {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, examples.UserCreation()); err != nil {
		t.Fatal(err)
	}
	c, diags, err := store.New(yamlcodec.New(settings.Schema())).Load(dir)
	if err != nil || diags.HasErrors() {
		t.Fatalf("loading example: %v %v", err, diags)
	}
	for i, e := range c.Environments {
		for j, v := range e.Vars {
			if v.Name == "base_url" {
				c.Environments[i].Vars[j].Value = baseURL
			}
		}
	}
	return c
}

func flowByPath(t *testing.T, c model.Collection, p string) model.Flow {
	t.Helper()
	for _, f := range c.Flows {
		if f.Path == p {
			return f
		}
	}
	t.Fatalf("no flow %s", p)
	return model.Flow{}
}

func TestRunFlow_user_creation_should_pass_against_demo_server(t *testing.T) {
	srv := httptest.NewServer(demoserver.Handler(demoserver.Options{Password: "pw"}))
	t.Cleanup(srv.Close)
	f := newFixture(t)
	c := exampleCollection(t, srv.URL)
	f.lookup.collections = []model.Collection{c}
	f.environ = []string{"FLOWI_SECRET_LOCAL_PASSWORD=pw"}
	f.vault = vault.New(vault.Options{File: filepath.Join(t.TempDir(), "v.enc"), WorkspaceID: wsID, Keys: &memKeys{}, Environ: f.environ})
	clock := newFakeClock()
	d := f.deps()
	d.After = clock.After
	fl := flowByPath(t, c, "user-creation")

	run := StartRun(testCtx(t), d, RunInput{Collection: c, Flow: fl, Env: "local"})
	go func() {
		clock.WaitForTimers(t, 1) // the wait-for-code delay
		clock.Advance(time.Minute)
	}()
	var events []Event
	for ev := range run.Events() {
		events = append(events, ev)
	}
	res := run.Wait()

	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
	verify, _ := finishedNode(events, "verify")
	if verify.Outputs["status"] != 204 {
		t.Fatalf("verify %+v", verify)
	}
	for _, ev := range events {
		if rs, ok := ev.(RequestSent); ok && rs.Path == "login" && (rs.Version != "2" || rs.VersionSource != "preferred") {
			t.Fatalf("login used %s (%s)", rs.Version, rs.VersionSource)
		}
	}
}

func inputFlow(f *fixture, name string, inputs ...model.Input) model.Flow {
	fl := f.addFlow(name, flowNode("show", "note", "text", "{{inputs.email}}"))
	fl.Inputs = inputs
	return fl
}

func TestRunFlow_inputs(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []model.Input
		given    map[string]string
		wantText string
		wantCode diag.Code
	}{
		{name: "given_overrides_default", inputs: []model.Input{{Name: "email", Default: "d@x"}}, given: map[string]string{"email": "{{'g' + '@x'}}"}, wantText: "g@x"},
		{name: "default_template", inputs: []model.Input{{Name: "email", Default: "{{'d' + '@x'}}"}}, wantText: "d@x"},
		{name: "missing", inputs: []model.Input{{Name: "email"}}, wantCode: diag.CodeRunInputMissing},
		{name: "undeclared", inputs: []model.Input{{Name: "email", Default: "d@x"}}, given: map[string]string{"nope": "1"}, wantCode: diag.CodeRunInputUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			in := f.runInput(inputFlow(f, tt.name, tt.inputs...))
			in.Inputs = tt.given

			events, res := runFlow(testCtx(t), t, f.deps(), in)

			if tt.wantCode != "" {
				if res.Status != StatusFailed || !hasCode(res.Diags, tt.wantCode) {
					t.Fatalf("result %+v", res)
				}
				return
			}
			show, _ := finishedNode(events, "show")
			if res.Status != StatusPassed || show.Outputs["text"] != tt.wantText {
				t.Fatalf("result %+v show %+v", res, show)
			}
		})
	}
}

func TestRunFlow_Given_run_prefer_version_Then_beats_flow_prefer_version(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("v", get("/echo/1"), get("/echo/2"), get("/echo/3"))
	f.update(func(c *model.Collection) { c.Requests[0].Default = "1" })
	fl := f.addFlow("prefer", requestNode("call", r))
	fl.PreferVersion = "2"
	in := f.runInput(fl)
	in.PreferVersion = "3"

	events, res := runFlow(testCtx(t), t, f.deps(), in)

	for _, ev := range events {
		if rs, ok := ev.(RequestSent); ok && rs.Version != "3" {
			t.Fatalf("sent version %s", rs.Version)
		}
	}
	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
}

func TestRunFlow_flow_level_run_till_should_repeat_until_and_fail_on_limit(t *testing.T) {
	tests := []struct {
		name       string
		until      string
		max        int
		wantStatus Status
		wantCalls  int
	}{
		{name: "until_holds", until: "nodes.poll.body.n == 3", max: 10, wantStatus: StatusPassed, wantCalls: 3},
		{name: "limit", until: "nodes.poll.body.n == 99", max: 4, wantStatus: StatusFailed, wantCalls: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			fl := f.addFlow("poll-"+tt.name, flowNode("poll", "fetch", "url", "{{env.base_url}}/counter/"+tt.name))
			fl.Run = &model.RunTill{Until: tt.until, MaxIterations: tt.max}

			events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

			if res.Status != tt.wantStatus || f.count(tt.name)-1 != tt.wantCalls {
				t.Fatalf("result %+v calls %d", res, f.count(tt.name)-1)
			}
			if tt.wantStatus == StatusFailed && !hasCode(res.Diags, diag.CodeRunTillLimit) {
				t.Fatalf("diags %v", res.Diags)
			}
			if _, ok := finishedNode(events, "[1]/poll"); !ok {
				t.Fatal("iterations must be namespaced [i]/")
			}
		})
	}
}

func TestSubflow_should_map_inputs_and_outputs_and_isolate_vars(t *testing.T) {
	f := newFixture(t)
	sub := f.addFlow("shared/greet", setNode("inner", "vars.secret_inner", "{{inputs.who}}!"), flowNode("say", "note", "text", "hi {{inputs.who}}"))
	sub.Inputs = []model.Input{{Name: "who"}, {Name: "greeting", Default: "hello"}}
	sub.Outputs = []model.Output{{Name: "text", Value: "{{inputs.greeting}} {{vars.secret_inner}}"}, {Name: "said", Value: "{{nodes.say.text}}"}}
	f.update(func(c *model.Collection) { c.Flows[len(c.Flows)-1] = sub })
	call := model.Node{ID: "greet", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: sub.ID, Path: sub.Path},
		With: []model.KeyValue{{Key: "who", Value: "Ada"}}}}
	fl := f.addFlow("outer", call, flowNode("after", "note", "text", "{{nodes.greet.text}}|{{vars.secret_inner ?? 'isolated'}}"))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	after, _ := finishedNode(events, "after")
	inner, _ := finishedNode(events, "greet/say")
	if res.Status != StatusPassed || after.Outputs["text"] != "hello Ada!|isolated" || inner.Parent != "greet" {
		t.Fatalf("result %+v after %+v inner %+v", res, after, inner)
	}
}

func TestSubflow_Given_missing_input_Then_FLW_RUN_009(t *testing.T) {
	f := newFixture(t)
	sub := f.addFlow("shared/need", flowNode("n", "note", "text", "x"))
	sub.Inputs = []model.Input{{Name: "who"}}
	f.update(func(c *model.Collection) { c.Flows[len(c.Flows)-1] = sub })
	fl := f.addFlow("outer", model.Node{ID: "s", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: sub.ID}}})

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	s, _ := finishedNode(events, "s")
	if res.Status != StatusFailed || !hasCode(s.Diags, diag.CodeRunInputMissing) {
		t.Fatalf("result %+v node %+v", res, s)
	}
}

func TestFetch_and_webhook_should_call_and_output(t *testing.T) {
	f := newFixture(t)
	fetch := flowNode("get", "fetch", "url", "{{env.base_url}}/echo/q", "query", []model.Field{{Key: "a", Value: "1"}},
		"transform", "{{response.body.path}}")
	hook := flowNode("post", "webhook", "url", "{{env.base_url}}/echo/hook", "payload", []model.Field{{Key: "ok", Value: true}})
	fl := f.addFlow("calls", fetch, hook, flowNode("check", "assert", "check", "nodes.get.value == '/echo/q' && nodes.get.status == 200 && nodes.post.body.body == '{\"ok\":true}'"))
	fl.Execution.RateLimit = "1000/s"

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
	var sent int
	for _, ev := range events {
		if rs, ok := ev.(RequestSent); ok && (rs.Path == "get" || rs.Path == "post") {
			sent++
		}
	}
	if sent != 2 {
		t.Fatalf("RequestSent for fetch/webhook: %d", sent)
	}
}

func TestWebhook_background_failure_Then_FLW_RUN_016_warning_and_run_passes(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("bg", flowNode("ping", "webhook", "url", "http://127.0.0.1:1/x", "wait", false), flowNode("after", "note", "text", "x"))

	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	if res.Status != StatusPassed || !hasCode(res.Diags, diag.CodeRunBackground) {
		t.Fatalf("result %+v", res)
	}
}

func TestWriteFile_Given_hostile_names_Then_FLW_RUN_017_and_nothing_written(t *testing.T) {
	for _, name := range []string{"../x", "/etc/x", "a/b", ".hidden", ""} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			fl := f.addFlow("save", flowNode("s", "save", "key", name, "value", "{{1}}"))

			events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

			s, _ := finishedNode(events, "s")
			if res.Status != StatusFailed || !hasCode(s.Diags, diag.CodeRunFileName) || len(res.Saved) != 0 {
				t.Fatalf("result %+v node %+v", res, s)
			}
			_ = filepath.WalkDir(f.dataDir, func(p string, d os.DirEntry, _ error) error {
				if d != nil && !d.IsDir() && !strings.Contains(p, string(filepath.Separator)+"runs"+string(filepath.Separator)) {
					t.Errorf("wrote %s", p) // only the run report may exist
				}
				return nil
			})
		})
	}
}

func TestWriteFile_should_write_saved_values_under_flow_dir(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("save-ok", flowNode("s", "save", "key", "resp", "value", "{{ {'status': 201} }}", "mode", "latest-by-status"))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	s, _ := finishedNode(events, "s")
	want := filepath.Join(f.dataDir, string(collID), "saved", string(fl.ID), "resp", "201.json")
	if res.Status != StatusPassed || s.Outputs["file"] != want || len(res.Saved) != 1 {
		t.Fatalf("result %+v node %+v", res, s)
	}
	if data, err := os.ReadFile(want); err != nil || !strings.Contains(string(data), "201") {
		t.Fatalf("file %q %v", data, err)
	}
}

func TestLanes_Given_cookies_isolated_shared_off_Then_jars_behave(t *testing.T) {
	tests := []struct {
		policy    string
		wantLane  string // what the branch's cookie echo returns
		wantAfter string // what the root sees after the parallel
	}{
		{policy: "isolated", wantLane: "lane", wantAfter: "root"},
		{policy: "shared", wantLane: "lane", wantAfter: "lane"},
		{policy: "off", wantLane: "", wantAfter: "root"},
	}
	for _, tt := range tests {
		t.Run(tt.policy, func(t *testing.T) {
			f := newFixture(t)
			f.update(func(c *model.Collection) { c.Manifest.Settings = model.Settings{"http.cookies": tt.policy} })
			fetch := func(id, p string) model.Node { return flowNode(id, "fetch", "url", "{{env.base_url}}"+p) }
			par := model.Node{ID: "par", Type: "parallel", Branches: []model.Branch{
				{ID: "a", Nodes: []model.Node{fetch("set", "/cookie/set/lane"), fetch("seen", "/cookie/echo")}},
				{ID: "b", Nodes: []model.Node{flowNode("idle", "note", "text", "x")}},
			}}
			fl := f.addFlow("jars-"+tt.policy, fetch("root", "/cookie/set/root"), par, fetch("after", "/cookie/echo"),
				flowNode("check", "assert", "check", "nodes.seen.body == '"+tt.wantLane+"' && nodes.after.body == '"+tt.wantAfter+"'"))

			_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

			if res.Status != StatusPassed {
				t.Fatalf("result %+v", res)
			}
		})
	}
}

var _ = context.Background

func TestRunFlow_onboarding_check_should_pass_against_demo_server(t *testing.T) {
	srv := httptest.NewServer(demoserver.Handler(demoserver.Options{Password: "pw"}))
	t.Cleanup(srv.Close)
	f := newFixture(t)
	c := exampleCollection(t, srv.URL)
	f.lookup.collections = []model.Collection{c}
	f.environ = []string{"FLOWI_SECRET_LOCAL_PASSWORD=pw"}
	f.vault = vault.New(vault.Options{File: filepath.Join(t.TempDir(), "v.enc"), WorkspaceID: wsID, Keys: &memKeys{}, Environ: f.environ})
	fl := flowByPath(t, c, "onboarding-check")

	events, res := runFlow(testCtx(t), t, f.deps(), RunInput{Collection: c, Flow: fl, Env: "local"})

	if res.Status != StatusPassed {
		for _, ev := range events {
			if nf, ok := ev.(NodeFinished); ok && nf.Status != StatusPassed {
				t.Logf("%s %s %v", nf.Path, nf.Status, nf.Diags)
			}
		}
		t.Fatalf("result %+v", res)
	}
	poll, _ := finishedNode(events, "poll")
	digest, ok := lastEvent[Digest](events)
	if poll.Outputs["iterations"] != 3 || !ok || !strings.Contains(digest.Text, "Onboarding check") || len(res.Saved) == 0 {
		t.Fatalf("poll %+v digest %v saved %v", poll, ok, res.Saved)
	}
}
