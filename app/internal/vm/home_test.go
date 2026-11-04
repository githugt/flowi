// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

func httpRequest() model.Request {
	return model.Request{Versions: []model.RequestVersion{{Label: "1"}}}
}

func suggestionIDs(ss []vm.FeatureIdea) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.ID)
	}
	return out
}

func TestSetupOf_should_count_what_the_workspace_holds(t *testing.T) {
	grpc := model.Request{Versions: []model.RequestVersion{{Label: "1", GRPC: &model.GRPC{}}}}
	async := model.Request{Versions: []model.RequestVersion{{Label: "1", Async: &model.Async{Protocol: "ws"}}}}
	withHooks := httpRequest()
	withHooks.Hooks.Post = []model.HookStep{{}}
	cols := []model.Collection{
		{Manifest: model.Manifest{ID: "c1"}, Requests: []model.Request{httpRequest(), withHooks, grpc, async}, Flows: []model.Flow{{}}, Blueprints: []model.Blueprint{{}}, Overlays: []model.Overlay{{}, {}}},
		{Manifest: model.Manifest{ID: "scratch", Name: "Scratch"}, Requests: []model.Request{httpRequest()}},
		// An unsaved collection the person made is still theirs: only the try-out space is left out.
		{Manifest: model.Manifest{ID: "unsaved", Name: "First Collection"}, Requests: []model.Request{httpRequest()}},
	}
	envs := []model.Environment{{Vars: []model.EnvVar{{Name: "base_url"}, {Name: "token", Secret: true}}}, {}}
	schedules := []model.Schedule{{Name: "a"}, {Name: "b", Disabled: true}}

	got := vm.SetupOf(cols, func(c model.Collection) bool { return c.Manifest.ID == "scratch" }, envs, schedules)

	want := vm.Setup{Collections: 2, Requests: 5, HTTP: 3, Messaging: 1, GRPC: 1, Flows: 1, Environments: 2, SecretVars: 1, Schedules: 1, Blueprints: 1, Overlays: 2, WithHooks: 1}
	if got != want {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestSuggestFeatures_Given_an_empty_workspace_Then_suggests_nothing_to_do_with_requests(t *testing.T) {
	got := vm.SuggestFeatures(vm.Setup{})

	if len(got) != 0 {
		t.Fatalf("an empty workspace has the get-started steps, not feature cards: %v", suggestionIDs(got))
	}
}

func TestSuggestFeatures_should_offer_a_feature_only_while_it_is_unused(t *testing.T) {
	used := vm.Setup{Collections: 1, Requests: 5, HTTP: 5, Flows: 2, Environments: 1, SecretVars: 1, Schedules: 1, Blueprints: 1, Overlays: 1, WithHooks: 1, TracingOn: true, Messaging: 1}
	idle := vm.Setup{Collections: 1, Requests: 5, HTTP: 5, Flows: 2, Environments: 1}

	if got := vm.SuggestFeatures(used); len(got) != 0 {
		t.Fatalf("everything in use, yet suggested %v", suggestionIDs(got))
	}
	got := suggestionIDs(vm.SuggestFeatures(idle))
	for _, want := range []string{"schedules", "secrets", "hooks", "overlays", "blueprints", "tracing", "protocols"} {
		if !slices.Contains(got, want) {
			t.Errorf("idle workspace lacks the %q suggestion: %v", want, got)
		}
	}
}

func TestSuggestFeatures_should_need_the_thing_a_feature_builds_on(t *testing.T) {
	noFlows := suggestionIDs(vm.SuggestFeatures(vm.Setup{Collections: 1, Requests: 5, HTTP: 5, Environments: 1}))
	oneRequest := suggestionIDs(vm.SuggestFeatures(vm.Setup{Collections: 1, Requests: 1, HTTP: 1, Environments: 1}))

	if slices.Contains(noFlows, "schedules") || slices.Contains(noFlows, "blueprints") {
		t.Errorf("schedules and blueprints need a flow: %v", noFlows)
	}
	if !slices.Contains(noFlows, "flows") {
		t.Errorf("five requests and no flow should suggest flows: %v", noFlows)
	}
	if slices.Contains(oneRequest, "flows") || slices.Contains(oneRequest, "overlays") {
		t.Errorf("one request is too early for flows or overlays: %v", oneRequest)
	}
}

func TestSuggestFeatures_should_say_why_and_where_to_go(t *testing.T) {
	for _, s := range vm.SuggestFeatures(vm.Setup{Collections: 1, Requests: 5, HTTP: 5, Flows: 1, Environments: 1}) {
		if s.Title == "" || s.Why == "" || s.Action == "" || s.Label == "" {
			t.Errorf("incomplete suggestion %+v", s)
		}
	}
}

func TestSetup_NoteSuggestions_should_remember_the_first_collection_that_has_flows_to_suggest(t *testing.T) {
	cols := []model.Collection{
		{Manifest: model.Manifest{ID: "scratch"}},
		{Manifest: model.Manifest{ID: "a"}},
		{Manifest: model.Manifest{ID: "b"}},
	}
	counts := map[model.ID]int{"scratch": 9, "a": 0, "b": 2}
	var s vm.Setup

	s.NoteSuggestions(cols, func(c model.Collection) bool { return c.Manifest.ID == "scratch" }, func(id model.ID) int { return counts[id] })

	if s.Suggestable != 2 || s.SuggestIn != "b" {
		t.Fatalf("got %d in %q", s.Suggestable, s.SuggestIn)
	}
}

func TestSuggestFeatures_Given_flows_to_suggest_and_none_made_Then_it_offers_them_in_that_collection(t *testing.T) {
	setup := vm.Setup{Collections: 1, Requests: 5, HTTP: 5, Environments: 1, Suggestable: 2, SuggestIn: "b"}

	got := vm.SuggestFeatures(setup)

	if got[0].ID != "suggested-flows" || got[0].Action != "suggest-flows:b" {
		t.Fatalf("first idea %+v", got[0])
	}
	if slices.Contains(suggestionIDs(got), "flows") {
		t.Errorf("the plain flows idea repeats the suggestion: %v", suggestionIDs(got))
	}
}

func TestSuggestFeatures_Given_flows_already_made_Then_the_suggestion_card_is_gone(t *testing.T) {
	setup := vm.Setup{Collections: 1, Requests: 5, HTTP: 5, Flows: 1, Suggestable: 2, SuggestIn: "b"}

	if got := suggestionIDs(vm.SuggestFeatures(setup)); slices.Contains(got, "suggested-flows") {
		t.Fatalf("got %v", got)
	}
}
