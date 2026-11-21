// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestTopSteps_Then_they_are_the_flows_own_list_in_order(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	if got := d.TopSteps(); !slices.Equal(got, []string{"a", "b", "p"}) {
		t.Fatalf("got %v", got)
	}
}

func TestCopySteps_Given_steps_with_arrows_Then_the_arrows_between_them_stay_and_the_others_go(t *testing.T) {
	// Before: the canvas had no step copy and paste, so reusing a few steps meant adding each again by hand.
	d, _, _ := newFlowDoc(t)
	d.AddNeed("p", "a") // p waits on b, written out, and now on a too

	both, err := d.CopySteps([]string{"p", "a"})
	apart, _ := d.CopySteps([]string{"b", "p"})

	if err != nil || !slices.Equal(ids(both), []string{"a", "p"}) || !slices.Equal(both[1].Needs, []string{"a"}) {
		t.Fatalf("err %v, copied %v, p waits on %v", err, ids(both), both[1].Needs)
	}
	if !slices.Equal(apart[1].Needs, []string{"b"}) {
		t.Errorf("p waits on %v: b is copied, a is not", apart[1].Needs)
	}
	if orig, _ := d.Node("p"); !slices.Equal(orig.Needs, []string{"b", "a"}) {
		t.Errorf("the flow's own step changed: %v", orig.Needs)
	}
}

func TestCopySteps_Given_a_step_inside_a_selected_container_Then_it_is_copied_once_with_the_container(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	got, err := d.CopySteps([]string{"x", "p"})

	if err != nil || len(got) != 1 || got[0].ID != "p" || len(got[0].Branches[0].Nodes) != 1 {
		t.Fatalf("err %v, copied %v", err, ids(got))
	}
}

func TestCopySteps_Given_nothing_or_a_missing_step_Then_it_says_so(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	_, none := d.CopySteps(nil)
	_, gone := d.CopySteps([]string{"zzz"})

	if none == nil || gone == nil {
		t.Fatalf("errors %v %v", none, gone)
	}
}

func TestPasteSteps_Given_copied_steps_When_pasted_after_a_step_Then_they_follow_it_with_fresh_ids_and_undo_removes_them(t *testing.T) {
	d, _, _ := newFlowDoc(t)
	copied, _ := d.CopySteps([]string{"a", "b"})

	made, err := d.PasteSteps(copied, "a")
	order := ids(d.Draft().Nodes)
	d.Undo()

	if err != nil || len(made) != 2 || made[0] == "a" || made[1] == "b" {
		t.Fatalf("err %v, made %v", err, made)
	}
	if want := []string{"a", made[0], made[1], "b", "p"}; !slices.Equal(order, want) {
		t.Fatalf("order %v, want %v", order, want)
	}
	if after := ids(d.Draft().Nodes); !slices.Equal(after, []string{"a", "b", "p"}) {
		t.Errorf("after undo %v", after)
	}
}

func TestPasteSteps_Given_steps_from_another_flow_When_pasted_Then_arrows_to_steps_this_flow_lacks_are_dropped_and_inner_ones_follow_the_new_ids(t *testing.T) {
	d, _, _ := newFlowDoc(t)
	foreign := []model.Node{
		{ID: "a", Type: "log", Needs: []string{"nowhere"}},
		{ID: "z", Type: "log", Needs: []string{"a", "b"}}, // a is pasted along (and renamed), b is in this flow
	}

	made, err := d.PasteSteps(foreign, "")
	z, _ := d.Node(made[1])

	if err != nil || len(made) != 2 {
		t.Fatalf("err %v, made %v", err, made)
	}
	first, _ := d.Node(made[0])
	if len(first.Needs) != 0 {
		t.Errorf("an arrow to a step this flow lacks was kept: %v", first.Needs)
	}
	if !slices.Equal(z.Needs, []string{made[0], "b"}) {
		t.Errorf("z waits on %v, want %v", z.Needs, []string{made[0], "b"})
	}
	if order := ids(d.Draft().Nodes); order[len(order)-1] != made[1] {
		t.Errorf("with no step to follow they go at the end: %v", order)
	}
}

func TestPasteSteps_Given_nothing_or_a_missing_step_to_follow_Then_it_refuses_and_changes_nothing(t *testing.T) {
	d, _, _ := newFlowDoc(t)

	_, empty := d.PasteSteps(nil, "a")
	_, gone := d.PasteSteps([]model.Node{{ID: "n", Type: "log"}}, "zzz")

	if empty == nil || gone == nil || !slices.Equal(ids(d.Draft().Nodes), []string{"a", "b", "p"}) {
		t.Fatalf("errors %v %v, nodes %v", empty, gone, ids(d.Draft().Nodes))
	}
}

func TestCopyText_Given_steps_When_copied_Then_the_text_is_the_backends_form_of_only_those_steps(t *testing.T) {
	// Before: a step could not be put on the clipboard, so a few steps could not go to another flow or a chat.
	d, _, _ := newFlowDoc(t)
	var text string
	var failed error

	d.CopyText([]string{"a", "b"}, func(s string) { text = s }, func(err error) { failed = err })

	if failed != nil || text != "nodes: 2\n" {
		t.Fatalf("text %q, err %v", text, failed)
	}
}

func TestCopyText_Given_no_selection_Then_it_fails_without_asking_the_backend(t *testing.T) {
	d, b, _ := newFlowDoc(t)
	var failed error

	d.CopyText(nil, func(string) { t.Fatal("copied nothing") }, func(err error) { failed = err })

	if failed == nil || slices.ContainsFunc(b.Calls, func(c string) bool { return strings.HasPrefix(c, "FlowSource") }) {
		t.Fatalf("err %v, calls %v", failed, b.Calls)
	}
}

func TestPasteText_Given_steps_text_When_pasted_Then_they_are_added_and_their_ids_come_back(t *testing.T) {
	d, b, _ := newFlowDoc(t)
	b.ParsedFlow = &model.Flow{Nodes: []model.Node{{ID: "a", Type: "log"}}}
	var got []string
	var failed error

	d.PasteText("anything", "a", func(ids []string) { got = ids }, func(err error) { failed = err })

	if failed != nil || len(got) != 1 || got[0] == "a" {
		t.Fatalf("ids %v, err %v", got, failed)
	}
	if now := ids(d.Draft().Nodes); !slices.Equal(now[:2], []string{"a", got[0]}) {
		t.Errorf("flow %v", now)
	}
}

func TestPasteText_Given_text_with_errors_or_no_steps_Then_the_flow_is_unchanged_and_it_says_why(t *testing.T) {
	d, b, _ := newFlowDoc(t)
	before := ids(d.Draft().Nodes)
	b.SourceDiags = diag.List{{Severity: diag.SeverityError, Code: "FLOW001"}}
	var broken error

	d.PasteText("https://example.com", "", func([]string) { t.Fatal("pasted a url") }, func(err error) { broken = err })
	b.SourceDiags = nil
	var empty error
	d.PasteText("", "", func([]string) { t.Fatal("pasted nothing") }, func(err error) { empty = err })

	if broken == nil || empty == nil || !slices.Equal(ids(d.Draft().Nodes), before) {
		t.Fatalf("errors %v %v, flow %v", broken, empty, ids(d.Draft().Nodes))
	}
}
