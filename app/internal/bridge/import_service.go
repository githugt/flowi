// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
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
	// Kind is request, flow, environment, auth, example, schema, document, proto or other; Name is the title of a
	// request or flow, "" for a file with none; Status is new, same (exists, would be written back as it is), differs
	// (exists and would change) or remove.
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Status string `json:"status"`
	// ExistingName is the title of the different request or flow already at this path, "" when none.
	ExistingName string `json:"existingName"`
	// Verb is what a request shows where an HTTP request shows its method (GET, MQTT, SQS …), "" for the rest.
	Verb string `json:"verb"`
}

// ImportView is the import wizard.
type ImportView struct {
	Kinds []ImportKindView `json:"kinds"`
	Kind  string           `json:"kind"`
	File  string           `json:"file"`
	Paste string           `json:"paste"`
	// Pasted is true when the source is text handed over by the window (a pasted curl command or
	// document), so the screen shows no file to choose.
	Pasted      bool     `json:"pasted"`
	Collections []Choice `json:"collections"`
	Collection  string   `json:"collection"`
	// Folder is the folder of the collection the import goes into ("demo/websocket"), "" for the top. Target says where
	// it goes in words ("Shop / demo / websocket") and TargetRow is the row of the Import into dialog that stands for it.
	Folder    string `json:"folder"`
	Target    string `json:"target"`
	TargetRow string `json:"targetRow"`
	// Previewing is true while a preview is being made; CanPreview says there is a source and a place to preview it for.
	Previewing bool `json:"previewing"`
	CanPreview bool `json:"canPreview"`
	// Waiting says what a preview needs that is missing, "" when none: shown where the preview would be.
	Waiting   string           `json:"waiting"`
	Previewed bool             `json:"previewed"`
	Summary   string           `json:"summary"`
	Files     []ImportFileView `json:"files"`
	Items     []ImportItemView `json:"items"`
	Lossy     []string         `json:"lossy"`
	Skipped   int              `json:"skipped"`
	Existing  int              `json:"existing"`
	CanImport bool             `json:"canImport"`
	Error     string           `json:"error,omitempty"`
	Done      string           `json:"done,omitempty"`
	// Notes say what the last import did apart from writing: what it kept because it exists, what it replaced.
	// Wrote is how many requests, flows and the like it wrote.
	Notes []string `json:"notes"`
	// Details are what it did as a matter of course, such as the copy of the document it kept.
	Details []string `json:"details"`
	Wrote   int      `json:"wrote"`
	// Detected says what the source was recognised as ("OpenAPI 3.1.0"), when it was and the kind was set from it.
	Detected string `json:"detected,omitempty"`
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
}

// newImportService builds the service for the import wizard.
func newImportService(a *App) *ImportService {
	s := &ImportService{app: a, vm: vm.NewImportWizard(a.backend, a.worker, a.workspace), first: newFirstLoad()}
	s.vm.SetScheduler(a.scheduler)
	sweepPasted(os.TempDir(), stalePasteAge, time.Now())
	s.vm.OnChange(func() {
		s.first.mark()
		a.emit("import", s.view())
	})
	return s
}

// State loads the collections and returns the wizard; changes arrive as the "import" event.
func (s *ImportService) State() ImportView {
	var v ImportView
	s.app.actor.Do(func() {
		if s.vm.Done() != "" {
			s.vm.StartOver() // the import is finished: the screen starts a new one
		}
		s.vm.Load()
	})
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// SetKind chooses the document kind.
func (s *ImportService) SetKind(kind string) {
	s.app.actor.Do(func() { s.vm.SetKind(flowi.ImportKind(kind)) })
}

// SetFile chooses the document: a file path, a folder (Bruno), or an http(s) URL. A file or folder on disk is
// recognised, and the kind is set to what it is: the screen says so.
func (s *ImportService) SetFile(path string) {
	found, ok := detectSource(path) // reads the start of the file: outside the actor
	s.app.actor.Do(func() {
		s.vm.SetFile(path)
		if ok {
			s.vm.SetDetected(found.Kind, found.Label)
		}
	})
}

// SetPaste sets pasted text, used instead of the file when not empty. Text that is recognised sets the kind.
func (s *ImportService) SetPaste(text string) {
	found, ok := flowi.DetectImportKind("", []byte(text[:min(len(text), flowi.DetectHeadBytes)]))
	s.app.actor.Do(func() {
		s.vm.SetPaste(text)
		if ok {
			s.vm.SetDetected(found.Kind, found.Label)
		}
	})
}

// detectSource recognises what the file or folder at path is. An address (http, https) is not fetched just to look.
func detectSource(path string) (flowi.Detected, bool) {
	path = strings.TrimSpace(path)
	if path == "" || flowi.IsImportURL(path) {
		return flowi.Detected{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return flowi.Detected{}, false
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return flowi.Detected{}, false
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return flowi.DetectImportDir(names)
	}
	f, err := os.Open(path) //nolint:gosec // the person chose this file in the window
	if err != nil {
		return flowi.Detected{}, false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, flowi.DetectHeadBytes)
	n, _ := io.ReadFull(f, head)
	return flowi.DetectImportKind(filepath.Base(path), head[:n])
}

// SetOverlay sets overlay files or URLs, separated by ";".
func (s *ImportService) SetOverlay(refs string) { s.app.actor.Do(func() { s.vm.SetOverlay(refs) }) }

// SetCollection chooses the collection the import goes into, at its top.
func (s *ImportService) SetCollection(id string) {
	s.app.actor.Do(func() { s.vm.SetCollection(model.ID(id)) })
}

// SetTarget chooses where the import goes from a row of the Import into dialog: a collection (its top) or one of
// the folders in it, at any depth. A row that is neither is ignored.
func (s *ImportService) SetTarget(row string) {
	s.app.actor.Do(func() {
		if collection, folder, ok := placeDestination(s.app.workspace, strings.TrimSpace(row), "requests"); ok {
			s.vm.SetTarget(collection, folder)
		}
	})
}

// StartOver clears the source and the result of the last import, to import something else into the same place.
func (s *ImportService) StartOver() { s.app.actor.Do(s.vm.StartOver) }

// Preview reads the source and plans the import. The wizard also does it by itself shortly after a choice changes;
// this is for refreshing it, for a source that changed on disk.
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
		Existing: w.Existing(), CanImport: w.CanImport(), Error: w.Err(), Done: w.Done(), Detected: w.Detected(), Waiting: w.Waiting(),
	}
	v.Pasted = w.Pasted()
	v.Folder, v.Previewing, v.CanPreview, v.Notes, v.Details, v.Wrote = w.Folder(), w.Previewing(), w.HasSource(), nonNil(w.Outcome().Notes), nonNil(w.Outcome().Details), w.Outcome().Wrote
	v.Target = s.vm.TargetName()
	v.TargetRow = targetRow(s.app.workspace, w.Collection(), w.Folder())
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
		v.Files = append(v.Files, ImportFileView{Rel: r.Rel, Exists: r.Exists, Remove: r.Remove, Kind: string(r.Kind), Name: r.Name, Status: fileStatus(r), ExistingName: r.ExistingName, Verb: r.Verb})
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

// fileStatus says what the import does to a file by itself: write a new one, find the same, change one, remove one.
func fileStatus(r vm.ImportRow) string {
	switch {
	case r.Remove:
		return "remove"
	case !r.Exists:
		return "new"
	case r.Same:
		return "same"
	}
	return "differs"
}

// targetRow is the row of the Import into dialog for a collection and a folder of its requests ("" for the top): the
// collection's own row, else the row of the folder, found at any depth. "" when the collection is not chosen.
func targetRow(ws *vm.Workspace, collection model.ID, folder string) string {
	for _, id := range ws.Children("") {
		n := ws.Node(id)
		if n.Kind != vm.NodeCollection || n.Collection != collection {
			continue
		}
		if folder == "" {
			return id
		}
		if row, ok := findFolder(ws, id, folder); ok {
			return row
		}
		return id // a folder the import will make: the dialog shows its collection
	}
	return ""
}

// findFolder walks the rows below row for the folder of the requests tree at path.
func findFolder(ws *vm.Workspace, row, path string) (string, bool) {
	for _, id := range ws.Children(row) {
		n := ws.Node(id)
		if n.Kind == vm.NodeFolder && n.Tree == "requests" && n.Path == path {
			return id, true
		}
		if n.Kind == vm.NodeRequestsGroup || n.Kind == vm.NodeFolder {
			if found, ok := findFolder(ws, id, path); ok {
				return found, true
			}
		}
	}
	return "", false
}
