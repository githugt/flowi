// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/schemainfer"

	"github.com/githugt/flowi/app/internal/bridge"
)

// Before: the page said which flows call an operation but not whether what they read from it still fit, or whether
// its responses had moved.
func TestCollection_Page_Given_impact_reports_Then_each_operation_counts_who_reads_it_and_what_broke(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Impacts = []analysis.Report{{CollectionID: "C1", Request: "auth/login", Versions: []analysis.VersionUsage{{
		Contract: analysis.ContractInfo{Source: analysis.SourceExample},
		Consumers: []analysis.Consumer{
			{Kind: analysis.KindNode, Flow: "pair", Node: "login", Status: analysis.Compatible},
			{Kind: analysis.KindNode, Flow: "pair", Node: "verify", Status: analysis.Broken, Reads: []analysis.Read{{Target: "$.token", Status: analysis.Broken}}},
			{Kind: analysis.KindNode, Flow: "other", Node: "x", Status: analysis.Unknown},
		},
	}}}}

	page, err := app.Collection().Page("C1")

	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(page.Endpoints, func(e bridge.PageEndpoint) bool { return e.Path == "auth/login" })
	if i < 0 {
		t.Fatalf("endpoints %+v", page.Endpoints)
	}
	got := page.Endpoints[i].Impact
	if got.Consumers != 3 || got.Broken != 1 || got.Unknown != 1 || got.Contract != "example" || len(got.Notes) != 1 || got.Notes[0] != "pair · verify reads $.token" {
		t.Fatalf("impact = %+v", got)
	}
}

func TestCollection_CheckDrift_Given_a_flow_When_checked_Then_the_page_shows_what_moved_for_each_request(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Drift = flowi.DriftReport{Flow: "pair", RunStatus: engine.StatusPassed, Requests: []flowi.RequestDrift{
		{Request: "auth/login", Version: "1", Status: 200, Differences: []schemainfer.Difference{
			{Path: "$.token", Kind: schemainfer.DriftChanged, Detail: "was string, is number"},
			{Path: "$.extra", Kind: schemainfer.DriftNew},
		}},
		{Request: "auth/verify", Version: "1", Status: 200, NoSchema: true},
	}}

	before, _ := app.Collection().Page("C1")
	run, err := app.Collection().CheckDrift("C1", "f:C1:F1")
	after, _ := app.Collection().Page("C1")

	if err != nil || run.Checked != 1 || len(run.Problems) != 0 {
		t.Fatalf("run %+v err %v", run, err)
	}
	find := func(p bridge.CollectionPage, path string) bridge.PageEndpoint {
		i := slices.IndexFunc(p.Endpoints, func(e bridge.PageEndpoint) bool { return e.Path == path })
		if i < 0 {
			t.Fatalf("no endpoint %s", path)
		}
		return p.Endpoints[i]
	}
	if find(before, "auth/login").Drift.Checked {
		t.Fatal("drift shown before any check")
	}
	login := find(after, "auth/login").Drift
	if !login.Checked || login.Changed != 1 || login.New != 1 || login.Moved() != 2 || !slices.Equal(login.Via, []string{"pair"}) || login.At == "" {
		t.Fatalf("login drift = %+v", login)
	}
	if len(login.Notes) != 2 || login.Notes[0] != "200 $.token: was string, is number" {
		t.Fatalf("notes = %v", login.Notes)
	}
	if v := find(after, "auth/verify").Drift; !v.Checked || !v.NoSchema || v.Moved() != 0 {
		t.Fatalf("verify drift = %+v", v)
	}
}

func TestCollection_CheckDrift_Given_an_unknown_flow_or_collection_Then_an_error(t *testing.T) {
	app, _ := checksApp(t)

	if _, err := app.Collection().CheckDrift("C1", "f:C1:nope"); err == nil {
		t.Fatal("an unknown flow was checked")
	}
	if _, err := app.Collection().CheckDrift("nope", ""); err == nil {
		t.Fatal("an unknown collection was checked")
	}
}

func TestCollection_CheckDrift_Given_a_failing_check_Then_the_problem_is_reported_and_the_rest_still_run(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Err = errors.New("the flow would not run")

	run, err := app.Collection().CheckDrift("C1", "")

	if err != nil || run.Checked != 0 || len(run.Problems) != 1 {
		t.Fatalf("run %+v err %v", run, err)
	}
}

// Before: the page could not hand a description to anyone outside Flowi.
func TestCollection_SaveExport_Given_a_description_When_saved_Then_the_dialog_starts_with_its_name_and_the_file_has_its_text(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Exports = []flowi.ExportItem{{ID: "generated", Label: "The requests as Flowi sends them (OpenAPI)", File: "requests.openapi.yaml"}}
	b.Check.Exported = map[string]string{"generated": "openapi: 3.0.3\n"}
	target := filepath.Join(t.TempDir(), "out.yaml")
	offered := ""
	app.SetPickers(bridge.Pickers{Save: func(_, name string) (string, error) { offered = name; return target, nil }})

	page, _ := app.Collection().Page("C1")
	path, err := app.Collection().SaveExport("C1", "generated")

	if err != nil || path != target || offered != "requests.openapi.yaml" {
		t.Fatalf("path %q offered %q err %v", path, offered, err)
	}
	if data, _ := os.ReadFile(target); string(data) != "openapi: 3.0.3\n" {
		t.Fatalf("file = %q", data)
	}
	if len(page.About.Exports) != 1 || page.About.Exports[0].ID != "generated" {
		t.Fatalf("exports = %+v", page.About.Exports)
	}
}

func TestCollection_SaveExport_Given_a_cancelled_dialog_or_an_unknown_id_Then_nothing_is_written(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Exports = []flowi.ExportItem{{ID: "generated", File: "requests.openapi.yaml"}}
	b.Check.Exported = map[string]string{"generated": "x"}

	if _, err := app.Collection().SaveExport("C1", "generated"); err == nil {
		t.Fatal("no dialog, no error")
	}
	app.SetPickers(bridge.Pickers{Save: func(_, _ string) (string, error) { return "", nil }})
	if path, err := app.Collection().SaveExport("C1", "generated"); err != nil || path != "" {
		t.Fatalf("cancelled: %q %v", path, err)
	}
	if _, err := app.Collection().SaveExport("C1", "kept:nope"); !errors.Is(err, flowi.ErrNoExport) {
		t.Fatalf("unknown id: %v", err)
	}
}
