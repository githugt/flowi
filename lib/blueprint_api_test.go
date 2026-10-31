// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func onboardingFlow(t *testing.T, w *Workspace) (FlowTarget, model.Flow) {
	t.Helper()
	target, err := w.FindFlow("onboarding-check", "")
	if err != nil {
		t.Fatal(err)
	}
	_, f, err := w.flowFor(target)
	if err != nil {
		t.Fatal(err)
	}
	return target, f
}

func TestCreateBlueprint_should_write_a_blueprint_with_parameters_for_the_flows_references(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)

	bt, err := w.CreateBlueprint(flow, "onboarding", "Onboarding template")

	if err != nil {
		t.Fatal(err)
	}
	bp, err := w.Blueprint(bt)
	if err != nil {
		t.Fatal(err)
	}
	if bp.Name != "Onboarding template" || bp.Path != "onboarding" {
		t.Fatalf("blueprint %+v", bp.Meta)
	}
	got := map[string]string{}
	for _, p := range bp.Params {
		got[p.Name] = string(p.Type) + ":" + p.Default
	}
	want := map[string]string{"login": "flow-ref:shared/login", "config": "request-ref:config"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("params %v, want %v", got, want)
	}
	if _, err := os.Stat(bp.File); err != nil || !strings.HasSuffix(bp.File, filepath.Join("blueprints", "onboarding.blueprint.yaml")) {
		t.Fatalf("file %q: %v", bp.File, err)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("the workspace has errors after creating a blueprint: %v", diags)
	}
}

func TestNewFlowFromBlueprint_Given_the_defaults_Then_the_flow_matches_the_original(t *testing.T) {
	w := demoWorkspace(t)
	flow, original := onboardingFlow(t, w)
	bt, err := w.CreateBlueprint(flow, "onboarding", "Onboarding template")
	if err != nil {
		t.Fatal(err)
	}

	made, err := w.NewFlowFromBlueprint(bt, flow.Collection, "copies/onboarding-copy", nil)

	if err != nil {
		t.Fatal(err)
	}
	_, copied, err := w.flowFor(made)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ID == original.ID || copied.Path != "copies/onboarding-copy" || copied.Name != "onboarding-copy" {
		t.Fatalf("identity %+v", copied.Meta)
	}
	if !reflect.DeepEqual(copied.Nodes, original.Nodes) || !reflect.DeepEqual(copied.Execution, original.Execution) {
		t.Fatalf("the copy differs from the original:\n%+v\n%+v", copied.Nodes, original.Nodes)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors after instantiating: %v", diags)
	}
}

func TestNewFlowFromBlueprint_should_use_the_values_given(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	bt, err := w.CreateBlueprint(flow, "onboarding", "Onboarding template")
	if err != nil {
		t.Fatal(err)
	}

	made, err := w.NewFlowFromBlueprint(bt, flow.Collection, "variant", map[string]string{"config": "auth/login"})

	if err != nil {
		t.Fatal(err)
	}
	_, variant, _ := w.flowFor(made)
	client := variant.Nodes[4].Branches[1].Nodes[0].Request.Request
	if client.Path != "auth/login" || client.ID == "" {
		t.Fatalf("client %+v", client)
	}
}

func TestNewFlowFromBlueprint_should_refuse_bad_input_and_write_nothing(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	bt, err := w.CreateBlueprint(flow, "onboarding", "Onboarding template")
	if err != nil {
		t.Fatal(err)
	}
	root := w.Collections()[0].Root
	before := countFiles(t, filepath.Join(root, "flows"))
	tests := map[string]struct {
		path   string
		values map[string]string
		code   diag.Code
	}{
		"traversal":         {"../escape", nil, diag.CodeWriteRequestPath},
		"empty path":        {"", nil, diag.CodeWriteRequestPath},
		"existing flow":     {"onboarding-check", nil, diag.CodeWriteRequestPath},
		"unknown parameter": {"x", map[string]string{"colour": "red"}, diag.CodeBlueprintUse},
		"request not found": {"x", map[string]string{"config": "no/such"}, diag.CodeBlueprintUse},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := w.NewFlowFromBlueprint(bt, flow.Collection, tt.path, tt.values)

			var list *diag.ListError
			if !errors.As(err, &list) || list.List[0].Code != tt.code {
				t.Fatalf("err %v, want %s", err, tt.code)
			}
			if after := countFiles(t, filepath.Join(root, "flows")); after != before {
				t.Fatalf("flows dir changed: %d → %d files", before, after)
			}
		})
	}
}

func TestCreateBlueprint_should_refuse_a_path_that_is_taken_or_unsafe(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	if _, err := w.CreateBlueprint(flow, "onboarding", "T"); err != nil {
		t.Fatal(err)
	}

	for name, path := range map[string]string{"taken": "onboarding", "traversal": "../../x", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			_, err := w.CreateBlueprint(flow, path, "T")

			var list *diag.ListError
			if !errors.As(err, &list) || list.List[0].Code != diag.CodeWriteRequestPath {
				t.Fatalf("err %v", err)
			}
		})
	}
	if _, err := w.CreateBlueprint(flow, "other", "  "); err == nil {
		t.Fatal("a blueprint without a name was accepted")
	}
}

func TestFindBlueprint_should_match_id_and_path_and_report_unknown(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	bt, err := w.CreateBlueprint(flow, "team/onboarding", "T")
	if err != nil {
		t.Fatal(err)
	}
	bp, _ := w.Blueprint(bt)

	byPath, errPath := w.FindBlueprint("team/onboarding", "")
	byID, errID := w.FindBlueprint(string(bp.ID), "")
	byFile, errFile := w.FindBlueprint("blueprints/team/onboarding.blueprint.yaml", "")
	_, errNone := w.FindBlueprint("nope", "")

	if errPath != nil || errID != nil || errFile != nil || byPath != bt || byID != bt || byFile != bt {
		t.Fatalf("path %v/%v id %v/%v file %v/%v want %v", byPath, errPath, byID, errID, byFile, errFile, bt)
	}
	var list *diag.ListError
	if !errors.As(errNone, &list) || list.List[0].Code != diag.CodeBlueprintNotFound {
		t.Fatalf("err %v", errNone)
	}
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestNewFlowFromBlueprint_should_work_with_the_blueprint_shipped_in_the_example(t *testing.T) {
	w := demoWorkspace(t)
	bt, err := w.FindBlueprint("two-calls", "")
	if err != nil {
		t.Fatal(err)
	}

	made, err := w.NewFlowFromBlueprint(bt, bt.Collection, "chain", map[string]string{"second": "auth/verify"})

	if err != nil {
		t.Fatal(err)
	}
	_, f, _ := w.flowFor(made)
	if len(f.Nodes) != 2 || f.Nodes[0].Request.Request.Path != "auth/login" || f.Nodes[1].Request.Request.Path != "auth/verify" {
		t.Fatalf("nodes %+v", f.Nodes)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors: %v", diags)
	}
}

func TestResolveCollection_should_pick_one_collection_by_name_id_or_by_being_alone(t *testing.T) {
	w := demoWorkspace(t)
	only := w.Collections()[0].Manifest
	second, err := w.InitCollection(filepath.Join(t.TempDir(), "other"), "Other", true)
	if err != nil {
		t.Fatal(err)
	}

	byName, errName := w.ResolveCollection("Other")
	byID, errID := w.ResolveCollection(string(only.ID))
	_, errMany := w.ResolveCollection("")
	_, errNone := w.ResolveCollection("nope")

	if errName != nil || errID != nil || byName.ID != second.ID || byID.ID != only.ID {
		t.Fatalf("name %v/%v id %v/%v", byName.ID, errName, byID.ID, errID)
	}
	for name, err := range map[string]error{"several mounted, none named": errMany, "unknown": errNone} {
		var list *diag.ListError
		if !errors.As(err, &list) {
			t.Errorf("%s: err %v", name, err)
		}
	}
}

func TestDeleteBlueprint_should_remove_the_file_and_the_blueprint(t *testing.T) {
	w := demoWorkspace(t)
	flow, _ := onboardingFlow(t, w)
	bt, err := w.CreateBlueprint(flow, "onboarding", "Onboarding template")
	if err != nil {
		t.Fatal(err)
	}
	bp, _ := w.Blueprint(bt)

	err = w.DeleteBlueprint(bt)

	_, findErr := w.Blueprint(bt)
	_, statErr := os.Stat(bp.File)
	if err != nil || findErr == nil || !os.IsNotExist(statErr) {
		t.Fatalf("err %v, still found %v, file %v", err, findErr, statErr)
	}
}

func TestDeleteBlueprint_Given_an_unknown_blueprint_Then_reports_it(t *testing.T) {
	w := demoWorkspace(t)

	err := w.DeleteBlueprint(BlueprintTarget{Collection: w.Collections()[0].Manifest.ID, Blueprint: "NOPE"})

	var list *diag.ListError
	if !errors.As(err, &list) || list.List[0].Code != diag.CodeBlueprintNotFound {
		t.Fatalf("err %v", err)
	}
}
