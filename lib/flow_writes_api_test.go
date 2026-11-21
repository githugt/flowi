// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func flowTargetOf(t *testing.T, w *Workspace, path string) FlowTarget {
	t.Helper()
	for _, c := range w.Collections() {
		for _, f := range c.Flows {
			if f.Path == path {
				return FlowTarget{Collection: c.Manifest.ID, Flow: f.ID}
			}
		}
	}
	t.Fatalf("no flow %q", path)
	return FlowTarget{}
}

// Before: a flow run in a protected environment asked whether or not it wrote anything, and never said what.
func TestFlowWrites_should_list_the_steps_that_change_data_with_how(t *testing.T) {
	w := demoWorkspace(t)

	got, err := w.FlowWrites(flowTargetOf(t, w, "user-creation"), nil)

	if err != nil || len(got) == 0 {
		t.Fatalf("got %+v %v", got, err)
	}
	first := got[0]
	if first.Step != "login" || first.Request != "auth/login" || first.What != "POST" || first.Via != "" {
		t.Errorf("first %+v", first)
	}
	for _, g := range got {
		if g.What == "GET" || g.What == "" {
			t.Errorf("a read was listed: %+v", g)
		}
	}
}

func TestFlowWrites_should_follow_subflows_and_nested_steps(t *testing.T) {
	w := demoWorkspace(t)

	got, err := w.FlowWrites(flowTargetOf(t, w, "onboarding-check"), nil)

	i := slices.IndexFunc(got, func(g FlowWrite) bool { return g.Via == "shared/login" })
	if err != nil || i < 0 || got[i].What != "POST" {
		t.Fatalf("no write through the shared/login subflow: %+v %v", got, err)
	}
}

func TestFlowWrites_Given_a_draft_Then_it_is_the_draft_that_is_read(t *testing.T) {
	w := demoWorkspace(t)
	target := flowTargetOf(t, w, "user-creation")
	_, saved, _ := w.flowFor(target)
	readOnly := saved
	readOnly.Nodes = []model.Node{{ID: "cfg", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA0000000000000000000014", Path: "config"}}}}

	none, errNone := w.FlowWrites(target, &readOnly)
	some, errSome := w.FlowWrites(target, nil)

	if errNone != nil || errSome != nil || len(none) != 0 || len(some) == 0 {
		t.Fatalf("draft %+v %v, saved %+v %v", none, errNone, some, errSome)
	}
}

func TestFlowWrites_should_count_messages_sent_calls_and_writing_queries_but_not_subscriptions_or_unknown_requests(t *testing.T) {
	cases := []struct {
		name string
		v    model.RequestVersion
		want string
	}{
		{"head", model.RequestVersion{Method: "HEAD"}, ""},
		{"options", model.RequestVersion{Method: "options"}, ""},
		{"delete", model.RequestVersion{Method: "delete"}, "DELETE"},
		{"grpc", model.RequestVersion{GRPC: &model.GRPC{}}, "gRPC call"},
		{"publish", model.RequestVersion{Async: &model.Async{Protocol: "mqtt", Send: &model.AsyncSend{}}}, "MQTT message sent"},
		{"subscribe", model.RequestVersion{Async: &model.Async{Protocol: "mqtt", Receive: &model.AsyncReceive{}}}, ""},
		{"read query", model.RequestVersion{Dataset: &model.Dataset{Driver: "sqlite"}}, ""},
		{"write query", model.RequestVersion{Dataset: &model.Dataset{Driver: "sqlite", Writes: true}}, "SQL query that may change data"},
		{"write file", model.RequestVersion{File: &model.DataFile{Format: "csv", Writes: true}}, "data file query that may change the file"},
	}
	for _, c := range cases {
		c.v.Label = "1"
		coll := model.Collection{Manifest: model.Manifest{Meta: model.Meta{ID: "C1"}},
			Requests: []model.Request{{Meta: model.Meta{ID: "R1"}, Path: "r", Default: "1", Versions: []model.RequestVersion{c.v}}}}
		call := &model.RequestCall{Request: model.Ref{ID: "R1"}}

		got, ok := requestWrites([]model.Collection{coll}, coll, model.Flow{}, call)

		if got != c.want || ok != (c.want != "") {
			t.Errorf("%s: got %q %v want %q", c.name, got, ok, c.want)
		}
	}
	if _, ok := requestWrites(nil, model.Collection{}, model.Flow{}, &model.RequestCall{Request: model.Ref{ID: "nope"}}); ok {
		t.Errorf("an unknown request was counted")
	}
}
