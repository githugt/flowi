// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/arazzo"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// replaceLast replaces the last occurrence: in a flow file the flow's own data comes after the derived steps.
func replaceLast(s, old, repl string) string {
	i := strings.LastIndex(s, old)
	if i < 0 {
		panic("not found: " + old)
	}
	return s[:i] + repl + s[i+len(old):]
}

// mergeFixture is the example flow as base, with two branches that each changed it somewhere else.
func mergeFixture(t *testing.T, edits func(ours, theirs string) (string, string)) (w *Workspace, file string, base, ours, theirs []byte) {
	t.Helper()
	requireGit(t)
	w = demoWorkspace(t)
	file = filepath.Join(w.Collections()[0].Root, "flows", "user-creation.arazzo.yaml")
	base, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	o, th := edits(string(base), string(base))
	return w, file, base, []byte(o), []byte(th)
}

// Before: every flow edit rewrote the derived steps and the digest, so two branches that changed different steps of a
// flow always conflicted on the digest line, which nobody wrote.
func TestMergeFlowFile_Given_edits_to_different_steps_Then_it_merges_and_writes_the_derived_parts_afresh(t *testing.T) {
	w, file, base, ours, theirs := mergeFixture(t, func(o, th string) (string, string) {
		o = replaceLast(o, `code: "123456"`, `code: "111111"`)
		th = strings.Replace(th, "      prefer_version: \"2\"", "      prefer_version: \"1\"", 1)
		return o, th
	})

	got, err := w.MergeFlowFile(file, base, ours, theirs, GitLineMerger(context.Background()))

	if err != nil || !got.Clean {
		t.Fatalf("%v clean=%v reason=%q", err, got.Clean, got.Reason)
	}
	text := string(got.Data)
	if !strings.Contains(text, `code: "111111"`) || !strings.Contains(text, `prefer_version: "1"`) {
		t.Fatalf("both edits must be there:\n%s", text)
	}
	if _, diags := arazzo.DecodeFlow(file, "user-creation", got.Data, w.store.FlowOptionsFor(w.Collections()[0], "user-creation")); len(diags) != 0 {
		t.Fatalf("the merged file must read back with no drift: %+v", diags)
	}
}

func TestMergeFlowFile_Given_both_sides_changing_the_same_line_Then_it_is_not_clean(t *testing.T) {
	w, file, base, ours, theirs := mergeFixture(t, func(o, th string) (string, string) {
		return replaceLast(o, `code: "123456"`, `code: "111111"`), replaceLast(th, `code: "123456"`, `code: "222222"`)
	})

	got, err := w.MergeFlowFile(file, base, ours, theirs, GitLineMerger(context.Background()))

	if err != nil || got.Clean || got.Reason != "conflict" || len(got.Data) != 0 {
		t.Fatalf("clean=%v reason=%q data=%d bytes err=%v", got.Clean, got.Reason, len(got.Data), err)
	}
}

func TestMergeFlowFile_Given_a_file_that_is_not_a_flow_Then_it_is_left_to_a_text_merge(t *testing.T) {
	w, file, base, _, _ := mergeFixture(t, func(o, th string) (string, string) { return o, th })

	got, err := w.MergeFlowFile(file, base, []byte("not: a flow\n"), base, GitLineMerger(context.Background()))
	outside, errOutside := w.MergeFlowFile(filepath.Join(t.TempDir(), "x.arazzo.yaml"), base, base, base, GitLineMerger(context.Background()))

	if err != nil || errOutside != nil || got.Clean || outside.Clean || got.Reason == "" || outside.Reason == "" {
		t.Fatalf("%+v %+v", got, outside)
	}
}

func TestGitLineMerger_should_report_conflicts_with_markers(t *testing.T) {
	requireGit(t)

	merged, clean, err := GitLineMerger(context.Background())([]byte("a\nb\n"), []byte("a\nours\n"), []byte("a\ntheirs\n"))

	if err != nil || clean || !bytes.Contains(merged, []byte("<<<<<<<")) {
		t.Fatalf("%q %v %v", merged, clean, err)
	}
}
