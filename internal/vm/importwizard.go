// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// ExistingFiles is what an import does with files that already exist.
type ExistingFiles int

// Choices for existing files; refusing makes the import fail with
// FLW-IMPORT-003 when there are any.
const (
	ImportRefuseExisting ExistingFiles = iota
	ImportOverwrite
	ImportSkipExisting
)

// CollectionChoice is a collection an import can go into.
type CollectionChoice struct {
	ID   model.ID
	Name string
}

// SourceMode is how a kind's source is chosen.
type SourceMode int

// Source modes.
const (
	SourceFile        SourceMode = iota // a file
	SourceFolder                        // a folder (Bruno)
	SourceFileOrPaste                   // a file or pasted text (curl, Mermaid)
)

// SourceModeOf is how the wizard asks for a kind's source.
func SourceModeOf(kind flowi.ImportKind) SourceMode {
	switch kind {
	case flowi.ImportBruno:
		return SourceFolder
	case flowi.ImportCurl, flowi.ImportMermaid, flowi.ImportHTTP:
		return SourceFileOrPaste
	}
	return SourceFile
}

// ImportKinds are the kinds the wizard offers, in menu order.
func ImportKinds() []flowi.ImportKind {
	return []flowi.ImportKind{flowi.ImportOpenAPI, flowi.ImportAsyncAPI, flowi.ImportProto, flowi.ImportArazzo, flowi.ImportPostman, flowi.ImportBruno,
		flowi.ImportInsomnia, flowi.ImportThunderClient, flowi.ImportHoppscotch, flowi.ImportOpenRPC,
		flowi.ImportHAR, flowi.ImportHTTP, flowi.ImportCurl, flowi.ImportMermaid}
}

// ImportItem is one item of the import report (Postman, Bruno, HAR,
// curl, Mermaid) and its problems.
type ImportItem struct {
	Name     string
	Skipped  bool
	Problems []ImportProblem
}

// ImportProblem is a diagnostic in an imported document.
type ImportProblem struct {
	Severity string   // error | warning
	Text     string   // code and message
	Hint     string   // what to do, or ""
	Where    string   // file name with line and column, or ""
	Excerpt  []string // the source line and a caret line
}

// ImportRow is one file the previewed import writes.
type ImportRow struct {
	Rel    string
	Exists bool
	Remove bool // a stale tool-owned file removed when overwriting
}

// ImportWizard imports a document (M5 Decision 8, M6 Decision 12): pick
// the kind, the source (a file, a folder, or pasted text) and the
// collection, preview the files and the report, then import. Changing
// any choice discards the preview, so what is imported is always what was
// shown. Items that cannot be imported block the import until the user
// chooses to import the rest.
type ImportWizard struct {
	b           Backend
	w           *Worker
	ws          *Workspace
	choices     []CollectionChoice
	kind        flowi.ImportKind
	file, paste string
	overlay     string // overlay files or URLs, separated by ";"
	collection  model.ID
	plan        *flowi.ImportPlan
	err, done   string
	busy        bool
	previews    int // successful previews so far
	onChange    []func()
}

// NewImportWizard returns a wizard for OpenAPI; ws is reloaded after an
// import.
func NewImportWizard(b Backend, w *Worker, ws *Workspace) *ImportWizard {
	return &ImportWizard{b: b, w: w, ws: ws, kind: flowi.ImportOpenAPI}
}

// OnChange registers f to run after any state change.
func (iw *ImportWizard) OnChange(f func()) { iw.onChange = append(iw.onChange, f) }

func (iw *ImportWizard) changed() {
	for _, f := range iw.onChange {
		f()
	}
}

// Load reads the collections to choose from; the first is preselected.
func (iw *ImportWizard) Load() {
	Run(iw.w, func() []CollectionChoice {
		var out []CollectionChoice
		for _, c := range iw.b.Collections() {
			out = append(out, CollectionChoice{ID: c.Manifest.ID, Name: c.Manifest.Name})
		}
		return out
	}, func(choices []CollectionChoice) {
		iw.choices = choices
		if iw.collection == "" && len(choices) > 0 {
			iw.collection = choices[0].ID
		}
		iw.changed()
	})
}

// Choices are the collections an import can go into.
func (iw *ImportWizard) Choices() []CollectionChoice { return iw.choices }

// Kind is the document kind to import.
func (iw *ImportWizard) Kind() flowi.ImportKind { return iw.kind }

// File is the document's path.
func (iw *ImportWizard) File() string { return iw.file }

// Collection is the collection to import into.
func (iw *ImportWizard) Collection() model.ID { return iw.collection }

// Err is the last preview's or import's problem, or "".
func (iw *ImportWizard) Err() string { return iw.err }

// Done says what the last import wrote, or "".
func (iw *ImportWizard) Done() string { return iw.done }

// SetKind chooses the document kind; it discards the preview.
func (iw *ImportWizard) SetKind(k flowi.ImportKind) { iw.choose(func() { iw.kind = k }) }

// SetFile chooses the document (a folder for Bruno); it discards the
// preview.
func (iw *ImportWizard) SetFile(path string) { iw.choose(func() { iw.file = path }) }

// SupportsOverlay reports whether a kind can take OpenAPI Overlay documents.
func SupportsOverlay(kind flowi.ImportKind) bool {
	return kind == flowi.ImportOpenAPI || kind == flowi.ImportAsyncAPI || kind == flowi.ImportArazzo
}

// SetOverlay sets the overlay files (or URLs), separated by ";", applied in order
// to an OpenAPI, AsyncAPI or Arazzo document before it is imported; it discards the
// preview.
func (iw *ImportWizard) SetOverlay(refs string) { iw.choose(func() { iw.overlay = refs }) }

// SetPaste sets pasted text, used instead of the file when not empty (for
// kinds whose SourceMode allows it); it discards the preview.
func (iw *ImportWizard) SetPaste(text string) { iw.choose(func() { iw.paste = text }) }

// Paste is the pasted text.
func (iw *ImportWizard) Paste() string { return iw.paste }

// SetCollection chooses the target collection; it discards the preview.
func (iw *ImportWizard) SetCollection(id model.ID) { iw.choose(func() { iw.collection = id }) }

// choose applies a choice and discards the preview it invalidates.
func (iw *ImportWizard) choose(set func()) {
	set()
	iw.plan, iw.err, iw.done = nil, "", ""
	iw.changed()
}

type previewResult struct {
	plan flowi.ImportPlan
	err  error
}

// Preview reads the source and plans the import in the background.
func (iw *ImportWizard) Preview() {
	kind, file, paste, collection, overlay := iw.kind, iw.file, iw.paste, iw.collection, iw.overlay
	Run(iw.w, func() previewResult {
		src, err := readSource(kind, file, paste)
		if err != nil {
			return previewResult{err: err}
		}
		if src.Overlays, err = readOverlays(kind, overlay); err != nil {
			return previewResult{err: err}
		}
		plan, err := iw.b.PreviewImportSource(src, collection)
		return previewResult{plan: plan, err: err}
	}, func(r previewResult) {
		if kind != iw.kind || file != iw.file || paste != iw.paste || collection != iw.collection || overlay != iw.overlay {
			return // a choice changed meanwhile: this preview is stale
		}
		iw.plan, iw.err, iw.done = nil, "", ""
		if r.err != nil {
			iw.err = ErrorText(r.err)
		} else {
			iw.plan = &r.plan
			iw.previews++
		}
		iw.changed()
	})
}

// readOverlays reads the overlay files or URLs; kinds that take none ignore them.
func readOverlays(kind flowi.ImportKind, refs string) ([][]byte, error) {
	if !SupportsOverlay(kind) {
		return nil, nil
	}
	var out [][]byte
	for _, ref := range strings.Split(refs, ";") {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		var data []byte
		var err error
		if flowi.IsImportURL(ref) {
			data, _, err = flowi.FetchImportSource(context.Background(), ref)
		} else {
			data, err = os.ReadFile(ref) //nolint:gosec // the overlay file the user named
		}
		if err != nil {
			return nil, fmt.Errorf("reading the overlay %s: %w", ref, err)
		}
		out = append(out, data)
	}
	return out, nil
}

// readSource builds the import source. It reads files, so it runs on the
// worker.
func readSource(kind flowi.ImportKind, file, paste string) (flowi.ImportSource, error) {
	src := flowi.ImportSource{Kind: kind, Name: file}
	switch {
	case SourceModeOf(kind) == SourceFolder:
		src.Dir = file
	case SourceModeOf(kind) == SourceFileOrPaste && paste != "":
		src.Name, src.Data = "pasted text", []byte(paste)
	case flowi.IsImportURL(file):
		data, name, err := flowi.FetchImportSource(context.Background(), file)
		if err != nil {
			return src, err
		}
		src.Name, src.Data = name, data
	default:
		data, err := os.ReadFile(file) //nolint:gosec // the file the user picked to import
		if err != nil {
			return src, fmt.Errorf("reading %s: %w", file, err)
		}
		src.Data = data
		src.RefDir = filepath.Dir(file) // a local file may refer to files beside it
	}
	return src, nil
}

// PreviewCount counts successful previews, so a screen can act once per
// preview (open the report when items were skipped).
func (iw *ImportWizard) PreviewCount() int { return iw.previews }

// Items is the previewed import's per-item report.
func (iw *ImportWizard) Items() []ImportItem {
	if iw.plan == nil {
		return nil
	}
	out := make([]ImportItem, len(iw.plan.Items))
	for i, it := range iw.plan.Items {
		out[i] = ImportItem{Name: it.Name, Skipped: it.Skipped}
		for _, d := range it.Diags {
			out[i].Problems = append(out[i].Problems, importProblem(d))
		}
	}
	return out
}

// importProblem describes a diagnostic in an imported document: the
// location is the document's, shown by base name.
func importProblem(d diag.Diagnostic) ImportProblem {
	msg, hint := diag.English().Message(d)
	where := ""
	if d.File != "" {
		where = filepath.Base(d.File)
		if d.Pos.Line > 0 {
			where += fmt.Sprintf(":%d:%d", d.Pos.Line, d.Pos.Column)
		}
	}
	return ImportProblem{Severity: d.Severity.String(), Text: string(d.Code) + ": " + msg, Hint: hint, Where: where, Excerpt: d.Excerpt}
}

// Skipped counts items that cannot be imported.
func (iw *ImportWizard) Skipped() int {
	n := 0
	for _, it := range iw.Items() {
		if it.Skipped {
			n++
		}
	}
	return n
}

// Rows are the previewed files.
func (iw *ImportWizard) Rows() []ImportRow {
	if iw.plan == nil {
		return nil
	}
	rows := make([]ImportRow, len(iw.plan.Files))
	for i, f := range iw.plan.Files {
		rows[i] = ImportRow{Rel: f.Rel, Exists: f.Exists, Remove: f.Remove}
	}
	return rows
}

// Existing counts previewed files that already exist.
func (iw *ImportWizard) Existing() int {
	n := 0
	for _, r := range iw.Rows() {
		if r.Exists {
			n++
		}
	}
	return n
}

// Lossy are the preview's reports of what did not carry over.
func (iw *ImportWizard) Lossy() []string {
	if iw.plan == nil {
		return nil
	}
	var out []string
	for _, d := range iw.plan.Diags {
		msg, _ := diag.English().Message(d)
		out = append(out, msg)
	}
	return out
}

// Summary counts the previewed requests, flows and files.
func (iw *ImportWizard) Summary() string {
	if iw.plan == nil {
		return ""
	}
	requests, flows := 0, 0
	for _, r := range iw.Rows() {
		if strings.HasSuffix(r.Rel, ".req.yaml") {
			requests++
		}
		if strings.HasSuffix(r.Rel, ".arazzo.yaml") {
			flows++
		}
	}
	out := fmt.Sprintf("%s, %s, %s", plural(requests, "request"), plural(flows, "flow"), plural(len(iw.plan.Files), "file"))
	switch n := iw.Existing(); {
	case n == 1:
		out += "; 1 already exists"
	case n > 1:
		out += fmt.Sprintf("; %d already exist", n)
	}
	return out
}

// CanImport is true once a preview is shown, not yet imported and not
// being imported.
func (iw *ImportWizard) CanImport() bool { return iw.plan != nil && iw.done == "" && !iw.busy }

// Import writes the previewed plan, then reloads the workspace. With
// skipped items it imports only when partial is set (the user chose to
// import the rest).
func (iw *ImportWizard) Import(existing ExistingFiles, partial bool) {
	if !iw.CanImport() {
		return
	}
	if n := iw.Skipped(); n > 0 && !partial {
		iw.err = fmt.Sprintf("%s cannot be imported; review the report, then import the rest", plural(n, "item"))
		iw.changed()
		return
	}
	iw.busy = true
	plan := *iw.plan
	o := flowi.ApplyOptions{Overwrite: existing == ImportOverwrite, SkipExisting: existing == ImportSkipExisting}
	written := len(plan.Files)
	if o.SkipExisting {
		written -= iw.Existing()
	}
	Run(iw.w, func() error { return iw.b.ApplyImport(plan, o) }, func(err error) {
		iw.busy, iw.err = false, ""
		if err != nil {
			iw.err = ErrorText(err)
			iw.changed()
			return
		}
		iw.done = fmt.Sprintf("Imported %s into %s", plural(written, "file"), iw.collectionName(plan.Collection))
		iw.changed()
		iw.ws.Load()
	})
}

func (iw *ImportWizard) collectionName(id model.ID) string {
	for _, c := range iw.choices {
		if c.ID == id {
			return c.Name
		}
	}
	return string(id)
}
