// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

var update = flag.Bool("update", false, "rewrite golden files") //nolint:gochecknoglobals // test flag

// fakeResolver knows a few operations of a petstore collection. getPet
// is version 1 of a request whose default is 1; version 2 is another
// operation.
type fakeResolver struct{}

func (fakeResolver) RequestFor(op string) (model.Ref, string, bool) {
	switch op {
	case "getPet":
		return model.Ref{ID: "01JA0000000000000000000040", Path: "pets/getPet"}, "1", true
	case "createPet":
		return model.Ref{ID: "01JA0000000000000000000041", Path: "pets/createPet"}, "", true
	}
	return model.Ref{}, "", false
}

func (fakeResolver) OperationOf(call model.RequestCall) (Operation, bool) {
	switch call.Request.ID {
	case "01JA0000000000000000000040":
		if call.Version != "" && call.Version != "1" {
			return Operation{}, false
		}
		return Operation{ID: "getPet", Params: map[string]Param{"petId": {Name: "petId", In: "path"}}}, true
	case "01JA0000000000000000000041":
		return Operation{ID: "createPet", Params: map[string]Param{"X_Trace": {Name: "X-Trace", In: "header"}}}, true
	}
	return Operation{}, false
}

func ids() func() model.ID {
	n := 0
	return func() model.ID { n++; return model.ID(fmt.Sprintf("01JA00000000000000000009%02d", n)) }
}

func importFile(t *testing.T, name string) (Result, diag.List) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return Import(data, Options{NewID: ids(), Resolve: fakeResolver{}})
}

func TestMapExpression(t *testing.T) {
	cases := []struct {
		expr, step, want string
		ok               bool
	}{
		{"$inputs.petId", "s", "{{inputs.petId}}", true},
		{"$steps.fetch.outputs.pet", "s", "{{vars.fetch_pet}}", true},
		{"$response.body#/data/id", "fetch", "{{nodes.fetch.body.data.id}}", true},
		{"$response.header.X-Rate", "fetch", `{{nodes.fetch.headers["x-rate"]}}`, true},
		{"$response.body#/items/0/a~1b", "my-step", `{{nodes["my-step"].body.items[0]["a/b"]}}`, true},
		{"$statusCode", "", "", false},
		{"literal {{braces}}", "s", "", false},
		{"$statusCode", "fetch", "{{nodes.fetch.status}}", true},
		{"plain text", "s", "plain text", true},
		{"Bearer {$steps.login.outputs.token}", "s", "Bearer {{vars.login_token}}", true},
		{"$message.payload#/orderId", "s", "", false},
		{"$outputs.orderId", "s", "", false},
	}
	for _, tc := range cases {
		got, ok := mapExpression(tc.expr, tc.step)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("mapExpression(%q) = %q %v, want %q %v", tc.expr, got, ok, tc.want, tc.ok)
		}
	}
}

func TestImport_official_10_maps_steps_outputs_and_workflow_outputs(t *testing.T) {
	r, diags := importFile(t, "official-1.0.yaml")

	if diags.HasErrors() || len(r.Flows) != 1 {
		t.Fatalf("diags %v flows %d", diags, len(r.Flows))
	}
	f := r.Flows[0]
	if f.Path != "arazzo/fetchPet" || len(f.Inputs) != 0 || len(f.Nodes) != 2 {
		t.Fatalf("flow %+v", f)
	}
	fetch, outputs := f.Nodes[0], f.Nodes[1]
	if fetch.Request == nil || fetch.Request.Request.Path != "pets/getPet" || fetch.Request.Version != "1" ||
		!slices.Equal(fetch.Request.With, []model.KeyValue{{Key: "petId", Value: "{{inputs.petId}}"}}) {
		t.Fatalf("fetch %+v", fetch)
	}
	if outputs.Type != "extract" || len(outputs.Params) != 1 || outputs.Params[0] != (model.Field{Key: "fetch_petId", Value: "{{nodes.fetch.body.id}}"}) {
		t.Fatalf("outputs %+v", outputs)
	}
	if !slices.Equal(f.Outputs, []model.KeyValue{{Key: "petId", Value: "{{vars.fetch_petId}}"}}) {
		t.Fatalf("flow outputs %v", f.Outputs)
	}
}

func TestImport_official_11_reports_every_lossy_element(t *testing.T) {
	r, diags := importFile(t, "official-1.1.yaml")

	f := r.Flows[0]
	if len(f.Inputs) != 2 || f.Nodes[0].ID != "fetchPet" || f.Nodes[0].Request == nil {
		t.Fatalf("flow %+v", f)
	}
	var whats []string
	for _, d := range diags {
		if d.Code == diag.CodeImportUnmapped {
			whats = append(whats, d.Args["what"])
		}
	}
	for _, want := range []string{"step placeOrder", "step confirmOrder", "workflow purchasePet successActions", "workflow purchasePet failureActions",
		"output pet of step fetchPet", "output orderId of workflow purchasePet"} {
		if !slices.Contains(whats, want) {
			t.Errorf("no lossy report for %q in %v", want, whats)
		}
	}
	notes := 0
	for _, n := range f.Nodes {
		if n.Type == "note" {
			notes++
		}
	}
	if notes < 2 {
		t.Fatalf("unmapped steps should stay visible as notes: %+v", f.Nodes)
	}
}

func TestMapCondition(t *testing.T) {
	cases := []struct{ cond, want string }{
		{"$statusCode == 200", "nodes.fetch.status == 200"},
		{"$response.body#/status == 'ok' && $inputs.n > 1", "nodes.fetch.body.status == 'ok' && inputs.n > 1"},
	}
	for _, tc := range cases {
		got, ok := mapCondition(tc.cond, "fetch")
		if !ok || got != tc.want {
			t.Errorf("mapCondition(%q) = %q %v, want %q", tc.cond, got, ok, tc.want)
		}
	}
	if _, ok := mapCondition("$message.header.x == 1", "fetch"); ok {
		t.Fatal("an unmappable expression inside a condition must be reported")
	}
}

func TestUnmapTemplate(t *testing.T) {
	vars := map[string]string{"fetch_pet": "$steps.fetch.outputs.pet"}
	cases := []struct {
		tmpl, want string
		ok         bool
	}{
		{"{{inputs.petId}}", "$inputs.petId", true},
		{"{{vars.fetch_pet}}", "$steps.fetch.outputs.pet", true},
		{"{{nodes.get.body.items[0].id}}", "$response.body#/items/0/id", true},
		{`{{nodes.get.headers["x-rate"]}}`, "$response.header.x-rate", true},
		{"{{nodes.get.status}}", "$statusCode", true},
		{"Bearer {{vars.fetch_pet}}", "Bearer {$steps.fetch.outputs.pet}", true},
		{"plain", "plain", true},
		{"{{env.base_url}}", "", false},
		{"{{nodes.other.body.id}}", "", false},
		{"{{vars.unknown}}", "", false},
		{"{{upper(inputs.x)}}", "", false},
	}
	for _, tc := range cases {
		got, ok := unmapTemplate(tc.tmpl, "get", vars)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("unmapTemplate(%q) = %q %v, want %q %v", tc.tmpl, got, ok, tc.want, tc.ok)
		}
	}
}

func TestExport_Given_a_version_that_is_not_the_operation_Then_the_node_is_reported(t *testing.T) {
	f := model.Flow{Meta: model.Meta{Name: "f"}, Path: "f", Nodes: []model.Node{
		{ID: "get", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA0000000000000000000040"}, Version: "2"}},
	}}

	_, diags := Export([]model.Flow{f}, ExportOptions{Resolve: fakeResolver{}, Source: "petstore"})

	if len(diags) != 1 || diags[0].Code != diag.CodeExportLossy || diags[0].Args["node"] != "get" {
		t.Fatalf("diags %v", diags)
	}
}

func TestExport_reports_values_it_cannot_express(t *testing.T) {
	f := model.Flow{Meta: model.Meta{Name: "f"}, Path: "f", Nodes: []model.Node{
		{ID: "get", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA0000000000000000000040"},
			With: []model.KeyValue{{Key: "petId", Value: "{{env.pet}}"}}}},
	}}

	_, diags := Export([]model.Flow{f}, ExportOptions{Resolve: fakeResolver{}, Source: "petstore"})

	if len(diags) != 1 || diags[0].Code != diag.CodeExportValue || diags[0].Args["node"] != "get" {
		t.Fatalf("diags %v", diags)
	}
}

func TestImport_should_report_reads_of_outputs_that_were_not_imported(t *testing.T) {
	doc := []byte(`arazzo: 1.0.1
info: {title: t, version: 1.0.0}
sourceDescriptions: [{name: api, url: ./api.yaml, type: openapi}]
workflows:
  - workflowId: w
    steps:
      - stepId: first
        operationId: getPet
        parameters:
          - {name: petId, in: path, value: $steps.later.outputs.id}
      - stepId: later
        operationId: getPet
        outputs:
          id: $response.body#/id
`)

	r, diags := Import(doc, Options{NewID: ids(), Resolve: fakeResolver{}})

	if len(r.Flows[0].Nodes[0].Request.With) != 0 || len(diags) != 1 || diags[0].Args["what"] != "parameter petId of step first" {
		t.Fatalf("with %v diags %v", r.Flows[0].Nodes[0].Request.With, diags)
	}
}

func TestRoundTrip_header_parameters_use_the_template_key_and_the_real_name(t *testing.T) {
	doc := []byte(`arazzo: 1.0.1
info: {title: t, version: 1.0.0}
sourceDescriptions: [{name: petstore, url: ./api.yaml, type: openapi}]
workflows:
  - workflowId: w
    steps:
      - stepId: create
        operationId: createPet
        parameters:
          - {name: X-Trace, in: header, value: abc}
`)

	r, _ := Import(doc, Options{NewID: ids(), Resolve: fakeResolver{}})
	out, diags := Export(r.Flows, ExportOptions{Resolve: fakeResolver{}, Source: "petstore"})

	if with := r.Flows[0].Nodes[0].Request.With; len(with) != 1 || with[0].Key != "X_Trace" {
		t.Fatalf("with %v", with)
	}
	if len(diags) != 0 || !strings.Contains(string(out), "name: X-Trace") || !strings.Contains(string(out), "in: header") {
		t.Fatalf("diags %v\n%s", diags, out)
	}
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	file := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("%s differs; got:\n%s", name, got)
	}
}

func TestRoundTrip_official_examples_to_golden(t *testing.T) {
	for _, name := range []string{"official-1.0", "official-1.1"} {
		t.Run(name, func(t *testing.T) {
			r, _ := importFile(t, name+".yaml")

			out, diags := Export(r.Flows, ExportOptions{Resolve: fakeResolver{}, Source: "petstore"})

			if diags.HasErrors() {
				t.Fatal(diags)
			}
			if _, err := libopenapi.NewArazzoDocument(out); err != nil {
				t.Fatalf("export does not parse as Arazzo: %v\n%s", err, out)
			}
			golden(t, name+".golden.yaml", out)
		})
	}
}

func TestRoundTrip_export_import_export_is_stable(t *testing.T) {
	r, _ := importFile(t, "official-1.0.yaml")
	first, _ := Export(r.Flows, ExportOptions{Resolve: fakeResolver{}, Source: "petstore"})

	again, diags := Import(first, Options{NewID: ids(), Resolve: fakeResolver{}})
	second, _ := Export(again.Flows, ExportOptions{Resolve: fakeResolver{}, Source: "petstore"})

	if diags.HasErrors() || string(first) != string(second) {
		t.Fatalf("diags %v\nfirst:\n%s\nsecond:\n%s", diags, first, second)
	}
}

func TestExport_reports_nodes_without_an_arazzo_equivalent(t *testing.T) {
	f := model.Flow{Meta: model.Meta{Name: "f"}, Path: "f", Nodes: []model.Node{
		{ID: "wait", Type: "delay", Params: []model.Field{{Key: "duration", Value: "1s"}}},
		{ID: "get", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA0000000000000000000040"},
			With: []model.KeyValue{{Key: "petId", Value: "{{inputs.id}}"}}}},
		{ID: "check", Type: "assert", Params: []model.Field{{Key: "check", Value: "nodes.get.status == 200"}}},
	}}

	out, diags := Export([]model.Flow{f}, ExportOptions{Resolve: fakeResolver{}, Source: "petstore"})

	if len(diags) != 1 || diags[0].Code != diag.CodeExportLossy || diags[0].Args["node"] != "wait" {
		t.Fatalf("diags %v", diags)
	}
	if _, err := libopenapi.NewArazzoDocument(out); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, want := range []string{"operationId: $sourceDescriptions.petstore.getPet", "in: path", "condition: $statusCode == 200", "x-flowi-skipped:", "- wait"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

// Before: step a-b output c and step a output b_c both became vars.a_b_c,
// so a read of one silently got the other.
func TestImport_Given_colliding_output_variables_Then_the_later_is_reported(t *testing.T) {
	doc := []byte(`arazzo: 1.0.1
info: {title: t, version: 1.0.0}
sourceDescriptions: [{name: api, url: ./api.yaml, type: openapi}]
workflows:
  - workflowId: w
    steps:
      - stepId: a
        operationId: getPet
        outputs: {b_c: $response.body#/x}
      - stepId: a-b
        operationId: getPet
        outputs: {c: $response.body#/y}
      - stepId: use
        operationId: getPet
        parameters:
          - {name: petId, in: path, value: $steps.a-b.outputs.c}
`)

	r, diags := Import(doc, Options{NewID: ids(), Resolve: fakeResolver{}})

	whats := []string{}
	for _, d := range diags {
		whats = append(whats, d.Args["what"])
	}
	if !slices.Contains(whats, "output c of step a-b") || !slices.Contains(whats, "parameter petId of step use") {
		t.Fatalf("diags %v", whats)
	}
	for _, n := range r.Flows[0].Nodes {
		if n.Request != nil && n.ID == "use" && len(n.Request.With) != 0 {
			t.Fatalf("the read must not reach step a's value: %v", n.Request.With)
		}
	}
}

// Before: "$inputs.name" inside a string literal was rewritten.
func TestMapCondition_should_leave_string_literals_alone(t *testing.T) {
	got, ok := mapCondition(`$response.body#/name == '$inputs.name' && $statusCode == 200`, "s")

	if !ok || got != `nodes.s.body.name == '$inputs.name' && nodes.s.status == 200` {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestUnmapCondition_should_refuse_what_arazzo_cannot_express(t *testing.T) {
	refused := []string{
		"nodes.get.status == 200 and true",
		"not (nodes.get.status == 200)",
		`nodes.get.body.name == "x"`,
		"nodes.get.status > 1 ? true : false",
		"nodes.get.status * 2 == 400",
		"nodes.get.body.list[-1] == 1",
		"nodes.get.body.tags[0:2] == 1",
		`nodes.get.body.name.startsWith('x')`,
		"nodes.get.body.x == nil",
		"nodes.get.status - 1 == 199",
		"nodes.get.status == 2.",
	}
	for _, check := range refused {
		if got, ok := unmapCondition(check, "get", nil); ok {
			t.Errorf("unmapCondition(%q) = %q, want refused", check, got)
		}
	}
	got, ok := unmapCondition("nodes.get.status == 200 && !(nodes.get.body.n >= -1.5) || nodes.get.body.x == null", "get", nil)
	if !ok || got != "$statusCode == 200 && !($response.body#/n >= -1.5) || $response.body#/x == null" {
		t.Fatalf("got %q %v", got, ok)
	}
}

// Before: when, on_error and concurrent were dropped silently, ids that
// are not Arazzo stepIds were written, and literals starting with $ came
// out as expressions.
func TestExport_reports_what_a_step_cannot_carry(t *testing.T) {
	get := func(id string, mod func(*model.Node)) model.Node {
		n := model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA0000000000000000000040"}}}
		mod(&n)
		return n
	}
	f := model.Flow{Meta: model.Meta{Name: "f"}, Path: "f", Nodes: []model.Node{
		get("conditional", func(n *model.Node) { n.When = "inputs.go" }),
		get("tolerant", func(n *model.Node) { n.OnError = "continue" }),
		get("parallel", func(n *model.Node) { n.Concurrent = true }),
		get("get.pet", func(*model.Node) {}),
		get("price", func(n *model.Node) { n.Request.With = []model.KeyValue{{Key: "petId", Value: "$100"}} }),
		get("brace", func(n *model.Node) { n.Request.With = []model.KeyValue{{Key: "petId", Value: "a {$x} b"}} }),
	}}

	out, diags := Export([]model.Flow{f}, ExportOptions{Resolve: fakeResolver{}, Source: "petstore"})

	reported := map[string]bool{}
	for _, d := range diags {
		reported[d.Args["node"]] = true
	}
	for _, id := range []string{"conditional", "tolerant", "parallel", "get.pet", "price", "brace"} {
		if !reported[id] {
			t.Errorf("%s not reported: %v", id, diags)
		}
	}
	if strings.Contains(string(out), "$100") || strings.Contains(string(out), "stepId: get.pet") {
		t.Fatalf("unsafe output:\n%s", out)
	}
}
