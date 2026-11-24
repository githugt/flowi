// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"
)

func pageFixture() pageInput {
	c := model.Collection{
		Manifest: model.Manifest{
			Meta:      model.Meta{ID: "C1", Name: "Pets", Description: "The pet shop API"},
			DependsOn: []model.Dependency{{ID: "C2", Name: "Shared", Repo: "git@example.test:shared.git"}, {ID: "C3", Name: "Gone"}},
			Variables: []model.KeyValue{{Key: "base_url", Value: "https://pets.test", Description: "where it lives"}, {Key: "token", Value: ""}},
		},
		Requests: []model.Request{{Meta: model.Meta{ID: "r1"}, Path: "pets/list"}, {Meta: model.Meta{ID: "r2"}, Path: "pets/add"}},
		Flows:    []model.Flow{{Meta: model.Meta{ID: "f1", Name: "checkout"}, Path: "checkout"}, {Meta: model.Meta{ID: "f2", Name: "soak"}, Path: "soak"}},
		Auths:    []model.NamedAuth{{Meta: model.Meta{Name: "bearer"}}},
		Broken:   []model.BrokenFile{{Path: "pets/odd", Kind: "request"}},
	}
	return pageInput{
		Collection: c,
		Specs:      []string{"pets"},
		Coverage: analysis.Coverage{Operations: []analysis.OperationCoverage{
			{Collection: "C1", Request: "pets/list", Used: []string{"checkout"}, Statuses: []analysis.StatusSeen{{Status: 200, Seen: true}, {Status: 404}}},
			{Collection: "C1", Request: "pets/add", Statuses: []analysis.StatusSeen{{Status: 201}}, Unread: []string{"$.id"}},
		}},
		LastRun: map[model.ID]*flowi.RunSummary{"f1": {Status: "failed", Env: "staging", Started: time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)}},
		Secrets: map[string][]flowi.SecretUse{
			"api_key": {{Kind: "request", Collection: "C1"}, {Kind: "request", Collection: "C1"}, {Kind: "flow", Collection: "C1"}, {Kind: "request", Collection: "OTHER"}},
			"theirs":  {{Kind: "request", Collection: "OTHER"}},
		},
		Workspace: []model.Environment{
			{Meta: model.Meta{Name: "staging"}, Vars: []model.EnvVar{{Name: "base_url", Value: "https://staging.pets.test"}, {Name: "token", Value: "s3cret", Secret: true}, {Name: "other"}}},
			{Meta: model.Meta{Name: "local"}},
		},
		Own:     []model.Environment{{Meta: model.Meta{Name: "staging"}}, {Meta: model.Meta{Name: "perf"}}},
		Mounted: map[model.ID]bool{"C2": true},
	}
}

func TestPageOf_Given_a_collection_When_built_Then_about_counts_and_dependencies(t *testing.T) {
	in := pageFixture()

	page := pageOf(in)

	if page.About.Name != "Pets" || page.About.Description != "The pet shop API" {
		t.Fatalf("about = %+v", page.About)
	}
	if got := page.About.Counts[0]; got.Label != "Requests" || got.Count != 2 {
		t.Fatalf("first count = %+v", got)
	}
	if got := page.About.Counts[2]; got.Label != "Operations" || got.Count != 2 {
		t.Fatalf("operations count = %+v", got)
	}
	if len(page.About.DependsOn) != 2 || !page.About.DependsOn[0].Mounted || page.About.DependsOn[1].Mounted {
		t.Fatalf("dependsOn = %+v", page.About.DependsOn)
	}
}

func TestPageOf_Given_operations_When_built_Then_uncalled_ones_come_first_with_their_row(t *testing.T) {
	in := pageFixture()

	st := pageOf(in).State

	if len(st.Operations) != 2 || st.Operations[0].Request != "pets/add" {
		t.Fatalf("operations = %+v", st.Operations)
	}
	if st.Operations[0].Node != "r:C1:r2" || st.Operations[0].Unread != 1 {
		t.Fatalf("first operation = %+v", st.Operations[0])
	}
	if st.Operations[1].StatusesSeen != 1 || st.Operations[1].Statuses != 2 {
		t.Fatalf("second operation = %+v", st.Operations[1])
	}
	if st.OperationsUsed != 1 || st.OperationsTotal != 2 {
		t.Fatalf("totals = %d of %d", st.OperationsUsed, st.OperationsTotal)
	}
}

func TestPageOf_Given_flows_When_built_Then_failing_and_never_run_are_counted(t *testing.T) {
	in := pageFixture()

	st := pageOf(in).State

	if st.Failing != 1 || st.NeverRan != 1 {
		t.Fatalf("failing %d, never ran %d", st.Failing, st.NeverRan)
	}
	if st.Flows[0].Name != "checkout" || st.Flows[0].Status != "failed" || st.Flows[0].Env != "staging" || st.Flows[0].Ran == "" || st.Flows[0].Node != "f:C1:f1" {
		t.Fatalf("checkout = %+v", st.Flows[0])
	}
	if st.Flows[1].Status != "" || st.Flows[1].Ran != "" {
		t.Fatalf("soak = %+v", st.Flows[1])
	}
	if len(st.Broken) != 1 || st.Broken[0] != "pets/odd" {
		t.Fatalf("broken = %v", st.Broken)
	}
}

func TestPageOf_Given_secrets_of_two_collections_When_built_Then_only_its_own_are_listed(t *testing.T) {
	in := pageFixture()

	uses := pageOf(in).Uses

	if len(uses.Secrets) != 1 || uses.Secrets[0].Name != "api_key" || uses.Secrets[0].Count != 3 || uses.Secrets[0].Where != "2 requests, 1 flow" {
		t.Fatalf("secrets = %+v", uses.Secrets)
	}
	if len(uses.Auths) != 1 || uses.Auths[0] != "bearer" {
		t.Fatalf("auths = %v", uses.Auths)
	}
}

func TestPageOf_Given_variables_When_built_Then_each_shows_the_environments_that_override_it_without_secrets(t *testing.T) {
	in := pageFixture()

	setup := pageOf(in).Setup

	base := setup.Variables[0]
	if base.Name != "base_url" || base.Value != "https://pets.test" || len(base.PerEnv) != 1 || base.PerEnv[0].Env != "staging" || base.PerEnv[0].Value != "https://staging.pets.test" {
		t.Fatalf("base_url = %+v", base)
	}
	token := setup.Variables[1]
	if len(token.PerEnv) != 1 || !token.PerEnv[0].Secret || token.PerEnv[0].Value != "" {
		t.Fatalf("token = %+v", token)
	}
	if setup.Environments[0].Name != "staging" || setup.Environments[0].Overrides != 2 || setup.Environments[1].Overrides != 0 {
		t.Fatalf("environments = %+v", setup.Environments)
	}
	if len(setup.Own) != 2 || !setup.Own[0].Have || setup.Own[1].Have {
		t.Fatalf("own = %+v", setup.Own)
	}
}

func TestPageOf_Given_an_empty_collection_When_built_Then_lists_are_empty_not_nil(t *testing.T) {
	in := pageInput{Collection: model.Collection{Manifest: model.Manifest{Meta: model.Meta{ID: "C9", Name: "Empty"}}}}

	page := emptyCollections(pageOf(in))

	if page.State.Operations == nil || page.State.Flows == nil || page.Uses.Secrets == nil || page.Setup.Variables == nil || page.About.Specs == nil {
		t.Fatalf("a list is nil: %+v", page)
	}
}

func TestPageOf_Given_spec_findings_When_built_Then_only_this_collections_are_listed_errors_first(t *testing.T) {
	in := pageFixture()
	in.Collection.Root = "/work/pets"
	at := func(file string, code diag.Code, sev diag.Severity) diag.Diagnostic {
		return diag.Diagnostic{Code: code, Severity: sev, Location: diag.Location{File: file, Path: "$.info"}, Args: map[string]string{"spec": "pets", "where": "GET /pets"}}
	}
	in.Problems = diag.List{
		at("/work/pets/descriptions/pets.openapi.yaml", diag.CodeSpecInfo, diag.SeverityWarning),
		at("/work/pets/descriptions/pets.openapi.yaml", diag.CodeSpecSummary, diag.SeverityError),
		at("/work/other/descriptions/x.openapi.yaml", diag.CodeSpecInfo, diag.SeverityWarning),
		{Code: diag.CodeWSMissing, Location: diag.Location{File: "/work/pets/flowi.yaml"}},
	}

	st := pageOf(in).State

	if st.SpecTotal != 2 || st.SpecErrors != 1 || len(st.Spec) != 2 {
		t.Fatalf("total %d errors %d list %+v", st.SpecTotal, st.SpecErrors, st.Spec)
	}
	if st.Spec[0].Severity != "error" || st.Spec[0].Message == "" || st.Spec[0].Hint == "" || st.Spec[0].Path != "$.info" {
		t.Fatalf("first = %+v", st.Spec[0])
	}
}

func TestPageOf_Given_more_spec_findings_than_the_page_lists_When_built_Then_the_rest_are_counted(t *testing.T) {
	in := pageFixture()
	in.Collection.Root = "/work/pets"
	for i := 0; i < maxSpecFindings+5; i++ {
		in.Problems = append(in.Problems, diag.Diagnostic{Code: diag.CodeSpecTags, Severity: diag.SeverityWarning, Location: diag.Location{File: "/work/pets/descriptions/a.openapi.yaml"}, Args: map[string]string{"spec": "a", "where": "GET /"}})
	}

	st := pageOf(in).State

	if len(st.Spec) != maxSpecFindings || st.SpecTotal != maxSpecFindings+5 {
		t.Fatalf("listed %d of %d", len(st.Spec), st.SpecTotal)
	}
}

func TestPageOf_Given_requests_When_built_Then_endpoints_carry_verb_url_path_tags_status_and_coverage(t *testing.T) {
	in := pageFixture()
	in.Collection.Manifest.Statuses = nil
	in.Collection.Requests = []model.Request{
		{Meta: model.Meta{ID: "r1", Name: "List pets", Description: "All pets"}, Path: "pets/list", Tags: []string{"read"}, Status: "review", Operation: "listPets", Default: "2",
			Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "{{env.base_url}}/old"}, {Label: "2", Method: "GET", URL: "{{env.base_url}}/pets?limit=5"}}},
		{Meta: model.Meta{ID: "r2", Name: "Feed"}, Path: "events/feed", Versions: []model.RequestVersion{{Label: "1", Async: &model.Async{Protocol: "ws", Channel: "feed.updates"}, Docs: "Live updates.\nMore text."}}},
		{Meta: model.Meta{ID: "r3", Name: "Top"}, Path: "top", Versions: []model.RequestVersion{{Label: "1", Method: "POST", URL: "https://api.example.test"}}},
	}
	in.Collection.Flows = []model.Flow{{Meta: model.Meta{ID: "f9", Name: "tour"}, Path: "tour", Nodes: []model.Node{
		{ID: "a", Type: "batch", Body: []model.Node{{ID: "b", Type: "request", Request: &model.RequestCall{Request: model.Ref{Path: "top"}}}}},
		{ID: "c", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "r1"}}},
	}}}
	in.Coverage = analysis.Coverage{Operations: []analysis.OperationCoverage{
		{Request: "pets/list", Used: []string{"checkout"}, Statuses: []analysis.StatusSeen{{Status: 200, Seen: true}}, Unread: []string{"$.a"}},
	}}

	got := pageOf(in).Endpoints

	if len(got) != 3 || got[0].Where != "/" || got[1].Where != "/pets" || got[2].Where != "feed.updates" && got[2].Where != "events/feed" {
		t.Fatalf("endpoints %+v", got)
	}
	pets := got[1]
	if pets.Verb != "GET" || pets.Folder != "pets" || pets.Summary != "All pets" || pets.Operation != "listPets" || len(pets.Versions) != 2 || pets.Node != "r:C1:r1" {
		t.Fatalf("pets = %+v", pets)
	}
	if len(pets.Used) != 2 || pets.Used[0] != "checkout" || pets.Used[1] != "tour" || got[0].Used[0] != "tour" || len(pets.Statuses) != 1 || !pets.Statuses[0].Seen || pets.Unread != 1 || len(pets.Tags) == 0 || pets.Status == nil || pets.Status.Name != "review" {
		t.Fatalf("pets facts = %+v", pets)
	}
	feed := got[2]
	if feed.Verb == "" || feed.Summary != "Live updates." || feed.Where != "feed.updates" {
		t.Fatalf("feed = %+v", feed)
	}
}

func TestPageURLPath_should_drop_the_host_the_template_base_and_the_query(t *testing.T) {
	for in, want := range map[string]string{
		"{{env.base_url}}/pets?x=1": "/pets", "https://h.test/a/b": "/a/b", "https://h.test": "/", "pets": "/pets", "{{env.base_url}}": "/", "/x": "/x",
	} {
		if got := pageURLPath(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// Before: the page's headline knew of failing flows and spec findings, not of a request something reads that no
// longer fits, nor of a response that moved.
func TestPageOf_Given_broken_readers_and_drift_When_built_Then_the_state_counts_the_requests_and_the_rows_carry_the_words(t *testing.T) {
	in := pageFixture()
	in.Impact = []analysis.Report{
		{CollectionID: "C1", Request: "pets/list", Versions: []analysis.VersionUsage{{Consumers: []analysis.Consumer{{Flow: "checkout", Node: "list", Status: analysis.Broken, Reason: "the request is gone"}}}}},
		{CollectionID: "C1", Request: "pets/add", Versions: []analysis.VersionUsage{{Consumers: []analysis.Consumer{{Status: analysis.Compatible}}}}},
	}
	in.Drift = []flowDrift{{flow: "f1", label: "checkout", at: time.Date(2026, 11, 24, 9, 30, 0, 0, time.Local), seen: []flowi.RequestDrift{
		{Request: "pets/list", Status: 200, Differences: []schemainfer.Difference{{Path: "$.id", Kind: schemainfer.DriftMissing}}},
	}}, {flow: "gone", label: "deleted flow", at: time.Now(), seen: []flowi.RequestDrift{
		{Request: "pets/add", Status: 201, Differences: []schemainfer.Difference{{Path: "$.x", Kind: schemainfer.DriftNew}}},
	}}}

	page := pageOf(in)

	if page.State.ImpactBroken != 1 || page.State.DriftMoved != 1 {
		t.Fatalf("state %+v, want one broken and one drifted (the deleted flow's check does not count)", page.State)
	}
	var list PageOperation
	for _, o := range page.State.Operations {
		if o.Request == "pets/list" {
			list = o
		}
	}
	if list.Impact.Broken != 1 || list.Impact.Notes[0] != "checkout · list: the request is gone" || list.Drift.Missing != 1 || list.Drift.At != "2026-11-24 09:30" {
		t.Fatalf("operation %+v", list)
	}
}
