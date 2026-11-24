// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func hoverCollection() model.Collection {
	c := vmtest.SampleCollection()
	c.Folders = []model.Folder{{Dir: "requests/auth", Status: "review", Tags: []string{"smoke"}}}
	c.Requests = append(c.Requests,
		model.Request{Meta: model.Meta{ID: "R4", Name: "Orders", Description: "Lists the open orders."}, Path: "orders", File: "/c/requests/orders.req.yaml", Tags: []string{"slow"}, Versions: []model.RequestVersion{
			{Label: "1", Method: "GET", URL: "https://api.test/orders", Assert: []string{"status == 200", "body.count > 0"}, Auth: &model.Auth{Type: "bearer"}},
			{Label: "2", Method: "GET", URL: "https://api.test/v2/orders"},
		}, Default: "1"},
		model.Request{Meta: model.Meta{ID: "R5", Name: "Ping"}, Path: "grpc/ping", File: "/c/requests/grpc/ping.req.yaml", Versions: []model.RequestVersion{
			{Label: "1", GRPC: &model.GRPC{Server: "grpc://svc:9000", Method: "pkg.Health/Ping"}}}},
		model.Request{Meta: model.Meta{ID: "R6", Name: "Count"}, Path: "db/count", File: "/c/requests/db/count.req.yaml", Versions: []model.RequestVersion{
			{Label: "1", Dataset: &model.Dataset{Driver: "postgres", Server: "postgres://app:hunter2@db:5432/shop", Query: "SELECT count(*)\n  FROM orders"}}}},
		model.Request{Meta: model.Meta{ID: "R7", Name: "Docs only"}, Path: "docs-only", File: "/c/requests/docs-only.req.yaml", Versions: []model.RequestVersion{
			{Label: "1", Method: "GET", URL: "/x", Docs: "# Title\n\nFetches the thing\nover two lines.\n\nMore."}}},
	)
	return c
}

func hoverWorkspace(t *testing.T) *vm.Workspace {
	t.Helper()
	ws, _ := loadedWorkspace(t, &vmtest.Backend{Cols: []model.Collection{hoverCollection()}})
	return ws
}

func hasFact(card vm.HoverCard, label, contains string) bool {
	for _, f := range card.Facts {
		if f.Label == label && strings.Contains(f.Value, contains) {
			return true
		}
	}
	return false
}

// Before: the hover text of a row repeated the name the row already showed.
func TestHover_Given_an_http_request_When_hovered_Then_it_shows_url_description_checks_and_labels(t *testing.T) {
	ws := hoverWorkspace(t)

	card := ws.Hover("r:C1:R4", nil)

	if card.Verb != "GET" || card.Kind != "HTTP request" || card.TargetLabel != "URL" || card.Target != "https://api.test/orders" {
		t.Errorf("card %+v", card)
	}
	if card.Description != "Lists the open orders." || card.Where != "Shop" {
		t.Errorf("description %q where %q", card.Description, card.Where)
	}
	for _, want := range [][2]string{{"Versions", "2, opens on 1"}, {"Auth", "bearer"}, {"Checks", "2"}, {"Tags", "slow"}} {
		if !hasFact(card, want[0], want[1]) {
			t.Errorf("no %s fact %q in %+v", want[0], want[1], card.Facts)
		}
	}
}

func TestHover_Given_a_request_in_a_status_folder_When_hovered_Then_status_names_the_folder_and_tags_add_up(t *testing.T) {
	ws := hoverWorkspace(t)

	card := ws.Hover("r:C1:R1", nil)

	if !hasFact(card, "Status", "review (from the folder auth)") || !hasFact(card, "Tags", "smoke") || card.Where != "Shop / auth" {
		t.Errorf("card %+v", card)
	}
}

func TestHover_Given_a_grpc_request_When_hovered_Then_it_shows_server_and_call(t *testing.T) {
	ws := hoverWorkspace(t)

	card := ws.Hover("r:C1:R5", nil)

	if card.Kind != "gRPC request" || card.Target != "grpc://svc:9000" || !hasFact(card, "Call", "pkg.Health/Ping") {
		t.Errorf("card %+v", card)
	}
}

// Before: n/a (new). A connection string in a hover card is shown to anyone looking at the screen: the password stays out.
func TestHover_Given_a_database_request_with_a_password_When_hovered_Then_the_password_is_hidden_and_the_query_on_one_line(t *testing.T) {
	ws := hoverWorkspace(t)

	card := ws.Hover("r:C1:R6", nil)

	if strings.Contains(card.Target, "hunter2") || card.Target != "postgres://app:***@db:5432/shop" {
		t.Errorf("target %q", card.Target)
	}
	if !hasFact(card, "Query", "SELECT count(*) FROM orders") || !hasFact(card, "Database", "postgres") {
		t.Errorf("facts %+v", card.Facts)
	}
}

func TestHover_Given_no_description_When_hovered_Then_the_first_paragraph_of_the_docs_stands_in(t *testing.T) {
	ws := hoverWorkspace(t)

	card := ws.Hover("r:C1:R7", nil)

	if card.Description != "Fetches the thing over two lines." {
		t.Errorf("description %q", card.Description)
	}
}

func TestHover_Given_problems_When_hovered_Then_the_first_few_are_listed_with_code_and_hint_and_the_rest_counted(t *testing.T) {
	ws := hoverWorkspace(t)
	var problems []vm.Problem
	for range 6 {
		problems = append(problems, vm.Problem{Severity: "error", Code: "FLW-X-001", Brief: "bad thing", Hint: "do that"})
	}

	card := ws.Hover("r:C1:R1", problems)

	if len(card.Problems) != 4 || card.MoreProblems != 2 {
		t.Fatalf("%d listed, %d more", len(card.Problems), card.MoreProblems)
	}
	if p := card.Problems[0]; p.Code != "FLW-X-001" || p.Message != "bad thing" || p.Hint != "do that" {
		t.Errorf("problem %+v", p)
	}
}

func TestHover_Given_a_flow_When_hovered_Then_it_shows_flow_and_its_steps(t *testing.T) {
	c := hoverCollection()
	c.Flows[0].Nodes = []model.Node{{}, {}, {}}
	c.Flows[0].Description = "Signs up a user."
	ws, _ := loadedWorkspace(t, &vmtest.Backend{Cols: []model.Collection{c}})

	card := ws.Hover("f:C1:F1", nil)

	if card.Kind != "Flow" || card.Verb != "FLOW" || card.Description != "Signs up a user." || !hasFact(card, "Steps", "3") {
		t.Errorf("card %+v", card)
	}
}

func TestHover_Given_a_folder_and_a_collection_When_hovered_Then_they_count_what_they_hold(t *testing.T) {
	ws := hoverWorkspace(t)

	folder := ws.Hover("c:C1/requests/auth", nil)
	collection := ws.Hover("c:C1", nil)

	if folder.Kind != "Folder" || !hasFact(folder, "Requests", "2") || folder.Where != "Shop" {
		t.Errorf("folder %+v", folder)
	}
	if collection.Kind != "Collection" || !hasFact(collection, "Requests", "7") || !hasFact(collection, "Flows", "1") || !hasFact(collection, "Folder", "/c") {
		t.Errorf("collection %+v", collection)
	}
}

func TestHover_Given_an_unknown_row_When_hovered_Then_the_card_is_empty(t *testing.T) {
	ws := hoverWorkspace(t)

	card := ws.Hover("r:C1:NOPE", nil)

	if card.Title != "" || len(card.Facts) != 0 {
		t.Errorf("card %+v", card)
	}
}

// Before: every row said where it lived only by the group rows above it, so a flat list could not name the folder.
func TestWorkspaceVM_should_tell_each_row_its_type_and_where_it_lives(t *testing.T) {
	ws := hoverWorkspace(t)

	login, ping, flow, folder := ws.Node("r:C1:R1"), ws.Node("r:C1:R5"), ws.Node("f:C1:F1"), ws.Node("c:C1/requests/auth")

	if login.Type != vm.TypeHTTP || login.Where != "Shop / auth" || ping.Type != vm.TypeGRPC || ping.TypeName != "gRPC" || ping.Where != "Shop / grpc" {
		t.Errorf("login %+v ping %+v", login, ping)
	}
	if flow.Type != vm.TypeFlow || flow.Where != "Shop" || folder.Where != "Shop" {
		t.Errorf("flow %+v folder %+v", flow, folder)
	}
}

func TestWhereOf_should_list_the_collection_then_the_folders_above_the_item(t *testing.T) {
	cases := map[string][2]string{
		"Shop":          {"Shop", "ping"},
		"Shop / a / b":  {"Shop", "a/b/ping"},
		"a":             {"", "a/ping"},
		"Drafts / grpc": {"Drafts", "grpc/x"},
	}

	for want, in := range cases {
		if got := vm.WhereOf(in[0], in[1]); got != want {
			t.Errorf("WhereOf(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}
