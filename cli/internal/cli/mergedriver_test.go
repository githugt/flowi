// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
)

type fakeMerge struct {
	result flowi.MergedFlow
}

func (f fakeMerge) MergeFlowFile(string, []byte, []byte, []byte, flowi.LineMerger) (flowi.MergedFlow, error) {
	return f.result, nil
}

func threeFiles(t *testing.T) (base, ours, theirs string) {
	t.Helper()
	dir := t.TempDir()
	base, ours, theirs = filepath.Join(dir, "base"), filepath.Join(dir, "ours"), filepath.Join(dir, "theirs")
	for f, text := range map[string]string{base: "b\n", ours: "o\n", theirs: "t\n"} {
		if err := os.WriteFile(f, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return base, ours, theirs
}

// Before: git merged flow files as text, and two branches that each changed a flow conflicted on the digest line.
func TestRunMergeDriver_Given_a_clean_flow_merge_Then_ours_holds_the_regenerated_file(t *testing.T) {
	base, ours, theirs := threeFiles(t)

	conflict, err := runMergeDriver(fakeMerge{flowi.MergedFlow{Clean: true, Data: []byte("merged flow\n")}}, base, ours, theirs, "flows/a.arazzo.yaml", nil)

	got, _ := os.ReadFile(ours)
	if err != nil || conflict || string(got) != "merged flow\n" {
		t.Fatalf("conflict %v err %v file %q", conflict, err, got)
	}
}

func TestRunMergeDriver_Given_a_flow_it_cannot_merge_Then_the_whole_files_are_merged_as_text(t *testing.T) {
	base, ours, theirs := threeFiles(t)
	lines := func(_, o, th []byte) ([]byte, bool, error) {
		return []byte("<<<<<<< ours\n" + string(o) + "=======\n" + string(th) + ">>>>>>> theirs\n"), false, nil
	}

	conflict, err := runMergeDriver(fakeMerge{flowi.MergedFlow{Reason: "conflict"}}, base, ours, theirs, "flows/a.arazzo.yaml", lines)

	got, _ := os.ReadFile(ours)
	if err != nil || !conflict || !strings.Contains(string(got), "<<<<<<< ours") {
		t.Fatalf("conflict %v err %v file %q", conflict, err, got)
	}
}

func TestEnsureAttributes_should_add_the_line_once_and_keep_what_is_there(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".gitattributes")
	if err := os.WriteFile(file, []byte("*.png binary"), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := ensureAttributes(dir)
	second, errAgain := ensureAttributes(dir)

	got, _ := os.ReadFile(file)
	if err != nil || errAgain != nil || !first || second || string(got) != "*.png binary\n"+attributesLine+"\n" {
		t.Fatalf("first %v second %v err %v %v file %q", first, second, err, errAgain, got)
	}
}
