// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"reflect"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func datasetCollection() model.Collection {
	c := asyncCollection()
	c.Requests = append(c.Requests, model.Request{
		Meta: model.Meta{ID: "01JA00000000000000000000D1", Name: "customers"}, Path: "customers/list",
		Versions: []model.RequestVersion{{Label: "1", Dataset: &model.Dataset{Driver: "sqlite", Server: "shop.db", Query: "SELECT 1"}}},
	})
	return c
}

func datasetFlow() model.Flow {
	return model.Flow{
		Meta: model.Meta{ID: "01JA0000000000000000000063", Name: "Reads"}, Path: "reads", File: "reads.arazzo.yaml",
		Nodes: []model.Node{
			{ID: "place", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA00000000000000000000A1"}}},
			{ID: "read", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA00000000000000000000D1"}}},
		},
	}
}

func datasetFlowOptions() FlowOptions {
	o := flowCodecOptions()
	o.Resolve = NewResolver(datasetCollection())
	o.Sources = append(o.Sources, SourceDescription{Name: "flowi-async", URL: "../descriptions/requests.asyncapi.yaml", Type: "asyncapi"})
	return o
}

func TestEncodeFlow_should_write_a_dataset_request_as_a_custom_step_and_say_it_is_a_limit(t *testing.T) {
	got, _, err := EncodeFlow(datasetFlow(), datasetFlowOptions())
	limits := Limits(datasetFlow(), datasetFlowOptions().Resolve)

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "operationId: $sourceDescriptions.flowi."+BuiltinOperation) {
		t.Errorf("the dataset query is not a custom step:\n%s", got)
	}
	if len(limits) != 1 || limits[0].Node != "read" || limits[0].What != "dataset query" {
		t.Errorf("limits %+v", limits)
	}
}

func TestDecodeFlow_should_give_back_the_flow_it_encoded_with_a_dataset_step(t *testing.T) {
	o := datasetFlowOptions()
	first, _, err := EncodeFlow(datasetFlow(), o)
	if err != nil {
		t.Fatal(err)
	}

	back, diags := DecodeFlow("reads.arazzo.yaml", "reads", first, o)
	second, _, _ := EncodeFlow(back, o)

	if diags.HasErrors() || len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
	if !reflect.DeepEqual(back.Nodes[1].Request, datasetFlow().Nodes[1].Request) || string(first) != string(second) {
		t.Fatalf("not the same flow:\n%s\n---\n%s", first, second)
	}
}

func TestDescribe_should_leave_a_dataset_request_out_of_the_described_operations(t *testing.T) {
	ops := operationsOf(datasetCollection())

	for _, op := range ops {
		if op.req.Path == "customers/list" {
			t.Fatalf("a dataset request is described as %s", op.id)
		}
	}
}
