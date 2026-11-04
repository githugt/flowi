// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"strings"
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// A flow of a bundle opens showing the run the bundle recorded, with its timeline, before anything is run.
func TestOpen_Given_a_bundles_flow_Then_the_recorded_run_is_already_on_the_canvas(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Pair"}, Path: "pair", File: "/c/flows/pair.arazzo.yaml", Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1", Path: "auth/login"}}},
	}}}
	start := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	rec := engine.RunResult{RunID: "r1", Status: engine.StatusPassed, Seed: 7}
	rec.Report = report.Run{Started: start, Duration: 30 * time.Millisecond, Nodes: []report.Node{
		{ID: "login", Path: "login", Type: "request", Status: "passed", Started: start, Duration: 30 * time.Millisecond},
	}}
	be := &vmtest.Backend{Cols: []model.Collection{c}, Originals: map[model.ID]engine.RunResult{c.Manifest.ID: rec}}
	pub := &published{}
	app := bridge.NewApp(be, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")

	v, err := app.Flows().Open(flow.ID)

	if err != nil {
		t.Fatal(err)
	}
	if v.Run.Statuses["login"] != "passed" || len(v.Run.Timeline.Bars) != 1 || v.Run.Timeline.TotalMS != 30 {
		t.Fatalf("run %+v", v.Run)
	}
}

func openForkable(t *testing.T, recorded bool) (*bridge.App, *vmtest.Backend, string) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Pair"}, Path: "pair", File: "/c/flows/pair.arazzo.yaml", Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1", Path: "auth/login"}}},
	}}}
	be := &vmtest.Backend{Cols: []model.Collection{c}}
	if recorded {
		be.Originals = map[model.ID]engine.RunResult{c.Manifest.ID: {RunID: "r1", Status: engine.StatusPassed}}
	}
	app := bridge.NewApp(be, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	return app, be, flow.ID
}

func TestFlowView_should_say_when_the_flow_comes_from_a_bundle(t *testing.T) {
	fromBundle, _, node := openForkable(t, true)
	ordinary, _, otherNode := openForkable(t, false)

	a, _ := fromBundle.Flows().Open(node)
	b, _ := ordinary.Flows().Open(otherNode)

	if !a.Replay || b.Replay {
		t.Fatalf("replay: bundle %v, ordinary %v", a.Replay, b.Replay)
	}
}

func TestFork_should_pass_the_edits_as_typed_values_and_list_what_changed(t *testing.T) {
	app, be, node := openForkable(t, true)
	be.Check.Fork = flowi.ForkResult{Run: engine.RunResult{Status: engine.StatusPassed, Report: report.Run{Nodes: []report.Node{{ID: "login", Status: "passed"}}}},
		Comparison: report.Comparison{FirstDivergence: "login", Changes: []report.Change{{Node: "login", What: "wrote city", From: "Fresno", To: "Paris"}}}}
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	got, err := app.Flows().Fork(node, bridge.ForkRequest{From: "login", Set: map[string]string{"retries": "3", "city": "Paris"}})

	if err != nil || got.Status != "passed" || len(got.Steps) != 1 || got.Changes[0].To != "Paris" {
		t.Fatalf("fork %+v err %v", got, err)
	}
	forks := be.CallsStarting("ForkReplay login")
	if len(forks) != 1 || !strings.Contains(forks[0], "retries:3") || !strings.Contains(forks[0], "city:Paris") {
		t.Fatalf("forks %v", forks)
	}
}

func TestFork_Given_a_failed_run_Then_it_says_why(t *testing.T) {
	app, be, node := openForkable(t, true)
	be.Check.Fork = flowi.ForkResult{Run: engine.RunResult{Status: engine.StatusFailed, Report: report.Run{Nodes: []report.Node{
		{ID: "login", Status: "failed", Diagnostics: []report.Diagnostic{{Message: "the recording has no response for this call"}}},
	}}}}
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	got, err := app.Flows().Fork(node, bridge.ForkRequest{From: "login"})

	if err != nil || !strings.Contains(got.Why, "recording") {
		t.Fatalf("fork %+v err %v", got, err)
	}
}

func TestRequestNode_should_name_the_tree_node_of_the_request_a_step_calls(t *testing.T) {
	app, _, node := openForkable(t, false)
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	got, err := app.Flows().RequestNode(node, "login")
	_, noStep := app.Flows().RequestNode(node, "nope")

	if err != nil || got != "r:"+string(vmtest.SampleCollection().Manifest.ID)+":R1" || noStep == nil {
		t.Fatalf("node %q err %v, unknown step err %v", got, err, noStep)
	}
}

func TestFork_Given_only_and_an_answer_Then_the_one_step_is_answered_with_it(t *testing.T) {
	app, be, node := openForkable(t, true)
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	_, err := app.Flows().Fork(node, bridge.ForkRequest{From: "login", Only: true, AnswerStatus: "503", AnswerBody: `{"error": "down"}`})

	if err != nil {
		t.Fatal(err)
	}
	forks := be.CallsStarting("ForkReplay login")
	if len(forks) != 1 || !strings.Contains(forks[0], "only=true") || !strings.Contains(forks[0], "status=503") || !strings.Contains(forks[0], "body=map[error:down]") {
		t.Fatalf("forks %v", forks)
	}
}

func TestFork_Given_an_answer_that_is_not_usable_Then_it_says_what_is_wrong_and_forks_nothing(t *testing.T) {
	app, be, node := openForkable(t, true)
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}
	cases := map[string]bridge.ForkRequest{
		"status is text":      {From: "login", AnswerStatus: "ok"},
		"status out of range": {From: "login", AnswerStatus: "99"},
		"body is not JSON":    {From: "login", AnswerBody: "not json"},
	}

	for name, req := range cases {
		_, err := app.Flows().Fork(node, req)

		if err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if forks := be.CallsStarting("ForkReplay"); len(forks) != 0 {
		t.Fatalf("a fork started anyway: %v", forks)
	}
}

func TestFork_Given_no_answer_Then_the_recorded_response_is_kept(t *testing.T) {
	app, be, node := openForkable(t, true)
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	_, err := app.Flows().Fork(node, bridge.ForkRequest{From: "login"})

	if err != nil {
		t.Fatal(err)
	}
	forks := be.CallsStarting("ForkReplay login")
	if len(forks) != 1 || strings.Contains(forks[0], "status=") || strings.Contains(forks[0], "body=") {
		t.Fatalf("forks %v", forks)
	}
}

func TestGhostBars_should_lay_an_earlier_saved_run_behind_the_one_shown(t *testing.T) {
	start := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	timed := func(d time.Duration) report.Run {
		return report.Run{Started: start, Duration: d, Nodes: []report.Node{{ID: "login", Path: "login", Type: "request", Status: "passed", Started: start, Duration: d}}}
	}
	app, be, node := openForkable(t, true)
	be.Originals[be.Cols[0].Manifest.ID] = engine.RunResult{RunID: "now", Status: engine.StatusPassed, Report: timed(300 * time.Millisecond)}
	be.RunDetails = map[string]flowi.RunDetail{"old": {ID: "old", Report: timed(100 * time.Millisecond)}}
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	got, err := app.Flows().GhostBars(node, "old")
	_, missing := app.Flows().GhostBars(node, "gone")

	if err != nil || len(got) != 1 || got[0].ID != "login" || got[0].EndMS != 100 || got[0].DeltaMS != 200 {
		t.Fatalf("ghosts %+v err %v", got, err)
	}
	if missing == nil {
		t.Fatal("a run that cannot be read should be an error, not an empty overlay")
	}
}

func TestForkVars_should_say_which_frozen_values_have_gone_stale(t *testing.T) {
	app, be, node := openForkable(t, true)
	be.Check.ForkVars = map[string]any{"expires_at": time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339), "city": "Fresno"}
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	got, err := app.Flows().ForkVars(node, "login")

	if err != nil || len(got) != 2 {
		t.Fatalf("vars %+v err %v", got, err)
	}
	byName := map[string]bridge.VarValue{got[0].Name: got[0], got[1].Name: got[1]}
	if !strings.Contains(byName["expires_at"].Stale, "expired") || byName["city"].Stale != "" {
		t.Fatalf("vars %+v", byName)
	}
}

func TestFork_Given_a_text_answer_Then_the_body_goes_as_text_with_its_content_type(t *testing.T) {
	app, be, node := openForkable(t, true)
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	_, err := app.Flows().Fork(node, bridge.ForkRequest{From: "login", Only: true, AnswerStatus: "502", AnswerBody: "<h1>Bad gateway</h1>", AnswerText: true, AnswerContentType: "text/html"})

	if err != nil {
		t.Fatal(err)
	}
	forks := be.CallsStarting("ForkReplay login")
	if len(forks) != 1 || !strings.Contains(forks[0], "text=<h1>Bad gateway</h1>") || !strings.Contains(forks[0], "type=text/html") || strings.Contains(forks[0], "body=") {
		t.Fatalf("forks %v", forks)
	}
}

func TestFork_Given_a_text_answer_Then_it_need_not_be_json(t *testing.T) {
	app, be, node := openForkable(t, true)
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	_, err := app.Flows().Fork(node, bridge.ForkRequest{From: "login", AnswerBody: "not json", AnswerText: true})

	if err != nil || len(be.CallsStarting("ForkReplay login")) != 1 {
		t.Fatalf("err %v calls %v", err, be.Calls)
	}
}
