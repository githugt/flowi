// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/arazzo"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/migrate"
	"github.com/githugt/flowi-lib/model"
)

// Load reads the collection at root. User problems are returned as
// diagnostics; the error is reserved for I/O failures and ErrNotCollection.
func (s *Store) Load(root string) (model.Collection, diag.List, error) {
	root, err := resolveRoot(root)
	if err != nil {
		return model.Collection{}, nil, err
	}
	c := model.Collection{Root: root}
	manifest, diags, err := s.loadManifest(root)
	if err != nil {
		return c, nil, err
	}
	c.Manifest = manifest
	if slices.ContainsFunc(diags, func(d diag.Diagnostic) bool { return d.Code == diag.CodeFormatTooNew }) {
		// Files written by a newer Flow-I may use syntax this version
		// misreads, so nothing else is decoded.
		return c, diags, nil
	}
	files, walkDiags, err := discover(root)
	if err != nil {
		return c, nil, err
	}
	diags = append(diags, walkDiags...)
	for _, f := range files {
		var fileDiags diag.List
		c, fileDiags, err = s.loadFile(c, f)
		if err != nil {
			return c, nil, err
		}
		diags = append(diags, fileDiags...)
	}
	return c, diags, nil
}

func resolveRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", root, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%s: %w", root, ErrNotCollection)
	}
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", root, err)
	}
	return resolved, nil
}

func (s *Store) loadManifest(root string) (model.Manifest, diag.List, error) {
	file := filepath.Join(root, manifestName)
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return model.Manifest{}, nil, fmt.Errorf("%s: %w", root, ErrNotCollection)
	}
	if err != nil {
		return model.Manifest{}, nil, fmt.Errorf("reading %s: %w", file, err)
	}
	s.remember(file, data)
	m, diags := s.codec.DecodeManifest(file, data)
	if diags.HasErrors() && m.Format == 0 {
		return m, diags, nil // the format line itself is broken; already reported
	}
	check := migrate.Check(m.Format)
	if check.Code == "" {
		return m, diags, nil
	}
	check.File = file
	if check.Code == diag.CodeFormatTooNew {
		// Other problems in a newer-format file are likely false alarms.
		return m, diag.List{s.Locate(check)}, nil
	}
	diags = append(diags, s.Locate(check))
	return m, diags, nil
}

// loadFile decodes one discovered file into the collection.
func (s *Store) loadFile(c model.Collection, f discovered) (model.Collection, diag.List, error) {
	if f.kind == kindUnrecognised {
		d := diag.New(diag.CodeFileUnrecognised, diag.Location{File: f.abs}, map[string]string{"name": path.Base(f.rel)})
		return c, diag.List{d}, nil
	}
	data, err := os.ReadFile(f.abs)
	if err != nil {
		// A dangling symlink or unreadable file is that file's problem only.
		return c, diag.List{unreadable(f.abs, err)}, nil
	}
	s.remember(f.abs, data)
	c, diags := s.decodeInto(c, f, data)
	return c, diags, nil
}

func (s *Store) decodeInto(c model.Collection, f discovered, data []byte) (model.Collection, diag.List) {
	var diags diag.List
	switch f.kind {
	case kindEnvironment:
		var env model.Environment
		env, diags = s.codec.DecodeEnvironment(f.abs, data)
		if env.ID == "" {
			break
		}
		if env.Name == "" {
			env.Name = strings.TrimSuffix(path.Base(f.rel), ".yaml")
		}
		c.Environments = append(c.Environments, env)
	case kindRequest:
		var r model.Request
		r, diags = s.codec.DecodeRequest(f.abs, entityPath(f.rel, "requests", requestSuffix), data)
		if r.ID != "" {
			c.Requests = append(c.Requests, r)
		}
	case kindFlow:
		var fl model.Flow
		fl, diags = arazzo.DecodeFlow(f.abs, entityPath(f.rel, "flows", flowSuffix), data, s.flowOptions(c, entityPath(f.rel, "flows", flowSuffix)))
		if fl.ID != "" {
			c.Flows = append(c.Flows, fl)
		}
	case kindBlueprint:
		var bp model.Blueprint
		bp, diags = s.codec.DecodeBlueprint(f.abs, entityPath(f.rel, "blueprints", blueprintSuffix), data)
		if bp.ID != "" {
			c.Blueprints = append(c.Blueprints, bp)
		}
	case kindFolder:
		var fo model.Folder
		fo, diags = s.codec.DecodeFolder(f.abs, path.Dir(f.rel), data)
		if !unparsed(diags) {
			c.Folders = append(c.Folders, fo)
		}
	}
	return c, diags
}

// ReloadFile re-reads one file and returns a new collection with that
// file's entity replaced, added or (if the file is gone) removed.
func (s *Store) ReloadFile(c model.Collection, file string) (model.Collection, diag.List, error) {
	file, err := resolveFile(file)
	if err != nil {
		return c, nil, err
	}
	rel, err := filepath.Rel(c.Root, file)
	if err != nil {
		return c, nil, fmt.Errorf("relating %s to %s: %w", file, c.Root, err)
	}
	rel = filepath.ToSlash(rel)
	if rel == manifestName {
		return s.reloadManifest(c)
	}
	next := withoutFile(c, file)
	if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
		// The path may have been a directory: drop everything beneath it.
		s.forget(file)
		return withoutPrefix(next, file+string(filepath.Separator)), nil, nil
	}
	kind := classify(rel)
	if kind == kindIgnored {
		return c, nil, nil
	}
	next, diags, err := s.loadFile(next, discovered{kind: kind, abs: file, rel: rel})
	if err != nil {
		return c, nil, err
	}
	return sortEntities(next), diags, nil
}

func (s *Store) reloadManifest(c model.Collection) (model.Collection, diag.List, error) {
	m, diags, err := s.loadManifest(c.Root)
	if err != nil {
		return c, nil, err
	}
	next := c
	next.Manifest = m
	return next, diags, nil
}

// withoutFile returns a copy of c with every entity loaded from file removed.
func withoutFile(c model.Collection, file string) model.Collection {
	c.Environments = slices.DeleteFunc(slices.Clone(c.Environments), func(e model.Environment) bool { return e.File == file })
	c.Requests = slices.DeleteFunc(slices.Clone(c.Requests), func(r model.Request) bool { return r.File == file })
	c.Flows = slices.DeleteFunc(slices.Clone(c.Flows), func(f model.Flow) bool { return f.File == file })
	c.Folders = slices.DeleteFunc(slices.Clone(c.Folders), func(f model.Folder) bool { return f.File == file })
	c.Blueprints = slices.DeleteFunc(slices.Clone(c.Blueprints), func(b model.Blueprint) bool { return b.File == file })
	return c
}

// withoutPrefix returns a copy of c without entities loaded from under dir.
func withoutPrefix(c model.Collection, dir string) model.Collection {
	under := func(file string) bool { return strings.HasPrefix(file, dir) }
	c.Environments = slices.DeleteFunc(slices.Clone(c.Environments), func(e model.Environment) bool { return under(e.File) })
	c.Requests = slices.DeleteFunc(slices.Clone(c.Requests), func(r model.Request) bool { return under(r.File) })
	c.Flows = slices.DeleteFunc(slices.Clone(c.Flows), func(f model.Flow) bool { return under(f.File) })
	c.Folders = slices.DeleteFunc(slices.Clone(c.Folders), func(f model.Folder) bool { return under(f.File) })
	c.Blueprints = slices.DeleteFunc(slices.Clone(c.Blueprints), func(b model.Blueprint) bool { return under(b.File) })
	return c
}

// sortEntities keeps entities in file-path order, matching Load.
func sortEntities(c model.Collection) model.Collection {
	slices.SortFunc(c.Environments, func(a, b model.Environment) int { return strings.Compare(a.File, b.File) })
	slices.SortFunc(c.Requests, func(a, b model.Request) int { return strings.Compare(a.File, b.File) })
	slices.SortFunc(c.Flows, func(a, b model.Flow) int { return strings.Compare(a.File, b.File) })
	slices.SortFunc(c.Folders, func(a, b model.Folder) int { return strings.Compare(a.File, b.File) })
	slices.SortFunc(c.Blueprints, func(a, b model.Blueprint) int { return strings.Compare(a.File, b.File) })
	return c
}

// resolveFile makes file absolute with symlinks resolved, like collection
// roots. Only the directory is resolved because the file may be deleted.
func resolveFile(file string) (string, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", file, err)
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", file, err)
	}
	return filepath.Join(dir, filepath.Base(abs)), nil
}

// unparsed reports whether a file could not be read as a mapping at all.
func unparsed(diags diag.List) bool {
	return slices.ContainsFunc(diags, func(d diag.Diagnostic) bool {
		return d.Code == diag.CodeYAMLSyntax || d.Code == diag.CodeYAMLEmpty || (d.Code == diag.CodeYAMLType && d.Path == "$")
	})
}

// LoadEnvironments reads the global environment files in dir (one
// <name>.yaml each). A missing dir means there are none. Every environment
// is stamped with scope, the vault scope of its secrets. Problems are
// diagnostics; a broken file does not hide the others.
func (s *Store) LoadEnvironments(dir string, scope model.ID) ([]model.Environment, diag.List, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var envs []model.Environment
	var diags diag.List
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") || isEditorScratch(e.Name()) {
			continue
		}
		file := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(file)
		if err != nil {
			diags = append(diags, unreadable(file, err))
			continue
		}
		s.remember(file, data)
		env, fileDiags := s.codec.DecodeEnvironment(file, data)
		diags = append(diags, fileDiags...)
		if env.ID == "" {
			continue
		}
		if env.Name == "" {
			env.Name = strings.TrimSuffix(e.Name(), ".yaml")
		}
		env.Scope = scope
		envs = append(envs, env)
	}
	return envs, diags, nil
}
