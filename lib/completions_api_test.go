// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func completionTexts(t *testing.T, w *Workspace, flow, nodePath string) []string {
	t.Helper()
	target, err := w.FindFlow(flow, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := w.Completions(target, nodePath)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range got {
		out = append(out, c.Text)
	}
	return out
}

func TestCompletions_should_offer_what_earlier_nodes_produce(t *testing.T) {
	w := demoWorkspace(t)

	got := completionTexts(t, w, "user-creation", "$.nodes[3]") // verify

	for _, want := range []string{"nodes.login.token", "nodes.login.status", "nodes.login.body.user.id", "vars.token", "vars.subject",
		"inputs.email", "env.base_url", "secret.password"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	for _, unwanted := range []string{"nodes.page-1.status", "nodes.verify.status", "vars.first_name", "env.password"} {
		if slices.Contains(got, unwanted) {
			t.Errorf("offered %s, which is not available there", unwanted)
		}
	}
}

func TestCompletions_inside_a_parallel_branch_see_earlier_top_level_nodes_only(t *testing.T) {
	w := demoWorkspace(t)

	got := completionTexts(t, w, "onboarding-check", "$.nodes[4].branches[0].nodes[0]") // notify

	if !slices.Contains(got, "nodes.profile") || !slices.Contains(got, "vars.nickname") || !slices.Contains(got, "nodes.checks") {
		t.Errorf("missing earlier nodes in %v", got)
	}
	for _, unwanted := range []string{"nodes.client", "nodes.config", "nodes.poll"} {
		if slices.ContainsFunc(got, func(s string) bool {
			return s == unwanted || len(s) > len(unwanted) && s[:len(unwanted)+1] == unwanted+"."
		}) {
			t.Errorf("offered %s in %v", unwanted, got)
		}
	}
}

func TestCompletions_inside_a_batch_offer_item_and_index(t *testing.T) {
	w := demoWorkspace(t)

	got := completionTexts(t, w, "onboarding-check", "$.nodes[3].body[1]") // double

	if !slices.Contains(got, "item") || !slices.Contains(got, "index") || !slices.Contains(got, "nodes.config") {
		t.Errorf("got %v", got)
	}
}

func TestCompletions_Given_unknown_node_path_Then_error(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindFlow("user-creation", "")

	_, err := w.Completions(target, "$.nodes[99]")

	if err == nil {
		t.Fatal("no error")
	}
}

// The canvas edits a draft: completions must follow its order, not the
// file's.
func TestCompletionsIn_use_the_draft_flow(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	draft := f
	draft.Nodes = append([]model.Node{f.Nodes[3]}, append(slices.Clone(f.Nodes[:3]), f.Nodes[4:]...)...) // verify first

	got, err := w.CompletionsIn(target.Collection, draft, "$.nodes[0]")

	for _, c := range got {
		if strings.HasPrefix(c.Text, "nodes.login") {
			t.Fatalf("offered %s before login runs in the draft", c.Text)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestResponseFields_should_offer_the_body_fields_a_request_is_known_to_return(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindRequest("auth/login", "")
	if err != nil {
		t.Fatal(err)
	}

	got, err := w.ResponseFields(target, "")

	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, c := range got {
		texts = append(texts, c.Text)
	}
	for _, want := range []string{"body", "body.data", "body.data.token"} {
		if !slices.Contains(texts, want) {
			t.Errorf("missing %s in %v", want, texts)
		}
	}
}

func TestResponseFields_should_name_a_list_by_its_first_item(t *testing.T) {
	body := map[string]any{"items": []any{map[string]any{"sku": "x"}}, "x-id": 1}

	got := bodyFieldTexts("body", body, 4)

	want := []string{"body.items", "body.items[0]", "body.items[0].sku"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v (keys that are not plain words are left out)", got, want)
	}
}
