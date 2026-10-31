// SPDX-License-Identifier: Apache-2.0

package mermaid

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/arazzo"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers/importerstest"
	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

// fakeResolver knows two requests and one flow.
type fakeResolver struct{}

func (fakeResolver) Request(path string) (model.Ref, bool) {
	if slices.Contains([]string{"auth/login", "users/create"}, path) {
		return model.Ref{ID: model.ID("R-" + path), Path: path}, true
	}
	return model.Ref{}, false
}

func (fakeResolver) Flow(path string) (model.Ref, bool) {
	if path == "shared/login" {
		return model.Ref{ID: "F-shared", Path: path}, true
	}
	return model.Ref{}, false
}

func TestParse_should_read_nodes_edges_and_labels(t *testing.T) {
	src := `%% a comment
flowchart LR
  a["req: auth/login"] --> b(delay: 2s) & c{{note: hi}}
  b -->|yes| d([assert: status == 200]); c -.-> d
  subgraph group [Grouped]
    d ==> e
  end
  style e fill:#f9f
  e --- f>"quoted #quot;x#quot;"]
`

	g, err := parse([]byte(src))

	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range g.nodes {
		ids = append(ids, n.id)
	}
	if !slices.Equal(ids, []string{"a", "b", "c", "d", "e", "f"}) {
		t.Fatalf("ids %v", ids)
	}
	if g.byID["a"].label != "req: auth/login" || g.byID["c"].label != "note: hi" || g.byID["f"].label != `quoted "x"` || g.byID["a"].line != 3 {
		t.Fatalf("labels a=%q c=%q f=%q", g.byID["a"].label, g.byID["c"].label, g.byID["f"].label)
	}
	want := []string{"a>b", "a>c", "b>d", "c>d", "d>e", "e>f"}
	var got []string
	for _, e := range g.edges {
		got = append(got, e.from+">"+e.to)
	}
	if !slices.Equal(got, want) || len(g.subgraphs) != 1 || !slices.Contains(g.edgeLabels, "yes") {
		t.Fatalf("edges %v subgraphs %v labels %v", got, g.subgraphs, g.edgeLabels)
	}
}

func TestParse_should_report_where_it_stops(t *testing.T) {
	cases := map[string]diag.Position{
		"sequenceDiagram\n  A->>B: hi\n": {Line: 1, Column: 1},
		"flowchart TD\n  a[open --> b\n": {Line: 2, Column: 4},
		"flowchart TD\n  a --> \n":       {Line: 2, Column: 8}, // just after the arrow
		"flowchart TD\n  a ~~ b\n":       {Line: 2, Column: 5},
	}
	for src, want := range cases {
		_, err := parse([]byte(src))
		se, ok := err.(*syntaxError) //nolint:errorlint // parse returns *syntaxError
		if !ok || se.pos != want {
			t.Errorf("parse(%q) = %v, want an error at %v", src, err, want)
		}
	}
}

func importDiagram(t *testing.T, src string) (model.Flow, diag.List) {
	t.Helper()
	r, fatal := Import([]byte(src), "sketch.mmd", fakeResolver{})
	if len(fatal) > 0 {
		t.Fatalf("fatal %v", fatal)
	}
	return r.Flows[0], r.Diags()
}

func TestImport_should_map_label_conventions(t *testing.T) {
	f, diags := importDiagram(t, `flowchart LR
  login[req: auth/login@2] --> wait[delay: 2s] --> grab[extract: token=$.data.token, id=$.id]
  grab --> mark[set: done=yes] --> check[assert: status == 201] --> say[log: hello]
  say --> sub[subflow: shared/login] --> memo[note: remember] --> make[users/create]
`)

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
	var ids []string
	for _, n := range f.Nodes {
		ids = append(ids, n.ID+":"+string(n.Type))
	}
	want := []string{"login:request", "wait:delay", "grab:set", "grab-2:set", "mark:set", "check:assert", "say:log", "sub:subflow", "memo:note", "make:request"}
	if !slices.Equal(ids, want) {
		t.Fatalf("nodes %v", ids)
	}
	if f.Nodes[0].Request.Version != "2" || f.Nodes[0].Request.Request.ID != "R-auth/login" || f.Path != "mermaid/sketch" {
		t.Fatalf("login %+v path %s", f.Nodes[0].Request, f.Path)
	}
	if got := f.Nodes[3].Values; len(got) != 1 || got[0] != (model.SetValue{Name: "id", From: "response", Where: "body", Path: "$.id"}) {
		t.Fatalf("second extract %+v", got)
	}
	if got := f.Nodes[4].Values; len(got) != 1 || got[0] != (model.SetValue{Name: "done", Value: "yes"}) {
		t.Fatalf("set %+v", got)
	}
}

func TestImport_should_turn_what_does_not_resolve_into_placeholders(t *testing.T) {
	f, diags := importDiagram(t, "flowchart TD\n  a[req: nope/missing] --> b[Do the thing]\n  b --> c[delay: soon]\n")

	if len(diags) != 3 || diags[0].Code != diag.CodeMermaidPlaceholder || diags[0].Pos != (diag.Position{Line: 2, Column: 3}) {
		t.Fatalf("diags %v", diags)
	}
	if f.Nodes[0].Type != "note" || f.Nodes[0].Params[0].Value != "placeholder: req: nope/missing" {
		t.Fatalf("node %+v", f.Nodes[0])
	}
	if len(diags[0].Excerpt) != 2 {
		t.Fatal("placeholders point at the diagram line")
	}
}

func TestImport_should_set_needs_only_where_the_drawing_differs_from_sequence(t *testing.T) {
	f, _ := importDiagram(t, "flowchart TD\n  a[note: a] --> b[note: b]\n  a --> c[note: c]\n  b --> d[note: d]\n  c --> d\n")

	needs := map[string][]string{}
	for _, n := range f.Nodes {
		needs[n.ID] = n.Needs
	}
	if len(needs["b"]) != 0 || !slices.Equal(needs["c"], []string{"a"}) || !slices.Equal(needs["d"], []string{"b", "c"}) {
		t.Fatalf("needs %v", needs)
	}
}

func TestImport_Given_a_cycle_Then_FLW_MERMAID_002_and_nothing(t *testing.T) {
	r, fatal := Import([]byte("flowchart TD\n  a[note: a] --> b[note: b] --> a\n"), "loop.mmd", fakeResolver{})

	if len(fatal) != 1 || fatal[0].Code != diag.CodeMermaidCycle || len(r.Flows) != 0 {
		t.Fatalf("fatal %v flows %d", fatal, len(r.Flows))
	}
}

func exampleFlows(t *testing.T) []model.Flow {
	t.Helper()
	files, err := filepath.Glob("../examples/user-creation/flows/*.arazzo.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no example flows: %v", err)
	}
	codec := yamlcodec.New(settings.Schema())
	var out []model.Flow
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		f, diags := arazzo.DecodeFlow(file, strings.TrimSuffix(filepath.Base(file), ".arazzo.yaml"), data, arazzo.FlowOptions{Body: codec})
		if diags.HasErrors() {
			t.Fatal(diags)
		}
		out = append(out, f)
	}
	return out
}

func TestExport_example_flows_to_golden(t *testing.T) {
	for _, f := range exampleFlows(t) {
		t.Run(f.Path, func(t *testing.T) {
			got := Export(f)

			if _, err := parse([]byte(got)); err != nil {
				t.Fatalf("export does not parse: %v\n%s", err, got)
			}
			importerstest.Golden(t, f.Path+".mmd", got)
		})
	}
}

func TestExport_then_import_is_stable_for_supported_nodes(t *testing.T) {
	src := "flowchart LR\n  login[req: auth/login@2] --> wait[delay: 2s] --> grab[extract: token=$.data.token]\n" +
		"  grab --> mark[set: done=yes] --> check[assert: status == 201] --> say[log: \"quoted\" text]\n" +
		"  login --> sub[subflow: shared/login]\n  say --> memo[note: remember]\n"
	f, _ := importDiagram(t, src)
	first := Export(f)

	again, diags := importDiagram(t, first)
	second := Export(again)

	if len(diags) != 0 || first != second {
		t.Fatalf("diags %v\nfirst:\n%s\nsecond:\n%s", diags, first, second)
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"flowchart LR\n a-->b", "graph TD\n a[\"x", "flowchart\n a & b --> c", "flowchart\n subgraph x\n", "flowchart\n a -->|l", "flowchart\n a((b))"} {
		f.Add(seed)
	}
	f.Fuzz(func(_ *testing.T, src string) {
		Import([]byte(src), "fuzz.mmd", fakeResolver{}) // must not panic or hang
	})
}

// Before: A-->B (no spaces) read the id as "A--".
func TestParse_should_read_arrows_without_spaces(t *testing.T) {
	g, err := parse([]byte("flowchart LR\n  A-->B\n  B-.->C\n  C---D\n  my-node-->E\n"))

	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range g.edges {
		got = append(got, e.from+">"+e.to)
	}
	if !slices.Equal(got, []string{"A>B", "B>C", "C>D", "my-node>E"}) {
		t.Fatalf("edges %v", got)
	}
}

// Before: a fork put another request between a request and the extract
// reading its response.
func TestImport_should_run_response_readers_right_after_their_request(t *testing.T) {
	f, _ := importDiagram(t, "flowchart TD\n  L[req: auth/login] --> O[req: users/create]\n  L --> X[extract: token=$.token]\n  L --> C[assert: status == 200]\n")

	var ids []string
	for _, n := range f.Nodes {
		ids = append(ids, n.ID)
	}
	if !slices.Equal(ids, []string{"L", "X", "C", "O"}) {
		t.Fatalf("order %v", ids)
	}
}

func attachedFlow() model.Flow {
	login := model.Ref{ID: "R-auth/login", Path: "auth/login"}
	create := model.Ref{ID: "R-users/create", Path: "users/create"}
	return model.Flow{Path: "demo", Nodes: []model.Node{
		{ID: "login", Type: "request", Request: &model.RequestCall{Request: login}},
		{ID: "check", Type: "assert", Attach: "create", Params: []model.Field{{Key: "check", Value: "status == 201"}}},
		{ID: "peek", Type: "assert", Attach: "create", AttachAt: "above", Params: []model.Field{{Key: "check", Value: "body != null"}}},
		{ID: "create", Type: "request", Request: &model.RequestCall{Request: create}},
		{ID: "done", Type: "log", Params: []model.Field{{Key: "value", Value: "ok"}}},
	}}
}

func TestExport_Given_attached_actions_When_exported_Then_they_share_a_subgraph(t *testing.T) {
	f := attachedFlow()

	got := Export(f)

	for _, want := range []string{
		"subgraph attach_create [\"create with its actions\"]",
		"direction LR", // one action sits beside the request
		"login --> attach_create",
		"attach_create --> done",
		"check --> peek",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "login --> check") {
		t.Errorf("arrows into a group must go to its subgraph:\n%s", got)
	}
}

func TestExport_Given_only_stacked_actions_When_exported_Then_the_group_runs_top_to_bottom(t *testing.T) {
	f := attachedFlow()
	f.Nodes[1].AttachAt = "below"

	got := Export(f)

	if !strings.Contains(got, "direction TB") {
		t.Errorf("want a vertical group:\n%s", got)
	}
}

func TestImport_Given_arrows_to_a_subgraph_When_imported_Then_they_reach_its_entry_and_exit_nodes(t *testing.T) {
	src := "flowchart LR\n  a[\"req: auth/login\"]\n  subgraph g [\"group\"]\n    direction TB\n    x[\"assert: 1 == 1\"] --> y[\"req: users/create\"]\n  end\n  z[\"log: ok\"]\n  a --> g\n  g --> z\n"

	f, diags := importDiagram(t, src)

	var ids []string
	for _, n := range f.Nodes {
		ids = append(ids, n.ID)
	}
	if want := []string{"a", "x", "y", "z"}; !slices.Equal(ids, want) {
		t.Fatalf("nodes %v, want %v (diags %v)", ids, want, diags)
	}
	if len(f.Nodes[1].Needs) != 0 || len(f.Nodes[3].Needs) != 0 {
		t.Errorf("a chain in order needs nothing: %v %v", f.Nodes[1].Needs, f.Nodes[3].Needs)
	}
}

func TestExport_then_import_Given_attached_actions_Then_the_nodes_and_their_order_survive(t *testing.T) {
	first := Export(attachedFlow())

	// the importer reports the grouping as left out; the steps and their order come back
	again, _ := importDiagram(t, first)

	var ids []string
	for _, n := range again.Nodes {
		ids = append(ids, n.ID)
	}
	if want := []string{"login", "check", "peek", "create", "done"}; !slices.Equal(ids, want) {
		t.Fatalf("nodes %v\n%s", ids, first)
	}
}
