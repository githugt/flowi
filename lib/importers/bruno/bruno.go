// SPDX-License-Identifier: Apache-2.0

// Package bruno imports a Bruno collection folder: every .bru request
// becomes a request (folders kept), environments/*.bru become
// environments, collection.bru and folder.bru headers and auth are
// inherited, asserts become request asserts and post-response vars become
// extracts. Scripts and tests are reported; credentials never written.
package bruno

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/model"
)

// settings are the headers and auth a folder passes to its requests.
type settings struct {
	headers []entry
	auth    *block // the auth:<mode> block, nil for none
	mode    string // the auth mode ("", "none", "bearer", …)
}

type importer struct {
	fsys    fs.FS
	base    string // shown before file names in reports
	folders map[string]settings
	runtime map[string]bool
	taken   importers.Names
	ignore  []string
	result  importers.Result
}

// Import reads the collection in fsys. base is how reports name the
// folder. A folder without bruno.json is FLW-IMPORT-002.
func Import(fsys fs.FS, base string) (importers.Result, diag.List) {
	data, err := fs.ReadFile(fsys, "bruno.json")
	if err != nil {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, diag.Location{File: base},
			map[string]string{"detail": "a Bruno collection folder has bruno.json"})}
	}
	var meta struct {
		Ignore []string `json:"ignore"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportSyntax, diag.Location{File: path.Join(base, "bruno.json")},
			map[string]string{"format": "bruno.json", "detail": err.Error()})}
	}
	im := &importer{fsys: fsys, base: base, folders: map[string]settings{},
		ignore: append(meta.Ignore, "node_modules", ".git", "environments")}
	im.runtime = im.runtimeVars()
	im.folders["."] = im.collectionSettings()
	if err := fs.WalkDir(fsys, ".", im.visit); err != nil {
		return importers.Result{}, diag.List{diag.New(diag.CodeImportInvalid, diag.Location{File: base}, map[string]string{"detail": err.Error()})}
	}
	im.environments()
	return im.result, nil
}

func (im *importer) loc(file string, line, col int) diag.Location {
	return diag.Location{File: path.Join(im.base, file), Pos: diag.Position{Line: line, Column: col}}
}

// read parses a .bru file; a syntax error is reported on it with an
// excerpt and the item skipped.
func (im *importer) read(file string, it *importers.Item) ([]block, bool) {
	src, err := fs.ReadFile(im.fsys, file)
	if err != nil {
		it.Skip(err.Error())
		return nil, false
	}
	src = trimBOM(src)
	blocks, err := parse(src)
	var se *syntaxError
	if errors.As(err, &se) {
		d := diag.New(diag.CodeImportSyntax, im.loc(file, se.line, se.col), map[string]string{"format": "Bruno", "detail": se.detail})
		d.Excerpt = diag.ExcerptAt(src, d.Pos)
		it.Report(d)
		it.Skip("the file cannot be read")
		return nil, false
	}
	return blocks, true
}

// runtimeVars are the names any request sets after its response; they
// are read bare so a flow sees the extracted value.
func (im *importer) runtimeVars() map[string]bool {
	out := map[string]bool{}
	_ = fs.WalkDir(im.fsys, ".", func(p string, d fs.DirEntry, err error) error { //nolint:errcheck // a pre-pass: walk problems are reported by the main walk
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".bru") {
			return nil
		}
		src, err := fs.ReadFile(im.fsys, p)
		if err != nil {
			return nil
		}
		blocks, err := parse(trimBOM(src))
		if err != nil {
			return nil
		}
		if b, ok := find(blocks, "vars:post-response"); ok {
			for _, e := range b.enabled() {
				out[e.key] = true
			}
		}
		return nil
	})
	return out
}

func (im *importer) collectionSettings() settings {
	it := importers.Item{Name: "collection", Source: im.loc("collection.bru", 1, 1)}
	if _, err := fs.Stat(im.fsys, "collection.bru"); err != nil {
		return settings{}
	}
	blocks, ok := im.read("collection.bru", &it)
	s := settings{}
	if ok {
		s = folderSettings(blocks, settings{})
		reportScripts(&it, "collection", blocks)
	}
	if len(it.Diags) > 0 {
		im.result.Items = append(im.result.Items, it)
	}
	return s
}

// folderSettings applies a collection.bru or folder.bru over its parent:
// headers add up, auth replaces unless it says inherit.
func folderSettings(blocks []block, parent settings) settings {
	s := settings{headers: slices.Clone(parent.headers), auth: parent.auth, mode: parent.mode}
	if b, ok := find(blocks, "headers"); ok {
		s.headers = append(s.headers, b.enabled()...)
	}
	if b, ok := find(blocks, "auth"); ok {
		if mode := b.value("mode"); mode != "" && mode != "inherit" {
			s.mode, s.auth = mode, authBlock(blocks, mode)
		}
	}
	return s
}

func authBlock(blocks []block, mode string) *block {
	if b, ok := find(blocks, "auth:"+mode); ok {
		return &b
	}
	return nil
}

func reportScripts(it *importers.Item, owner string, blocks []block) {
	for _, b := range blocks {
		if (strings.HasPrefix(b.name, "script:") || b.name == "tests") && strings.TrimSpace(b.text) != "" {
			it.Unmapped(owner+" "+b.name, "left out", "scripts are JavaScript; use hooks and extract or assert nodes instead")
		}
	}
}

func (im *importer) visit(p string, d fs.DirEntry, err error) error {
	switch {
	case err != nil:
		return err
	case d.IsDir() && p != "." && slices.Contains(im.ignore, d.Name()):
		return fs.SkipDir
	case d.IsDir():
		im.folder(p)
	case strings.HasSuffix(p, ".bru") && !slices.Contains([]string{"collection.bru", "folder.bru"}, path.Base(p)):
		im.request(p)
	}
	return nil
}

func (im *importer) folder(dir string) {
	if dir == "." {
		return
	}
	parent := im.folders[path.Dir(dir)]
	file := path.Join(dir, "folder.bru")
	if _, err := fs.Stat(im.fsys, file); err != nil {
		im.folders[dir] = parent
		return
	}
	it := importers.Item{Name: "folder " + dir, Source: im.loc(file, 1, 1)}
	if blocks, ok := im.read(file, &it); ok {
		im.folders[dir] = folderSettings(blocks, parent)
		reportScripts(&it, "folder "+dir, blocks)
	} else {
		im.folders[dir] = parent
	}
	if len(it.Diags) > 0 {
		im.result.Items = append(im.result.Items, it)
	}
}

// requestPath is the request's path: its folders and file stem.
func requestPath(file string) string {
	var segs []string
	for _, s := range strings.Split(strings.TrimSuffix(file, ".bru"), "/") {
		if s = importers.FileName(s); s != "" {
			segs = append(segs, s)
		}
	}
	return strings.Join(segs, "/")
}

func (im *importer) request(file string) {
	it := importers.Item{Name: requestPath(file), Source: im.loc(file, 1, 1)}
	defer func() { im.result.Items = append(im.result.Items, it) }()
	blocks, ok := im.read(file, &it)
	if !ok {
		return
	}
	b := requestBuilder{im: im, file: file, it: &it, blocks: blocks, inherited: im.folders[path.Dir(file)],
		t: &importers.Translator{Item: &it, Runtime: im.runtime}}
	req, ok := b.build()
	if !ok {
		return
	}
	req.Path = im.taken.Unique(cmpOr(requestPath(file), "request"))
	it.Name = req.Path
	im.result.Requests = append(im.result.Requests, req)
}

// environments reads environments/*.bru: vars and vars:secret.
func (im *importer) environments() {
	files, _ := fs.Glob(im.fsys, "environments/*.bru") // the pattern is valid
	for _, file := range files {
		it := importers.Item{Name: "environment " + strings.TrimSuffix(path.Base(file), ".bru"), Source: im.loc(file, 1, 1)}
		blocks, ok := im.read(file, &it)
		if ok {
			im.result.Environments = append(im.result.Environments, environment(&it, file, blocks))
		}
		im.result.Items = append(im.result.Items, it)
	}
}

func environment(it *importers.Item, file string, blocks []block) model.Environment {
	env := model.Environment{Meta: model.Meta{Name: cmpOr(importers.FileName(strings.ToLower(strings.TrimSuffix(path.Base(file), ".bru"))), "bruno")}}
	var secrets []string
	if b, ok := find(blocks, "vars:secret"); ok {
		secrets = b.list
	}
	t := importers.Translator{Item: it}
	if b, ok := find(blocks, "vars"); ok {
		for _, e := range b.enabled() {
			if slices.Contains(secrets, e.key) || importers.SensitiveHeader(e.key) {
				continue
			}
			env.Vars = append(env.Vars, model.EnvVar{Name: e.key, Value: t.Text(e.value)})
		}
	}
	for _, name := range secretNames(blocks, secrets) {
		it.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{"where": "variable " + name, "placeholder": "a secret declaration"}))
		env.Vars = append(env.Vars, model.EnvVar{Name: name, Secret: true})
	}
	return env
}

// secretNames are the declared secrets plus credential-like plain vars.
func secretNames(blocks []block, declared []string) []string {
	out := slices.Clone(declared)
	if b, ok := find(blocks, "vars"); ok {
		for _, e := range b.enabled() {
			if importers.SensitiveHeader(e.key) && !slices.Contains(out, e.key) {
				out = append(out, e.key)
			}
		}
	}
	return out
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
