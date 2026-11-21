// SPDX-License-Identifier: Apache-2.0

package flowdiff_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/flowdiff"
	"github.com/githugt/flowi/lib/model"
)

func req(id, path string) model.Node {
	return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{Path: path}}}
}

func lines(cs []flowdiff.Change) string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.String()
	}
	return strings.Join(out, "\n")
}

func TestDiff_Given_the_same_flow_Then_nothing_changed(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{req("a", "auth/login"), {ID: "b", Type: "log", Needs: []string{"a"}}}}

	got := flowdiff.Diff(f, f)

	if len(got) != 0 {
		t.Fatalf("changes: %s", lines(got))
	}
}

func TestDiff_Given_steps_added_and_removed_Then_each_is_named_by_type_and_name(t *testing.T) {
	// Before: an added step showed as lines of YAML.
	a := model.Flow{Nodes: []model.Node{req("a", "x"), {ID: "old", Type: "log"}}}
	b := model.Flow{Nodes: []model.Node{req("a", "x"), {ID: "new", Type: "assert", Name: "Status is 200"}}}

	got := lines(flowdiff.Diff(a, b))

	want := "step new added (assert \"Status is 200\")\nstep old removed (log)"
	if got != want {
		t.Fatalf("got:\n%s", got)
	}
}

func TestDiff_Given_a_step_taken_to_the_end_Then_only_that_step_is_moved(t *testing.T) {
	// Before: moving one step shifted every position, so a position diff named all of them.
	node := func(id string) model.Node { return model.Node{ID: id, Type: "log"} }
	a := model.Flow{Nodes: []model.Node{node("x"), node("a"), node("b"), node("c")}}
	b := model.Flow{Nodes: []model.Node{node("a"), node("b"), node("c"), node("x")}}

	got := flowdiff.Diff(a, b)

	if len(got) != 1 || got[0].Kind != flowdiff.Moved || got[0].Step != "x" {
		t.Fatalf("changes: %s", lines(got))
	}
}

func TestDiff_Given_a_step_moved_into_a_branch_Then_it_is_a_move_and_its_path_goes_through_the_container(t *testing.T) {
	log := model.Node{ID: "l", Type: "log"}
	a := model.Flow{Nodes: []model.Node{{ID: "if", Type: "condition", Then: []model.Node{}}, log}}
	b := model.Flow{Nodes: []model.Node{{ID: "if", Type: "condition", Then: []model.Node{log}}}}

	got := flowdiff.Diff(a, b)

	if len(got) != 1 || got[0].Kind != flowdiff.Moved || got[0].Path != "if/l" || !strings.Contains(got[0].String(), "if (then), position 1") {
		t.Fatalf("changes: %s", lines(got))
	}
}

func TestDiff_Given_a_renamed_retyped_and_rewired_step_Then_each_is_its_own_line(t *testing.T) {
	a := model.Flow{Nodes: []model.Node{{ID: "a", Type: "log"}, {ID: "b", Type: "log", Name: "Old", Needs: []string{"a"}}}}
	b := model.Flow{Nodes: []model.Node{{ID: "a", Type: "log"}, {ID: "b", Type: "note", Name: "New", AfterFailureOf: []string{"a"}, When: "{{ x }}"}}}

	got := lines(flowdiff.Diff(a, b))

	for _, want := range []string{
		`step b changed type from log to note`,
		`step b renamed from Old to New`,
		`step b: runs after removed: a`,
		`step b: runs after failure of added: a`,
		`step b: when changed from (none) to {{ x }}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestDiff_Given_a_request_step_with_changed_wiring_Then_the_change_is_key_by_key(t *testing.T) {
	a := req("r", "auth/login")
	a.Request.With = []model.KeyValue{{Key: "user", Value: "ann"}, {Key: "gone", Value: "1"}}
	a.Request.Headers = []model.KeyValue{{Key: "X-A", Value: "1"}}
	b := req("r", "auth/login-v2")
	b.Request.With = []model.KeyValue{{Key: "user", Value: "bob"}, {Key: "new", Value: "2"}}
	b.Request.Version = "v2"
	b.Request.Outcomes = []model.Outcome{{Name: "denied", When: "{{ status == 401 }}"}}

	got := lines(flowdiff.Diff(model.Flow{Nodes: []model.Node{a}}, model.Flow{Nodes: []model.Node{b}}))

	for _, want := range []string{
		`step r: request changed from auth/login to auth/login-v2`,
		`step r: request version changed from (none) to v2`,
		`step r: with user changed from ann to bob`,
		`step r: with new added: 2`,
		`step r: with gone removed: 1`,
		`step r: header X-A removed: 1`,
		`step r: outcome denied added: {{ status == 401 }}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestDiff_Given_a_flag_that_went_from_unset_to_false_Then_it_is_not_a_change(t *testing.T) {
	a := model.Flow{Nodes: []model.Node{{ID: "a", Type: "log"}}}
	b := model.Flow{Nodes: []model.Node{{ID: "a", Type: "log", Concurrent: false, Request: nil}}}

	if got := flowdiff.Diff(a, b); len(got) != 0 {
		t.Fatalf("changes: %s", lines(got))
	}
}

func TestDiff_Given_changes_to_the_flow_itself_Then_they_come_first_and_name_no_step(t *testing.T) {
	a := model.Flow{Meta: model.Meta{Name: "Signup"}, Inputs: []model.Input{{Name: "email", Type: "string"}}, Nodes: []model.Node{{ID: "a", Type: "log"}}}
	b := model.Flow{Meta: model.Meta{Name: "Sign up"}, Inputs: []model.Input{{Name: "email", Type: "string", Default: "a@b.example"}, {Name: "plan"}}, Nodes: []model.Node{{ID: "a", Type: "log", Name: "x"}}}

	got := flowdiff.Diff(a, b)

	if len(got) < 4 || got[0].Step != "" || got[0].Field != "name" || got[len(got)-1].Step != "a" {
		t.Fatalf("changes: %s", lines(got))
	}
	text := lines(got)
	for _, want := range []string{`flow: input email changed from string to string default a@b.example`, `flow: input plan added: (none)`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

func TestDiff_Given_set_values_and_cases_Then_they_are_compared_by_name(t *testing.T) {
	a := model.Flow{Nodes: []model.Node{{ID: "s", Type: "set", Values: []model.SetValue{{Name: "token", From: "response", Where: "body", Path: "$.t"}}}}}
	b := model.Flow{Nodes: []model.Node{{ID: "s", Type: "set", Values: []model.SetValue{{Name: "token", From: "response", Where: "body", Path: "$.token"}}}}}

	got := lines(flowdiff.Diff(a, b))

	if want := `step s: value token changed from response body $.t to response body $.token`; got != want {
		t.Fatalf("got:\n%s", got)
	}
}
