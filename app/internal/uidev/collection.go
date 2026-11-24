// SPDX-License-Identifier: AGPL-3.0-or-later

package uidev

import (
	"context"
	"slices"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"
)

// What the collection page has to show beyond the shop's requests: variables to edit, kept descriptions with findings
// (an OpenAPI one, an AsyncAPI one and a .proto), who reads each operation, and a drift check that moves. All of it is
// made up and lives in memory, so the page can be looked at and its edits tried without a workspace.

// madeUpVariables are the collection variables the page starts with.
func madeUpVariables() []model.KeyValue {
	return []model.KeyValue{
		{Key: "base_url", Value: "https://shop.example", Description: "where the API lives"},
		{Key: "region", Value: "eu"},
		{Key: "retries", Value: "3", Description: "how many times a flaky call is retried"},
	}
}

// CollectionVariables are the variables the page edits, kept in memory.
func (b *Backend) CollectionVariables(model.ID) ([]model.KeyValue, error) {
	b.varsMu.Lock()
	defer b.varsMu.Unlock()
	return slices.Clone(b.vars), nil
}

// SetCollectionVariables replaces them, and the collection's manifest with them, so the page reads what was typed.
func (b *Backend) SetCollectionVariables(id model.ID, rows []model.KeyValue, _ flowi.SaveOptions) error {
	b.varsMu.Lock()
	b.vars = slices.Clone(rows)
	b.varsMu.Unlock()
	b.Mu.Lock()
	defer b.Mu.Unlock()
	for i := range b.Cols {
		if b.Cols[i].Manifest.ID == id {
			b.Cols[i].Manifest.Variables = slices.Clone(rows)
		}
	}
	return nil
}

// DescriptionFiles are the descriptions the made-up collection kept.
func (b *Backend) DescriptionFiles(model.ID) []string {
	return []string{"descriptions/shop.openapi.yaml", "descriptions/orders.asyncapi.yaml", "protos/shop.proto"}
}

// specFinding is a finding of the made-up descriptions: the rule it comes from, where, and what it says.
type specFinding struct {
	rule string
	code diag.Code
	file string
	path string
	line int
	args map[string]string
}

func madeUpFindings() []specFinding {
	api := "/demo/descriptions/shop.openapi.yaml"
	return []specFinding{
		{"info-description", diag.CodeSpecInfo, api, "$.info", 2, map[string]string{"spec": "shop"}},
		{"operation-description", diag.CodeSpecSummary, api, "$.paths./health.get", 14, map[string]string{"spec": "shop", "where": "GET /health"}},
		{"operation-tags", diag.CodeSpecTags, api, "$.paths./orders.post", 23, map[string]string{"spec": "shop", "where": "POST /orders"}},
		{"unused-component", diag.CodeSpecUnused, api, "$.components.schemas.Legacy", 61, map[string]string{"spec": "shop", "component": "schemas/Legacy"}},
		{"server-encrypted", diag.CodeSpecPlaintext, api, "$.servers[1]", 8, map[string]string{"spec": "shop", "server": "http://api.shop.example"}},
		{"operation-description", diag.CodeSpecSummary, "/demo/descriptions/orders.asyncapi.yaml", "$.channels.'orders/created'.publish", 9, map[string]string{"spec": "orders", "where": "PUBLISH orders/created"}},
		{"operation-description", diag.CodeSpecSummary, "/demo/protos/shop.proto", "", 8, map[string]string{"spec": "shop.proto", "where": "Orders.Cancel"}},
	}
}

// Validate is the fixture's problems with the made-up description findings added, at the levels the Rules editor chose.
func (b *Backend) Validate() diag.List {
	out := slices.Clone(b.Backend.Validate())
	b.rulesMu.Lock()
	defer b.rulesMu.Unlock()
	for _, f := range madeUpFindings() {
		level := "warning"
		if l, ok := b.rules[f.rule]; ok {
			level = l
		}
		if level == "off" {
			continue
		}
		d := diag.New(f.code, diag.Location{File: f.file, Path: f.path, Pos: diag.Position{Line: f.line}}, f.args)
		if level == "error" {
			d.Severity = diag.SeverityError
		}
		out = append(out, d)
	}
	return out
}

// CollectionImpact is who reads each operation: login is read by two steps that fit; creating an order is read by the
// payment step, which wants a field the response no longer has; the order list is read by something it cannot judge.
func (b *Backend) CollectionImpact(model.ID) ([]analysis.Report, error) {
	report := func(request, source string, consumers ...analysis.Consumer) analysis.Report {
		rep := analysis.Report{CollectionID: "C-demo", Request: request, Versions: []analysis.VersionUsage{{Version: "1", Contract: analysis.ContractInfo{Source: source}, Consumers: consumers}}}
		for _, c := range consumers {
			if c.Status == analysis.Broken {
				rep.Status = analysis.Broken
			}
		}
		return rep
	}
	node := func(flow, id string, status analysis.Status, reads ...analysis.Read) analysis.Consumer {
		return analysis.Consumer{Kind: analysis.KindNode, Flow: flow, Node: id, Status: status, Reads: reads}
	}
	return []analysis.Report{
		report("auth/login", analysis.SourceExample, node("checkout", "create", analysis.Compatible), node("soak", "step001", analysis.Compatible)),
		report("orders/create", analysis.SourceResponse, node("checkout", FailingStep, analysis.Broken, analysis.Read{Target: "$.order.id", Status: analysis.Broken})),
		report("orders/list", "", node("soak", "step004", analysis.Unknown)),
	}, nil
}

// CheckDrift answers after a pause long enough to see the page wait: login's token changed type and a field is new;
// creating an order matches its schema; paying has none to compare with.
func (b *Backend) CheckDrift(_ context.Context, _ flowi.FlowTarget, _ flowi.DriftOptions) (flowi.DriftReport, error) {
	time.Sleep(700 * time.Millisecond)
	return flowi.DriftReport{Flow: "checkout", RunStatus: engine.StatusPassed, Requests: []flowi.RequestDrift{
		{Request: "auth/login", Version: "1", Status: 200, Differences: []schemainfer.Difference{
			{Path: "$.token", Kind: schemainfer.DriftChanged, Detail: "was string, is object"},
			{Path: "$.session", Kind: schemainfer.DriftNew, Detail: "a field the schema does not know"},
		}},
		{Request: "orders/create", Version: "1", Status: 201},
		{Request: "orders/pay", Version: "1", Status: 200, NoSchema: true},
	}}, nil
}

// madeUpCoverage is what the page's State tab lists: the requests that came from a description, with what runs saw.
func madeUpCoverage() analysis.Coverage {
	return analysis.Coverage{Operations: []analysis.OperationCoverage{
		{Collection: "C-demo", Request: "auth/login", Version: "1", Used: []string{"checkout", "soak"}, Statuses: []analysis.StatusSeen{{Status: 200, Seen: true}, {Status: 401}}},
		{Collection: "C-demo", Request: "orders/create", Version: "1", Used: []string{"checkout"}, Statuses: []analysis.StatusSeen{{Status: 201, Seen: true}, {Status: 422}}, Unread: []string{"$.order.total"}},
		{Collection: "C-demo", Request: "orders/list", Version: "1", Used: []string{"soak"}, Statuses: []analysis.StatusSeen{{Status: 200}}},
		{Collection: "C-demo", Request: "orders/pay", Version: "1", Statuses: []analysis.StatusSeen{{Status: 200}, {Status: 402}}},
	}}
}

// madeUpExports are the descriptions the page offers to save, with some text for each.
func madeUpExports() ([]flowi.ExportItem, map[string]string) {
	items := []flowi.ExportItem{
		{ID: flowi.ExportGenerated, Label: "The requests as Flowi sends them (OpenAPI)", File: "requests.openapi.yaml"},
		{ID: "kept:shop.openapi.yaml", Label: "shop as it stands, with its overlays applied", File: "shop.openapi.yaml"},
	}
	return items, map[string]string{
		flowi.ExportGenerated:    "openapi: 3.0.3\ninfo: {title: Demo shop, version: '1'}\npaths: {}\n",
		"kept:shop.openapi.yaml": "openapi: 3.0.3\ninfo: {title: Shop, version: '1'}\npaths: {}\n",
	}
}
