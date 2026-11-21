// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func touch(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRepoAdvice_Given_a_repo_with_examples_and_layouts_Then_it_says_what_to_do(t *testing.T) {
	// Before: nothing told the person which generated files belong in git.
	root := t.TempDir()
	touch(t, filepath.Join(root, ".git", "HEAD"), "ref")
	touch(t, filepath.Join(root, "col", "requests", "a.examples", "ok.json"), "{}")
	touch(t, filepath.Join(root, "col", "flows", "f.layout.yaml"), "x: 1")

	got := RepoAdvice(filepath.Join(root, "col"))

	if len(got) != 2 || !strings.Contains(got[0], "tokens") || !strings.Contains(got[1], "merge=union") {
		t.Fatalf("advice %q", got)
	}
}

func TestRepoAdvice_Given_the_repo_already_handles_them_Then_it_is_quiet(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, ".git", "HEAD"), "ref")
	touch(t, filepath.Join(root, ".gitignore"), "*.examples/\n")
	touch(t, filepath.Join(root, ".gitattributes"), "*.layout.yaml merge=union\n")
	touch(t, filepath.Join(root, "requests", "a.examples", "ok.json"), "{}")
	touch(t, filepath.Join(root, "flows", "f.layout.yaml"), "x: 1")

	if got := RepoAdvice(root); len(got) != 0 {
		t.Fatalf("advice %q, want none", got)
	}
}

func TestRepoAdvice_Given_no_repository_Then_it_says_nothing(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "requests", "a.examples", "ok.json"), "{}")

	if got := RepoAdvice(root); got != nil {
		t.Fatalf("advice %q, want none", got)
	}
}
