// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/bridge"
)

func TestSetText_Given_bad_values_When_the_user_types_Then_each_field_refuses_what_it_cannot_hold(t *testing.T) {
	flows, v, _ := openTwoUnits(t)
	flows.Select(v.Node, "login")

	for _, tt := range []struct{ field, value string }{
		{"maxConcurrency", "-1"},
		{"maxConcurrency", "many"},
		{"timeout", "5 parsecs"},
		{"nonsense", "x"},
	} {
		if err := flows.SetText(v.Node, tt.field, tt.value); err == nil {
			t.Errorf("SetText(%s, %q) was accepted", tt.field, tt.value)
		}
	}
	for _, tt := range []struct{ field, value string }{{"maxConcurrency", "4"}, {"maxConcurrency", ""}, {"timeout", "30s"}, {"timeout", ""}} {
		if err := flows.SetText(v.Node, tt.field, tt.value); err != nil {
			t.Errorf("SetText(%s, %q): %v", tt.field, tt.value, err)
		}
	}
}

func TestSetText_Given_a_timeout_When_set_and_cleared_Then_the_form_follows(t *testing.T) {
	flows, v, pub := openTwoUnits(t)
	flows.Select(v.Node, "wait")

	_ = flows.SetText(v.Node, "timeout", "1m")
	got := lastView(pub).Form.Step.Timeout
	_ = flows.SetText(v.Node, "timeout", "")
	cleared := lastView(pub).Form.Step.Timeout

	if got != "1m0s" && got != "1m" {
		t.Errorf("timeout after set: %q", got)
	}
	if cleared != "" && cleared != "0s" {
		t.Errorf("timeout after clear: %q", cleared)
	}
}

func TestSetPairs_Given_rows_for_a_request_step_When_replaced_Then_with_and_headers_land_on_the_call(t *testing.T) {
	flows, v, pub := openTwoUnits(t)
	flows.Select(v.Node, "login")

	err := flows.SetPairs(v.Node, "with", []bridge.Pair{{Key: "user", Value: "ann"}})
	err2 := flows.SetPairs(v.Node, "headers", []bridge.Pair{{Key: "X-Trace", Value: "1"}})
	call := lastView(pub).Form.Step.Call

	if err != nil || err2 != nil {
		t.Fatalf("errors: %v, %v", err, err2)
	}
	if call == nil || !slices.Equal(call.With, []bridge.Pair{{Key: "user", Value: "ann"}}) || !slices.Equal(call.Headers, []bridge.Pair{{Key: "X-Trace", Value: "1"}}) {
		t.Fatalf("call %+v", call)
	}
	if err := flows.SetPairs(v.Node, "bogus", nil); err == nil {
		t.Fatal("an unknown editor was accepted")
	}
}

func TestSetPairs_Given_flow_inputs_and_outputs_When_replaced_Then_the_start_and_end_views_follow(t *testing.T) {
	flows, v, pub := openTwoUnits(t)

	_ = flows.SetPairs(v.Node, "inputs", []bridge.Pair{{Key: "id", Value: "7"}})
	_ = flows.SetPairs(v.Node, "outputs", []bridge.Pair{{Key: "token", Value: "{{ login.token }}"}})
	got := lastView(pub)

	if len(got.Inputs) != 1 || got.Inputs[0].Name != "id" || got.Inputs[0].Value != "7" {
		t.Fatalf("inputs %+v", got.Inputs)
	}
	if len(got.Outputs) != 1 || got.Outputs[0].Name != "token" {
		t.Fatalf("outputs %+v", got.Outputs)
	}
}

func TestSetConcurrentAndClose_Given_a_request_step_When_toggled_Then_the_form_reflects_both(t *testing.T) {
	flows, v, pub := openTwoUnits(t)
	flows.Select(v.Node, "login")

	flows.SetConcurrent(v.Node, true)
	flows.SetClose(v.Node, true)
	step := lastView(pub).Form.Step

	if !step.Concurrent || step.Call == nil || !step.Call.Close {
		t.Fatalf("concurrent %v, call %+v", step.Concurrent, step.Call)
	}
}

func TestSetCall_Given_an_unknown_reference_When_chosen_Then_the_step_keeps_its_request(t *testing.T) {
	flows, v, pub := openTwoUnits(t)
	flows.Select(v.Node, "login")
	before := lastView(pub).Form.Step.Call.Ref

	flows.SetCall(v.Node, "no-such-request")
	flows.SetCall(v.Node, "R2")
	after := lastView(pub).Form.Step.Call.Ref

	if before == "" || after == before {
		t.Fatalf("the valid choice should switch the call: %q -> %q", before, after)
	}
}

func TestFlowView_Given_a_named_step_Then_its_card_is_titled_with_the_name_and_keeps_the_id_in_the_tip(t *testing.T) {
	// Before: the Name the Inspector offers was drawn nowhere; every card was titled with its id.
	flows, v, pub := openTwoUnits(t)
	flows.Select(v.Node, "login")
	_ = flows.SetText(v.Node, "name", "Log in as admin")
	_ = flows.SetText(v.Node, "description", "uses the admin account")

	got := lastView(pub)
	var title, tip string
	for _, c := range got.Cards {
		if c.Title == "Log in as admin" {
			title, tip = c.Title, c.Tip
		}
	}

	if title == "" || !strings.Contains(tip, "(login)") || !strings.Contains(tip, "uses the admin account") {
		t.Fatalf("title %q tip %q in %+v", title, tip, got.Cards)
	}
}

func TestTargetOf_Given_a_request_step_Then_it_names_the_request_row_and_a_wait_step_names_none(t *testing.T) {
	flows, v, _ := openTwoUnits(t)

	// Before: nothing could take the user from a step to its request.
	got := flows.TargetOf(v.Node, "login")
	none := flows.TargetOf(v.Node, "wait")

	if !strings.HasPrefix(got, "r:") || strings.Count(got, ":") != 2 {
		t.Errorf("target of login %q, want r:<collection>:<id>", got)
	}
	if none != "" {
		t.Errorf("target of wait %q, want none", none)
	}
}

func TestAddRequestStep_Given_a_request_id_Then_the_new_step_calls_it_and_an_unknown_id_is_refused(t *testing.T) {
	flows, v, _ := openTwoUnits(t)
	existing := flows.TargetOf(v.Node, "login")
	id := existing[strings.LastIndex(existing, ":")+1:]

	// "Add to flow" in the tree menu does this.
	step, err := flows.AddRequestStep(v.Node, id)
	_, bad := flows.AddRequestStep(v.Node, "nope")

	if err != nil || step == "" || flows.TargetOf(v.Node, step) != existing {
		t.Fatalf("new step %q (%v) calls %q, want %q", step, err, flows.TargetOf(v.Node, step), existing)
	}
	if bad == nil {
		t.Fatal("an unknown request was accepted")
	}
}

// Before: a step's retries could not be set from the window.
func TestSetText_should_set_retries_and_their_wait_on_a_request_step_and_refuse_nonsense(t *testing.T) {
	flows, v, pub := openTwoUnits(t)
	flows.Select(v.Node, "login")

	errs := []error{flows.SetText(v.Node, "retries", "3"), flows.SetText(v.Node, "retryWait", "2s")}
	step := lastView(pub).Form.Step
	refused := []error{flows.SetText(v.Node, "retries", "11"), flows.SetText(v.Node, "retries", "x"), flows.SetText(v.Node, "retries", "-1"), flows.SetText(v.Node, "retryWait", "soon"), flows.SetText(v.Node, "retryWait", "-2s")}
	_ = flows.SetText(v.Node, "retries", "")
	cleared := lastView(pub).Form.Step
	flows.Select(v.Node, "wait")
	delay := lastView(pub).Form.Step

	if errs[0] != nil || errs[1] != nil || !step.CanRetry || step.Retries != "3" || step.RetryWait != "2s" {
		t.Fatalf("errs %v step %+v", errs, step)
	}
	for i, err := range refused {
		if err == nil {
			t.Errorf("refusal %d was accepted", i)
		}
	}
	if cleared.Retries != "" || delay.CanRetry {
		t.Errorf("cleared %q, a delay step can retry: %v", cleared.Retries, delay.CanRetry)
	}
}
