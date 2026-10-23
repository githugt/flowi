// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"os"
	"path/filepath"
	"strings"
)

// GitBranch is the branch checked out in the git repository that holds dir,
// "" when dir is in none. A detached HEAD reads as "detached at 1a2b3c4".
// It reads the repository's files and runs no git command, so it is cheap and
// works where git is not installed.
func GitBranch(dir string) string {
	gitDir := findGitDir(dir)
	if gitDir == "" {
		return ""
	}
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	return branchFromHead(strings.TrimSpace(string(head)))
}

func branchFromHead(head string) string {
	if ref, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		return ref
	}
	if ref, ok := strings.CutPrefix(head, "ref: "); ok {
		return ref
	}
	if len(head) >= 7 {
		return "detached at " + head[:7]
	}
	return ""
}

// findGitDir walks up from dir to the first ".git": a directory, or, in a
// linked worktree or submodule, a file saying where the real one is.
func findGitDir(dir string) string {
	for {
		candidate := filepath.Join(dir, ".git")
		info, err := os.Stat(candidate)
		switch {
		case err == nil && info.IsDir():
			return candidate
		case err == nil:
			return gitDirFromFile(dir, candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// gitDirFromFile reads "gitdir: <path>", relative to the file's folder when not absolute.
func gitDirFromFile(dir, file string) string {
	text, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	path, ok := strings.CutPrefix(strings.TrimSpace(string(text)), "gitdir: ")
	if !ok {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return path
}
