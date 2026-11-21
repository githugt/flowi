// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
)

func TestPrintDiagnostics_github_should_write_one_annotation_per_diagnostic(t *testing.T) {
	base := t.TempDir()
	var out bytes.Buffer

	err := PrintDiagnostics(&out, OutputGitHub, base, sampleDiagnostics(base))

	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines:\n%s", len(lines), out.String())
	}
	wantPrefix := "::error file=" + strings.NewReplacer(":", "%3A", ",", "%2C").Replace(filepath.Join("ws", "workspace.yaml")) + ",line=4,col=11,title=FLW-WS-001::"
	if !strings.HasPrefix(lines[0], wantPrefix) || !strings.Contains(lines[0], "collection 01J not found at /gone") {
		t.Fatalf("got %q, want prefix %q", lines[0], wantPrefix)
	}
	if !strings.HasPrefix(lines[1], "::warning file=") || strings.Contains(lines[1], ",line=") {
		t.Fatalf("a warning with no line should have no line: %q", lines[1])
	}
}

func TestPrintDiagnostics_github_should_escape_newlines_and_percent_in_the_message(t *testing.T) {
	d := diag.New(diag.CodeFileUnrecognised, diag.Location{File: "a,b:c.yaml"}, map[string]string{"name": "50%\nbad"})
	var out bytes.Buffer

	if err := PrintDiagnostics(&out, OutputGitHub, "", diag.List{d}); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, "50%25%0Abad") || !strings.Contains(got, "file=a%2Cb%3Ac.yaml") {
		t.Fatalf("got %q", got)
	}
}

func TestWantsAnnotations_should_follow_the_flag_and_the_CI_variable(t *testing.T) {
	ci := []string{"GITHUB_ACTIONS=true"}
	cases := []struct {
		output  string
		environ []string
		want    bool
	}{
		{OutputText, nil, false},
		{OutputText, ci, true},
		{OutputText, []string{"GITHUB_ACTIONS=false"}, false},
		{OutputGitHub, nil, true},
		{OutputJSON, ci, false}, // JSON output stays clean for the program that reads it
	}
	for _, c := range cases {
		if got := wantsAnnotations(c.output, c.environ); got != c.want {
			t.Errorf("wantsAnnotations(%q, %v) = %v, want %v", c.output, c.environ, got, c.want)
		}
	}
}

func TestWriteStepSummary_should_append_the_run_and_its_failures(t *testing.T) {
	file := filepath.Join(t.TempDir(), "summary.md")
	failures := diag.List{diag.New(diag.CodeWSMissing, diag.Location{File: "ws/workspace.yaml"}, map[string]string{"id": "01J", "path": "a|b"})}
	res := engine.RunResult{RunID: "r1", Seed: 7, Status: engine.StatusFailed}

	if err := writeStepSummary([]string{"GITHUB_STEP_SUMMARY=" + file}, res, "", failures); err != nil {
		t.Fatal(err)
	}
	if err := writeStepSummary([]string{"GITHUB_STEP_SUMMARY=" + file}, res, "", nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Count(got, "### Flowi run: failed") != 2 || !strings.Contains(got, "| FLW-WS-001 | ws/workspace.yaml |") || !strings.Contains(got, `a\|b`) {
		t.Fatalf("got:\n%s", got)
	}
}

func TestWriteStepSummary_should_do_nothing_without_the_variable(t *testing.T) {
	if err := writeStepSummary(nil, engine.RunResult{}, "", nil); err != nil {
		t.Fatal(err)
	}
}
