// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

// lockedLog records what the app publishes; the run publishes from another goroutine.
type lockedLog struct {
	mu    sync.Mutex
	names []string
}

func (l *lockedLog) emit(name string, _ any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.names = append(l.names, name)
}

func (l *lockedLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.names = nil
}

func (l *lockedLog) seen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.names...)
}

// startHeldRun opens the sample flow and starts a run that stays going until the test releases it.
func startHeldRun(t *testing.T) (*bridge.App, *vmtest.Backend, string, chan struct{}, *lockedLog) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Pair"}, Path: "pair", File: "/c/flows/pair.arazzo.yaml", Nodes: []model.Node{
		{ID: "wait", Type: "delay", Params: []model.Field{{Key: "duration", Value: "1s"}}},
	}}}
	hold := make(chan struct{})
	b := &vmtest.Backend{Cols: []model.Collection{c}, NextRun: &vmtest.Running{EventList: []engine.Event{engine.NodeStarted{Path: "wait", Node: "wait"}}, Hold: hold}}
	pub := &lockedLog{}
	// A real actor: the inline one would run the held run inside Run and never return.
	app := bridge.NewApp(b, bridge.NewActor(t.Context()), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	app.Flows().Run(v.Node, nil)
	return app, b, v.Node, hold, pub
}

// runContext waits until the backend has been asked to run the flow, and returns the context it was given.
func runContext(t *testing.T, b *vmtest.Backend) context.Context {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		b.Mu.Lock()
		ctx := b.LastRunContext
		b.Mu.Unlock()
		if ctx != nil {
			return ctx
		}
	}
	t.Fatal("the flow never started")
	return nil
}

func contextEnded(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(time.Second):
		return false
	}
}

func TestFlowServiceClose_Given_a_running_flow_When_its_tab_closes_Then_the_run_stops(t *testing.T) {
	// Before: Close only deleted the document, so the run kept sending real requests and could no longer be cancelled.
	app, b, node, hold, _ := startHeldRun(t)
	defer close(hold)
	ctx := runContext(t, b)
	if got := app.Flows().Running(); len(got) != 1 || got[0] != node {
		t.Fatalf("running %v, want [%s]", got, node)
	}

	app.Flows().Close(node)

	if !contextEnded(ctx) {
		t.Fatal("closing the tab did not cancel the run")
	}
	if got := app.Flows().Running(); len(got) != 0 {
		t.Fatalf("still running %v", got)
	}
}

func TestFlowServiceClose_Given_a_closed_tab_When_a_late_event_arrives_Then_nothing_is_published(t *testing.T) {
	// Before: a late run event re-created the flow in the page after its tab was closed.
	app, b, node, hold, pub := startHeldRun(t)
	runContext(t, b)
	app.Flows().Close(node)
	pub.reset()

	close(hold) // the run's event channel ends after the tab is gone

	time.Sleep(50 * time.Millisecond)
	for _, n := range pub.seen() {
		if n == "flow" || n == "flowrun" {
			t.Fatalf("published %q for a closed tab", n)
		}
	}
}

func TestAppShutdown_Given_a_running_flow_Then_the_run_is_cancelled(t *testing.T) {
	// Before: quitting the app left a run going until the process ended.
	app, b, _, hold, _ := startHeldRun(t)
	defer close(hold)
	ctx := runContext(t, b)

	app.Shutdown()

	if !contextEnded(ctx) {
		t.Fatal("quitting did not cancel the run")
	}
}

func TestRequestService_Send_Given_two_open_requests_Then_only_the_one_that_sends_is_published(t *testing.T) {
	// Before: every send change published the view of every open request, so a live stream with 20 tabs open
	// re-sent 20 full views for each message.
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	b.Values = map[string]any{"defaults.environment": "staging"}
	b.NextSend = &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusPassed, Seed: 7,
		Record: &engine.Record{Status: 200, Version: "1", RequestPath: "auth/login", Body: engine.Body{JSON: map[string]any{"ok": true}}}}}
	var mu sync.Mutex
	published := map[string]int{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), func(name string, data any) {
		if v, ok := data.(bridge.RequestView); ok && name == "request" {
			mu.Lock()
			published[v.Node]++
			mu.Unlock()
		}
	})
	app.Start(t.Context())
	var nodes []string
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
		for _, leaf := range n.Children {
			nodes = append(nodes, leaf.ID)
		}
	}
	if len(nodes) < 2 {
		t.Fatalf("the sample has %d requests", len(nodes))
	}
	for _, n := range nodes[:2] {
		if _, err := app.Requests().Open(n); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	clear(published)
	mu.Unlock()

	app.Requests().Send(nodes[0], "")

	mu.Lock()
	defer mu.Unlock()
	if published[nodes[0]] == 0 {
		t.Fatalf("the request that sent was not published: %v", published)
	}
	if published[nodes[1]] != 0 {
		t.Fatalf("a request that did not send was published %d times", published[nodes[1]])
	}
}
