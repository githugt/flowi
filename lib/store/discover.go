// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/flowioverlay"
)

// fileKind classifies a file found in a collection directory.
type fileKind int

const (
	kindIgnored fileKind = iota
	kindEnvironment
	kindRequest
	kindFlow
	kindFolder
	kindBlueprint
	kindOverlay
	kindUnrecognised
)

const (
	manifestName    = "flowi.yaml"
	folderName      = "_folder.yaml"
	requestSuffix   = ".req.yaml"
	flowSuffix      = ".arazzo.yaml"
	blueprintSuffix = ".blueprint.yaml"
	layoutSuffix    = ".layout.yaml"
	examplesDir     = ".examples"
)

// discovered is one classified file. rel is slash-separated and relative
// to the collection root.
type discovered struct {
	kind fileKind
	abs  string
	rel  string
}

// classify decides what a collection-relative path is. It is shared with
// the watcher so both agree on which files matter.
func classify(rel string) fileKind {
	if isEditorScratch(path.Base(rel)) {
		return kindIgnored
	}
	top, rest, nested := strings.Cut(rel, "/")
	isYAML := strings.HasSuffix(rel, ".yaml")
	switch {
	case !nested || !isYAML:
		return kindIgnored
	case top == "environments":
		if strings.Contains(rest, "/") {
			return kindUnrecognised
		}
		return kindEnvironment
	case top == "blueprints":
		if strings.HasSuffix(rel, blueprintSuffix) {
			return kindBlueprint
		}
		return kindUnrecognised
	case top == "overlays":
		if strings.HasSuffix(rel, flowioverlay.Suffix) && !strings.Contains(rest, "/") {
			return kindOverlay
		}
		return kindUnrecognised
	case top != "requests" && top != "flows":
		return kindIgnored
	}
	return classifyTree(top, path.Base(rel))
}

// isEditorScratch recognises lock, backup and autosave files that editors
// leave next to the files being edited (Emacs ".#x", "#x#", "x~").
func isEditorScratch(base string) bool {
	return strings.HasPrefix(base, ".#") || strings.HasPrefix(base, "#") || strings.HasSuffix(base, "~")
}

func classifyTree(top, base string) fileKind {
	switch {
	case base == folderName:
		return kindFolder
	case strings.HasSuffix(base, layoutSuffix):
		return kindIgnored
	case top == "requests" && strings.HasSuffix(base, requestSuffix):
		return kindRequest
	case top == "flows" && strings.HasSuffix(base, flowSuffix):
		return kindFlow
	}
	return kindUnrecognised
}

// discover walks root and returns every relevant file in path order.
// VCS directories and example directories are skipped. Entries that
// cannot be read are reported and skipped, so one bad entry never hides
// the rest of the collection.
func discover(root string) ([]discovered, diag.List, error) {
	var out []discovered
	var diags diag.List
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			diags = append(diags, unreadable(p, err))
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || strings.HasSuffix(d.Name(), examplesDir) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if kind := classify(rel); kind != kindIgnored {
			out = append(out, discovered{kind: kind, abs: p, rel: rel})
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("scanning collection %s: %w", root, err)
	}
	slices.SortFunc(out, func(a, b discovered) int { return strings.Compare(a.rel, b.rel) })
	return out, diags, nil
}

func unreadable(file string, err error) diag.Diagnostic {
	return diag.New(diag.CodeFileUnreadable, diag.Location{File: file},
		map[string]string{"name": filepath.Base(file), "detail": rootCause(err)})
}

// rootCause strips the path that fs errors repeat, keeping "permission
// denied" or "no such file or directory".
func rootCause(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// entityPath turns "requests/auth/login.req.yaml" into "auth/login".
func entityPath(rel, top, suffix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(rel, top+"/"), suffix)
}
