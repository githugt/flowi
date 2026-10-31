// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGitBranch_should_find_the_branch_of_the_repository_holding_the_folder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/feature/login\n")
	collection := filepath.Join(root, "apis", "shop")
	if err := os.MkdirAll(collection, 0o755); err != nil {
		t.Fatal(err)
	}

	got := vm.GitBranch(collection)

	if got != "feature/login" {
		t.Fatalf("got %q", got)
	}
}

func TestGitBranch_Given_a_detached_head_Then_names_the_commit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".git", "HEAD"), "1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d\n")

	got := vm.GitBranch(root)

	if got != "detached at 1a2b3c4" {
		t.Fatalf("got %q", got)
	}
}

func TestGitBranch_Given_a_linked_worktree_Then_follows_the_gitdir_file(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, "main.git", "worktrees", "wt")
	writeFile(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/topic\n")
	work := filepath.Join(root, "wt")
	writeFile(t, filepath.Join(work, ".git"), "gitdir: "+gitDir+"\n")

	got := vm.GitBranch(work)

	if got != "topic" {
		t.Fatalf("got %q", got)
	}
}

func TestGitBranch_Given_no_repository_Then_is_empty(t *testing.T) {
	if got := vm.GitBranch(t.TempDir()); got != "" {
		t.Fatalf("got %q", got)
	}
}
