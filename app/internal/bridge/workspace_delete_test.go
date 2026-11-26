// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
)

func TestDeletedNotice_should_name_one_item_and_count_several(t *testing.T) {
	one := deletedNotice([]flowi.TrashedItem{{ID: "t1", Name: "Login"}})
	unnamed := deletedNotice([]flowi.TrashedItem{{ID: "t2", Path: "auth/login"}})
	many := deletedNotice([]flowi.TrashedItem{{ID: "t3"}, {ID: "t4"}, {ID: "t5"}})
	none := deletedNotice(nil)

	if one.Text != "Deleted Login" || !slices.Equal(one.IDs, []string{"t1"}) {
		t.Errorf("one: %+v", one)
	}
	if unnamed.Text != "Deleted auth/login" {
		t.Errorf("unnamed: %+v", unnamed)
	}
	if many.Text != "Deleted 3 items" || len(many.IDs) != 3 {
		t.Errorf("many: %+v", many)
	}
	if none.Text != "" || len(none.IDs) != 0 {
		t.Errorf("none: %+v", none)
	}
}

// Before: every delete asked "This cannot be undone", including the ones that can now be undone.
func TestLeafDeleteQuestion_should_ask_only_about_what_is_removed_for_good(t *testing.T) {
	if got := leafDeleteQuestion(nil, 3, nil); got != "" {
		t.Errorf("requests and flows go to the trash and need no question: %q", got)
	}
	env := leafDeleteQuestion([]string{"staging"}, 0, nil)
	mixed := leafDeleteQuestion([]string{"staging"}, 2, nil)

	if !strings.Contains(env, "staging") || !strings.Contains(env, "cannot be undone") {
		t.Errorf("environment: %q", env)
	}
	if !strings.Contains(mixed, "Trash") {
		t.Errorf("mixed: %q must say the others can be brought back", mixed)
	}
}

func TestDelete_menu_should_not_ask_about_a_request_but_still_asks_about_an_environment(t *testing.T) {
	app, _, _ := twoCollections(t)
	login := rowOf(t, app, "request", "Login")
	env := rowOf(t, app, "environment", "local")

	confirmOf := func(ids ...string) string {
		for _, e := range app.Workspace().Menu(ids) {
			if e.Action == "delete" {
				return e.Confirm
			}
		}
		t.Fatalf("no delete entry for %v", ids)
		return ""
	}

	if got := confirmOf(login); got != "" {
		t.Errorf("a request: %q", got)
	}
	if got := confirmOf(env); !strings.Contains(got, "cannot be undone") {
		t.Errorf("an environment: %q", got)
	}
}

func TestDelete_then_Undo_should_bring_the_request_back_and_announce_the_delete(t *testing.T) {
	app, pub, _ := twoCollections(t)
	login := rowOf(t, app, "request", "Login")

	app.Workspace().Do("delete", []string{login}, "", "")

	d, ok := pub.last["deleted"].(Deleted)
	if !ok || len(d.IDs) != 1 || !strings.HasPrefix(d.Text, "Deleted ") {
		t.Fatalf("the window was told %+v", pub.last["deleted"])
	}
	if len(app.Workspace().Tree().Roots) == 0 || hasRow(app, "request", "Login") {
		t.Fatal("the request must be gone from the tree")
	}
	if problem := app.Trash().Undo(d.IDs); problem != "" {
		t.Fatalf("undo: %s", problem)
	}
	if !hasRow(app, "request", "Login") {
		t.Fatal("undo must put the request back in the tree")
	}
}

func hasRow(app *App, kind, label string) bool {
	var walk func([]TreeNode) bool
	walk = func(nodes []TreeNode) bool {
		for _, n := range nodes {
			if n.Kind == kind && n.Label == label || walk(n.Children) {
				return true
			}
		}
		return false
	}
	return walk(app.Workspace().Tree().Roots)
}

func TestEnvEffects_Given_secrets_and_children_Then_the_question_names_them(t *testing.T) {
	// Before: the question named the environment and nothing of what depended on it.
	none := envEffects("staging", flowi.EnvImpact{})
	both := envEffects("staging", flowi.EnvImpact{Secrets: []string{"token", "key"}, Dependents: []string{"prod"}})
	several := envEffects("staging", flowi.EnvImpact{Dependents: []string{"a", "b"}})

	if none != "" {
		t.Errorf("nothing to say: %q", none)
	}
	for _, want := range []string{"2 secrets", "token, key", "stored values", "prod builds on staging"} {
		if !strings.Contains(both, want) {
			t.Errorf("%q is missing %q", both, want)
		}
	}
	if !strings.Contains(several, "a, b build on staging") {
		t.Errorf("several: %q", several)
	}
	q := leafDeleteQuestion([]string{"staging"}, 0, []string{both})
	if !strings.Contains(q, "cannot be undone") || !strings.Contains(q, "token, key") {
		t.Errorf("question %q", q)
	}
}

func TestDelete_menu_for_an_environment_should_name_its_secrets_and_the_environments_that_build_on_it(t *testing.T) {
	app, _, b := twoCollections(t)
	env := rowOf(t, app, "environment", "local")
	b.EnvImpacts = map[string]flowi.EnvImpact{app.workspace.Node(env).File: {Secrets: []string{"password"}, Dependents: []string{"qa"}}}

	var got string
	for _, e := range app.Workspace().Menu([]string{env}) {
		if e.Action == "delete" {
			got = e.Confirm
		}
	}

	for _, want := range []string{"cannot be undone", "password", "stored values", "qa builds on local"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing %q", got, want)
		}
	}
}

func deleteEntry(t *testing.T, app *App, ids ...string) MenuEntry {
	t.Helper()
	for _, e := range app.Workspace().Menu(ids) {
		if e.Action == "delete" {
			return e
		}
	}
	t.Fatalf("no delete entry for %v", ids)
	return MenuEntry{}
}

func TestUsesText_Given_steps_Then_names_each_place_and_what_happens(t *testing.T) {
	// Before: deleting a request that flows ran said nothing, and each of them showed "unknown request id" afterwards.
	none := usesText(nil)
	one := usesText([]deleteUse{{Request: "Login", Where: "Shop › signup, step login"}})
	many := usesText([]deleteUse{{"Login", "Shop › a, step x"}, {"Login", "Shop › b, step y"}})
	mixed := usesText([]deleteUse{{"Login", "Shop › a, step x"}, {"Verify", "Shop › a, step z"}})

	if none != "" {
		t.Errorf("nothing uses it: %q", none)
	}
	for _, want := range []string{"Login is used by 1 step:", "• Shop › signup, step login", "show an error"} {
		if !strings.Contains(one, want) {
			t.Errorf("one: %q is missing %q", one, want)
		}
	}
	if !strings.Contains(many, "Login is used by 2 steps:") {
		t.Errorf("many: %q", many)
	}
	if !strings.Contains(mixed, "2 steps run the requests you are deleting:") || !strings.Contains(mixed, "(Verify)") {
		t.Errorf("mixed: %q", mixed)
	}
}

func TestUsesText_Given_more_steps_than_it_lists_Then_counts_the_rest(t *testing.T) {
	var uses []deleteUse
	for i := range maxUsesListed + 3 {
		uses = append(uses, deleteUse{Request: "Login", Where: fmt.Sprintf("Shop › flow%d, step s", i)})
	}

	got := usesText(uses)

	if !strings.Contains(got, "and 3 more") || strings.Contains(got, fmt.Sprintf("flow%d,", maxUsesListed)) {
		t.Errorf("got %q", got)
	}
}

func TestDelete_menu_Given_a_flow_runs_the_request_Then_the_question_names_the_flow_and_the_button_says_anyway(t *testing.T) {
	// Before: a request went to the trash without a word, and the flow that ran it broke.
	app, _, b := twoCollections(t)
	login := rowOf(t, app, "request", "Login")
	n := app.workspace.Node(login)
	b.CallersOf = map[flowi.RequestTarget][]analysis.Caller{
		{Collection: n.Collection, Request: n.Request}: {{Collection: "Shop", File: "/s/flows/signup.arazzo.yaml", Flow: "signup", Node: "login"}},
	}

	e := deleteEntry(t, app, login)

	for _, want := range []string{"Login is used by 1 step", "Shop › signup, step login", "Undo brings the request back"} {
		if !strings.Contains(e.Confirm, want) {
			t.Errorf("%q is missing %q", e.Confirm, want)
		}
	}
	if e.ConfirmAction != "Delete anyway" || e.ConfirmTitle == "" {
		t.Errorf("title %q, button %q", e.ConfirmTitle, e.ConfirmAction)
	}
}

func TestDelete_menu_Given_the_only_user_is_a_flow_deleted_too_Then_nothing_is_asked(t *testing.T) {
	// Before: a step that goes with its flow counted as left pointing at nothing.
	app, _, b := twoCollections(t)
	login := rowOf(t, app, "request", "Login")
	flow := firstRow(t, app, "flow")
	n := app.workspace.Node(login)
	b.CallersOf = map[flowi.RequestTarget][]analysis.Caller{
		{Collection: n.Collection, Request: n.Request}: {{Collection: "Shop", File: app.workspace.Node(flow).File, Flow: "signup", Node: "login"}},
	}

	e := deleteEntry(t, app, login, flow)

	if e.Confirm != "" || e.ConfirmAction != "" {
		t.Errorf("both go together, so no step is left behind: %q %q", e.Confirm, e.ConfirmAction)
	}
}

func TestDelete_menu_Given_no_flow_runs_the_request_Then_it_still_goes_without_a_question(t *testing.T) {
	app, _, _ := twoCollections(t)
	login := rowOf(t, app, "request", "Login")

	e := deleteEntry(t, app, login)

	if e.Confirm != "" || e.ConfirmAction != "" || e.ConfirmTitle != "" {
		t.Errorf("%+v", e)
	}
}

// firstRow finds the first tree row of a kind, whatever its label.
func firstRow(t *testing.T, app *App, kind string) string {
	t.Helper()
	var walk func([]TreeNode) string
	walk = func(nodes []TreeNode) string {
		for _, n := range nodes {
			if n.Kind == kind {
				return n.ID
			}
			if id := walk(n.Children); id != "" {
				return id
			}
		}
		return ""
	}
	if id := walk(app.Workspace().Tree().Roots); id != "" {
		return id
	}
	t.Fatalf("no %s row", kind)
	return ""
}
