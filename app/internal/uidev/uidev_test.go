// SPDX-License-Identifier: AGPL-3.0-or-later

package uidev_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/uidev"
)

type feed struct {
	mu   sync.Mutex
	last map[string]any
}

func (f *feed) emit(name string, data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last == nil {
		f.last = map[string]any{}
	}
	f.last[name] = data
}

func (f *feed) get(name string) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.last[name]
}

func start(t *testing.T) (*bridge.App, *feed) {
	t.Helper()
	b := uidev.New()
	b.Pace = time.Nanosecond
	f := &feed{}
	app := bridge.NewApp(b, bridge.NewActor(t.Context()), f.emit)
	app.Start(t.Context())
	return app, f
}

func nodesOf(app *bridge.App, kind string) []bridge.TreeNode {
	var out []bridge.TreeNode
	var walk func([]bridge.TreeNode)
	walk = func(ns []bridge.TreeNode) {
		for _, n := range ns {
			if n.Kind == kind {
				out = append(out, n)
			}
			walk(n.Children)
		}
	}
	walk(app.Workspace().Tree().Roots)
	return out
}

func flowNode(t *testing.T, app *bridge.App, label string) string {
	t.Helper()
	for _, n := range nodesOf(app, "flow") {
		if strings.HasPrefix(n.Label, label) {
			return n.ID
		}
	}
	var labels []string
	for _, n := range nodesOf(app, "flow") {
		labels = append(labels, n.Label)
	}
	t.Fatalf("no flow %q in the tree: %v", label, labels)
	return ""
}

// run starts the flow and waits for the run to end.
func run(t *testing.T, app *bridge.App, f *feed, node string) bridge.RunView {
	t.Helper()
	app.Flows().Run(node, nil)
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if v, ok := f.get("flow").(bridge.FlowView); ok && v.Node == node && !v.Run.Running && len(v.Run.Statuses) > 0 {
			return v.Run
		}
	}
	t.Fatal("the run did not end")
	return bridge.RunView{}
}

func TestFixture_Should_show_requests_four_environments_and_two_flows_in_the_tree(t *testing.T) {
	app, _ := start(t)

	// Before: every request and flow in the tree was the shop's; the sidebar fixture adds collections of its own, so the
	// shop's are the ones whose place starts with its name.
	inShop := func(nodes []bridge.TreeNode) (n int) {
		for _, node := range nodes {
			if strings.HasPrefix(node.Where, "Demo shop") {
				n++
			}
		}
		return n
	}
	requests, flows := inShop(nodesOf(app, "request")), inShop(nodesOf(app, "flow"))
	envs := app.Workspace().Tree().Environments

	if requests != 5 || flows != 2 || len(envs) != 4 {
		t.Fatalf("%d requests, %d flows, environments %v", requests, flows, envs)
	}
}

func TestFixture_Given_the_checkout_flow_When_it_runs_Then_the_pay_step_fails_and_the_one_after_is_skipped(t *testing.T) {
	app, f := start(t)
	node := flowNode(t, app, "checkout")
	if _, err := app.Flows().Open(node); err != nil {
		t.Fatal(err)
	}

	got := run(t, app, f, node)

	if got.Statuses["login"] != "passed" || got.Statuses[uidev.FailingStep] != "failed" || got.Statuses["done"] != "skipped" {
		t.Fatalf("statuses %v", got.Statuses)
	}
}

func TestFixture_Given_the_long_flow_Then_it_has_300_steps_and_every_one_passes(t *testing.T) {
	app, f := start(t)
	node := flowNode(t, app, "soak")
	opened, err := app.Flows().Open(node)
	if err != nil {
		t.Fatal(err)
	}
	var steps int
	for _, u := range opened.Diagram.Units {
		steps += len(u.Nodes)
	}

	got := run(t, app, f, node)

	if steps != uidev.SoakSteps || len(got.Statuses) != uidev.SoakSteps {
		t.Fatalf("%d steps drawn, %d with a status", steps, len(got.Statuses))
	}
	for id, s := range got.Statuses {
		if s != "passed" {
			t.Fatalf("%s is %s", id, s)
		}
	}
}

func TestFixture_Given_a_request_When_it_is_sent_Then_the_canned_response_shows(t *testing.T) {
	app, f := start(t)
	var login string
	for _, n := range nodesOf(app, "request") {
		if n.Label == "Login" {
			login = n.ID
		}
	}
	if _, err := app.Requests().Open(login); err != nil {
		t.Fatal(err)
	}

	app.Requests().Send(login, "")

	var title string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && title == ""; time.Sleep(10 * time.Millisecond) {
		if v, ok := f.get("request").(bridge.RequestView); ok && v.Response != nil {
			title = v.Response.Title
		}
	}
	if !strings.Contains(title, "200") {
		t.Fatalf("response title %q", title)
	}
}
