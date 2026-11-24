// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
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

// SourceModeOf is how the wizard asks for a kind's source. Any kind but a folder can also be
// pasted as text: SourceFileOrPaste only says the kind is usually pasted (a command, a sketch).
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
	Same   bool // it exists and the import would write exactly what is there
	Remove bool // a stale tool-owned file removed when overwriting
	Kind   flowi.PlannedKind
	Name   string // the title of a request or flow, the name of an environment; "" for the rest
	// ExistingName is the title of the request or flow already at this path when it is a different one, "" otherwise.
	ExistingName string
	Verb         string // what a request shows where an HTTP request shows its method (GET, MQTT, SQS …); "" for the rest
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
	detected    string // what the source was recognised as, for the screen; "" when not
	overlay     string // overlay files or URLs, separated by ";"
	collection  model.ID
	into        string // the folder of the collection the requests and flows go in; "" for where the importer puts them
	plan        *flowi.ImportPlan
	err, done   string
	outcome     ImportOutcome // what the last import did, in words
	busy        bool
	pending     int // previews being made; a stale one still counts until it comes back
	previews    int // successful previews so far
	onChange    []func()
	sched       Scheduler // times the automatic preview; nil previews only when asked
	cancelAuto  func()    // stops the automatic preview that is waiting, if one is
}

// ImportPreviewDelay is how long the wizard waits after the last change of a choice before it previews by itself:
// long enough for typing or a paste to settle, short enough to feel immediate.
const ImportPreviewDelay = 400 * time.Millisecond

// NewImportWizard returns a wizard for OpenAPI; ws is reloaded after an
// import.
func NewImportWizard(b Backend, w *Worker, ws *Workspace) *ImportWizard {
	return &ImportWizard{b: b, w: w, ws: ws, kind: flowi.ImportOpenAPI}
}

// SetScheduler makes the wizard preview by itself, ImportPreviewDelay after the last change of a choice that leaves it
// with a source to read and a place to put it.
//
// Before: the preview came only from a Preview button, and the window kept that button behind "More options" for a
// pasted address, so the first thing the person saw of an import was a screen that had read nothing.
func (iw *ImportWizard) SetScheduler(s Scheduler) { iw.sched = s }

// schedulePreview arranges the automatic preview, replacing one that is waiting. It does nothing without a scheduler
// or without something ready to read.
func (iw *ImportWizard) schedulePreview() {
	iw.stopScheduled()
	if iw.sched == nil || iw.Waiting() != "" {
		return
	}
	iw.cancelAuto = iw.sched.After(ImportPreviewDelay, func() {
		iw.cancelAuto = nil
		iw.Preview()
	})
}

func (iw *ImportWizard) stopScheduled() {
	if iw.cancelAuto != nil {
		iw.cancelAuto()
		iw.cancelAuto = nil
	}
}

// Waiting says in words what the wizard needs before it can preview, "" when it can: a source, a place to import into,
// and for an address, one that is complete, and for a path, something there.
//
// Before: each letter of a typed path was previewed once typing paused, so "/home/me/api" flashed a "no such file"
// failure on the way to "/home/me/api.yaml". A path with nothing at it is now waited for, not reported as a failure;
// Refresh still says so when the person asks.
func (iw *ImportWizard) Waiting() string {
	switch {
	case iw.collection == "":
		return "Choose where to import into."
	case !iw.HasSource():
		if SourceModeOf(iw.kind) == SourceFolder {
			return "Choose the folder to import."
		}
		return "Choose a file or paste the text to see what would be imported."
	case !iw.Pasted() && !addressComplete(iw.file):
		return "Waiting for a complete address."
	case !iw.Pasted() && !flowi.IsImportURL(strings.TrimSpace(iw.file)) && !pathExists(iw.file):
		if SourceModeOf(iw.kind) == SourceFolder {
			return "There is no folder at that path yet."
		}
		return "There is no file at that path yet."
	}
	return ""
}

// pathExists says whether something is at the path, with the spaces around a typed path left out.
func pathExists(p string) bool {
	_, err := os.Stat(strings.TrimSpace(p))
	return err == nil
}

// addressComplete is false for an address that is still being typed ("https://exa"): it needs a host with a dot, a
// port or the name localhost. Anything that is not an address (a path) is complete.
func addressComplete(file string) bool {
	file = strings.TrimSpace(file)
	if !flowi.IsImportURL(file) {
		return true
	}
	u, err := url.Parse(file)
	if err != nil || u.Hostname() == "" {
		return false
	}
	h := u.Hostname()
	return strings.Contains(h, ".") || u.Port() != "" || h == "localhost"
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
			iw.schedulePreview()
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

// SetKind chooses the document kind; it discards the preview and the note of what was detected.
func (iw *ImportWizard) SetKind(k flowi.ImportKind) {
	iw.choose(func() { iw.kind, iw.detected = k, "" })
}

// SetDetected sets the kind to what the source was recognised as, with the words that say what it is ("OpenAPI 3.1.0"),
// which the screen shows so the change of kind is not a surprise.
func (iw *ImportWizard) SetDetected(k flowi.ImportKind, label string) {
	iw.kind, iw.detected = k, label
	iw.changed()
}

// Detected says what the source was recognised as, "" when it was not or the person chose the kind.
func (iw *ImportWizard) Detected() string { return iw.detected }

// SetFile chooses the document (a folder for Bruno); it discards the
// preview.
func (iw *ImportWizard) SetFile(path string) {
	iw.choose(func() { iw.file, iw.detected = path, "" })
}

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
func (iw *ImportWizard) SetPaste(text string) {
	iw.choose(func() { iw.paste, iw.detected = text, "" })
}

// Paste is the pasted text.
func (iw *ImportWizard) Paste() string { return iw.paste }

// SetCollection chooses the target collection, at its top; it discards the preview.
func (iw *ImportWizard) SetCollection(id model.ID) { iw.SetTarget(id, "") }

// SetTarget chooses the target collection and the folder in it ("" for the top of the collection, "a/b" for a
// sub-folder); it discards the preview. The folder is the one of the requests tree and is made by the import when
// it is not there yet.
func (iw *ImportWizard) SetTarget(id model.ID, folder string) {
	iw.choose(func() { iw.collection, iw.into = id, strings.Trim(folder, "/") })
}

// Folder is the folder of the collection the import goes into, "" for the top.
func (iw *ImportWizard) Folder() string { return iw.into }

// HasSource says whether there is something to preview: pasted text, a file, a folder or an address.
func (iw *ImportWizard) HasSource() bool {
	return iw.collection != "" && (strings.TrimSpace(iw.file) != "" || (iw.paste != "" && SourceModeOf(iw.kind) != SourceFolder))
}

// Previewing says whether a preview is being made right now.
func (iw *ImportWizard) Previewing() bool { return iw.pending > 0 }

// Pasted says whether the source is text the person pasted, not a file.
func (iw *ImportWizard) Pasted() bool { return iw.paste != "" && SourceModeOf(iw.kind) != SourceFolder }

// choose applies a choice and discards the preview it invalidates.
func (iw *ImportWizard) choose(set func()) {
	set()
	iw.plan, iw.err, iw.done = nil, "", ""
	iw.outcome = ImportOutcome{}
	iw.schedulePreview()
	iw.changed()
}

// StartOver forgets the source, the preview and the result of an import that is done, so the screen opens on a
// fresh import, into the same place. Choices that say what to import (kind, file, text, overlays) go, and the kind returns to the first one; where it goes stays.
//
// Before: opening the import screen again showed the last import's source, its whole preview and "Imported …", so what
// had already been imported looked like what was about to be; and the kind stayed as the last import (or its paste's
// detection) left it, so an unrecognised paste sat on a stale kind.
func (iw *ImportWizard) StartOver() {
	iw.stopScheduled()
	iw.kind = flowi.ImportOpenAPI
	iw.file, iw.paste, iw.overlay, iw.detected = "", "", "", ""
	iw.plan, iw.err, iw.done, iw.outcome = nil, "", "", ImportOutcome{}
	iw.changed()
}

type previewResult struct {
	plan flowi.ImportPlan
	err  error
}

// Preview reads the source and plans the import in the background. It does nothing without a source (see
// HasSource): there is nothing to show yet, and a screen asks again when one is chosen.
func (iw *ImportWizard) Preview() {
	iw.stopScheduled()
	if !iw.HasSource() {
		return
	}
	kind, file, paste, collection, into, overlay := iw.kind, iw.file, iw.paste, iw.collection, iw.into, iw.overlay
	iw.pending++
	iw.changed()
	Run(iw.w, func() previewResult {
		src, err := readSource(kind, file, paste)
		if err != nil {
			return previewResult{err: err}
		}
		src.Into = into
		if src.Overlays, err = readOverlays(kind, overlay); err != nil {
			return previewResult{err: err}
		}
		plan, err := iw.b.PreviewImportSource(src, collection)
		return previewResult{plan: plan, err: err}
	}, func(r previewResult) {
		iw.pending--
		if kind != iw.kind || file != iw.file || paste != iw.paste || collection != iw.collection || into != iw.into || overlay != iw.overlay {
			iw.changed()
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
	case paste != "":
		src.Name, src.Data = flowi.PastedSourceName, []byte(paste)
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
		rows[i] = ImportRow{Rel: f.Rel, Exists: f.Exists, Same: f.Same, Remove: f.Remove, Kind: f.Kind, Name: f.Name, ExistingName: f.ExistingName, Verb: f.Verb}
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

// Summary counts what the previewed import makes: its requests and flows, then what else it writes.
func (iw *ImportWizard) Summary() string {
	if iw.plan == nil {
		return ""
	}
	return summaryOf(iw.Rows(), iw.Existing())
}

// contentKind names a kind of file in a sentence.
type contentKind struct {
	kind     flowi.PlannedKind
	singular string
}

// contentKinds are the kinds of file a person thinks of as what they import; the rest (kept documents, schemas)
// are written for the import's own sake.
func contentKinds() []contentKind {
	return []contentKind{
		{flowi.PlannedRequest, "request"}, {flowi.PlannedFlow, "flow"}, {flowi.PlannedEnvironment, "environment"},
		{flowi.PlannedAuth, "authentication"}, {flowi.PlannedExample, "example"}, {flowi.PlannedProto, ".proto file"},
	}
}

// summaryOf is "2 requests, 0 flows; 1 kept document; 1 already exists".
func summaryOf(rows []ImportRow, existing int) string {
	count := map[flowi.PlannedKind]int{}
	for _, r := range rows {
		count[r.Kind]++
	}
	// Requests and flows are always named, even at none: "0 flows" says the document had none.
	parts := []string{plural(count[flowi.PlannedRequest], "request"), plural(count[flowi.PlannedFlow], "flow")}
	for _, k := range contentKinds()[2:] {
		if n := count[k.kind]; n > 0 {
			parts = append(parts, plural(n, k.singular))
		}
	}
	out := strings.Join(parts, ", ")
	var extra []string
	if n := count[flowi.PlannedDocument]; n > 0 {
		extra = append(extra, plural(n, "kept document"))
	}
	if n := len(rows) - countOf(count, contentKinds()) - count[flowi.PlannedDocument]; n > 0 {
		extra = append(extra, plural(n, "supporting file"))
	}
	if len(extra) > 0 {
		out += "; " + strings.Join(extra, ", ")
	}
	switch {
	case existing == 1:
		out += "; 1 already exists"
	case existing > 1:
		out += fmt.Sprintf("; %d already exist", existing)
	}
	return out
}

func countOf(count map[flowi.PlannedKind]int, kinds []contentKind) int {
	n := 0
	for _, k := range kinds {
		n += count[k.kind]
	}
	return n
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
	type applied struct {
		report flowi.ImportReport
		err    error
	}
	Run(iw.w, func() applied {
		r, err := iw.b.ApplyImportReport(plan, o)
		return applied{r, err}
	}, func(a applied) {
		iw.busy, iw.err = false, ""
		if a.err != nil {
			iw.err = ErrorText(a.err)
			iw.changed()
			return
		}
		iw.outcome = describeReport(a.report, iw.targetName(plan.Collection, plan.Into))
		iw.done = iw.outcome.Headline
		iw.plan = nil // what was previewed has been dealt with: the screen shows what happened, not the plan again
		iw.changed()
		iw.ws.Load()
	})
}

// Outcome says in detail what the last import did.
func (iw *ImportWizard) Outcome() ImportOutcome { return iw.outcome }

// TargetName says where the import goes in words: "Shop", or "Shop / demo / websocket" for a folder.
func (iw *ImportWizard) TargetName() string { return iw.targetName(iw.collection, iw.into) }

// targetName is "Shop" or "Shop / demo / websocket": where the import went.
func (iw *ImportWizard) targetName(id model.ID, folder string) string {
	name := iw.collectionName(id)
	if folder == "" {
		return name
	}
	return name + " / " + strings.ReplaceAll(folder, "/", " / ")
}

// ImportOutcome is what an import did, for the screen: a headline, and a note for everything that was not simply
// written (a file kept because it exists, one replaced, one already the same).
type ImportOutcome struct {
	Headline string
	// Notes are what the person should look at: a file kept because it exists, one replaced, one removed. Details are
	// what happened as a matter of course (a copy of the document kept for updates).
	Notes   []string
	Details []string
	// Wrote is how many requests, flows and the like were written or replaced; 0 means nothing the person would
	// call imported happened, whatever else was kept.
	Wrote int
}

// describeReport turns the report of ApplyImportReport into words.
//
// Before: "Imported 2 files into Shop" counted the kept copy of the document as a file and said nothing of a request
// that was kept because one of that name was already there, so an import that wrote one request and kept another
// looked like it had made two, or none.
func describeReport(r flowi.ImportReport, target string) ImportOutcome {
	written, replaced := contentCounts(r.Written), contentCounts(r.Replaced)
	out := ImportOutcome{Wrote: written.total + replaced.total}
	switch {
	case written.total > 0 && replaced.total > 0:
		out.Headline = fmt.Sprintf("Imported %s into %s, and replaced %s", written.words(), target, replaced.words())
	case written.total > 0:
		out.Headline = fmt.Sprintf("Imported %s into %s", written.words(), target)
	case replaced.total > 0:
		out.Headline = fmt.Sprintf("Replaced %s in %s", replaced.words(), target)
	default:
		out.Headline = "Nothing was imported into " + target
	}
	for _, f := range r.Kept {
		switch {
		case isContent(f.Kind) && f.Same:
			out.Notes = append(out.Notes, fmt.Sprintf("Your %s %s (%s) is already the same as the one in the source.", kindWord(f.Kind), quoted(f), f.Rel))
		case isContent(f.Kind):
			out.Notes = append(out.Notes, fmt.Sprintf("Kept your %s %s (%s): it already exists, and you chose to keep yours.", kindWord(f.Kind), quoted(f), f.Rel))
		}
	}
	if n := len(contentOf(r.Unchanged)); n > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf("%s already matched what you have and did not change.", plural(n, "file")))
	}
	if n := len(r.Removed); n > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf("Removed %s the document no longer has.", plural(n, "stale file")))
	}
	for _, f := range r.Written {
		if f.Kind == flowi.PlannedDocument {
			out.Details = append(out.Details, fmt.Sprintf("Kept a copy of the document in %s, for updating these requests later.", f.Rel))
		}
	}
	if out.Wrote == 0 && len(r.Kept) > 0 {
		out.Notes = append([]string{"Everything it would write was already there, so nothing changed. Choose \"Overwrite them\" to replace yours, or import into another folder."}, out.Notes...)
	}
	return out
}

func isContent(kind flowi.PlannedKind) bool {
	for _, k := range contentKinds() {
		if k.kind == kind {
			return true
		}
	}
	return false
}

func contentOf(list []flowi.PlannedFile) []flowi.PlannedFile {
	var out []flowi.PlannedFile
	for _, f := range list {
		if isContent(f.Kind) {
			out = append(out, f)
		}
	}
	return out
}

// kindWord is what to call a kind of file in a sentence.
func kindWord(kind flowi.PlannedKind) string {
	for _, k := range contentKinds() {
		if k.kind == kind {
			return k.singular
		}
	}
	return "file"
}

// quoted is the name of a file in quotes, or its path when it has no name.
func quoted(f flowi.PlannedFile) string {
	if f.Name == "" {
		return f.Rel
	}
	return "\u201c" + f.Name + "\u201d"
}

// counts is how many of each kind of content a list holds.
type counts struct {
	by    map[flowi.PlannedKind]int
	total int
}

func contentCounts(list []flowi.PlannedFile) counts {
	c := counts{by: map[flowi.PlannedKind]int{}}
	for _, f := range contentOf(list) {
		c.by[f.Kind]++
		c.total++
	}
	return c
}

// words is "2 requests and 1 flow".
func (c counts) words() string {
	var parts []string
	for _, k := range contentKinds() {
		if n := c.by[k.kind]; n > 0 {
			parts = append(parts, plural(n, k.singular))
		}
	}
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func (iw *ImportWizard) collectionName(id model.ID) string {
	for _, c := range iw.choices {
		if c.ID == id {
			return c.Name
		}
	}
	return string(id)
}
