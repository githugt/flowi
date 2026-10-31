// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"reflect"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func asyncFlow() model.Flow {
	call := func(id, req string) model.Node {
		return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: model.ID(req)}}}
	}
	return model.Flow{
		Meta: model.Meta{ID: "01JA0000000000000000000060", Name: "Orders"}, Path: "orders", File: "orders.arazzo.yaml",
		Nodes: []model.Node{call("place", "01JA00000000000000000000A1"), call("watch", "01JA00000000000000000000A2"), call("ask", "01JA00000000000000000000A3"), call("lookup", "01JA00000000000000000000H1")},
	}
}

func asyncFlowOptions() FlowOptions {
	o := flowCodecOptions()
	o.Resolve = NewResolver(asyncCollection())
	o.Sources = append(o.Sources, SourceDescription{Name: "flowi-async", URL: "../descriptions/requests.asyncapi.yaml", Type: "asyncapi"})
	return o
}

func TestEncodeFlow_should_write_send_and_receive_steps_for_async_requests(t *testing.T) {
	got, _, err := EncodeFlow(asyncFlow(), asyncFlowOptions())

	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		"operationId: $sourceDescriptions.flowi-async.orders_placed", "action: send",
		"operationId: $sourceDescriptions.flowi-async.orders_feed", "action: receive", "timeout: 5000",
		"operationId: $sourceDescriptions.flowi.http_get", "type: asyncapi",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("document lacks %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "action:") != 3 { // place (send), watch (receive), ask (send of a request and reply)
		t.Errorf("want 3 actions:\n%s", text)
	}
}

func TestDecodeFlow_should_give_back_the_flow_it_encoded_with_async_steps(t *testing.T) {
	o := asyncFlowOptions()
	first, _, err := EncodeFlow(asyncFlow(), o)
	if err != nil {
		t.Fatal(err)
	}

	back, diags := DecodeFlow("orders.arazzo.yaml", "orders", first, o)
	second, _, _ := EncodeFlow(back, o)

	if diags.HasErrors() || len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
	if !reflect.DeepEqual(back.Nodes[0].Request, asyncFlow().Nodes[0].Request) || string(first) != string(second) {
		t.Fatalf("round trip differs:\n%s\n---\n%s", first, second)
	}
}

func TestArazzoLimits_should_name_what_an_async_step_loses(t *testing.T) {
	c := asyncCollection()
	c.Requests[2].Versions[0].Async.Receive.Match = "body.ok"

	got := Limits(asyncFlow(), NewResolver(c))

	var what []string
	for _, l := range got {
		what = append(what, l.Node+":"+l.What)
	}
	want := []string{"ask:reply", "ask:match"}
	if !reflect.DeepEqual(what, want) {
		t.Fatalf("limits %v, want %v", what, want)
	}
}

func TestArazzoLimits_Given_only_send_and_receive_steps_Then_none(t *testing.T) {
	f := asyncFlow()
	f.Nodes = f.Nodes[:2]

	if got := Limits(f, NewResolver(asyncCollection())); len(got) != 0 {
		t.Fatalf("limits %+v", got)
	}
}

func TestDecodeFlow_should_read_a_foreign_receive_step_as_a_request_node(t *testing.T) {
	doc := `arazzo: 1.1.0
info: {title: x, version: 1.0.0}
sourceDescriptions:
  - {name: orders, url: ./orders.asyncapi.yaml, type: asyncapi}
workflows:
  - workflowId: wait
    steps:
      - stepId: listen
        operationId: $sourceDescriptions.orders.orders_feed
        action: receive
        timeout: 5000
`
	o := asyncFlowOptions()

	f, diags := DecodeFlow("wait.arazzo.yaml", "wait", []byte(doc), o)

	if diags.HasErrors() || len(f.Nodes) == 0 || f.Nodes[0].Request == nil || f.Nodes[0].Request.Request.ID != "01JA00000000000000000000A2" {
		t.Fatalf("nodes %+v diags %v", f.Nodes, diags)
	}
}

func TestExport_should_declare_the_asyncapi_source_and_use_arazzo_1_1_for_async_steps(t *testing.T) {
	out, _ := Export([]model.Flow{asyncFlow()}, ExportOptions{Resolve: NewResolver(asyncCollection()), Source: "flowi"})

	text := string(out)
	for _, want := range []string{"arazzo: 1.1.0", "name: flowi-async", "type: asyncapi", "url: ./requests.asyncapi.yaml", "action: send"} {
		if !strings.Contains(text, want) {
			t.Errorf("export lacks %q:\n%s", want, text)
		}
	}
}

func TestExport_Given_only_http_steps_Then_stays_on_arazzo_1_0(t *testing.T) {
	out, _ := Export([]model.Flow{sampleFlow()}, ExportOptions{Resolve: fakeResolver{}, Source: "flowi"})

	if strings.Contains(string(out), "asyncapi") || !strings.Contains(string(out), "arazzo: "+Version) {
		t.Fatalf("export:\n%s", out)
	}
}

func TestEncodeFlow_should_write_a_receive_step_with_a_timeout_for_a_managed_queue_request(t *testing.T) {
	flow := model.Flow{
		Meta: model.Meta{ID: "01JA0000000000000000000061", Name: "Jobs"}, Path: "jobs", File: "jobs.arazzo.yaml",
		Nodes: []model.Node{{ID: "take", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA00000000000000000000B1"}}}},
	}
	o := flowCodecOptions()
	o.Resolve = NewResolver(managedCollection())
	o.Sources = append(o.Sources, SourceDescription{Name: "flowi-async", URL: "../descriptions/requests.asyncapi.yaml", Type: "asyncapi"})

	got, _, err := EncodeFlow(flow, o)

	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"operationId: $sourceDescriptions.flowi-async.queue_jobs", "action: receive", "timeout: 5000"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("document lacks %q:\n%s", want, got)
		}
	}
}
