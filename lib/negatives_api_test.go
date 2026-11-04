// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func requestWithBody(t *testing.T, w *Workspace, body string, headers ...model.KeyValue) RequestTarget {
	t.Helper()
	target, err := w.newRequestWith(w.Collections()[0].Manifest.ID, "users/create",
		model.RequestVersion{Label: "1", Method: "POST", URL: "https://api.test/users", Headers: headers, Body: model.Body{JSON: body}})
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func overlayNames(ovs []model.Overlay) string {
	var names []string
	for _, o := range ovs {
		names = append(names, o.Name)
	}
	return strings.Join(names, ",")
}

func TestSuggestNegatives_should_break_the_fields_the_stored_schema_requires(t *testing.T) {
	w := demoWorkspace(t)
	target := requestWithBody(t, w, `{"email":"a@b.c","nick":"x"}`)
	_, r, _ := w.requestFor(target)
	_ = writeSchema(mustSchemaFile(t, r.File, SchemaPart{Kind: "request"}), map[string]any{"type": "object", "required": []any{"email"}})

	got, err := w.SuggestNegatives(target, "1")

	if err != nil || overlayNames(got) != "bad-email,bad-nick,no-email" {
		t.Fatalf("got %s err %v", overlayNames(got), err)
	}
	if got[0].Requests[0].Request != target.Request {
		t.Fatalf("the overlay must be scoped to the request: %+v", got[0].Requests)
	}
}

func mustSchemaFile(t *testing.T, requestFile string, part SchemaPart) string {
	t.Helper()
	file, err := schemaFile(requestFile, "1", part)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestSuggestNegatives_Given_an_authorization_header_Then_it_adds_the_stale_token(t *testing.T) {
	w := demoWorkspace(t)
	target := requestWithBody(t, w, `{"email":"a@b.c"}`, model.KeyValue{Key: "authorization", Value: "Bearer {{env.token}}"})

	got, err := w.SuggestNegatives(target, "1")

	if err != nil || !slices.Contains(strings.Split(overlayNames(got), ","), "stale-token") {
		t.Fatalf("got %s err %v", overlayNames(got), err)
	}
}

func TestSuggestNegatives_Given_a_body_that_is_not_json_Then_only_the_token_is_broken(t *testing.T) {
	w := demoWorkspace(t)
	target := requestWithBody(t, w, `{"n": {{env.n}}}`)

	got, err := w.SuggestNegatives(target, "1")

	if err != nil || len(got) != 0 {
		t.Fatalf("got %s err %v", overlayNames(got), err)
	}
}

func TestAcceptNegatives_should_save_each_overlay_and_leave_existing_ones_alone(t *testing.T) {
	w := demoWorkspace(t)
	target := requestWithBody(t, w, `{"email":"a@b.c"}`)
	suggested, _ := w.SuggestNegatives(target, "1")
	collection := target.Collection
	existing := model.Overlay{Name: "no-email", Description: "mine", Requests: suggested[1].Requests}
	if err := w.WriteOverlay(collection, existing, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	saved, skipped, err := w.AcceptNegatives(collection, suggested)

	if err != nil || len(saved) != 1 || saved[0] != "bad-email" || len(skipped) != 1 {
		t.Fatalf("saved %v skipped %v err %v", saved, skipped, err)
	}
	kept, _ := w.Overlay(collection, "no-email")
	if kept.Description != "mine" {
		t.Fatalf("an existing overlay was overwritten: %+v", kept)
	}
}

func TestAcceptNegatives_should_save_overlays_that_break_the_request_when_applied(t *testing.T) {
	w := demoWorkspace(t)
	target := requestWithBody(t, w, `{"email":"a@b.c","nick":"x"}`)
	suggested, _ := w.SuggestNegatives(target, "1")
	if _, _, err := w.AcceptNegatives(target.Collection, suggested); err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)

	dropped, problems, err := w.ApplyOverlay(target, "no-email", r.Versions[0])
	retyped, _, _ := w.ApplyOverlay(target, "bad-nick", r.Versions[0])

	if err != nil || len(problems) != 0 || strings.Contains(dropped.Body.JSON, "email") || !strings.Contains(dropped.Body.JSON, "nick") {
		t.Fatalf("no-email gave %s, problems %v err %v", dropped.Body.JSON, problems, err)
	}
	if !strings.Contains(retyped.Body.JSON, "12345") {
		t.Fatalf("bad-nick gave %s", retyped.Body.JSON)
	}
}
