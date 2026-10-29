// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"sync"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// ImportKindView is a document kind the wizard offers.
type ImportKindView struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Source  string `json:"source"` // file, folder or paste (a file or pasted text)
	Overlay bool   `json:"overlay"`
}

// ImportProblemView is a diagnostic found in an imported document.
type ImportProblemView struct {
	Severity string   `json:"severity"`
	Text     string   `json:"text"`
	Hint     string   `json:"hint,omitempty"`
	Where    string   `json:"where,omitempty"`
	Excerpt  []string `json:"excerpt,omitempty"`
}

// ImportItemView is one item of the report.
type ImportItemView struct {
	Name     string              `json:"name"`
	Skipped  bool                `json:"skipped"`
	Problems []ImportProblemView `json:"problems"`
}

// ImportFileView is a file the import writes.
type ImportFileView struct {
	Rel    string `json:"rel"`
	Exists bool   `json:"exists"`
	Remove bool   `json:"remove"`
}

// ImportView is the import wizard.
type ImportView struct {
	Kinds       []ImportKindView `json:"kinds"`
	Kind        string           `json:"kind"`
	File        string           `json:"file"`
	Paste       string           `json:"paste"`
	Collections []Choice         `json:"collections"`
	Collection  string           `json:"collection"`
	Previewed   bool             `json:"previewed"`
	Summary     string           `json:"summary"`
	Files       []ImportFileView `json:"files"`
	Items       []ImportItemView `json:"items"`
	Lossy       []string         `json:"lossy"`
	Skipped     int              `json:"skipped"`
	Existing    int              `json:"existing"`
	CanImport   bool             `json:"canImport"`
	Error       string           `json:"error,omitempty"`
	Done        string           `json:"done,omitempty"`
}

func importLabels() map[flowi.ImportKind]string {
	return map[flowi.ImportKind]string{
		flowi.ImportOpenAPI: "OpenAPI", flowi.ImportAsyncAPI: "AsyncAPI", flowi.ImportProto: "gRPC .proto", flowi.ImportArazzo: "Arazzo",
		flowi.ImportPostman: "Postman", flowi.ImportBruno: "Bruno", flowi.ImportInsomnia: "Insomnia", flowi.ImportThunderClient: "Thunder Client", flowi.ImportHoppscotch: "Hoppscotch", flowi.ImportOpenRPC: "OpenRPC", flowi.ImportHAR: "HAR", flowi.ImportHTTP: ".http file",
		flowi.ImportCurl: "curl", flowi.ImportMermaid: "Mermaid",
	}
}

// ImportService is the import wizard.
type ImportService struct {
	app   *App
	vm    *vm.ImportWizard
	first *firstLoad

	pasteMu   sync.Mutex // guards pasteFile
	pasteFile string     // holds pasted text for kinds the wizard reads from a file; "" until a paste needs it
}

// newImportService builds the service for the import wizard.
func newImportService(a *App) *ImportService {
	s := &ImportService{app: a, vm: vm.NewImportWizard(a.backend, a.worker, a.workspace), first: newFirstLoad()}
	s.vm.OnChange(func() {
		s.first.mark()
		a.emit("import", s.view())
	})
	return s
}

// State loads the collections and returns the wizard; changes arrive as the "import" event.
func (s *ImportService) State() ImportView {
	var v ImportView
	s.app.actor.Do(s.vm.Load)
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// SetKind chooses the document kind.
func (s *ImportService) SetKind(kind string) {
	s.app.actor.Do(func() { s.vm.SetKind(flowi.ImportKind(kind)) })
}

// SetFile chooses the document: a file path, a folder (Bruno), or an http(s) URL.
func (s *ImportService) SetFile(path string) { s.app.actor.Do(func() { s.vm.SetFile(path) }) }

// SetPaste sets pasted text, used instead of the file when not empty.
func (s *ImportService) SetPaste(text string) { s.app.actor.Do(func() { s.vm.SetPaste(text) }) }

// SetOverlay sets overlay files or URLs, separated by ";".
func (s *ImportService) SetOverlay(refs string) { s.app.actor.Do(func() { s.vm.SetOverlay(refs) }) }

// SetCollection chooses where the import goes.
func (s *ImportService) SetCollection(id string) {
	s.app.actor.Do(func() { s.vm.SetCollection(model.ID(id)) })
}

// Preview reads the source and plans the import.
func (s *ImportService) Preview() { s.app.actor.Do(s.vm.Preview) }

// Import writes the previewed files. existing is refuse, overwrite or skip;
// partial imports the rest when some items cannot be imported.
func (s *ImportService) Import(existing string, partial bool) {
	mode := map[string]vm.ExistingFiles{"refuse": vm.ImportRefuseExisting, "overwrite": vm.ImportOverwrite, "skip": vm.ImportSkipExisting}[existing]
	s.app.actor.Do(func() { s.vm.Import(mode, partial) })
}

func (s *ImportService) view() ImportView {
	w := s.vm
	v := ImportView{
		Kinds: []ImportKindView{}, Kind: string(w.Kind()), File: w.File(), Paste: w.Paste(), Collections: []Choice{}, Collection: string(w.Collection()),
		Files: []ImportFileView{}, Items: []ImportItemView{}, Lossy: nonNil(w.Lossy()), Summary: w.Summary(), Skipped: w.Skipped(),
		Existing: w.Existing(), CanImport: w.CanImport(), Error: w.Err(), Done: w.Done(),
	}
	v.Previewed = w.Summary() != "" // the summary exists once a preview is shown
	for _, k := range vm.ImportKinds() {
		src := "file"
		switch vm.SourceModeOf(k) {
		case vm.SourceFolder:
			src = "folder"
		case vm.SourceFileOrPaste:
			src = "paste"
		}
		v.Kinds = append(v.Kinds, ImportKindView{ID: string(k), Label: importLabels()[k], Source: src, Overlay: vm.SupportsOverlay(k)})
	}
	for _, c := range w.Choices() {
		v.Collections = append(v.Collections, Choice{ID: string(c.ID), Label: c.Name, Path: c.Name})
	}
	for _, r := range w.Rows() {
		v.Files = append(v.Files, ImportFileView{Rel: r.Rel, Exists: r.Exists, Remove: r.Remove})
	}
	for _, it := range w.Items() {
		item := ImportItemView{Name: it.Name, Skipped: it.Skipped, Problems: []ImportProblemView{}}
		for _, p := range it.Problems {
			item.Problems = append(item.Problems, ImportProblemView{Severity: p.Severity, Text: p.Text, Hint: p.Hint, Where: p.Where, Excerpt: p.Excerpt})
		}
		v.Items = append(v.Items, item)
	}
	return v
}
