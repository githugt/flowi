// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
	"github.com/githugt/flowi/lib/vault"
)

func echoed(t *testing.T, rec *Record) map[string]any {
	t.Helper()
	if rec == nil {
		t.Fatal("no record")
	}
	m, ok := rec.Body.JSON.(map[string]any)
	if !ok {
		t.Fatalf("body is not JSON: %+v", rec.Body)
	}
	return m
}

func echoedHeader(t *testing.T, rec *Record, name string) string {
	t.Helper()
	h, _ := echoed(t, rec)["headers"].(map[string]any)
	v, _ := h[name].(string)
	return v
}

func hasCode(l diag.List, c diag.Code) bool {
	for _, d := range l {
		if d.Code == c {
			return true
		}
	}
	return false
}

func TestSend_should_send_rendered_request_and_return_masked_record(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("auth/login", model.RequestVersion{
		Method: "POST", URL: "{{env.base_url}}/login",
		Body:    model.Body{JSON: `{"email":"{{email}}","password":"{{secret.password}}"}`},
		Extract: []model.Extraction{{Name: "token", From: "body", Path: "$.data.token"}},
		Assert:  []string{"status == 200"},
	})
	in := f.input(r)
	in.With = map[string]string{"email": "a@example.com"}

	res, _ := f.send(in)

	if res.Status != StatusPassed || res.Record == nil || res.Record.Status != 200 || res.Record.Outputs["token"] != "t-1" {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), secretV) {
		t.Fatalf("secret in result: %s", data)
	}
	if !strings.Contains(res.Record.Request.Body.Text, vault.Mask) {
		t.Fatalf("body not masked: %s", res.Record.Request.Body.Text)
	}
}

func TestSend_should_resolve_version_by_all_four_rules(t *testing.T) {
	tests := []struct {
		name, def, pinned, prefer string
		wantVersion, wantSource   string
	}{
		{"pinned", "1", "2", "", "2", "pinned"},
		{"preferred present", "1", "", "2", "2", "preferred"},
		{"preferred absent falls back to default", "1", "", "3", "1", "default"},
		{"no default means highest", "", "", "", "2", "highest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			r := f.addRequest("versions", get("/echo/v1"), get("/echo/v2"))
			f.update(func(c *model.Collection) { c.Requests[0].Default = tt.def })
			in := f.input(r)
			in.PinnedVersion, in.PreferVersion = tt.pinned, tt.prefer

			res, _ := f.send(in)

			if res.Record == nil || res.Record.Version != tt.wantVersion || res.Record.VersionSource != tt.wantSource {
				t.Fatalf("record %+v diags %v", res.Record, res.Diags)
			}
			if got := echoed(t, res.Record)["path"]; got != "/echo/v"+tt.wantVersion {
				t.Fatalf("sent to %v", got)
			}
		})
	}
}

func TestSend_Given_pinned_version_missing_Then_FLW_VER_004_and_nothing_sent(t *testing.T) {
	f := newFixture(t)
	in := f.input(f.addRequest("x", get("/echo/")))
	in.PinnedVersion = "9"

	res, _ := f.send(in)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeVersionPinnedAbsent) || f.hits.Load() != 0 {
		t.Fatalf("status %s diags %v hits %d", res.Status, res.Diags, f.hits.Load())
	}
}

func traceStep(label string) model.HookStep {
	return inlineHook(setNode("t-"+label, "vars.trace", `{{(vars.trace ?? "") + "`+label+`,"}}`))
}

func TestSend_should_run_hooks_in_order_and_expose_hook_vars(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Hooks = model.Hooks{Pre: []model.HookStep{traceStep("v-pre")}, Post: []model.HookStep{traceStep("v-post")}}
	r := f.addRequest("auth/login", v)
	f.update(func(c *model.Collection) {
		c.Manifest.Hooks = model.Hooks{Pre: []model.HookStep{traceStep("c-pre")}, Post: []model.HookStep{traceStep("c-post")}}
		c.Folders = []model.Folder{{Dir: "requests/auth", Hooks: model.Hooks{Pre: []model.HookStep{traceStep("f-pre")}, Post: []model.HookStep{traceStep("f-post")}}}}
		c.Requests[0].Hooks = model.Hooks{Pre: []model.HookStep{traceStep("r-pre")}, Post: []model.HookStep{traceStep("r-post")}}
	})

	res, events := f.send(f.input(r))

	want := "c-pre,f-pre,r-pre,v-pre,v-post,r-post,f-post,c-post,"
	if res.Status != StatusPassed || res.Vars["trace"] != want {
		t.Fatalf("status %s trace %v diags %v", res.Status, res.Vars["trace"], res.Diags)
	}
	started := 0
	for _, ev := range events {
		if _, ok := ev.(HookStarted); ok {
			started++
		}
	}
	if started != 8 {
		t.Fatalf("%d HookStarted events", started)
	}
}

func TestSend_Given_pre_hook_sets_var_used_by_header_Then_header_rendered_with_it(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Headers = []model.KeyValue{{Key: "Authorization", Value: "Bearer {{vars.token}}"}}
	v.Hooks.Pre = []model.HookStep{inlineHook(setNode("tok", "vars.token", "abc"))}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if got := echoedHeader(t, res.Record, "Authorization"); got != "Bearer abc" {
		t.Fatalf("server saw %q (diags %v)", got, res.Diags)
	}
}

func TestSend_Given_pre_hook_reads_request_body_Then_sees_rendered_body(t *testing.T) {
	f := newFixture(t)
	v := model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/echo/", Body: model.Body{JSON: `{"n":"{{$int(1, 1000000)}}","e":"{{$email}}"}`}}
	v.Hooks.Pre = []model.HookStep{inlineHook(setNode("sign", "request.headers.X-Sig", "{{request.body}}"))}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	body, _ := echoed(t, res.Record)["body"].(string)
	if sig := echoedHeader(t, res.Record, "X-Sig"); body == "" || sig != body {
		t.Fatalf("signature %q, body %q (diags %v)", sig, body, res.Diags)
	}
}

func TestSend_Given_pre_hook_overrides_Then_applied_on_top_of_render(t *testing.T) {
	f := newFixture(t)
	v := model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/echo/a", Body: model.Body{JSON: `{"user":{"name":"x"},"keep":1}`}}
	v.Hooks.Pre = []model.HookStep{inlineHook(
		setNode("u", "request.url", "{{env.base_url}}/echo/b"),
		setNode("q", "request.query.page", "2"),
		setNode("b", "request.body.user.name", "y"),
		setNode("c", "cookies.flavour", "oat"),
	)}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	e := echoed(t, res.Record)
	var sent map[string]any
	_ = json.Unmarshal([]byte(e["body"].(string)), &sent)
	if e["path"] != "/echo/b" || e["query"] != "page=2" || sent["user"].(map[string]any)["name"] != "y" || sent["keep"] != 1.0 {
		t.Fatalf("echo %v (diags %v)", e, res.Diags)
	}
	if echoedHeader(t, res.Record, "Cookie") != "flavour=oat" {
		t.Fatalf("cookie %q", echoedHeader(t, res.Record, "Cookie"))
	}
}

func TestSend_Given_pre_hook_assert_fails_Then_not_sent_and_failed(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Hooks.Pre = []model.HookStep{inlineHook(assertNode("never", "1 == 2"))}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if res.Status != StatusFailed || res.Record != nil || f.hits.Load() != 0 || !hasCode(res.Diags, diag.CodeRunAssertFailed) {
		t.Fatalf("status %s record %v hits %d diags %v", res.Status, res.Record, f.hits.Load(), res.Diags)
	}
}

func TestSend_Given_assertion_fails_Then_post_hooks_still_run_and_failed(t *testing.T) {
	f := newFixture(t)
	v := get("/status/500")
	v.Assert = []string{"status == 200"}
	v.Hooks.Post = []model.HookStep{inlineHook(setNode("p", "vars.post", "ran"))}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if res.Status != StatusFailed || res.Vars["post"] != "ran" || !hasCode(res.Diags, diag.CodeRunAssertFailed) {
		t.Fatalf("status %s vars %v diags %v", res.Status, res.Vars, res.Diags)
	}
}

func TestSend_Given_no_hooks_or_skip_post_Then_hooks_skipped(t *testing.T) {
	tests := []struct {
		ctl       model.HookControl
		pre, post bool
	}{
		{model.HookControl{Mode: "none"}, false, false},
		{model.HookControl{Skip: []string{"post"}}, true, false},
		{model.HookControl{Skip: []string{"pre"}}, false, true},
	}
	for _, tt := range tests {
		f := newFixture(t)
		v := get("/echo/")
		v.Hooks = model.Hooks{Pre: []model.HookStep{inlineHook(setNode("a", "vars.pre", "y"))}, Post: []model.HookStep{inlineHook(setNode("b", "vars.post", "y"))}}
		in := f.input(f.addRequest("x", v))
		in.Hooks = tt.ctl

		res, _ := f.send(in)

		if (res.Vars["pre"] == "y") != tt.pre || (res.Vars["post"] == "y") != tt.post {
			t.Errorf("%+v: vars %v", tt.ctl, res.Vars)
		}
	}
}

// hookFlowFixture: request "main" has pre-hook flow F, which calls request
// "config" and copies its token into vars.cfg. "config" has its own
// pre-hook (which sets X-Inner) and, when cyclic, also uses F.
func hookFlowFixture(t *testing.T, nested, cyclic bool) (*fixture, model.Request) {
	t.Helper()
	f := newFixture(t)
	cfgVersion := get("/config")
	cfg := f.addRequest("config", cfgVersion)
	flow := f.addFlow("hooks/fetch-config",
		model.Node{ID: "fetch", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: cfg.ID, Path: "config"}}},
		setNode("copy", "vars.cfg", "{{nodes.fetch.body.token}}"),
	)
	flowStep := model.HookStep{Flow: &model.Ref{ID: flow.ID, Path: flow.Path}}
	f.update(func(c *model.Collection) {
		pre := []model.HookStep{inlineHook(setNode("inner", "request.headers.X-Inner", "yes"))}
		if cyclic {
			pre = append(pre, flowStep)
		}
		c.Requests[0].Hooks.Pre = pre
		if nested {
			c.Requests[0].Versions[0].Settings = model.Settings{"hooks.nested": true, "hooks.max_depth": int64(2)}
		}
	})
	mainVersion := get("/echo/")
	mainVersion.Hooks.Pre = []model.HookStep{flowStep}
	return f, f.addRequest("main", mainVersion)
}

func TestSend_Given_hook_flow_with_request_node_Then_inner_request_runs_without_hooks(t *testing.T) {
	f, main := hookFlowFixture(t, false, false)

	res, _ := f.send(f.input(main))

	if res.Status != StatusPassed || res.Vars["cfg"] != "cfg-token" {
		t.Fatalf("status %s vars %v diags %v", res.Status, res.Vars, res.Diags)
	}
	if f.hits.Load() != 2 {
		t.Fatalf("hits %d", f.hits.Load())
	}
}

func TestSend_Given_nested_true_Then_inner_hooks_run_until_max_depth_then_FLW_RUN_006(t *testing.T) {
	f, main := hookFlowFixture(t, true, true)

	res, _ := f.send(f.input(main))

	if res.Status != StatusFailed || !hasCode(res.Diags, diag.CodeRunHookDepth) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

// Hook flows run on the flow scheduler, so every node type works in them
// (M2's serial runner refused anything but set/extract/assert/log with
// FLW-RUN-003).
func TestSend_Given_hook_flow_with_condition_and_parallel_Then_runs_through_scheduler(t *testing.T) {
	f := newFixture(t)
	cond := model.Node{ID: "pick", Type: "condition", Params: []model.Field{{Key: "check", Value: "env.base_url != ''"}},
		Then: []model.Node{setNode("yes", "vars.branch", "then")}}
	par := model.Node{ID: "both", Type: "parallel", Branches: []model.Branch{
		{ID: "a", Nodes: []model.Node{setNode("sa", "vars.a", "1")}},
		{ID: "b", Nodes: []model.Node{setNode("sb", "vars.b", "2")}},
	}}
	flow := f.addFlow("hooks/rich", cond, par)
	v := get("/echo/")
	v.Hooks.Pre = []model.HookStep{{Flow: &model.Ref{ID: flow.ID, Path: flow.Path}}}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if res.Status != StatusPassed || res.Vars["branch"] != "then" || res.Vars["a"] != "1" || res.Vars["b"] != "2" || f.hits.Load() != 1 {
		t.Fatalf("status %s vars %v diags %v", res.Status, res.Vars, res.Diags)
	}
}

func TestSend_hook_flow_events_carry_paths_and_depth(t *testing.T) {
	f := newFixture(t)
	cfg := f.addRequest("cfg", get("/config"))
	flow := f.addFlow("hooks/cfg", model.Node{ID: "fetch", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: cfg.ID, Path: cfg.Path}}})
	v := get("/echo/")
	v.Hooks.Pre = []model.HookStep{{Flow: &model.Ref{ID: flow.ID, Path: flow.Path}}}
	r := f.addRequest("x", v)

	res, events := f.send(f.input(r))

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	var hook HookStarted
	var node NodeStarted
	var inner RequestSent
	for _, ev := range events {
		switch e := ev.(type) {
		case HookStarted:
			hook = e
		case NodeStarted:
			node = e
		case RequestSent:
			if e.Depth > 0 {
				inner = e
			}
		}
	}
	if hook.Path != "" || hook.Index != 0 || hook.Depth != 0 || hook.Level != "version" {
		t.Fatalf("hook %+v", hook)
	}
	if node.Path != "pre:version:0/fetch" || node.Parent != "" || node.Lane != "pre:version:0" || node.Depth != 1 {
		t.Fatalf("node %+v", node)
	}
	if inner.Path != "pre:version:0/fetch" || inner.Depth != 1 || inner.VersionSource != "highest" {
		t.Fatalf("inner request %+v", inner)
	}
}

func TestSend_Given_two_missing_secrets_Then_two_FLW_SEC_001_and_nothing_sent(t *testing.T) {
	f := newFixture(t)
	f.vault = vault.New(vault.Options{File: filepath.Join(t.TempDir(), "empty.enc"), WorkspaceID: wsID, Keys: &memKeys{}})
	v := get("/echo/")
	v.Headers = []model.KeyValue{{Key: "X-A", Value: "{{secret.password}}"}, {Key: "X-B", Value: "{{secret.api_key}}"}}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	missing := 0
	for _, d := range res.Diags {
		if d.Code == diag.CodeSecretMissing && d.Args["env"] == "local" {
			missing++
		}
	}
	if res.Status != StatusError || missing != 2 || f.hits.Load() != 0 {
		t.Fatalf("status %s diags %v hits %d", res.Status, res.Diags, f.hits.Load())
	}
}

func TestSend_Given_FLOWI_SECRET_override_Then_used_without_vault_unlock(t *testing.T) {
	f := newFixture(t)
	f.environ = []string{"FLOWI_SECRET_LOCAL_PASSWORD=from-env"}
	f.vault = vault.New(vault.Options{File: filepath.Join(t.TempDir(), "none.enc"), WorkspaceID: wsID, Environ: f.environ})
	v := get("/echo/")
	v.Headers = []model.KeyValue{{Key: "X-A", Value: "{{secret.password}}"}}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if res.Status != StatusPassed || echoedHeader(t, res.Record, "X-A") != vault.Mask {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_server_echoes_secret_Then_record_body_masked(t *testing.T) {
	f := newFixture(t)
	v := get("/echo-secret")
	v.Headers = []model.KeyValue{{Key: "X-A", Value: "{{secret.password}}"}}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if res.Record == nil || strings.Contains(res.Record.Body.Text, secretV) || !strings.Contains(res.Record.Body.Text, vault.Mask) {
		t.Fatalf("body %+v diags %v", res.Record, res.Diags)
	}
}

func TestSend_should_persist_cookies_between_sends_of_same_collection(t *testing.T) {
	f := newFixture(t)
	set := f.addRequest("set", get("/cookie/set"))
	echo := f.addRequest("echo", get("/cookie/echo"))

	f.send(f.input(set))
	res, _ := f.send(f.input(echo))

	if res.Record == nil || res.Record.Body.Text != "s-1" {
		t.Fatalf("record %+v", res.Record)
	}
}

func TestSend_Given_save_policy_Then_writes_expected_files(t *testing.T) {
	tests := []struct {
		policy string
		files  []string
	}{
		{"none", nil},
		{"last", []string{"last.json"}},
		{"per-status", []string{"201.json", "last.json"}},
	}
	for _, tt := range tests {
		t.Run(tt.policy, func(t *testing.T) {
			f := newFixture(t)
			in := f.input(f.addRequest("x", get("/status/201")))
			in.Save = tt.policy

			res, _ := f.send(in)

			dir := filepath.Join(f.dataDir, string(collID), "responses", string(in.Request.ID), "1")
			entries, _ := os.ReadDir(dir)
			var got []string
			for _, e := range entries {
				got = append(got, e.Name())
			}
			if strings.Join(got, ",") != strings.Join(tt.files, ",") || len(res.Saved) != len(tt.files) {
				t.Fatalf("files %v saved %v", got, res.Saved)
			}
		})
	}
}

func TestSend_Given_non_utf8_and_invalid_json_and_204_Then_recorded_and_saved(t *testing.T) {
	tests := []struct {
		path  string
		check func(b Body) bool
	}{
		{"/binary", func(b Body) bool { return b.Base64 != "" && b.Text == "" }},
		{"/badjson", func(b Body) bool { return b.ParseErr != "" && b.Text == "{not json" && b.JSON == nil }},
		{"/nocontent", func(b Body) bool { return b.Size == 0 && b.Text == "" }},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			f := newFixture(t)

			res, _ := f.send(f.input(f.addRequest("x", get(tt.path))))

			if res.Record == nil || !tt.check(res.Record.Body) || len(res.Saved) != 1 {
				t.Fatalf("record %+v saved %v diags %v", res.Record, res.Saved, res.Diags)
			}
			data, err := os.ReadFile(res.Saved[0])
			var back Record
			if err != nil || json.Unmarshal(data, &back) != nil {
				t.Fatalf("saved file unreadable: %v", err)
			}
		})
	}
}

func TestSend_Given_cancel_mid_request_Then_cancelled_promptly_and_no_files_written(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("x", get("/slow"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := StartSend(ctx, f.deps(), f.input(r))
	var events []Event
	for ev := range s.Events() {
		events = append(events, ev)
		if _, sent := ev.(RequestSent); sent {
			cancel()
		}
	}
	start := time.Now()

	res := s.Wait()

	if res.Status != StatusCancelled || time.Since(start) > time.Second {
		t.Fatalf("status %s after %v", res.Status, time.Since(start))
	}
	if _, ok := events[len(events)-1].(RunCancelled); !ok {
		t.Fatalf("last event %T", events[len(events)-1])
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, string(collID), "responses")); !os.IsNotExist(err) {
		t.Fatalf("responses written: %v", err)
	}
}

func TestSend_Given_same_seed_Then_same_generated_body(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("x", model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/echo/", Body: model.Body{JSON: `{"e":"{{$email}}","n":{{$int(1, 1000000)}}}`}})
	in := f.input(r)
	in.Seed = 42

	a, _ := f.send(in)
	b, _ := f.send(in)
	in.Seed = 43
	c, _ := f.send(in)

	if a.Record.Request.Body.Text != b.Record.Request.Body.Text || a.Seed != 42 {
		t.Fatalf("seed 42 differs: %s vs %s", a.Record.Request.Body.Text, b.Record.Request.Body.Text)
	}
	if a.Record.Request.Body.Text == c.Record.Request.Body.Text {
		t.Fatal("seed 43 gave the same body")
	}
}

func TestSend_Given_no_seed_Then_random_seed_reported(t *testing.T) {
	f := newFixture(t)

	res, events := f.send(f.input(f.addRequest("x", get("/echo/"))))

	started, _ := events[0].(RunStarted)
	if res.Seed == 0 || started.Seed != res.Seed {
		t.Fatalf("seed %d, RunStarted %+v", res.Seed, events[0])
	}
}

func TestSend_should_emit_events_in_order(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Hooks = model.Hooks{Pre: []model.HookStep{inlineHook(setNode("a", "vars.x", "1"))}, Post: []model.HookStep{inlineHook(model.Node{ID: "l", Type: "log", Params: []model.Field{{Key: "value", Value: "done"}}})}}
	r := f.addRequest("x", v)

	_, events := f.send(f.input(r))

	var names []string
	for _, ev := range events {
		names = append(names, ev.EventName())
	}
	// Hook nodes run on the flow scheduler, so each reports NodeStarted and
	// NodeFinished inside its hook (added in M3).
	want := "RunStarted,HookStarted,NodeStarted,NodeFinished,HookFinished,RequestSent,ResponseReceived,HookStarted,NodeStarted,Log,NodeFinished,HookFinished,RunFinished"
	if strings.Join(names, ",") != want {
		t.Fatalf("events %v", names)
	}
}

func TestSend_Given_connection_refused_Then_error_with_FLW_HTTP_001(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("x", model.RequestVersion{Method: "GET", URL: "http://127.0.0.1:1/x"})

	res, _ := f.send(f.input(r))

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeHTTPFailed) || res.Record == nil {
		t.Fatalf("status %s diags %v record %v", res.Status, res.Diags, res.Record)
	}
}

func TestPinExample_should_write_status_headers_body_masked(t *testing.T) {
	f := newFixture(t)
	v := get("/echo-secret")
	v.Headers = []model.KeyValue{{Key: "X-A", Value: "{{secret.password}}"}}
	r := f.addRequest("auth/login", v)
	res, _ := f.send(f.input(r))
	reqFile := filepath.Join(t.TempDir(), "requests", "auth", "login.req.yaml")

	file, err := PinExample(*res.Record, reqFile, "")

	if err != nil || file != filepath.Join(filepath.Dir(reqFile), "login.examples", "200.json") {
		t.Fatalf("file %s err %v", file, err)
	}
	data, _ := os.ReadFile(file)
	var ex struct {
		Status int
		Body   string
	}
	if json.Unmarshal(data, &ex) != nil || ex.Status != 200 || strings.Contains(string(data), secretV) {
		t.Fatalf("example %s", data)
	}
}

func TestPinExample_should_record_the_version(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("auth/login", get("/echo-secret"))
	res, _ := f.send(f.input(r))
	reqFile := filepath.Join(t.TempDir(), "requests", "auth", "login.req.yaml")

	file, err := PinExample(*res.Record, reqFile, "")

	data, _ := os.ReadFile(file)
	var ex struct{ Version string }
	if err != nil || json.Unmarshal(data, &ex) != nil || ex.Version != res.Record.Version || ex.Version == "" {
		t.Fatalf("example %s err %v, want version %q", data, err, res.Record.Version)
	}
}

func TestSend_Given_missing_secret_read_by_pre_hook_view_Then_still_reported_and_not_sent(t *testing.T) {
	f := newFixture(t)
	f.vault = vault.New(vault.Options{File: filepath.Join(t.TempDir(), "empty.enc"), WorkspaceID: wsID, Keys: &memKeys{}})
	v := model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/echo/", Body: model.Body{JSON: `{"pw":"{{secret.password}}"}`}}
	v.Hooks.Pre = []model.HookStep{inlineHook(setNode("t", "vars.x", "1"))}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeSecretMissing) || f.hits.Load() != 0 {
		t.Fatalf("status %s diags %v hits %d", res.Status, res.Diags, f.hits.Load())
	}
}

// derivedLeaks reports where base64("user:"+secret) — a Basic credential
// built from the secret, whose base64 does not line up with the bare
// secret's — appears in a result, its events or its saved files.
func derivedLeaks(t *testing.T, res SendResult, events []Event) []string {
	t.Helper()
	derived := base64.StdEncoding.EncodeToString([]byte("user:" + secretV))
	var where []string
	if data, _ := json.Marshal(res); strings.Contains(string(data), derived) || strings.Contains(string(data), secretV) {
		where = append(where, "result")
	}
	if data, _ := json.Marshal(events); strings.Contains(string(data), derived) || strings.Contains(string(data), secretV) {
		where = append(where, "events")
	}
	for _, f := range res.Saved {
		if data, _ := os.ReadFile(f); strings.Contains(string(data), derived) {
			where = append(where, f)
		}
	}
	return where
}

func TestSend_Given_secret_derived_value_through_vars_Then_masked_everywhere(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Headers = []model.KeyValue{{Key: "Authorization", Value: "Basic {{vars.basic}}"}}
	v.Hooks.Pre = []model.HookStep{inlineHook(setNode("b", "vars.basic", `{{toBase64("user:" + secret.password)}}`))}
	r := f.addRequest("x", v)

	res, events := f.send(f.input(r))

	if res.Status != StatusPassed {
		t.Fatalf("status %s %v", res.Status, res.Diags)
	}
	if where := derivedLeaks(t, res, events); len(where) > 0 {
		t.Fatalf("derived credential leaked in %v", where)
	}
}

func TestSend_Given_secret_derived_header_override_Then_masked_everywhere(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Hooks.Pre = []model.HookStep{inlineHook(setNode("b", "request.headers.Authorization", `Basic {{toBase64("user:" + secret.password)}}`))}
	r := f.addRequest("x", v)

	res, events := f.send(f.input(r))

	if res.Status != StatusPassed {
		t.Fatalf("status %s %v", res.Status, res.Diags)
	}
	if where := derivedLeaks(t, res, events); len(where) > 0 {
		t.Fatalf("derived credential leaked in %v", where)
	}
}

func TestSend_Given_secret_derived_header_echoed_by_server_Then_masked(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Headers = []model.KeyValue{{Key: "Authorization", Value: `Basic {{toBase64("user:" + secret.password)}}`}}
	r := f.addRequest("x", v)

	res, events := f.send(f.input(r))

	if where := derivedLeaks(t, res, events); len(where) > 0 {
		t.Fatalf("derived credential leaked in %v", where)
	}
}

func TestSend_Given_log_of_secret_derived_value_Then_message_masked(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Hooks.Pre = []model.HookStep{inlineHook(model.Node{ID: "l", Type: "log", Params: []model.Field{{Key: "value", Value: `{{toBase64("user:" + secret.password)}}`}}})}
	r := f.addRequest("x", v)

	res, events := f.send(f.input(r))

	if where := derivedLeaks(t, res, events); len(where) > 0 {
		t.Fatalf("derived credential leaked in %v", where)
	}
}

func TestSend_Given_large_integers_in_json_Then_kept_exact_in_record_extracts_and_checks(t *testing.T) {
	f := newFixture(t)
	v := get("/bignum")
	v.Extract = []model.Extraction{{Name: "id", From: "body", Path: "$.id"}}
	v.Assert = []string{"response.body.id == 9007199254740993", "response.body.z == 1", "response.body.ratio < 1"}
	v.Hooks.Post = []model.HookStep{inlineHook(setNode("c", "vars.id", "{{response.body.id}}"))}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if res.Status != StatusPassed {
		t.Fatalf("status %s %v", res.Status, res.Diags)
	}
	data, _ := json.Marshal(res.Record.Body.JSON)
	if !strings.Contains(string(data), `"id":9007199254740993`) {
		t.Fatalf("record body %s", data)
	}
	if out, _ := json.Marshal(res.Record.Outputs["id"]); string(out) != "9007199254740993" {
		t.Fatalf("extracted %s", out)
	}
	if got := template.String(res.Vars["id"]); got != "9007199254740993" {
		t.Fatalf("var %s", got)
	}
}

func TestSend_Given_wrong_passphrase_Then_asked_once_per_run(t *testing.T) {
	f := newFixture(t)
	file := filepath.Join(t.TempDir(), "pw.enc")
	right := vault.New(vault.Options{File: file, WorkspaceID: wsID, Passphrase: func() (string, error) { return "right", nil }})
	if err := right.Set(vault.Entry{Collection: collID, Environment: envID, Name: "password"}, secretV); err != nil {
		t.Fatal(err)
	}
	prompts := 0
	f.vault = vault.New(vault.Options{File: file, WorkspaceID: wsID, Passphrase: func() (string, error) { prompts++; return "wrong", nil }})
	v := get("/echo/")
	v.Headers = []model.KeyValue{{Key: "X-A", Value: "{{secret.password}}"}}
	v.Hooks.Pre = []model.HookStep{inlineHook(setNode("a", "vars.a", "1"), setNode("b", "vars.b", "2"))}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if prompts != 1 || !hasCode(res.Diags, diag.CodeSecretLocked) || hasCode(res.Diags, diag.CodeTemplateUndefined) || f.hits.Load() != 0 {
		t.Fatalf("prompts %d diags %v hits %d", prompts, res.Diags, f.hits.Load())
	}
}

// Before: the request id and version label were joined into the data dir
// path as they were loaded, so an id like "../../x" saved outside it.
func TestSend_Given_a_request_id_that_is_not_a_plain_name_Then_nothing_is_saved_outside(t *testing.T) {
	f := newFixture(t)
	req := f.addRequest("x", get("/status/201"))
	req.ID = "../../../escaped"
	in := f.input(req)
	in.Save = "last"

	res, _ := f.send(in)

	outside := filepath.Join(f.dataDir, "..", "escaped")
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("a response was saved outside the data dir: %v", err)
	}
	if len(res.Saved) != 0 || res.Record == nil {
		t.Fatalf("saved %v record %v", res.Saved, res.Record)
	}
}

func TestAutoPin_should_follow_the_responses_pin_setting(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("auth/login", get("/echo-secret"))
	res, _ := f.send(f.input(r))
	reqFile := filepath.Join(t.TempDir(), "requests", "auth", "login.req.yaml")
	example := filepath.Join(filepath.Dir(reqFile), "login.examples", "200.json")
	newer := *res.Record
	newer.Body.JSON = map[string]any{"newer": true}

	offFile, offErr := autoPin(res.Record, reqFile, "off", vault.NewMasker())
	_, offStat := os.Stat(example)
	firstFile, firstErr := autoPin(res.Record, reqFile, "first", vault.NewMasker())
	keptFile, keptErr := autoPin(&newer, reqFile, "first", vault.NewMasker())
	afterFirst, _ := os.ReadFile(example)
	lastFile, lastErr := autoPin(&newer, reqFile, "last", vault.NewMasker())
	afterLast, _ := os.ReadFile(example)

	if offFile != "" || offErr != nil || !os.IsNotExist(offStat) {
		t.Fatalf("off pinned %q %v %v", offFile, offErr, offStat)
	}
	if firstFile != example || firstErr != nil || keptFile != "" || keptErr != nil || strings.Contains(string(afterFirst), "newer") {
		t.Fatalf("first: %q %v, second %q %v, file %s", firstFile, firstErr, keptFile, keptErr, afterFirst)
	}
	if lastFile != example || lastErr != nil || !strings.Contains(string(afterLast), "newer") {
		t.Fatalf("last: %q %v, file %s", lastFile, lastErr, afterLast)
	}
}

func TestAutoPin_Given_a_record_without_a_status_Then_pins_nothing(t *testing.T) {
	reqFile := filepath.Join(t.TempDir(), "requests", "x.req.yaml")

	file, err := autoPin(&Record{}, reqFile, "last", vault.NewMasker())

	if file != "" || err != nil {
		t.Fatalf("pinned %q %v", file, err)
	}
}

func TestSend_Given_overlay_adds_a_hook_flow_Then_the_hook_runs_for_that_send_only(t *testing.T) {
	// Arrange
	f, _ := hookFlowFixture(t, false, false)
	flow := f.coll().Flows[0]
	plain := f.addRequest("plain", get("/echo/"))
	add := []model.OverlayOp{{Verb: model.OverlayAdd, Path: "hooks.pre", Value: string(flow.ID)}}
	withOverlay := f.input(plain)
	withOverlay.Overlay = &Overlay{Name: "configured", Ops: add}

	// Act
	laid, _ := f.send(withOverlay)
	bare, _ := f.send(f.input(plain))

	// Assert
	if laid.Status != StatusPassed || laid.Vars["cfg"] != "cfg-token" {
		t.Fatalf("status %s vars %v diags %v", laid.Status, laid.Vars, laid.Diags)
	}
	if _, ran := bare.Vars["cfg"]; ran {
		t.Fatal("the hook must not run without the overlay")
	}
}

func TestSend_should_read_collection_variables_and_let_the_environment_override_one_of_the_same_name(t *testing.T) {
	// Arrange
	f := newFixture(t)
	v := get("/echo/")
	v.Headers = []model.KeyValue{{Key: "X-Region", Value: "{{collection_var.region}}"}, {Key: "X-Plan", Value: "{{collection_var.plan}}"}}
	r := f.addRequest("echo", v)
	f.update(func(c *model.Collection) {
		c.Manifest.Variables = []model.KeyValue{{Key: "region", Value: "eu"}, {Key: "plan", Value: "free"}}
		c.Environments[0].Vars = append(c.Environments[0].Vars, model.EnvVar{Name: "plan", Value: "gold"})
	})

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if got := f.lastEcho()["X-Region"]; got != "eu" {
		t.Errorf("X-Region = %q: a collection variable is its own value", got)
	}
	if got := f.lastEcho()["X-Plan"]; got != "gold" {
		t.Errorf("X-Plan = %q: the environment's variable of the same name wins", got)
	}
}

func TestRun_should_read_collection_variables_in_a_flow(t *testing.T) {
	f := newFixture(t)
	f.addFlow("uses-collection-var", assertNode("a", `collection_var.region == "eu"`))
	f.update(func(c *model.Collection) { c.Manifest.Variables = []model.KeyValue{{Key: "region", Value: "eu"}} })
	fl := f.coll().Flows[0]

	_, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	if res.Status != StatusPassed {
		t.Fatalf("run %s (%v)", res.Status, res.Diags)
	}
}
