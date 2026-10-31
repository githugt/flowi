// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/card"
	"github.com/githugt/flowi/app/internal/vm"
)

func cardCollection() model.Collection {
	return model.Collection{
		Requests: []model.Request{
			{Meta: model.Meta{ID: "R1", Name: "Ditto"}, Path: "pokemon/ditto", Default: "1", Hooks: model.Hooks{Pre: []model.HookStep{{}}}, Versions: []model.RequestVersion{{
				Label: "1", Method: "get", URL: "{{env.base_url}}/pokemon/ditto?x=1",
				Query: []model.KeyValue{{Key: "a"}, {Key: "b"}}, Headers: []model.KeyValue{{Key: "Authorization", Value: "Bearer {{secret.t}}"}, {Key: "X-A"}, {Key: "X-B"}, {Key: "X-C"}},
				Body:     model.Body{JSON: "{}"},
				Hooks:    model.Hooks{Post: []model.HookStep{{}, {}}},
				Settings: model.Settings{"retry.max": int64(2), "timeout": 10 * time.Second},
			}}},
			{Meta: model.Meta{ID: "R2"}, Path: "orders/placed", Versions: []model.RequestVersion{{
				Label: "1", Async: &model.Async{Protocol: "mqtt", Channel: "orders/placed", Send: &model.AsyncSend{Body: model.Body{JSON: "{}"}, Headers: []model.KeyValue{{Key: "type"}}},
					Binding: []model.KeyValue{{Key: "qos", Value: "1"}, {Key: "retain", Value: "true"}}},
			}}},
			{Meta: model.Meta{ID: "R3"}, Path: "svc/call", Versions: []model.RequestVersion{{
				Label: "1", GRPC: &model.GRPC{Server: "grpc://h:1", Method: "pkg.Service/Method", Body: model.Body{JSON: "{}"}},
				Headers: []model.KeyValue{{Key: "a"}, {Key: "b"}},
			}}},
		},
		Flows: []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Shared login"}, Path: "shared/login", Outputs: []model.Output{{Name: "token"}, {Name: "id"}}}},
	}
}

func TestCardFacts_Given_an_http_request_step_Then_the_facts_come_from_its_version(t *testing.T) {
	n := model.Node{ID: "n", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1"}}}

	got := vm.CardFacts(cardCollection(), n)

	want := card.Facts{Kind: "http", Method: "get", Target: "/pokemon/ditto", Auth: "Bearer", Query: 2, Headers: 3, Body: "JSON", PreHooks: 1, PostHooks: 2, Retry: 2, Timeout: "10s"}
	if got.Kind != want.Kind || got.Method != want.Method || got.Target != want.Target || got.Auth != want.Auth ||
		got.Query != want.Query || got.Headers != want.Headers || got.Body != want.Body ||
		got.PreHooks != want.PreHooks || got.PostHooks != want.PostHooks || got.Retry != want.Retry || got.Timeout != want.Timeout {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCardFacts_Given_a_message_request_Then_kind_target_and_extras_come_from_its_async_part(t *testing.T) {
	n := model.Node{ID: "n", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R2"}}}

	got := vm.CardFacts(cardCollection(), n)

	if got.Kind != "mqtt" || got.Target != "orders/placed" || !got.Sends || got.Receives || got.Body != "JSON" || got.Props != 1 {
		t.Fatalf("got %+v", got)
	}
	if strings.Join(got.SendExtras, "|") != "QoS 1|retain" {
		t.Fatalf("extras %v", got.SendExtras)
	}
}

func TestCardFacts_Given_a_grpc_request_Then_it_is_unary_with_metadata_counted(t *testing.T) {
	n := model.Node{ID: "n", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R3"}}}

	got := vm.CardFacts(cardCollection(), n)

	if got.Kind != "grpc" || got.Method != "unary" || got.Target != "pkg.Service/Method" || got.Props != 2 || got.Body != "JSON" {
		t.Fatalf("got %+v", got)
	}
}

func TestCardFacts_Given_a_flow_step_Then_the_target_is_the_flow_name_and_returns_are_counted(t *testing.T) {
	n := model.Node{ID: "n", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: "F1"}}}

	got := vm.CardFacts(cardCollection(), n)

	if got.Target != "Shared login" || got.Returns != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestCardFacts_Given_a_request_that_is_gone_Then_the_facts_are_empty_but_the_card_still_draws(t *testing.T) {
	n := model.Node{ID: "n", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "nope", Path: "old/path"}}}

	got := vm.CardFacts(cardCollection(), n)

	if got.Kind != "http" || got.Target != "old/path" {
		t.Fatalf("got %+v", got)
	}
}

func TestCardFacts_Given_the_hook_control_of_the_step_Then_only_the_hooks_that_run_are_counted(t *testing.T) {
	tests := []struct {
		name      string
		control   model.HookControl
		pre, post int
	}{
		{"all", model.HookControl{}, 1, 2},
		{"none", model.HookControl{Mode: "none"}, 0, 0},
		{"skip_pre", model.HookControl{Skip: []string{"pre"}}, 0, 2},
		{"skip_post", model.HookControl{Skip: []string{"post"}}, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := model.Node{ID: "n", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1"}, Hooks: tt.control}}

			got := vm.CardFacts(cardCollection(), n)

			if got.PreHooks != tt.pre || got.PostHooks != tt.post {
				t.Fatalf("hooks %d before, %d after; want %d and %d", got.PreHooks, got.PostHooks, tt.pre, tt.post)
			}
		})
	}
}

func TestCardFacts_Given_a_version_with_written_outcomes_Then_the_facts_carry_them_in_order(t *testing.T) {
	c := cardCollection()
	c.Requests[0].Versions[0].Outcomes = []model.Outcome{{Name: "success"}, {Name: "failure", Disabled: true}, {Name: "ok", When: "status == 200"}}
	n := model.Node{ID: "n", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1"}}}

	got := vm.CardFacts(c, n).Outcomes
	none := vm.CardFacts(cardCollection(), n).Outcomes

	want := []card.OutcomeFact{{Name: "success"}, {Name: "failure", Disabled: true}, {Name: "ok"}}
	if !reflect.DeepEqual(got, want) || none != nil {
		t.Fatalf("got %+v (want %+v), without written outcomes %+v", got, want, none)
	}
}

func TestCardFacts_Given_a_step_with_its_own_outcomes_Then_they_win_over_the_requests(t *testing.T) {
	c := cardCollection()
	c.Requests[0].Versions[0].Outcomes = []model.Outcome{{Name: "success"}, {Name: "ok", When: "status == 200"}}
	n := model.Node{ID: "n", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "R1"}, Outcomes: []model.Outcome{{Name: "failure"}, {Name: "gone", When: "status == 410"}}}}

	got := vm.CardFacts(c, n).Outcomes

	want := []card.OutcomeFact{{Name: "failure"}, {Name: "gone"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
