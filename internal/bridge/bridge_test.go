// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"fmt"
	"log/slog"
	"maps"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/bridge"
	"github.com/githugt/flowi/internal/canvas"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

type published struct {
	names []string
	last  map[string]any
}

func (p *published) emit(name string, data any) {
	p.names = append(p.names, name)
	if p.last == nil {
		p.last = map[string]any{}
	}
	p.last[name] = data
}

func fixture(t *testing.T) (*bridge.App, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Signup"}, Path: "signup", File: "/c/flows/signup.arazzo.yaml", Nodes: []model.Node{
		{ID: "a", Type: "set", Params: []model.Field{{Key: "target", Value: "vars.x"}, {Key: "value", Value: "1"}}},
		{ID: "b", Type: "log", Params: []model.Field{{Key: "value", Value: "hi"}}},
	}}}
	pub := &published{}
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), pub.emit)
	return app, pub
}

func findNode(nodes []bridge.TreeNode, kind string) (bridge.TreeNode, bool) {
	for _, n := range nodes {
		if n.Kind == kind {
			return n, true
		}
		if c, ok := findNode(n.Children, kind); ok {
			return c, true
		}
	}
	return bridge.TreeNode{}, false
}

func TestWorkspaceService_Tree_should_list_collections_with_their_flows(t *testing.T) {
	app, pub := fixture(t)

	app.Start(t.Context())
	tree := app.Workspace().Tree()

	flow, ok := findNode(tree.Roots, "flow")
	if !ok || flow.Label != "signup" {
		t.Fatalf("no flow node in %+v", tree.Roots)
	}
	if !slices.Contains(pub.names, "workspace") {
		t.Fatalf("no workspace event, got %v", pub.names)
	}
}

func openSignup(t *testing.T) (*bridge.FlowService, bridge.FlowView, *published) {
	t.Helper()
	app, pub := fixture(t)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	flows := app.Flows()
	v, err := flows.Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	return flows, v, pub
}

func TestFlowService_Open_should_show_cards_for_each_unit(t *testing.T) {
	_, v, _ := openSignup(t)

	if v.Name != "Signup" || len(v.Diagram.Units) == 0 {
		t.Fatalf("view %+v", v)
	}
	u := v.Diagram.Units[0]
	if v.Cards[u.ID].Title == "" {
		t.Fatalf("no card for %s: %+v", u.ID, v.Cards)
	}
}

func TestFlowService_Insert_then_Undo_should_publish_each_state(t *testing.T) {
	flows, v, pub := openSignup(t)

	id := flows.Insert(v.Node, "log", "$.nodes", 1)
	inserted := pub.last["flow"].(bridge.FlowView)
	flows.Undo(v.Node)
	undone := pub.last["flow"].(bridge.FlowView)

	if id == "" || !inserted.Dirty || !inserted.CanUndo || inserted.Cards["a"].Title != "a · log · b" || undone.Cards["a"].Title != "a · b" {
		t.Fatalf("inserted %q: %+v / undone %+v", id, inserted, undone)
	}
}

func TestFlowService_Open_should_refuse_a_node_that_is_not_a_flow(t *testing.T) {
	app, _ := fixture(t)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")

	_, err := app.Flows().Open(req.ID)

	if err == nil {
		t.Fatal("opened a request as a flow")
	}
}

func lastView(pub *published) bridge.FlowView { return pub.last["flow"].(bridge.FlowView) }

func TestFlowService_Select_should_show_the_step_in_the_inspector(t *testing.T) {
	flows, v, pub := openSignup(t)

	flows.Select(v.Node, "b")
	got := lastView(pub)

	if got.Selected != "b" || got.Form.Step == nil || got.Form.Step.Type != "log" || got.Cards["a"].Title != "a · [b]" {
		t.Fatalf("selected %q, form %+v, card %q", got.Selected, got.Form.Step, got.Cards["a"].Title)
	}
}

func TestFlowService_SetText_then_SetParam_should_edit_the_selected_step(t *testing.T) {
	flows, v, pub := openSignup(t)
	flows.Select(v.Node, "b")

	err := flows.SetText(v.Node, "name", "Say hi")
	flows.SetParam(v.Node, "value", "hello")
	step := lastView(pub).Form.Step

	if err != nil || step.Name != "Say hi" {
		t.Fatalf("name %q, err %v", step.Name, err)
	}
	var value string
	for _, p := range step.Params {
		if p.Name == "value" {
			value = p.Value
		}
	}
	if value != "hello" {
		t.Fatalf("params %+v", step.Params)
	}
}

func TestFlowService_SetText_should_refuse_a_bad_timeout(t *testing.T) {
	flows, v, _ := openSignup(t)
	flows.Select(v.Node, "b")

	err := flows.SetText(v.Node, "timeout", "soon")

	if err == nil {
		t.Fatal("accepted a timeout that is not a duration")
	}
}

func TestFlowService_AddStep_should_append_and_select_when_not_placed(t *testing.T) {
	flows, v, pub := openSignup(t)

	id := flows.AddStep(v.Node, "log", 0, 0, 0, false)
	got := lastView(pub)

	if id == "" || got.Selected != id || got.Cards["a"].Title != "a · b · ["+id+"]" {
		t.Fatalf("id %q, selected %q, title %q", id, got.Selected, got.Cards["a"].Title)
	}
}

// Behaviour change: a step added from the palette used to go to the end of the
// flow even with another step selected; it goes after the selected one.
func TestFlowService_AddStep_should_go_after_the_selected_step(t *testing.T) {
	flows, v, pub := openSignup(t)
	flows.Select(v.Node, "a")

	id := flows.AddStep(v.Node, "log", 0, 0, 0, false)
	got := lastView(pub)

	if want := "a · [" + id + "] · b"; got.Cards["a"].Title != want {
		t.Fatalf("title %q, want %q", got.Cards["a"].Title, want)
	}
}

func TestFlowService_AddStepNear_should_add_on_the_side_of_the_step_it_was_asked_for(t *testing.T) {
	tests := []struct {
		name   string
		step   string
		before bool
		want   string // the strip's title, with %s for the new step
	}{
		{name: "before the first", step: "a", before: true, want: "[%s] · a · b"},
		{name: "after the first", step: "a", want: "a · [%s] · b"},
		{name: "before the last", step: "b", before: true, want: "a · [%s] · b"},
		{name: "after the last", step: "b", want: "a · b · [%s]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flows, v, pub := openSignup(t)

			id := flows.AddStepNear(v.Node, "log", tt.step, tt.before)
			got := lastView(pub)

			title := ""
			for _, c := range got.Cards {
				if strings.Contains(c.Title, "["+id+"]") {
					title = c.Title
				}
			}
			if want := fmt.Sprintf(tt.want, id); title != want || got.Selected != id {
				t.Fatalf("title %q selected %q, want %q selected %q", title, got.Selected, want, id)
			}
		})
	}
}

func TestFlowService_Drop_should_move_a_lifted_card_to_the_slot_under_the_pointer(t *testing.T) {
	flows, v, pub := openSignup(t)
	u := v.Diagram.Units[0] // the strip a · b

	// Lift "b" and drop it before the first card.
	flows.Drop(v.Node, u.ID, "b", true, u.X+5, u.Y+u.H/2, 24, 0, 0)
	got := lastView(pub)

	if got.Cards["b"].Title != "b · a" {
		t.Fatalf("cards %+v", got.Cards)
	}
}

func openRunnable(t *testing.T, events []engine.Event) (*bridge.FlowService, bridge.FlowView, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Signup"}, Path: "signup", File: "/c/flows/signup.arazzo.yaml", Nodes: []model.Node{
		{ID: "a", Type: "set", Params: []model.Field{{Key: "target", Value: "vars.x"}, {Key: "value", Value: "1"}}},
		{ID: "b", Type: "log"},
	}}}
	b := &vmtest.Backend{Cols: []model.Collection{c}, NextRun: &vmtest.Running{EventList: events, Result: engine.RunResult{Status: engine.StatusPassed, RunID: "r1", Seed: 7}}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	flows := app.Flows()
	v, err := flows.Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	return flows, v, pub
}

func TestFlowService_Run_should_publish_statuses_and_summary(t *testing.T) {
	flows, v, pub := openRunnable(t, []engine.Event{
		engine.NodeStarted{Path: "a", Node: "a"},
		engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusPassed, Duration: 5 * time.Millisecond, Outputs: map[string]any{"x": "1"}},
	})

	flows.Run(v.Node, nil)
	ev := pub.last["flowrun"].(bridge.FlowRunEvent)

	if ev.Run.Running || ev.Run.Statuses["a"] != "passed" || ev.Run.Results["a"] != "passed · 5ms" || ev.Run.Summary != "passed · seed 7" {
		t.Fatalf("run %+v", ev.Run)
	}
}

func TestFlowService_NodeData_should_show_the_outputs_of_a_step_that_ran(t *testing.T) {
	flows, v, _ := openRunnable(t, []engine.Event{
		engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusPassed, Outputs: map[string]any{"x": "1"}},
	})
	flows.Run(v.Node, nil)

	got, err := flows.NodeData(v.Node, "a", "json")
	before, _ := flows.NodeData(v.Node, "b", "json")

	if err != nil || got.Status != "passed" || len(got.Output) != 1 || !strings.Contains(got.Output[0].Code, `"x"`) {
		t.Fatalf("data %+v, err %v", got, err)
	}
	if before.Status != "" || len(before.Output) != 0 {
		t.Fatalf("a step that did not run has data: %+v", before)
	}
}

func TestFlowService_Run_should_refuse_while_the_flow_has_unsaved_edits(t *testing.T) {
	flows, v, pub := openRunnable(t, nil)
	flows.Insert(v.Node, "log", "$.nodes", 0)

	flows.Run(v.Node, nil)

	if got := pub.last["flowrun"].(bridge.FlowRunEvent).Run.Error; !strings.Contains(got, "save the flow first") {
		t.Fatalf("error %q", got)
	}
}

func TestFlowService_Source_should_show_the_flow_and_ApplySource_should_replace_it(t *testing.T) {
	flows, v, _ := openSignup(t)

	src, err := flows.Source(v.Node)
	problems, applyErr := flows.ApplySource(v.Node, src.Flow)

	if err != nil || src.Flow == "" || src.File == "" {
		t.Fatalf("source %+v, err %v", src, err)
	}
	if applyErr != nil || len(problems) != 0 {
		t.Fatalf("applying the shown text: problems %v, err %v", problems, applyErr)
	}
}

func openLogin(t *testing.T) (*bridge.RequestService, bridge.RequestView, *published, *vmtest.Backend) {
	t.Helper()
	reqs, v, pub, _, b := openLoginWith(t)
	return reqs, v, pub, b
}

// openLoginWith is openLogin that also returns the app (to pick an environment).
func openLoginWith(t *testing.T) (*bridge.RequestService, bridge.RequestView, *published, *bridge.App, *vmtest.Backend) {
	t.Helper()
	return openLoginOptions(t)
}

// openLoginOptions is openLoginWith for an app built with opts.
func openLoginOptions(t *testing.T, opts ...bridge.Option) (*bridge.RequestService, bridge.RequestView, *published, *bridge.App, *vmtest.Backend) {
	t.Helper()
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	b.Values = map[string]any{"defaults.environment": "staging"}
	b.NextSend = &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusPassed, Seed: 7,
		Record: &engine.Record{Status: 200, Version: "1", RequestPath: "auth/login", Body: engine.Body{JSON: map[string]any{"ok": true}}}}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit, opts...)
	app.Start(t.Context())
	var node bridge.TreeNode
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children { // requests > auth
		for _, c := range n.Children {
			if c.Label == "Login" {
				node = c
			}
		}
	}
	reqs := app.Requests()
	v, err := reqs.Open(node.ID)
	if err != nil {
		t.Fatalf("%v (node %+v)", err, node)
	}
	return reqs, v, pub, app, b
}

func TestRequestService_Open_should_show_the_selected_version(t *testing.T) {
	_, v, _, _ := openLogin(t)

	if v.Name != "Login" || v.Kind != "http" || v.Current.Method != "POST" || v.Selected != "1" || v.Dirty {
		t.Fatalf("view %+v", v)
	}
}

func TestRequestService_Edit_should_change_the_version_and_mark_it_dirty(t *testing.T) {
	reqs, v, pub, _ := openLogin(t)
	next := v.Current
	next.URL = "{{env.base_url}}/signin"

	reqs.Edit(v.Node, next)
	got := pub.last["request"].(bridge.RequestView)

	if got.Current.URL != "{{env.base_url}}/signin" || !got.Dirty {
		t.Fatalf("view %+v", got)
	}
}

func TestRequestService_Send_should_publish_the_response(t *testing.T) {
	reqs, v, pub, b := openLogin(t)

	reqs.Send(v.Node, "")
	got := pub.last["request"].(bridge.RequestView)

	if got.Response == nil || !got.Response.Passed || len(got.Response.Received) == 0 || got.Sending {
		t.Fatalf("response %+v, sending %v", got.Response, got.Sending)
	}
	if !b.Called("Send") {
		t.Fatal("the backend was not asked to send")
	}
}

func TestRequestService_setters_should_edit_the_selected_version(t *testing.T) {
	reqs, v, pub, _ := openLogin(t)

	reqs.SetURL(v.Node, "{{env.base_url}}/signin?next=home")
	reqs.SetAuth(v.Node, "bearer", "{{secret.token}}")
	reqs.SetBodyKind(v.Node, "text")
	reqs.SetBodyText(v.Node, `{"a":1}`)
	got := pub.last["request"].(bridge.RequestView)

	if got.Current.URL != "{{env.base_url}}/signin" || len(got.Current.Query) != 1 || got.Current.Query[0].Key != "next" {
		t.Fatalf("url %q query %+v", got.Current.URL, got.Current.Query)
	}
	if got.Auth.Kind != "bearer" || got.Auth.Value != "{{secret.token}}" || got.BodyKind != "text" || got.Current.Body.JSON != `{"a":1}` {
		t.Fatalf("auth %+v, body kind %q, body %+v", got.Auth, got.BodyKind, got.Current.Body)
	}
}

func TestWorkspaceService_Menu_should_offer_what_applies_to_the_row(t *testing.T) {
	app, _ := fixture(t)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")

	menu := app.Workspace().Menu([]string{req.ID})

	var actions []string
	for _, e := range menu {
		actions = append(actions, e.Action)
	}
	// Rename used to ask for the whole path ("auth/login") under the title
	// "Rename / move", yet could not move to another collection. It now asks
	// for the name, and Move to… does the moving.
	if strings.Join(actions, ",") != "open,duplicate,rename,move,delete" {
		t.Fatalf("menu %v", actions)
	}
	if menu[2].Prompt == nil || menu[2].Prompt.Initial != "login" || menu[3].Dialog != "move" || menu[4].Confirm == "" {
		t.Fatalf("prompt %+v, move %+v, confirm %q", menu[2].Prompt, menu[3], menu[4].Confirm)
	}
}

func TestWorkspaceService_Do_newRequest_should_create_it_and_ask_to_open_it(t *testing.T) {
	app, pub := fixture(t)
	app.Start(t.Context())
	coll := app.Workspace().Tree().Roots[0]

	app.Workspace().Do("newRequest", []string{coll.ID}, "health-check", "")

	if opened, _ := pub.last["open"].(string); opened == "" {
		t.Fatalf("no open event; events %v", pub.names)
	}
}

func TestEnvironmentService_Open_then_SetRows_should_show_variables_and_mark_dirty(t *testing.T) {
	app, pub := fixture(t)
	app.Start(t.Context())
	var env bridge.TreeNode
	for _, n := range app.Workspace().Tree().Roots {
		for _, c := range n.Children {
			if c.Kind == "environment" && c.Label == "staging" {
				env = c
			}
		}
	}
	svc := app.Environments()

	v, err := svc.Open(env.ID)
	svc.SetRows(env.ID, append(v.Rows, bridge.EnvRow{Name: "region", Value: "eu"}))
	got := pub.last["environment"].(bridge.EnvView)

	if err != nil || v.Name != "staging" || len(v.Rows) != 2 || !v.Rows[1].Secret {
		t.Fatalf("view %+v, err %v", v, err)
	}
	if !got.Dirty || len(got.Rows) != 3 {
		t.Fatalf("after edit %+v", got)
	}
}

func TestPanelsService_should_publish_the_run_log_and_history_after_a_send(t *testing.T) {
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}, Values: map[string]any{"defaults.environment": "staging"}}
	b.NextSend = &vmtest.Sending{
		EventList: []engine.Event{
			engine.RequestSent{Method: "POST", URL: "https://s/login", RequestPath: "auth/login", Version: "1"},
			engine.ResponseReceived{Status: 200, Duration: 12 * time.Millisecond},
		},
		Result: engine.SendResult{Status: engine.StatusPassed, Record: &engine.Record{Status: 200, Version: "1", RequestPath: "auth/login"}},
	}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	panels := app.Panels()
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")
	reqs := app.Requests()
	if _, err := reqs.Open(req.ID); err != nil {
		t.Fatal(err)
	}

	reqs.Send(req.ID, "")
	got := panels.State()

	if len(got.History) != 1 || got.History[0].Node != req.ID {
		t.Fatalf("history %+v, request node %q", got.History, req.ID)
	}
	var lines []string
	for _, sec := range got.Log {
		for _, l := range sec.Lines {
			lines = append(lines, l.Message)
		}
	}
	if !strings.Contains(strings.Join(lines, "\n"), "→ POST https://s/login") {
		t.Fatalf("log %v", lines)
	}
	if _, ok := pub.last["panels"]; !ok {
		t.Fatal("no panels event")
	}

	panels.ClearLog()
	panels.ClearHistory()
	panels.ClearProblems()
	cleared := panels.State()

	if len(cleared.History) != 0 || len(cleared.Log) != 0 {
		t.Fatalf("after clearing: history %+v, log %+v", cleared.History, cleared.Log)
	}
}

func TestSettingsService_State_should_list_groups_and_the_appearance(t *testing.T) {
	app, pub := fixture(t)
	svc := app.Settings()

	v := svc.State()
	svc.Set("appearance.theme", "dark")

	if len(v.Groups) == 0 || v.Appearance.FontSize == 0 {
		t.Fatalf("settings %+v", v.Appearance)
	}
	if _, ok := pub.last["settings"]; !ok {
		t.Fatal("no settings event")
	}
}

func TestSecretsService_State_should_list_secrets_without_values(t *testing.T) {
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}, SecretList: []flowi.SecretInfo{{Environment: "staging", Name: "password"}}}
	svc := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit).Secrets()

	v := svc.State()

	if len(v.Secrets) == 0 || v.Secrets[0].Name != "password" || v.Secrets[0].State != "missing" {
		t.Fatalf("secrets %+v", v.Secrets)
	}
}

func TestManagerService_State_should_list_the_mounted_collections(t *testing.T) {
	app, _ := fixture(t)
	svc := app.Manager()

	v := svc.State()

	if len(v.Collections) != 1 || v.Collections[0].Name != "Shop" {
		t.Fatalf("collections %+v", v.Collections)
	}
}

func TestImportService_Preview_should_show_the_planned_files(t *testing.T) {
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	b.Plan = flowi.ImportPlan{Collection: c.Manifest.ID, Files: []flowi.PlannedFile{{Rel: "requests/health.req.yaml"}}}
	pub := &published{}
	svc := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit).Import()
	v := svc.State()

	svc.SetKind("curl")
	svc.SetPaste("curl https://example.test/health")
	svc.Preview()
	got := pub.last["import"].(bridge.ImportView)

	if len(v.Kinds) < 8 || v.Collection != string(c.Manifest.ID) {
		t.Fatalf("wizard %+v", v)
	}
	if !got.CanImport || len(got.Files) != 1 || got.Files[0].Rel != "requests/health.req.yaml" || !strings.Contains(got.Summary, "1 request") {
		t.Fatalf("preview %+v", got)
	}
}

func TestSchedulesService_Save_should_add_a_schedule_and_report_bad_input(t *testing.T) {
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	svc := app.Schedules()
	svc.State()

	problem := svc.Save("", bridge.ScheduleView{Name: "nightly", Flow: "signup", Cron: "@daily", Inputs: "email=a@b.c"})
	badInput := svc.Save("", bridge.ScheduleView{Name: "x", Flow: "signup", Cron: "@daily", Inputs: "no-equals"})
	v := svc.State()

	if problem != "" || len(v.Schedules) != 1 || v.Schedules[0].Name != "nightly" || v.Schedules[0].FlowLabel != "Shop › signup" {
		t.Fatalf("problem %q, schedules %+v", problem, v.Schedules)
	}
	if !strings.Contains(badInput, "name=value") {
		t.Fatalf("bad input said %q", badInput)
	}
}

func TestReplayerService_Start_should_report_a_bad_latency_without_starting(t *testing.T) {
	app, _ := fixture(t)
	svc := app.Replayer()
	svc.State()

	svc.Start(bridge.ReplayerForm{Latency: "soon"})
	v := svc.State()

	if v.State != "stopped" || !strings.Contains(v.Error, "latency must be a duration") {
		t.Fatalf("replayer %+v", v)
	}
}

func TestBlueprintsService_State_should_list_blueprints_of_the_collections(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Blueprints = []model.Blueprint{{Meta: model.Meta{ID: "B1", Name: "Login journey"}, Path: "login-journey", Params: []model.Param{{Name: "user", Default: "ann"}}}}
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), (&published{}).emit)
	svc := app.Blueprints()
	app.Start(t.Context())

	v := svc.State()
	own := slices.DeleteFunc(slices.Clone(v.Blueprints), func(b bridge.BlueprintView) bool { return b.BuiltIn })

	if len(own) != 1 || own[0].Kind != "flow" || own[0].Params[0].Default != "ann" {
		t.Fatalf("blueprints %+v", v.Blueprints)
	}
}

// The screen was empty for a workspace without blueprints of its own, though
// new requests start with Flow-I's built-in hooks: those are listed too.
func TestBlueprintsService_State_Given_no_blueprints_Then_it_lists_the_built_in_hooks(t *testing.T) {
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	v := app.Blueprints().State()

	want := map[string]string{"built-in/http-post": "post-hooks", "built-in/async-pre": "pre-hooks", "built-in/async-post": "post-hooks", "built-in/grpc-pre": "pre-hooks", "built-in/grpc-post": "post-hooks"}
	got := map[string]string{}
	for _, b := range v.Blueprints {
		if !b.BuiltIn || b.Nodes == 0 || b.Name == "" {
			t.Fatalf("blueprint %+v, want a named built-in with steps", b)
		}
		got[b.Key] = b.Kind
	}
	if !maps.Equal(got, want) {
		t.Fatalf("built-ins %v, want %v", got, want)
	}
}

func TestRequestService_Snippet_should_follow_edits_and_formats(t *testing.T) {
	reqs, v, pub, b := openLogin(t)
	b.PreviewResult = engine.Preview{Masked: engine.PreviewRequest{Method: "POST", URL: "https://s/login"}}

	reqs.SetSnippetFormat(v.Node, "wget")
	got := pub.last["request"].(bridge.RequestView)

	if got.Snippet.Format != "wget" || len(got.Snippet.Formats) != 4 {
		t.Fatalf("snippet %+v", got.Snippet)
	}
}

func TestRequestService_UsedBy_and_Cookies_should_answer_for_the_request(t *testing.T) {
	reqs, v, _, _ := openLogin(t)

	used := reqs.UsedBy(v.Node)
	cookies := reqs.Cookies(v.Node)

	if used.Rows == nil || cookies.Cookies == nil {
		t.Fatalf("used by %+v, cookies %+v", used, cookies)
	}
}

func TestRequestService_async_and_grpc_requests_should_expose_their_blocks(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Requests = append(c.Requests,
		model.Request{Meta: model.Meta{ID: "R8", Name: "Events"}, Path: "events", File: "/c/requests/events.req.yaml", Default: "1",
			Versions: []model.RequestVersion{{Label: "1", Async: &model.Async{Protocol: "nats", Server: "nats://localhost", Channel: "orders", Send: &model.AsyncSend{}}}}},
		model.Request{Meta: model.Meta{ID: "R9", Name: "Rpc"}, Path: "rpc", File: "/c/requests/rpc.req.yaml", Default: "1",
			Versions: []model.RequestVersion{{Label: "1", GRPC: &model.GRPC{Server: "grpc://localhost:9", Method: "x.Svc/Get"}}}})
	pub := &published{}
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	reqs := app.Requests()
	open := func(label string) bridge.RequestView {
		for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
			if n.Label == label {
				v, err := reqs.Open(n.ID)
				if err != nil {
					t.Fatal(err)
				}
				return v
			}
		}
		t.Fatalf("no request %q", label)
		return bridge.RequestView{}
	}

	async := open("Events")
	reqs.SetMode(async.Node, "send_receive")
	got := pub.last["request"].(bridge.RequestView)
	grpc := open("Rpc")

	if async.Kind != "async" || async.Async == nil || async.Async.Mode != "send" || len(async.Async.Protocols) < 5 {
		t.Fatalf("async %+v", async.Async)
	}
	if got.Async.Mode != "send_receive" || got.Async.Block.Receive == nil {
		t.Fatalf("after SetMode %+v", got.Async)
	}
	if grpc.Kind != "grpc" || grpc.GRPC == nil || grpc.GRPC.Block.Method != "x.Svc/Get" {
		t.Fatalf("grpc %+v", grpc.GRPC)
	}
}

func TestHooksService_should_edit_the_hooks_as_two_documents_and_save_them(t *testing.T) {
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")
	if _, err := app.Requests().Open(req.ID); err != nil {
		t.Fatal(err)
	}
	hooks := app.Hooks()

	v, err := hooks.Open(req.ID)
	app.Flows().Insert(v.Post, "log", "$.nodes", 0)
	dirty := pub.last["hooks"].(bridge.HooksView).Dirty
	problem := hooks.Save(req.ID)

	if err != nil || v.Pre == "" || v.Post == "" || !dirty {
		t.Fatalf("view %+v, err %v, dirty %v", v, err, dirty)
	}
	if problem != "" || !b.Called("SaveRequest") {
		t.Fatalf("save problem %q, backend calls", problem)
	}
}

func TestSessionService_should_keep_and_restore_the_open_tabs(t *testing.T) {
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}, Session: t.TempDir(), Values: map[string]any{"ui.restore_tabs": true}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	session := app.Session()

	session.SaveTabs([]string{"r:C1:R1", "f:C1:F1"})
	app.Shutdown() // waits for the queued write
	got := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit, bridge.WithLogger(slog.New(slog.DiscardHandler))).Session().Restore()

	if strings.Join(got, ",") != "r:C1:R1,f:C1:F1" {
		t.Fatalf("restored %v", got)
	}
}

func TestApp_Dirty_should_report_unsaved_edits_in_open_documents(t *testing.T) {
	c := vmtest.SampleCollection()
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")
	v, err := app.Requests().Open(req.ID)
	if err != nil {
		t.Fatal(err)
	}
	clean := app.Dirty()

	app.Requests().SetURL(v.Node, "{{env.base_url}}/signin")

	if clean || !app.Dirty() {
		t.Fatalf("clean %v, dirty after an edit %v", clean, app.Dirty())
	}
}

func TestApp_KeepEdits_Given_an_edited_request_When_quitting_Then_the_edit_is_stored_not_lost(t *testing.T) {
	c := vmtest.SampleCollection()
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")
	v, err := app.Requests().Open(req.ID)
	if err != nil {
		t.Fatal(err)
	}
	app.Requests().SetURL(v.Node, "{{env.base_url}}/signin")

	lost := app.KeepEdits()

	if lost {
		t.Fatal("an edit of a request is kept in its slot on quit; the user should not be asked about it")
	}
}

func TestApp_KeepEdits_Given_an_edited_flow_When_quitting_Then_the_edit_counts_as_lost(t *testing.T) {
	app, _ := fixture(t)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	app.Flows().AddStep(v.Node, "log", 0, 0, 0, false)

	lost := app.KeepEdits()

	if !lost {
		t.Fatal("flows have no edit slot: unsaved edits must be confirmed")
	}
}

func TestWelcomeService_should_show_the_latest_changes_and_the_telemetry_choice(t *testing.T) {
	app, _ := fixture(t)
	svc := app.Welcome()

	v := svc.State()
	tele := svc.Telemetry()

	if v.Version == "" || tele.Level != "off" || tele.Previews["full"] == "" || tele.Explanation == "" {
		t.Fatalf("welcome %+v, telemetry %+v", v, tele)
	}
}

func openTwoUnits(t *testing.T) (*bridge.FlowService, bridge.FlowView, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Pair"}, Path: "pair", File: "/c/flows/pair.arazzo.yaml", Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1", Path: "auth/login"}}},
		{ID: "wait", Type: "delay", Params: []model.Field{{Key: "duration", Value: "1s"}}},
		{ID: "verify", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R2", Path: "auth/verify"}}},
	}}}
	pub := &published{}
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	return app.Flows(), v, pub
}

func TestFlowService_Connect_should_make_a_step_wait_for_another(t *testing.T) {
	flows, v, pub := openTwoUnits(t)

	flows.Connect(v.Node, "verify", "login") // login now waits for verify: a step after it
	flows.Select(v.Node, "login")
	got := pub.last["flow"].(bridge.FlowView)

	if got.Form.Step == nil || len(got.Form.Step.Needs) != 1 || got.Form.Step.Needs[0] != "verify" {
		t.Fatalf("needs %+v", got.Form.Step)
	}
	var kinds []string
	for _, c := range got.Chips {
		kinds = append(kinds, c.Kind)
	}
	if strings.Join(kinds, ",") != "needs,unsaved" {
		t.Fatalf("chips %v", kinds)
	}

	flows.RepairNeeds(v.Node)
	if after := pub.last["flow"].(bridge.FlowView); len(after.Chips) != 1 || after.Chips[0].Kind != "unsaved" {
		t.Fatalf("after repair %+v", after.Chips)
	}
}

func TestFlowService_ToggleSelect_and_DeleteSelected_should_work_on_several_steps(t *testing.T) {
	flows, v, pub := openTwoUnits(t)

	flows.Select(v.Node, "login")
	flows.ToggleSelect(v.Node, "verify")
	both := pub.last["flow"].(bridge.FlowView)
	flows.DeleteSelected(v.Node)
	after := pub.last["flow"].(bridge.FlowView)

	if strings.Join(both.Selection, ",") != "login,verify" || both.Selected != "verify" {
		t.Fatalf("selection %v, primary %q", both.Selection, both.Selected)
	}
	if len(after.Diagram.Units) != 1 || len(after.Selection) != 0 {
		t.Fatalf("after delete: units %d, selection %v", len(after.Diagram.Units), after.Selection)
	}
}

func TestRequestService_should_send_the_json_body_and_pin_and_extract(t *testing.T) {
	reqs, v, pub, _ := openLogin(t)
	reqs.Send(v.Node, "")
	got := pub.last["request"].(bridge.RequestView)

	err := reqs.ExtractAsVariable(v.Node, "ok", "$.ok")
	file, pinErr := reqs.PinExample(v.Node, "success")

	if got.Response == nil || got.Response.BodyJSON == nil {
		t.Fatalf("no json body: %+v", got.Response)
	}
	if err != nil || pinErr != nil || file == "" {
		t.Fatalf("extract %v, pin %q %v", err, file, pinErr)
	}
}

func TestRequestService_URLSpans_and_Suggest_should_know_the_environment(t *testing.T) {
	reqs, v, _, _ := openLogin(t)

	spans := reqs.URLSpans(v.Node, "https://x.test/{{env.base_url}}/{{env.nope}}")
	got := reqs.Suggest(v.Node, "https://x.test/{{env.", len("https://x.test/{{env."))

	var parts []string
	for _, s := range spans {
		parts = append(parts, s.Part)
	}
	joined := strings.Join(parts, ",")
	if !strings.Contains(joined, "variable") || !strings.Contains(joined, "unknown-variable") {
		t.Fatalf("parts %v", parts)
	}
	if len(got.Items) == 0 || !strings.HasPrefix(got.Items[0].Insert, "env.") || got.From != len("https://x.test/{{") {
		t.Fatalf("suggestions %+v", got)
	}
}

func TestRequestService_Settings_should_list_what_a_version_may_set(t *testing.T) {
	reqs, v, _, _ := openLogin(t)

	reqs.AddSetting(v.Node, "timeout")
	got := reqs.Settings(v.Node)
	problems := reqs.SetSettings(v.Node, []bridge.SettingRowView{{Key: "timeout", Raw: "soon"}})
	schemas := reqs.Schemas(v.Node)

	if len(got.Rows) != 1 || got.Rows[0].Key != "timeout" || len(got.Available) == 0 {
		t.Fatalf("settings %+v", got)
	}
	if len(problems) != 1 {
		t.Fatalf("problems %v", problems)
	}
	if schemas == nil {
		t.Fatal("schemas must be a list")
	}
}

func TestRequestService_ShowExchange_should_show_an_earlier_send_until_ShowLatest(t *testing.T) {
	reqs, v, pub, _ := openLogin(t)
	reqs.Send(v.Node, "")
	reqs.ShowExchange(v.Node, 1)
	shown := pub.last["request"].(bridge.RequestView)
	reqs.ShowLatest(v.Node)
	latest := pub.last["request"].(bridge.RequestView)

	if !shown.ShowingHistory || !strings.HasPrefix(shown.Response.Title, "History · ") || latest.ShowingHistory {
		t.Fatalf("shown %+v, latest %+v", shown.ShowingHistory, latest.ShowingHistory)
	}
}

func TestWorkspaceService_NewIn_and_NewScratchRequest_should_create_and_ask_to_open(t *testing.T) {
	app, pub := fixture(t)
	app.Start(t.Context())
	coll := app.Workspace().Tree().Roots[0]

	app.Workspace().NewScratchRequest()
	scratch, _ := pub.last["open"].(string)
	app.Workspace().NewIn("flow", coll.ID, "journeys/signup", "")
	flow, _ := pub.last["open"].(string)

	if scratch == "" || flow == "" || scratch == flow {
		t.Fatalf("opened %q then %q", scratch, flow)
	}
}

func TestRequestService_SaveTo_should_move_the_request_and_ask_to_open_it(t *testing.T) {
	reqs, v, pub, _ := openLogin(t)
	_ = v

	reqs.SaveTo(v.Node, "c:C1", "kept/login")

	if opened, _ := pub.last["open"].(string); opened == "" || pub.last["close"] != v.Node {
		t.Fatalf("open %v, close %v", pub.last["open"], pub.last["close"])
	}
}

func TestFlowService_Mermaid_and_Relayout_should_work_on_an_open_flow(t *testing.T) {
	flows, v, pub := openTwoUnits(t)
	flows.SetPosition(v.Node, "login", 500, 500)

	flows.Relayout(v.Node)
	chart := flows.Mermaid(v.Node)
	got := pub.last["flow"].(bridge.FlowView)

	if !strings.Contains(chart, "flowchart") {
		t.Fatalf("mermaid %q", chart)
	}
	for _, u := range got.Diagram.Units {
		if u.ID == "login" && (u.X != 0 || u.Y != 0) {
			t.Fatalf("login stayed at %d,%d", u.X, u.Y)
		}
	}
}

func TestRequestService_view_should_name_the_verb_the_tab_and_pickers_colour(t *testing.T) {
	_, v, _, _ := openLogin(t)

	if v.Verb != "POST" {
		t.Fatalf("verb %q", v.Verb)
	}
}

func TestRequestService_DeleteVersion_should_refuse_the_only_version(t *testing.T) {
	reqs, v, _, b := openLogin(t)

	needsConfirm, err := reqs.DeleteVersion(v.Node, false)

	if err == nil || needsConfirm || len(b.Saved) != 0 {
		t.Fatalf("needs confirm %v, err %v, saved %d", needsConfirm, err, len(b.Saved))
	}
}

func TestFlowService_SetAttach_should_draw_the_step_in_one_block_with_its_request(t *testing.T) {
	flows, v, pub := openSignup(t)
	req := flows.AddStep(v.Node, "request", 0, 0, 0, false)
	flows.Select(v.Node, "b")
	form := lastView(pub).Form.Step

	flows.SetAttach(v.Node, req)
	got := lastView(pub)

	var block *canvas.Unit
	for i, u := range got.Diagram.Units {
		if u.Kind == canvas.KindRequest {
			block = &got.Diagram.Units[i]
		}
	}
	if form == nil || len(form.AttachTo) != 1 || form.AttachTo[0] != req {
		t.Fatalf("attach choices %+v", form)
	}
	if block == nil || len(block.Nodes) != 2 || block.Nodes[block.Core] != req || got.Cards[block.ID].Title != req {
		t.Fatalf("block %+v, cards %+v", block, got.Cards)
	}
	if got.Form.Step.Attach != req {
		t.Fatalf("form %+v", got.Form.Step)
	}
}

// Behaviour change: an empty lane of a condition was 16 units wide, so a step
// dropped "into" it missed and landed after the container; and a lane grew
// from a sliver to a box when it got its first step. An empty lane is now a
// target as wide as a step needs.
func TestFlowService_Drop_should_put_a_lifted_card_into_an_empty_lane_and_back_out(t *testing.T) {
	flows, v, pub := openSignup(t)
	flows.AddStep(v.Node, "condition", 0, 0, 0, false)
	g := lastView(pub)
	strip, lane := g.Diagram.Units[0], g.Diagram.Lanes[0]

	flows.Drop(v.Node, strip.ID, "b", true, lane.X+20, lane.Y+20, 24, 0, 0)
	in := lastView(pub)
	var b canvas.Unit
	for _, u := range in.Diagram.Units {
		if u.ID == "b" {
			b = u
		}
	}
	flows.Drop(v.Node, "b", "b", true, 5, 30, 24, 0, 0)
	out := lastView(pub)

	if lane.W < canvas.ActionW+canvas.StripPad {
		t.Fatalf("an empty lane is %d wide", lane.W)
	}
	if b.Parent != "condition" || in.Diagram.Lanes[0].W != lane.W || in.Diagram.Lanes[0].H != lane.H {
		t.Fatalf("b %+v, lane before %+v, lane with the step %+v", b, lane, in.Diagram.Lanes[0])
	}
	if out.Diagram.Units[0].Nodes[0] != "b" || out.Diagram.Lanes[0].W != lane.W || out.Diagram.Lanes[0].H != lane.H {
		t.Fatalf("after moving out: units %+v, lane %+v (was %+v)", out.Diagram.Units, out.Diagram.Lanes[0], lane)
	}
}

func TestFlowService_AddAttached_should_add_a_step_before_or_after_the_request_drawn_with_it(t *testing.T) {
	flows, v, pub := openSignup(t)
	req := flows.AddStep(v.Node, "request", 0, 0, 0, false)

	before, err1 := flows.AddAttached(v.Node, "log", false)
	flows.Select(v.Node, req)
	after, err2 := flows.AddAttached(v.Node, "assert", true)
	flows.Select(v.Node, req)
	got := lastView(pub)
	flows.Select(v.Node, "a")
	_, errNotRequest := flows.AddAttached(v.Node, "log", true)

	if err1 != nil || err2 != nil || errNotRequest == nil {
		t.Fatalf("errors: %v %v, adding to a plain step: %v", err1, err2, errNotRequest)
	}
	step := got.Form.Step
	if step == nil || len(step.Attached) != 2 || step.Attached[0] != before || step.Attached[1] != after {
		t.Fatalf("attached to the request: %+v (before %q, after %q)", step, before, after)
	}
}

// Dropping a single action inside a request block puts it next to the request
// and draws it with it: left half before, right half after. A plain move out of
// the block is not attached any more.
func TestFlowService_Drop_should_attach_an_action_dropped_inside_a_request_block(t *testing.T) {
	flows, v, pub := openSignup(t)
	req := flows.AddStep(v.Node, "request", 0, 0, 0, false)
	g := lastView(pub)
	var strip, block canvas.Unit
	for _, u := range g.Diagram.Units {
		if u.Kind == canvas.KindRequest {
			block = u
		} else {
			strip = u
		}
	}

	preview := flows.Preview(v.Node, strip.ID, "b", true, block.X+10, block.Y+block.H/2, 24)
	flows.Drop(v.Node, strip.ID, "b", true, block.X+10, block.Y+block.H/2, 24, 0, 0)
	got := lastView(pub)

	if !preview.OK || preview.Attach != req {
		t.Fatalf("preview %+v", preview)
	}
	var drawn canvas.Unit
	for _, u := range got.Diagram.Units {
		if u.Kind == canvas.KindRequest {
			drawn = u
		}
	}
	if len(drawn.Nodes) != 2 || drawn.Nodes[0] != "b" || drawn.Nodes[drawn.Core] != req {
		t.Fatalf("block after the drop %+v", drawn)
	}

	flows.Drop(v.Node, drawn.ID, "b", true, 5, 30, 24, 0, 0) // lifted out, to the start of the flow
	out := lastView(pub)
	for _, u := range out.Diagram.Units {
		if u.Kind == canvas.KindRequest && len(u.Nodes) != 1 {
			t.Fatalf("b is still drawn with the request: %+v", u)
		}
	}
}

// blockOf adds a request with a log before it and an assert after it, drawn in
// one block, and returns the ids in the block's order.
func blockOf(t *testing.T) (flows *bridge.FlowService, node string, pub *published, ids [3]string) {
	t.Helper()
	flows, v, pub := openSignup(t)
	req := flows.AddStep(v.Node, "request", 0, 0, 0, false)
	before, err1 := flows.AddAttached(v.Node, "log", false)
	flows.Select(v.Node, req)
	after, err2 := flows.AddAttached(v.Node, "assert", true)
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	return flows, v.Node, pub, [3]string{before, req, after}
}

func requestBlock(t *testing.T, pub *published) canvas.Unit {
	t.Helper()
	for _, u := range lastView(pub).Diagram.Units {
		if u.Kind == canvas.KindRequest {
			return u
		}
	}
	t.Fatal("no request block")
	return canvas.Unit{}
}

// Behaviour change: the actions of a request block could not be put in another
// order: dropping one on a cell other than the request itself was not a target,
// so it fell out of the block.
func TestFlowService_Drop_should_reorder_the_actions_of_a_request_block(t *testing.T) {
	flows, node, pub, ids := blockOf(t)
	block := requestBlock(t, pub)
	cellX := canvas.CellsOf(block)[0].X

	flows.Drop(node, block.ID, ids[2], true, block.X+cellX+10, block.Y+block.H/2, 24, 0, 0) // left half of the first cell
	got := requestBlock(t, pub)

	want := []string{ids[2], ids[0], ids[1]}
	if strings.Join(got.Nodes, ",") != strings.Join(want, ",") || got.Nodes[got.Core] != ids[1] {
		t.Fatalf("block %v (core %d), want %v around the request", got.Nodes, got.Core, want)
	}
}

func TestFlowService_MoveAttachedBy_should_swap_an_action_with_its_neighbour_in_the_block(t *testing.T) {
	flows, node, pub, ids := blockOf(t)

	flows.MoveAttachedBy(node, ids[2], -1)
	moved := requestBlock(t, pub).Nodes
	flows.MoveAttachedBy(node, ids[0], -1) // already first in its block: nothing to swap with
	stay := requestBlock(t, pub).Nodes

	if strings.Join(moved, ",") != strings.Join([]string{ids[0], ids[2], ids[1]}, ",") {
		t.Fatalf("after moving up: %v", moved)
	}
	if strings.Join(stay, ",") != strings.Join(moved, ",") {
		t.Fatalf("an action at the edge of its block moved: %v", stay)
	}
}

// Dragging a link from the step right before one used to store a needs that
// said nothing and made the step a card of its own with a dashed arrow.
func TestFlowService_Connect_Given_the_step_before_Then_says_so_and_stores_no_needs(t *testing.T) {
	flows, v, pub := openTwoUnits(t)

	flows.Connect(v.Node, "wait", "verify") // verify follows wait in the list
	flows.Select(v.Node, "verify")
	got := lastView(pub)

	if len(got.Form.Step.Needs) != 0 || got.Form.Step.Previous != "wait" {
		t.Fatalf("needs %v, previous %q", got.Form.Step.Needs, got.Form.Step.Previous)
	}
	n, ok := pub.last["notice"].(bridge.Notice)
	if !ok || !strings.Contains(n.Text, "follows wait") {
		t.Fatalf("notice %+v", pub.last["notice"])
	}
}

func TestFlowService_AddNeed_and_RemoveNeed_should_edit_what_the_selected_step_waits_for(t *testing.T) {
	flows, v, pub := openSignup(t) // a, b
	c := flows.AddStep(v.Node, "log", 0, 0, 0, false)
	flows.Select(v.Node, c)

	flows.AddNeed(v.Node, "a")
	with := lastView(pub).Form.Step
	flows.RemoveNeed(v.Node, "a")
	without := lastView(pub).Form.Step

	if len(with.Needs) != 1 || with.Needs[0] != "a" || len(without.Needs) != 0 {
		t.Fatalf("needs with %v, without %v", with.Needs, without.Needs)
	}
	if len(with.NeedChoices) < 2 {
		t.Fatalf("choices %v", with.NeedChoices)
	}
}

func TestFlowService_view_should_list_the_values_that_pass_between_steps(t *testing.T) {
	flows, v, pub := openSignup(t)
	flows.Select(v.Node, "a")
	flows.SetParam(v.Node, "value", "{{nodes.b.value}}")
	flows.Select(v.Node, "b")
	got := lastView(pub)

	if len(got.Data) == 0 && len(got.Form.Step.UsedBy) == 0 {
		t.Fatalf("no data links: %+v", got.Form.Step)
	}
}

func TestWorkspaceService_Location_should_say_where_the_collection_of_a_node_lives(t *testing.T) {
	app, _ := fixture(t)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")

	got := app.Workspace().Location(req.ID)
	none := app.Workspace().Location("screen:welcome")

	if got.Name == "" || got.Scratch || none.Name != "" {
		t.Fatalf("location %+v, of a screen %+v", got, none)
	}
}

// await polls until cond holds, for at most a few seconds (the runner changes state on
// its own goroutines, and the app's actor runs what they post).
func await(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		runtime.Gosched()
	}
}

func TestSchedulesService_StartRunner_should_run_the_schedules_and_StopRunner_end_them(t *testing.T) {
	c := vmtest.SampleCollection()
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewActor(t.Context()), (&published{}).emit)
	svc := app.Schedules()
	svc.State()

	svc.StartRunner()
	await(t, "the schedules to run", func() bool { return svc.State().Runner.State == "running" })
	started := svc.State().Runner
	svc.StopRunner()
	await(t, "the schedules to stop", func() bool { return svc.State().Runner.State == "stopped" })
	app.Shutdown()

	if len(started.Upcoming) != 1 || started.Upcoming[0].Name != "nightly" || started.TriggerAddr == "" {
		t.Fatalf("runner %+v", started)
	}
}

func TestBlueprintsService_Where_should_name_the_collection_and_folder_of_a_row(t *testing.T) {
	c := vmtest.SampleCollection()
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")

	got := app.Blueprints().Where(req.ID)
	none := app.Blueprints().Where("screen:welcome")

	if got.Collection == "" || none.Collection != "" {
		t.Fatalf("where %+v, none %+v", got, none)
	}
}

func TestSystemService_ExpressionHelp_should_say_what_a_field_can_read(t *testing.T) {
	app, _ := fixture(t)

	got := app.System().ExpressionHelp("message.check")

	if got.Title == "" || len(got.Names) == 0 || len(got.Tools) == 0 || len(got.Generators) == 0 || len(got.Examples) == 0 {
		t.Fatalf("help %+v", got)
	}
}

func TestRequestService_stored_responses_should_be_listed_shown_pinned_and_deleted(t *testing.T) {
	c := vmtest.SampleCollection()
	rec := engine.Record{Status: 201, Version: "1"}
	b := &vmtest.Backend{Cols: []model.Collection{c}, SavedRecord: &rec, Stored: []flowi.StoredResponse{
		{Kind: flowi.StoredExample, Name: "ok", Status: 200},
		{Kind: flowi.StoredSaved, Version: "1", Name: "201", Status: 201},
		{Kind: flowi.StoredSaved, Version: "1", Name: "last", Status: 201},
	}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")
	v, err := app.Requests().Open(req.ID)
	if err != nil {
		t.Fatal(err)
	}
	svc := app.Requests()

	listed := svc.StoredResponses(v.Node)
	shown := svc.ShowStored(v.Node, "1", "201")
	title := ""
	if rv, err := svc.Open(v.Node); err == nil && rv.Response != nil {
		title = rv.Response.Title
	}
	problem := svc.DeleteStored(v.Node, "saved", "1", "last")
	after := svc.StoredResponses(v.Node)

	if len(listed) != 3 || listed[0].Kind != "example" || len(after) != 2 || problem != "" || shown != "" {
		t.Fatalf("listed %+v, after %+v, problem %q shown %q", listed, after, problem, shown)
	}
	if !strings.Contains(title, "Saved 201") {
		t.Fatalf("the response pane shows %q", title)
	}
}

func TestFlowService_AddStep_should_start_a_condition_with_a_test_about_the_request_before_it(t *testing.T) {
	flows, v, pub := openTwoUnits(t)
	flows.Select(v.Node, "login") // a request

	id := flows.AddStep(v.Node, "condition", 0, 0, 0, false)
	got := lastView(pub).Form.Step

	if got == nil || got.ID != id || len(got.Params) == 0 || got.Params[0].Name != "check" || got.Params[0].Value != "nodes.login.status == 200" {
		t.Fatalf("the new condition %+v", got)
	}
	flows.Select(v.Node, "wait") // not a request
	flows.AddStep(v.Node, "condition", 0, 0, 0, false)
	if second := lastView(pub).Form.Step; second.Params[0].Value != "" {
		t.Fatalf("a condition after a delay started with %q", second.Params[0].Value)
	}
}

func TestWorkspaceService_Reorder_should_keep_the_order_of_a_folder_and_show_it_in_the_tree(t *testing.T) {
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	group, _ := findNode(app.Workspace().Tree().Roots, "requests")

	app.Workspace().Reorder(group.ID, []string{"verify", "login"}, false)

	if !b.Called("SetFolderOrder requests verify,login") {
		t.Fatal("the order was not set")
	}
}

func TestWorkspaceService_Reorder_Given_a_row_that_is_not_a_folder_Then_says_so(t *testing.T) {
	app, pub := fixture(t)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")

	app.Workspace().Reorder(req.ID, []string{"x"}, false)

	n, ok := pub.last["notice"].(bridge.Notice)
	if !ok || n.Level != "error" {
		t.Fatalf("notice %+v", pub.last["notice"])
	}
}

// Behaviour change: attached actions could only be drawn beside their request, so a
// block with several of them grew wide. They can now be stacked above or below it.
func TestFlowService_attached_actions_can_be_drawn_above_and_below_their_request(t *testing.T) {
	flows, node, pub, ids := blockOf(t)
	flows.Select(node, ids[1])
	up, errUp := flows.AddAttachedAt(node, "note", "above")
	flows.Select(node, ids[1])
	down, errDown := flows.AddAttachedAt(node, "note", "below")
	block := requestBlock(t, pub)

	if errUp != nil || errDown != nil || len(block.Nodes) != 5 {
		t.Fatalf("errors %v %v, block %v", errUp, errDown, block.Nodes)
	}
	cells := map[string]canvas.Cell{}
	for i, id := range block.Nodes {
		cells[id] = block.Cells[i]
	}
	core := cells[ids[1]]
	if cells[up].Y >= core.Y || cells[up].X != core.X || cells[down].Y <= core.Y || block.Above == 0 {
		t.Fatalf("the stacks: above %+v, request %+v, below %+v", cells[up], core, cells[down])
	}

	// A drop in the top third of the request puts a step in the stack above it, in the list right before it.
	flows.Select(node, up)
	flows.SetAttachAt(node, "") // beside it again
	if got := requestBlock(t, pub); got.Cells[slices.Index(got.Nodes, up)].Y != got.Cells[got.Core].Y {
		t.Fatalf("after setting it beside the request: %+v", got.Cells)
	}
	flows.SetAttachAt(node, "above")
	back := requestBlock(t, pub)
	moved := flows.Preview(node, "", ids[0], true, block.X+core.X+20, block.Y+core.Y+4, 24)
	if !moved.OK || moved.Side != "above" || !moved.Horizontal || moved.Attach != ids[1] || back.Above == 0 {
		t.Fatalf("preview of a drop on the top of the request: %+v", moved)
	}
}

func TestFlowService_cards_should_show_the_hooks_that_run_around_a_request_step(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Manifest.Hooks = model.Hooks{
		Pre:  []model.HookStep{{Inline: []model.Node{{ID: "sign", Type: "log"}}}},
		Post: []model.HookStep{{Inline: []model.Node{{ID: "audit", Type: "log"}}}},
	}
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "One"}, Path: "one", File: "/c/flows/one.arazzo.yaml", Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1", Path: "auth/login"}}},
		{ID: "pause", Type: "delay", Params: []model.Field{{Key: "duration", Value: "1s"}}},
	}}}
	pub := &published{}
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}

	app.Flows().Select(v.Node, "login") // any change publishes the view
	cards := lastView(pub).Cards

	ticks := cards["login"].Hooks
	if len(ticks) != 2 || ticks[0].Phase != "pre" || ticks[0].Label != "inline" || ticks[1].Phase != "post" || ticks[0].Level != "collection" || ticks[0].State != "" {
		t.Fatalf("hooks of the request step %+v", ticks)
	}
	if len(cards["pause"].Hooks) != 0 {
		t.Fatalf("a delay step has hooks: %+v", cards["pause"].Hooks)
	}
}

func TestRequestService_SetVersionDetails_should_give_the_selected_version_a_name_and_a_description(t *testing.T) {
	c := vmtest.SampleCollection()
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	req, _ := findNode(app.Workspace().Tree().Roots, "request")
	v, err := app.Requests().Open(req.ID)
	if err != nil {
		t.Fatal(err)
	}

	app.Requests().SetVersionDetails(v.Node, "  Before the tax change ", "Prices without VAT.")
	got, _ := app.Requests().Open(v.Node)

	if got.Current.Name != "Before the tax change" || got.Current.Docs != "Prices without VAT." || got.VersionNames[0] != "Before the tax change" || len(got.VersionNames) != len(got.Versions) {
		t.Fatalf("version %+v, names %v", got.Current, got.VersionNames)
	}
}

func TestTrashService_Sweep_should_move_stale_drafts_except_those_open_in_a_tab(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Values: map[string]any{"ui.drafts_trash_after": 7 * 24 * time.Hour}, StaleDrafts: []flowi.TrashedItem{
		{ID: "T1", Kind: flowi.TrashRequest, Name: "RQ1", Path: "untitled"},
		{ID: "T2", Kind: flowi.TrashRequest, Name: "RQ2", Path: "untitled-2"},
	}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	svc := app.Trash()

	moved := svc.Sweep([]string{"r:SCRATCH:RQ2"}) // a tab of the second one is open
	view := svc.State()

	if moved != 1 || len(view.Items) != 1 || view.Items[0].Path != "untitled" || view.After != "7 days" {
		t.Fatalf("moved %d, trash %+v", moved, view)
	}
	path, err := svc.Restore("T1")
	problem := svc.Empty()
	if err != nil || path != "untitled" || problem != "" || len(svc.State().Items) != 0 {
		t.Fatalf("restored %q, %v, emptied %q", path, err, problem)
	}
}

func TestRequestService_Missing_should_list_undefined_variables_once_each(t *testing.T) {
	usePassword := func(v *model.RequestVersion) { v.Body.JSON = `{"pw":"{{secret.password}}"}` }
	tests := []struct {
		name    string
		env     string
		secrets []flowi.SecretInfo // the vault's answer
		edit    func(v *model.RequestVersion)
		want    []bridge.MissingVar
	}{
		{name: "all_defined", env: "staging", edit: func(*model.RequestVersion) {}, want: []bridge.MissingVar{}},
		{
			name: "declared_secret_without_a_value_is_missing", env: "staging", edit: usePassword,
			want: []bridge.MissingVar{{Expr: "secret.password", Name: "password", Secret: true}},
		},
		{
			name: "stored_secret_is_defined", env: "staging", edit: usePassword,
			secrets: []flowi.SecretInfo{{Environment: "staging", Name: "password", Set: true}}, want: []bridge.MissingVar{},
		},
		{
			name: "overridden_secret_is_defined", env: "staging", edit: usePassword,
			secrets: []flowi.SecretInfo{{Environment: "staging", Name: "password", Override: true}}, want: []bridge.MissingVar{},
		},
		{
			name: "env_and_secret_missing_in_empty_environment", env: "local",
			edit: func(v *model.RequestVersion) {
				v.Body.JSON = `{"pw":"{{secret.password}}","again":"{{env.base_url}}"}`
			},
			want: []bridge.MissingVar{{Expr: "env.base_url", Name: "base_url"}, {Expr: "secret.password", Name: "password", Secret: true}},
		},
		{
			name: "own_variables_and_generators_count_as_defined", env: "staging",
			edit: func(v *model.RequestVersion) {
				v.Variables = []model.KeyValue{{Key: "email", Value: "a@b.c"}}
				v.Headers = []model.KeyValue{{Key: "X-Mail", Value: "{{var.email}} {{email}} {{$uuid-v4}} {{token}} {{$nope}}"}}
			},
			want: []bridge.MissingVar{{Expr: "token", Name: "token"}, {Expr: "$nope"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, v, _, app, b := openLoginWith(t)
			b.SecretList = tt.secrets
			app.Workspace().SelectEnvironment(tt.env)
			next := v.Current
			tt.edit(&next)
			reqs.Edit(v.Node, next)

			got := reqs.Missing(v.Node)

			if got.Environment != tt.env || !slices.Equal(got.Vars, tt.want) {
				t.Fatalf("got %+v, want %+v in %q", got, tt.want, tt.env)
			}
		})
	}
}

func TestWorkspaceService_EnvironmentOffers_and_ImportEnvironments_should_copy_what_the_workspace_lacks(t *testing.T) {
	c := vmtest.SampleCollection() // the workspace has staging and local
	shipped := []model.Environment{
		{Meta: model.Meta{Name: "staging"}, Vars: []model.EnvVar{{Name: "base_url", Value: "https://other"}}},
		{Meta: model.Meta{Name: "qa"}, Vars: []model.EnvVar{{Name: "base_url", Value: "https://qa"}, {Name: "password", Secret: true}}},
	}
	b := &vmtest.Backend{Cols: []model.Collection{c}, CollEnvs: map[model.ID][]model.Environment{"C1": shipped}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	ws := app.Workspace()

	offers := ws.EnvironmentOffers()
	added, err := ws.ImportEnvironments("C1")

	if len(offers) != 1 || offers[0].Collection != "C1" || !slices.Equal(offers[0].Environments, []string{"qa"}) {
		t.Fatalf("offers %+v", offers)
	}
	if err != nil || !slices.Equal(added, []string{"qa"}) {
		t.Fatalf("added %v, err %v", added, err)
	}
	if len(b.SavedEnvs) != 1 || b.SavedEnvs[0].File != "/d/qa.yaml" || !slices.Equal(b.SavedEnvs[0].Vars, shipped[1].Vars) {
		t.Fatalf("saved %+v", b.SavedEnvs)
	}
}

func TestEnvironmentService_Unset_should_list_env_and_secret_names_the_requests_use(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Requests[2].Versions[0].Headers = []model.KeyValue{{Key: "Authorization", Value: "Bearer {{secret.token}} {{email}}"}}
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	envs, _ := findNode(app.Workspace().Tree().Roots, "environments")
	var local bridge.TreeNode
	for _, n := range envs.Children {
		if n.Label == "local" {
			local = n
		}
	}
	if _, err := app.Environments().Open(local.ID); err != nil {
		t.Fatal(err)
	}

	got := app.Environments().Unset(local.ID)

	want := []bridge.MissingVar{{Expr: "env.base_url", Name: "base_url"}, {Expr: "secret.token", Name: "token", Secret: true}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Before: the hook ticks on the cards are part of the flow view, but a run only published the
// run view, so a tick kept its colour from the run before until something else changed the view.
func TestFlowService_Run_should_publish_the_flow_view_again_when_the_run_changes(t *testing.T) {
	flows, v, pub := openSignup(t)

	flows.Run(v.Node, nil)

	if n := len(pub.names); n == 0 || pub.names[n-1] != "flow" {
		t.Fatalf("the run's last change did not publish the flow view: %v", pub.names)
	}
}

func TestApp_KeepFlowEdits_Given_an_edited_flow_When_it_is_opened_again_Then_the_edit_is_back_as_unsaved(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Signup"}, Path: "signup", File: "/c/flows/signup.arazzo.yaml", Nodes: []model.Node{
		{ID: "a", Type: "log"}, {ID: "b", Type: "log"},
	}}}
	back := &vmtest.Backend{Cols: []model.Collection{c}}
	first := bridge.NewApp(back, bridge.NewInlineActor(), (&published{}).emit)
	first.Start(t.Context())
	flow, _ := findNode(first.Workspace().Tree().Roots, "flow")
	v, _ := first.Flows().Open(flow.ID)
	first.Flows().AddStep(v.Node, "log", 0, 0, 0, false)
	edited := c.Flows[0]
	edited.Nodes = append(slices.Clone(edited.Nodes), model.Node{ID: "c", Type: "log"})
	back.ParsedFlow = &edited

	err := first.KeepFlowEdits()
	pub := &published{}
	second := bridge.NewApp(back, bridge.NewInlineActor(), pub.emit)
	second.Start(t.Context())
	if _, err := second.Flows().Open(flow.ID); err != nil {
		t.Fatal(err)
	}

	if err != nil || !back.Called("KeepFlowEdits F1") || !back.Called("KeptFlowEdits F1") {
		t.Fatalf("err %v calls %v", err, back.Calls)
	}
	if got := lastView(pub); !got.Dirty {
		t.Fatal("the kept edit did not come back as an unsaved change")
	}
}

func TestWelcomeService_Given_a_changelog_Then_whats_new_is_its_newest_release(t *testing.T) {
	c := vmtest.SampleCollection()
	log := "# Changelog\n\n## [1.2.0] - 2026-10-01\n\n### Features\n\n- **flows:** a canvas\n"
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), (&published{}).emit, bridge.WithChangelog(log))

	v := app.Welcome().State()

	if !strings.Contains(v.WhatsNew, "a canvas") {
		t.Fatalf("what's new %q", v.WhatsNew)
	}
}
