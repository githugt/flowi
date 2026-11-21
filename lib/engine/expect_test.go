// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/blueprint"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// seeded is the post hook a new request is created with: log the status, fail unless it is a 2xx.
func seeded() model.Hooks {
	return model.Hooks{Post: []model.HookStep{{Inline: blueprint.Default().Body.Nodes}}}
}

func statusRequest(code, expect string) model.RequestVersion {
	v := get("/status/" + code)
	v.Hooks, v.Expect = seeded(), expect
	return v
}

// Before: a negative test ("a wrong password gives 401") failed on the seeded hook, so the only way to write one was
// to edit or remove that hook.
func TestSend_Given_an_expected_401_When_the_status_is_401_Then_it_passes_despite_the_seeded_hook(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("wrong-password", statusRequest("401", "401"))

	res, _ := f.send(f.input(r))

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if len(res.Checks) != 1 || res.Checks[0].Line != "expect: 401" || res.Checks[0].Verdict != CheckPassed {
		t.Fatalf("checks %+v", res.Checks)
	}
}

func TestSend_Given_an_expected_401_When_the_status_is_200_Then_it_fails_saying_what_it_got(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("wrong-password", statusRequest("200", "401"))

	res, _ := f.send(f.input(r))

	if res.Status != StatusFailed {
		t.Fatalf("status %s", res.Status)
	}
	if !slices.ContainsFunc(res.Diags, func(d diag.Diagnostic) bool {
		return d.Code == diag.CodeRunExpectFailed && d.Args["expected"] == "401" && d.Args["status"] == "200"
	}) {
		t.Fatalf("diags %v", res.Diags)
	}
	if c := res.Checks[0]; c.Verdict != CheckFailed || c.Note != "got 200" {
		t.Fatalf("check %+v", c)
	}
}

func TestSend_Given_a_range_or_a_list_Then_any_status_in_it_passes(t *testing.T) {
	for _, tt := range []struct{ code, expect string }{{"404", "4xx"}, {"418", "400-499"}, {"200", "200, 404"}, {"404", "200, 404"}} {
		f := newFixture(t)
		r := f.addRequest("probe", statusRequest(tt.code, tt.expect))

		res, _ := f.send(f.input(r))

		if res.Status != StatusPassed {
			t.Errorf("%s expect %s: %s %v", tt.code, tt.expect, res.Status, res.Diags)
		}
	}
}

// A request that says nothing keeps what it always did: the seeded hook fails a non-2xx.
func TestSend_Given_no_expect_When_the_status_is_401_Then_the_seeded_hook_still_fails_it(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("plain", statusRequest("401", ""))

	res, _ := f.send(f.input(r))

	if res.Status != StatusFailed {
		t.Fatalf("status %s", res.Status)
	}
}

func TestSend_Given_a_hook_the_user_wrote_Then_expect_leaves_it_alone(t *testing.T) {
	f := newFixture(t)
	v := get("/status/401")
	v.Expect = "401"
	v.Hooks = model.Hooks{Post: []model.HookStep{inlineHook(model.Node{ID: "mine", Type: "assert", Params: []model.Field{{Key: "check", Value: "status == 200"}}})}}
	r := f.addRequest("own-hook", v)

	res, _ := f.send(f.input(r))

	if res.Status != StatusFailed {
		t.Fatalf("the user's own hook must still run and fail: %s", res.Status)
	}
}

func TestSend_Given_a_malformed_expect_Then_it_is_refused_rather_than_guessed(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("bad", statusRequest("200", "ok"))

	res, _ := f.send(f.input(r))

	if res.Status != StatusError || !slices.ContainsFunc(res.Diags, func(d diag.Diagnostic) bool { return d.Code == diag.CodeExpectInvalid }) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestWithoutStatusHook_should_keep_the_log_node_of_the_seeded_hook(t *testing.T) {
	chain := HookChain{Post: []HookRef{{Level: levelRequest, Step: seeded().Post[0]}, {Level: levelRequest, Step: model.HookStep{Flow: &model.Ref{Path: "hooks/x"}}}}}

	got := chain.withoutStatusHook()

	if len(got.Post) != 2 || len(got.Post[0].Step.Inline) != 1 || got.Post[0].Step.Inline[0].ID != "response" {
		t.Fatalf("%+v", got.Post)
	}
	if len(chain.Post[0].Step.Inline) != 2 {
		t.Fatal("the original chain must not change")
	}
}
