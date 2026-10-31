// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/openapi"
	"github.com/githugt/flowi/lib/specoverlay"
)

// KeptSpec is an OpenAPI description an import kept in descriptions/, with
// the overlays that apply to it.
type KeptSpec struct {
	Name     string   // the file's base name: "pets" for pets.openapi.yaml
	File     string   // absolute
	Rel      string   // slash-separated, relative to the collection root
	Overlays []string // absolute, in the order they apply: <name>.overlay.yaml, <name>.overlay-2.yaml, …
	// Edits is the overlay Flowi saves request edits to,
	// <name>.edits.overlay.yaml. It applies last. It may not exist yet.
	Edits string
}

// FilesDir is where the files the spec refers to are kept, beside it:
// <name>.files/, with the paths the spec writes them with.
func (k KeptSpec) FilesDir() string {
	return filepath.Join(filepath.Dir(k.File), k.Name+".files")
}

// refFiles is where a re-import of the kept spec reads the files it refers
// to, nil when none were kept.
func (k KeptSpec) refFiles() fs.FS {
	if info, err := os.Stat(k.FilesDir()); err != nil || !info.IsDir() {
		return nil
	}
	return os.DirFS(k.FilesDir())
}

var (
	keptSpecFile    = regexp.MustCompile(`^(.+)\.openapi\.(yaml|yml|json)$`)
	keptOverlayFile = regexp.MustCompile(`^(.+)\.overlay(?:-(\d+))?\.yaml$`)
)

// KeptSpecs lists the OpenAPI descriptions kept in the collection's
// descriptions/ folder, by name. The description Flowi generates
// (requests.openapi.yaml) is not one.
func (w *Workspace) KeptSpecs(collection model.ID) ([]KeptSpec, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return keptSpecs(c.Root)
}

func keptSpecs(root string) ([]KeptSpec, error) {
	dir := filepath.Join(root, "descriptions")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if m := keptSpecFile.FindStringSubmatch(e.Name()); m != nil && m[1] != "requests" {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	out := make([]KeptSpec, 0, len(names))
	for _, file := range names {
		name := keptSpecFile.FindStringSubmatch(file)[1]
		out = append(out, KeptSpec{Name: name, File: filepath.Join(dir, file), Rel: "descriptions/" + file,
			Overlays: overlaysOf(dir, name, entries), Edits: filepath.Join(dir, name+".edits.overlay.yaml")})
	}
	return out, nil
}

// overlaysOf are the kept overlays of the spec called name, in the order
// the import that kept them applied them.
func overlaysOf(dir, name string, entries []fs.DirEntry) []string {
	type numbered struct {
		n    int
		file string
	}
	var found []numbered
	for _, e := range entries {
		m := keptOverlayFile.FindStringSubmatch(e.Name())
		if m == nil || m[1] != name {
			continue // another spec's, or the edits overlay (its name ends in .edits)
		}
		n := 1
		if m[2] != "" {
			n, _ = strconv.Atoi(m[2]) // the pattern allows digits only
		}
		found = append(found, numbered{n: n, file: filepath.Join(dir, e.Name())})
	}
	slices.SortFunc(found, func(a, b numbered) int { return a.n - b.n })
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.file
	}
	return out
}

func keptSpecNamed(root, name string) (KeptSpec, bool) {
	specs, err := keptSpecs(root)
	if err != nil {
		return KeptSpec{}, false // an unreadable folder keeps nothing: the import goes on as a first import
	}
	i := slices.IndexFunc(specs, func(k KeptSpec) bool { return k.Name == name })
	if i < 0 {
		return KeptSpec{}, false
	}
	return specs[i], true
}

// readOverlays reads the kept overlays, and the edits overlay when there is one.
func (k KeptSpec) readOverlays() (overlays [][]byte, edits []byte, err error) {
	for _, f := range k.Overlays {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("reading the overlay %s: %w", f, err)
		}
		overlays = append(overlays, data)
	}
	edits, err = os.ReadFile(k.Edits)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("reading the overlay %s: %w", k.Edits, err)
	}
	return overlays, edits, nil
}

// withEdits appends the kept spec's edits overlay to an import's overlays.
func withEdits(overlays [][]byte, kept KeptSpec, hasKept bool) ([][]byte, error) {
	if !hasKept {
		return overlays, nil
	}
	_, edits, err := kept.readOverlays()
	if err != nil || edits == nil {
		return overlays, err
	}
	return append(slices.Clone(overlays), edits), nil
}

// previousImport is what the kept spec, with its overlays, imports as: the
// base a re-import merges against. nil when there is no kept spec or it no
// longer imports; the re-import then takes the spec's fields as they come.
func previousImport(kept KeptSpec, hasKept bool) map[string]model.Request {
	if !hasKept {
		return nil
	}
	r, err := importKept(kept)
	if err != nil {
		return nil
	}
	out := map[string]model.Request{}
	for _, req := range r.Requests {
		out[req.Path] = req
	}
	return out
}

// importKept imports the kept spec with all of its overlays, writing nothing.
func importKept(kept KeptSpec) (openapi.Result, error) {
	spec, err := os.ReadFile(kept.File)
	if err != nil {
		return openapi.Result{}, fmt.Errorf("reading %s: %w", kept.File, err)
	}
	overlays, edits, err := kept.readOverlays()
	if err != nil {
		return openapi.Result{}, err
	}
	if edits != nil {
		overlays = append(overlays, edits)
	}
	data, diags := specoverlay.Apply(spec, overlays)
	if diags.HasErrors() {
		return openapi.Result{}, diags.Err()
	}
	r, diags := openapi.Import(data, openapi.Options{NewID: func() model.ID { return "" }, Files: kept.refFiles()})
	if diags.HasErrors() {
		return openapi.Result{}, diags.Err()
	}
	return r, nil
}

// mergeWithExisting merges an imported request into the one on disk at
// the same path, if there is one (see mergeReimport).
func mergeWithExisting(c model.Collection, base map[string]model.Request, req model.Request) (model.Request, diag.List) {
	file := filepath.Join(c.Root, "requests", filepath.FromSlash(req.Path)+".req.yaml")
	i := slices.IndexFunc(c.Requests, func(x model.Request) bool { return x.File == file })
	if i < 0 {
		return req, nil
	}
	var prev *model.Request
	if b, ok := base[req.Path]; ok {
		prev = &b
	}
	return mergeReimport(prev, c.Requests[i], req)
}

// PreviewSpecUpdate previews importing the kept spec called name again,
// with its overlays: from the kept copy when data is nil, else from data, a
// newer version of the document (which then becomes the kept copy). Requests
// it replaces are merged with what is on disk (see mergeReimport); apply
// the plan with ApplyImport and Overwrite.
func (w *Workspace) PreviewSpecUpdate(collection model.ID, name string, data []byte) (ImportPlan, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return ImportPlan{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	kept, ok := keptSpecNamed(c.Root, name)
	if !ok {
		return ImportPlan{}, fmt.Errorf("%w: %q", ErrNoKeptSpec, name)
	}
	if data == nil {
		var err error
		if data, err = os.ReadFile(kept.File); err != nil {
			return ImportPlan{}, fmt.Errorf("reading %s: %w", kept.File, err)
		}
	}
	overlays, _, err := kept.readOverlays()
	if err != nil {
		return ImportPlan{}, err
	}
	// The kept file's own name keeps the copy, its overlays and the merge
	// base under the same name.
	// The files the spec refers to are read from the copies kept beside it.
	return w.previewIn(c, ImportSource{Kind: ImportOpenAPI, Name: kept.File, Data: data, Overlays: overlays, RefDir: kept.FilesDir()})
}

// ErrNoKeptSpec is returned for a kept spec name the collection does not have.
var ErrNoKeptSpec = errors.New("the collection keeps no spec of that name")
