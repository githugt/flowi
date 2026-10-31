// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"reflect"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func grpcCollection() model.Collection {
	c := asyncCollection()
	c.Requests = append(c.Requests, model.Request{
		Meta: model.Meta{ID: "01JA00000000000000000000G1", Name: "get"}, Path: "orders/get",
		Versions: []model.RequestVersion{{Label: "1", GRPC: &model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Body: model.Body{JSON: `{"id":1}`}}}},
	})
	return c
}

func grpcFlow() model.Flow {
	return model.Flow{
		Meta: model.Meta{ID: "01JA0000000000000000000062", Name: "Calls"}, Path: "calls", File: "calls.arazzo.yaml",
		Nodes: []model.Node{
			{ID: "place", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA00000000000000000000A1"}}},
			{ID: "call", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA00000000000000000000G1"}}},
		},
	}
}

func grpcFlowOptions() FlowOptions {
	o := flowCodecOptions()
	o.Resolve = NewResolver(grpcCollection())
	o.Sources = append(o.Sources, SourceDescription{Name: "flowi-async", URL: "../descriptions/requests.asyncapi.yaml", Type: "asyncapi"})
	return o
}

func TestEncodeFlow_should_write_a_grpc_request_as_a_custom_step_and_keep_the_rest_standard(t *testing.T) {
	got, _, err := EncodeFlow(grpcFlow(), grpcFlowOptions())

	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "operationId: $sourceDescriptions.flowi-async.orders_placed") {
		t.Errorf("the async step is still standard:\n%s", text)
	}
	if !strings.Contains(text, "operationId: $sourceDescriptions.flowi."+BuiltinOperation) {
		t.Errorf("the gRPC call is not a custom step:\n%s", text)
	}
	if strings.Contains(text, "orders/get") && strings.Contains(text, "a.B/C") && !strings.Contains(text, "x-flowi-flow") {
		t.Errorf("the call must live in x-flowi-flow")
	}
}

func TestDecodeFlow_should_give_back_the_flow_it_encoded_with_a_grpc_step(t *testing.T) {
	o := grpcFlowOptions()
	first, _, err := EncodeFlow(grpcFlow(), o)
	if err != nil {
		t.Fatal(err)
	}

	back, diags := DecodeFlow("calls.arazzo.yaml", "calls", first, o)
	second, _, _ := EncodeFlow(back, o)

	if diags.HasErrors() || len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
	if !reflect.DeepEqual(back.Nodes[1].Request, grpcFlow().Nodes[1].Request) || string(first) != string(second) {
		t.Fatalf("round trip differs:\n%s\n---\n%s", first, second)
	}
}

func TestArazzoLimits_should_name_a_grpc_call_as_something_other_tools_do_not_see(t *testing.T) {
	got := Limits(grpcFlow(), NewResolver(grpcCollection()))

	var what []string
	for _, l := range got {
		what = append(what, l.Node+":"+l.What)
	}
	if !reflect.DeepEqual(what, []string{"call:gRPC call"}) {
		t.Fatalf("limits %v", what)
	}
}
