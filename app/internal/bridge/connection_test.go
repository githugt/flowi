// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// openBroker opens a message request whose broker is the backend's to answer for.
func openBroker(t *testing.T, b *vmtest.Backend) (*bridge.RequestService, string, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "R8", Name: "Orders"}, Path: "orders", File: "/c/requests/orders.req.yaml", Default: "1",
		Versions: []model.RequestVersion{{Label: "1", Async: &model.Async{Protocol: "mqtt", Server: "mqtt://broker:1883", Channel: "orders"}}}})
	b.Cols = []model.Collection{c}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
		if n.Label == "Orders" {
			if _, err := app.Requests().Open(n.ID); err != nil {
				t.Fatal(err)
			}
			return app.Requests(), n.ID, pub
		}
	}
	t.Fatal("no Orders request")
	return nil, "", nil
}

func TestRequestService_TestConnection_should_show_the_verdict_until_the_server_changes(t *testing.T) {
	// Before: no protocol had a "Test connection"; a wrong broker showed only after a full send or a 30 s timeout.
	b := &vmtest.Backend{ConnectionProbe: engine.Probe{Status: engine.ProbeOK, Target: "mqtt://broker:1883", Detail: "connected and signed in", Took: 23 * time.Millisecond}}
	reqs, node, pub := openBroker(t, b)
	before := pub.last["request"]

	reqs.TestConnection(node)
	tested := pub.last["request"].(bridge.RequestView).Connection
	block := pub.last["request"].(bridge.RequestView).Async.Block
	block.Server = "mqtt://other:1883"
	reqs.SetAsync(node, block)
	stale := pub.last["request"].(bridge.RequestView).Connection

	if before != nil && before.(bridge.RequestView).Connection == nil {
		t.Fatal("a message request offers the test from the start")
	}
	if tested == nil || tested.Status != "ok" || tested.Target != "mqtt://broker:1883" || tested.TookMs != 23 || tested.Busy {
		t.Fatalf("tested %+v", tested)
	}
	if stale == nil || stale.Status != "" {
		t.Fatalf("after the server changed the old verdict must not stand: %+v", stale)
	}
	if n := len(slices.DeleteFunc(slices.Clone(b.Calls), func(c string) bool { return c != "TestConnection" })); n != 1 {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestRequestService_TestConnection_should_report_a_refusal_and_a_failure_to_test(t *testing.T) {
	b := &vmtest.Backend{ConnectionProbe: engine.Probe{Status: engine.ProbeFailed, Target: "mqtt://broker:1883", Detail: "not authorized"}}
	reqs, node, pub := openBroker(t, b)

	reqs.TestConnection(node)
	refused := pub.last["request"].(bridge.RequestView).Connection
	b.ConnectionErr = errors.New("the vault is locked")
	reqs.TestConnection(node)
	failed := pub.last["request"].(bridge.RequestView).Connection

	if refused.Status != "failed" || refused.Detail != "not authorized" {
		t.Fatalf("refused %+v", refused)
	}
	if failed.Status != "failed" || failed.Detail == "" {
		t.Fatalf("failed %+v", failed)
	}
}

func TestRequestService_Open_Given_an_http_request_Then_it_has_no_connection_test(t *testing.T) {
	pub := &published{}
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	var node string
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
		if n.Kind == "request" {
			node = n.ID
			break
		}
	}

	if _, err := app.Requests().Open(node); err != nil {
		t.Fatal(err)
	}

	if v, ok := pub.last["request"].(bridge.RequestView); !ok || v.Connection != nil {
		t.Fatalf("view %+v", pub.last["request"])
	}
}
