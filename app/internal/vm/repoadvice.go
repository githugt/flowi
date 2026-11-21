// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxAdviceEntries bounds how many directory entries the advice looks at: a collection inside a huge tree must not
// make the page slow.
const maxAdviceEntries = 20000

// RepoAdvice tells what to do about the files Flowi generates, for a collection kept in a git repository: saved
// example folders can hold tokens from real responses, and layout files conflict easily. It says nothing when the
// collection is not in a repository, or when the repository already handles the files.
//
// Before: nothing told the person which generated files belong in git.
func RepoAdvice(collectionDir string) []string {
	root := gitRoot(collectionDir)
	if root == "" {
		return nil
	}
	examples, layouts := 0, 0
	seen := 0
	_ = filepath.WalkDir(collectionDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if seen++; seen > maxAdviceEntries {
			return fs.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			switch {
			case name == ".git" || name == "node_modules":
				return fs.SkipDir
			case strings.HasSuffix(name, ".examples"):
				examples++
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".layout.yaml") {
			layouts++
		}
		return nil
	})
	var out []string
	if examples > 0 && !mentions(filepath.Join(root, ".gitignore"), ".examples") {
		out = append(out, fmt.Sprintf("%d saved example %s (*.examples) can hold tokens copied from real responses. Review them before committing, or add `*.examples/` to .gitignore.", examples, folders(examples)))
	}
	if layouts > 0 && !mentions(filepath.Join(root, ".gitattributes"), "merge=union") {
		out = append(out, fmt.Sprintf("%d layout %s (*.layout.yaml) change whenever a step moves and conflict easily. Add `*.layout.yaml merge=union` to .gitattributes.", layouts, files(layouts)))
	}
	return out
}

func folders(n int) string {
	if n == 1 {
		return "folder"
	}
	return "folders"
}

func files(n int) string {
	if n == 1 {
		return "file"
	}
	return "files"
}

// gitRoot is the directory with a .git entry at or above dir (up to eight levels), or "".
func gitRoot(dir string) string {
	d, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

// mentions reports whether the text file at path contains word. A missing file mentions nothing.
func mentions(path, word string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), word)
}
