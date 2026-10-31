// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
)

func codesOfErr(err error) []diag.Code {
	var le *diag.ListError
	if !errors.As(err, &le) {
		return nil
	}
	var out []diag.Code
	for _, d := range le.List {
		out = append(out, d.Code)
	}
	return out
}

func TestFindFlow(t *testing.T) {
	w := demoWorkspace(t)
	second := copyDir(t, filepath.Join("examples", "user-creation"))
	manifest := filepath.Join(second, "flowi.yaml")
	data, _ := os.ReadFile(manifest)
	_ = os.WriteFile(manifest, []byte(strings.Replace(string(data), "01JA0000000000000000000001", "01JA0000000000000000000099", 1)), 0o644)
	tests := []struct {
		name, ref, collection string
		want                  diag.Code
		mountSecond           bool
	}{
		{name: "path", ref: "user-creation"},
		{name: "id", ref: "01JA0000000000000000000020"},
		{name: "prefix_and_suffix", ref: "flows/shared/login.arazzo.yaml"},
		{name: "not_found", ref: "nope", want: diag.CodeFlowNotFound},
		{name: "ambiguous", ref: "user-creation", mountSecond: true, want: diag.CodeFlowAmbiguous},
		{name: "unknown_collection", ref: "user-creation", collection: "ghost", want: diag.CodeWSUnknownCollection},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mountSecond {
				mustMount(t, w, second)
			}

			got, err := w.FindFlow(tt.ref, tt.collection)

			if tt.want == "" {
				if err != nil || got.Flow == "" || got.Collection != "01JA0000000000000000000001" {
					t.Fatalf("got %+v, %v", got, err)
				}
				return
			}
			if codes := codesOfErr(err); len(codes) != 1 || codes[0] != tt.want {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func drainRun(run *engine.Run) engine.RunResult {
	for range run.Events() {
	}
	return run.Wait()
}

func TestRunFlow_Given_validation_error_Then_ListError_and_no_run(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	broken := filepath.Join(c.Root, "flows", "broken.arazzo.yaml")
	if err := os.WriteFile(broken, []byte(arazzoFlow("id: 01JA0000000000000000000077\nnodes:\n  - id: a\n    type: teleport\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	reloadAll(t, w)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}

	run, err := w.RunFlow(context.Background(), target, RunOptions{Env: "local"})

	if run != nil || !slicesContains(codesOfErr(err), diag.CodeNodeUnknownType) {
		t.Fatalf("run %v err %v", run, err)
	}
}

func TestRunFlow_Given_undeclared_or_missing_input_Then_FLW_RUN_010_or_009(t *testing.T) {
	w := demoWorkspace(t)
	login, _ := w.FindFlow("shared/login", "")
	target, _ := w.FindFlow("user-creation", "")
	tests := []struct {
		name   string
		target FlowTarget
		inputs map[string]string
		want   diag.Code
	}{
		{name: "undeclared", target: target, inputs: map[string]string{"nope": "1"}, want: diag.CodeRunInputUnknown},
		{name: "declared_ok", target: login, inputs: map[string]string{"email": "a@b.c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run, err := w.RunFlow(context.Background(), tt.target, RunOptions{Env: "local", Inputs: tt.inputs})

			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				if res := drainRun(run); res.Status != engine.StatusPassed {
					t.Fatalf("result %+v", res)
				}
				return
			}
			if run != nil || !slicesContains(codesOfErr(err), tt.want) {
				t.Fatalf("run %v err %v", run, err)
			}
		})
	}
}

func TestRunFlow_Given_missing_required_input_Then_FLW_RUN_009(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	flow := filepath.Join(c.Root, "flows", "needs-input.arazzo.yaml")
	body := "id: 01JA0000000000000000000078\ninputs:\n  who: {}\nnodes:\n  - id: n\n    type: note\n    text: \"{{inputs.who}}\"\n"
	if err := os.WriteFile(flow, []byte(arazzoFlow(body)), 0o644); err != nil {
		t.Fatal(err)
	}
	reloadAll(t, w)
	target, err := w.FindFlow("needs-input", "")
	if err != nil {
		t.Fatal(err)
	}

	run, err := w.RunFlow(context.Background(), target, RunOptions{Env: "local"})

	if run != nil || !slicesContains(codesOfErr(err), diag.CodeRunInputMissing) {
		t.Fatalf("run %v err %v", run, err)
	}
}

func TestRunFlow_onboarding_check_should_pass_and_save_report(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("onboarding-check", "")
	if err != nil {
		t.Fatal(err)
	}

	run, err := w.RunFlow(context.Background(), target, RunOptions{Env: "local", Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	res := drainRun(run)

	if res.Status != engine.StatusPassed || res.Seed != 7 {
		t.Fatalf("result %+v", res)
	}
	want := filepath.Join(w.DataDir(target.Collection), "runs", string(target.Flow))
	if !strings.HasPrefix(res.ReportFile, want) {
		t.Fatalf("report %s", res.ReportFile)
	}
	if _, err := os.Stat(res.ReportFile); err != nil {
		t.Fatal(err)
	}
}

// reloadAll rescans every mounted collection.
func reloadAll(t *testing.T, w *Workspace) {
	t.Helper()
	for _, c := range w.Collections() {
		if _, err := w.Reload(Change{Root: c.Root, File: c.Root, Op: ChangeRescan}); err != nil {
			t.Fatal(err)
		}
	}
}

func slicesContains(codes []diag.Code, c diag.Code) bool {
	for _, x := range codes {
		if x == c {
			return true
		}
	}
	return false
}

func TestCheckSkips_Given_a_skipped_step_that_a_later_step_reads_Then_a_problem_names_the_reader(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}

	got, err := w.CheckSkips(target, []string{"login"})

	if err != nil {
		t.Fatal(err)
	}
	var readers []string
	for _, d := range got {
		if d.Code != diag.CodeRunSkipRead || d.Severity != diag.SeverityError || d.Args["skipped"] != "login" {
			t.Fatalf("unexpected problem %+v", d)
		}
		readers = append(readers, d.Args["reader"])
	}
	if len(readers) == 0 || readers[0] != "session" {
		t.Fatalf("readers %v, want session first", readers)
	}
}

func TestCheckSkips_Given_nothing_skipped_Then_no_problem(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindFlow("user-creation", "")

	got, err := w.CheckSkips(target, nil)

	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}
