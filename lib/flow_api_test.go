// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func userCreation(t *testing.T, w *Workspace) (FlowTarget, model.Flow) {
	t.Helper()
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}
	_, f, err := w.flowFor(target)
	if err != nil {
		t.Fatal(err)
	}
	return target, f
}

func TestFlowLayout_Given_no_file_Then_empty(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)

	l, diags, err := w.FlowLayout(target)

	if err != nil || len(diags) != 0 || len(l.Nodes) != 0 || l.Flow != f.ID {
		t.Fatalf("layout %+v %v %v", l, diags, err)
	}
}

func TestSaveFlowLayout_should_round_trip_and_drop_unknown_ids(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	l := model.Layout{Nodes: map[string]model.Point{"login": {X: 10, Y: -20}, "ghost": {X: 1, Y: 1}}, Collapsed: []string{"session", "ghost"}}

	err := w.SaveFlowLayout(target, l)
	got, diags, loadErr := w.FlowLayout(target)

	file := strings.TrimSuffix(f.File, ".arazzo.yaml") + ".layout.yaml"
	data, _ := os.ReadFile(file)
	if err != nil || loadErr != nil || len(diags) != 0 || got.Nodes["login"] != (model.Point{X: 10, Y: -20}) || len(got.Nodes) != 1 ||
		!slices.Equal(got.Collapsed, []string{"session"}) || got.Flow != f.ID {
		t.Fatalf("got %+v %v %v %v\n%s", got, diags, err, loadErr, data)
	}
}

func TestFlowLayout_Given_malformed_file_Then_diagnostic_not_error(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	file := strings.TrimSuffix(f.File, ".arazzo.yaml") + ".layout.yaml"
	if err := os.WriteFile(file, []byte("nodes: [oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	l, diags, err := w.FlowLayout(target)

	if err != nil || len(diags) == 0 || len(l.Nodes) != 0 {
		t.Fatalf("layout %+v diags %v err %v", l, diags, err)
	}
}

func TestSaveFlow_should_write_canonical_and_reload(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	f.Description = "Edited on the canvas."

	err := w.SaveFlow(target, f, SaveOptions{})

	_, got, _ := w.flowFor(target)
	if err != nil || got.Description != "Edited on the canvas." {
		t.Fatalf("err %v got %q", err, got.Description)
	}
}

func TestSaveFlow_Given_comments_Then_FLW_WRITE_001(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	data, _ := os.ReadFile(f.File)
	if err := os.WriteFile(f.File, append([]byte("# mine\n"), data...), 0o644); err != nil {
		t.Fatal(err)
	}

	err := w.SaveFlow(target, f, SaveOptions{})

	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeWriteDropsComments}) {
		t.Fatalf("got %v", err)
	}
}

func TestNewFlow_should_create_and_reject_bad_paths(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]

	target, err := w.NewFlow(c.Manifest.ID, "journeys/signup")
	_, bad := w.NewFlow(c.Manifest.ID, "../x")
	_, dup := w.NewFlow(c.Manifest.ID, "user-creation")

	_, f, findErr := w.flowFor(target)
	if err != nil || findErr != nil || f.Path != "journeys/signup" || len(f.Nodes) != 0 {
		t.Fatalf("err %v %v flow %+v", err, findErr, f)
	}
	if _, statErr := os.Stat(filepath.Join(c.Root, "flows", "journeys", "signup.arazzo.yaml")); statErr != nil {
		t.Fatal(statErr)
	}
	for _, e := range []error{bad, dup} {
		if codes := codesOfErr(e); !slices.Equal(codes, []diag.Code{diag.CodeWriteRequestPath}) {
			t.Fatalf("got %v", e)
		}
	}
}

func TestNodeCatalog_should_cover_every_node_type(t *testing.T) {
	cat := NodeCatalog()

	for _, typ := range model.NodeTypes() {
		i := slices.IndexFunc(cat, func(s NodeSpec) bool { return s.Type == string(typ) })
		if i < 0 || cat[i].Group == "" || cat[i].Summary == "" {
			t.Fatalf("missing or incomplete spec for %s: %+v", typ, cat)
		}
	}
	delay := cat[slices.IndexFunc(cat, func(s NodeSpec) bool { return s.Type == "delay" })]
	runTill := cat[slices.IndexFunc(cat, func(s NodeSpec) bool { return s.Type == "run_till" })]
	if !slices.ContainsFunc(delay.Params, func(p NodeParam) bool { return p.Name == "duration" && p.Kind == "text" && p.Required }) ||
		!slices.ContainsFunc(runTill.Params, func(p NodeParam) bool { return p.Name == "interval" && p.Kind == "duration" }) {
		t.Fatalf("delay %+v run_till %+v", delay.Params, runTill.Params)
	}
}

func TestNewUntitledFlow_should_pick_the_next_free_name_without_asking_for_a_path(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)

	first, err1 := w.NewUntitledFlow(flow.Collection)
	second, err2 := w.NewUntitledFlow(flow.Collection)

	if err1 != nil || err2 != nil {
		t.Fatalf("%v %v", err1, err2)
	}
	_, a, _ := w.flowFor(first)
	_, b, _ := w.flowFor(second)
	if a.Path != "untitled" || b.Path != "untitled-2" {
		t.Fatalf("paths %q %q", a.Path, b.Path)
	}
}

// Steps only Flow-I can run are told to the person editing, not raised by
// Validate: the example flows are full of them and must stay clean.
func TestFlowCompatibility_should_name_the_steps_other_Arazzo_tools_cannot_run(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindFlow("user-creation", "")

	got := w.FlowCompatibility(target)

	if len(got) == 0 {
		t.Fatal("the example flow has Flow-I-only steps")
	}
	for _, d := range got {
		if d.Code != diag.CodeArazzoCustom || d.Severity != diag.SeverityWarning || d.Pos.Line == 0 || !strings.HasSuffix(d.File, "user-creation.arazzo.yaml") {
			t.Fatalf("diagnostic %+v", d)
		}
	}
	if diags := w.Validate(); slices.ContainsFunc(diags, func(d diag.Diagnostic) bool { return d.Code == diag.CodeArazzoCustom }) {
		t.Fatal("Validate raises the custom-step warning")
	}
}

func TestFlowCompatibility_should_name_what_an_async_step_loses_in_arazzo(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	req := saveAsync(t, w, "rpc/ask", &model.Async{Protocol: "ws", Server: "ws://h", Channel: "c",
		Send: &model.AsyncSend{}, Receive: &model.AsyncReceive{Match: "body.ok"}})
	flow, err := w.NewFlow(c.Manifest.ID, "ask-flow")
	if err != nil {
		t.Fatal(err)
	}
	_, f, _ := w.flowFor(flow)
	f.Nodes = []model.Node{{ID: "ask", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: req.Request, Path: "rpc/ask"}}}}
	if err := w.SaveFlow(flow, f, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	got := w.FlowCompatibility(flow)

	if len(got) != 2 || got[0].Code != diag.CodeArazzoLimit || got[1].Code != diag.CodeArazzoLimit {
		t.Fatalf("got %v", got)
	}
}

func TestFlowSource_should_show_the_draft_as_flow_text_and_as_the_arazzo_file(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	f.Outputs = append(f.Outputs, model.KeyValue{Key: "added", Value: "1"}) // an unsaved edit

	src, err := w.FlowSource(target, f)

	if err != nil || !strings.Contains(src.Flow, "added") || !strings.Contains(src.Flow, "nodes:") ||
		!strings.HasPrefix(src.File, "arazzo:") || !strings.Contains(src.File, "x-flowi-flow") {
		t.Fatalf("source %+v %v", src, err)
	}
}

func TestParseFlowSource_should_round_trip_what_FlowSource_shows(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	src, _ := w.FlowSource(target, f)

	got, diags, err := w.ParseFlowSource(target, src.Flow)

	if err != nil || len(diags) != 0 || len(got.Nodes) != len(f.Nodes) || got.ID != f.ID || got.File != f.File || got.Path != f.Path {
		t.Fatalf("got %+v %v %v", got, diags, err)
	}
}

func TestParseFlowSource_Given_broken_text_Then_diagnostics_and_no_error(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)

	_, diags, err := w.ParseFlowSource(target, "nodes: [oops\n")

	if err != nil || len(diags) == 0 {
		t.Fatalf("diags %v err %v", diags, err)
	}
}

func TestParseFlowSource_Given_unknown_key_Then_diagnostic(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)

	_, diags, _ := w.ParseFlowSource(target, "bogus: 1\nnodes: []\n")

	if len(diags) == 0 {
		t.Fatal("an unknown key should be reported")
	}
}
