// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func newImportWizard(t *testing.T) (*vm.ImportWizard, *vmtest.Backend, *vm.Worker, string) {
	t.Helper()
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	b.Plan = flowi.ImportPlan{Collection: "C1", Files: []flowi.PlannedFile{
		{Rel: "requests/pets/getPet.req.yaml", Exists: true, Kind: flowi.PlannedRequest, Name: "Get a pet"},
		{Rel: "requests/pets/getPet.schemas/1-200.json", Kind: flowi.PlannedSchema},
		{Rel: "flows/suggested/a-to-b.arazzo.yaml", Kind: flowi.PlannedFlow, Name: "a to b"},
	}, Diags: diag.List{diag.New(diag.CodeImportUnmapped, diag.Location{},
		map[string]string{"what": "step x", "how": "imported as a note", "detail": "AsyncAPI steps are not supported"})}}
	file := filepath.Join(t.TempDir(), "pets.yaml")
	if err := os.WriteFile(file, []byte("openapi: 3.0.3"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := vmtest.SyncWorker()
	iw := vm.NewImportWizard(b, w, vm.NewWorkspace(b, w))
	iw.Load()
	w.Wait()
	return iw, b, w, file
}

func TestImportWizard_should_default_to_the_first_collection_and_openapi(t *testing.T) {
	iw, _, _, _ := newImportWizard(t)

	if iw.Kind() != flowi.ImportOpenAPI || iw.Collection() != "C1" || len(iw.Choices()) != 1 || iw.Choices()[0].Name != "Shop" {
		t.Fatalf("kind %s collection %s choices %+v", iw.Kind(), iw.Collection(), iw.Choices())
	}
}

func TestImportWizard_preview_lists_files_existing_and_lossy_reports(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	iw.SetKind(flowi.ImportArazzo)
	iw.SetFile(file)

	iw.Preview()
	w.Wait()

	if !b.Called("PreviewImport arazzo C1") || string(b.Previewed[0]) != "openapi: 3.0.3" {
		t.Fatalf("calls %v", b.Calls)
	}
	rows := iw.Rows()
	if len(rows) != 3 || !rows[0].Exists || rows[1].Exists || iw.Existing() != 1 {
		t.Fatalf("rows %+v", rows)
	}
	if lossy := iw.Lossy(); len(lossy) != 1 || !strings.Contains(lossy[0], "step x has no Flowi equivalent") {
		t.Fatalf("lossy %v", lossy)
	}
	if got := iw.Summary(); got != "1 request, 1 flow; 1 supporting file; 1 already exists" {
		t.Fatalf("summary %q", got)
	}
	if !iw.CanImport() {
		t.Fatal("a previewed plan can be imported")
	}
}

func TestImportWizard_changing_a_choice_discards_the_preview(t *testing.T) {
	iw, _, w, file := newImportWizard(t)
	iw.SetFile(file)
	iw.Preview()
	w.Wait()

	iw.SetKind(flowi.ImportArazzo)

	if len(iw.Rows()) != 0 || iw.CanImport() {
		t.Fatalf("stale preview kept: %+v", iw.Rows())
	}
}

func TestImportWizard_import_passes_the_choice_for_existing_files_and_reports(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	iw.SetFile(file)
	iw.Preview()
	w.Wait()

	iw.Import(vm.ImportOverwrite, false)
	w.Wait()

	if len(b.ApplyOpts) != 1 || !b.ApplyOpts[0].Overwrite || b.ApplyOpts[0].SkipExisting {
		t.Fatalf("opts %+v", b.ApplyOpts)
	}
	if iw.Done() != "Imported 1 flow into Shop, and replaced 1 request" || iw.Err() != "" || iw.CanImport() {
		t.Fatalf("done %q err %q", iw.Done(), iw.Err())
	}
}

func TestImportWizard_skipping_counts_only_new_files(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	iw.SetFile(file)
	iw.Preview()
	w.Wait()

	iw.Import(vm.ImportSkipExisting, false)
	w.Wait()

	if !b.ApplyOpts[0].SkipExisting || iw.Done() != "Imported 1 flow into Shop" {
		t.Fatalf("opts %+v done %q", b.ApplyOpts, iw.Done())
	}
}

func TestImportWizard_errors_are_shown_and_keep_the_preview(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	iw.SetFile(file)
	iw.Preview()
	w.Wait()
	b.ApplyErr = diag.List{diag.New(diag.CodeImportExists, diag.Location{}, map[string]string{"count": "1", "files": "requests/pets/getPet.req.yaml"})}.Err()

	iw.Import(vm.ImportRefuseExisting, false)
	w.Wait()

	if !strings.Contains(iw.Err(), "FLW-IMPORT-003") || iw.Done() != "" || !iw.CanImport() {
		t.Fatalf("err %q done %q", iw.Err(), iw.Done())
	}
}

func TestImportWizard_unreadable_file_and_preview_errors_are_shown(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	iw.SetFile(filepath.Join(t.TempDir(), "missing.yaml"))
	iw.Preview()
	w.Wait()
	missing := iw.Err()
	b.PreviewErr = errors.New("not a document Flowi can import")
	iw.SetFile(file)
	iw.Preview()
	w.Wait()

	if !strings.Contains(missing, "missing.yaml") || !strings.Contains(iw.Err(), "not a document") || iw.CanImport() {
		t.Fatalf("missing %q, preview err %q", missing, iw.Err())
	}
}

func TestImportWizard_paste_and_folder_sources(t *testing.T) {
	iw, b, w, _ := newImportWizard(t)
	iw.SetKind(flowi.ImportCurl)
	iw.SetPaste("curl https://x/a")
	iw.Preview()
	w.Wait()
	iw.SetKind(flowi.ImportBruno)
	iw.SetFile("/home/ada/bru")
	iw.Preview()
	w.Wait()

	if len(b.PreviewedSources) != 2 {
		t.Fatalf("sources %+v", b.PreviewedSources)
	}
	pasted, folder := b.PreviewedSources[0], b.PreviewedSources[1]
	if string(pasted.Data) != "curl https://x/a" || pasted.Name != "pasted text" || folder.Dir != "/home/ada/bru" || folder.Data != nil {
		t.Fatalf("pasted %+v folder %+v", pasted, folder)
	}
	if vm.SourceModeOf(flowi.ImportBruno) != vm.SourceFolder || vm.SourceModeOf(flowi.ImportMermaid) != vm.SourceFileOrPaste || vm.SourceModeOf(flowi.ImportHAR) != vm.SourceFile {
		t.Fatal("source modes")
	}
}

func TestImportWizard_report_lists_items_and_skipped_items_need_partial(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	bad := importers.Item{Name: "broken", Source: diag.Location{File: "/c.json", Pos: diag.Position{Line: 3, Column: 2}}}
	syntax := diag.New(diag.CodeImportSyntax, diag.Location{File: "/c.json", Pos: diag.Position{Line: 3, Column: 2}}, map[string]string{"format": "curl", "detail": "unclosed quote"})
	syntax.Excerpt = []string{"curl 'x", " ^"}
	bad.Report(syntax)
	bad.Skip("the command cannot be read")
	b.Plan.Items = []importers.Item{{Name: "ok"}, bad}
	iw.SetKind(flowi.ImportCurl)
	iw.SetFile(file)
	iw.Preview()
	w.Wait()

	items := iw.Items()
	if len(items) != 2 || items[1].Skipped != true || iw.Skipped() != 1 {
		t.Fatalf("items %+v", items)
	}
	p := items[1].Problems[0]
	if p.Where != "c.json:3:2" || !strings.Contains(p.Text, "unclosed quote") || len(p.Excerpt) != 2 || p.Severity != "error" {
		t.Fatalf("problem %+v", p)
	}

	iw.Import(vm.ImportRefuseExisting, false)
	w.Wait()
	refused := len(b.ApplyOpts)
	iw.Import(vm.ImportRefuseExisting, true)
	w.Wait()

	if refused != 0 || !strings.Contains(iw.Done(), "Imported") || len(b.ApplyOpts) != 1 {
		t.Fatalf("refused %d done %q err %q", refused, iw.Done(), iw.Err())
	}
}

func TestImportWizard_target_folder_goes_to_the_source_and_belongs_to_its_collection(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	iw.SetFile(file)

	iw.SetTarget("C1", "/demo/websocket/")
	iw.Preview()
	w.Wait()

	if iw.Folder() != "demo/websocket" || len(b.PreviewedSources) != 1 || b.PreviewedSources[0].Into != "demo/websocket" {
		t.Fatalf("folder %q sources %+v", iw.Folder(), b.PreviewedSources)
	}
	if got := iw.TargetName(); got != "Shop / demo / websocket" {
		t.Fatalf("target %q", got)
	}
	iw.SetCollection("C1")
	if iw.Folder() != "" || iw.CanImport() {
		t.Fatalf("a folder belongs to its collection: folder %q, preview kept %v", iw.Folder(), iw.CanImport())
	}
}

func TestImportWizard_Preview_Given_no_source_Then_nothing_is_asked_of_the_backend(t *testing.T) {
	iw, b, w, _ := newImportWizard(t)

	iw.Preview()
	w.Wait()

	// Before: a preview of nothing went to the backend and came back as "reading : no such file".
	if len(b.PreviewedSources) != 0 || iw.Err() != "" || iw.Previewing() || iw.HasSource() {
		t.Fatalf("sources %v err %q previewing %v", b.PreviewedSources, iw.Err(), iw.Previewing())
	}
}

func TestImportWizard_pasted_text_is_the_source_of_any_kind_and_is_named_as_pasted(t *testing.T) {
	iw, b, w, _ := newImportWizard(t)
	iw.SetKind(flowi.ImportAsyncAPI)
	iw.SetPaste("asyncapi: 3.0.0\n")

	iw.Preview()
	w.Wait()

	// Before: only curl, .http and Mermaid took pasted text; an AsyncAPI document went through a temporary file.
	if len(b.PreviewedSources) != 1 || b.PreviewedSources[0].Name != flowi.PastedSourceName || string(b.PreviewedSources[0].Data) != "asyncapi: 3.0.0\n" || !iw.Pasted() {
		t.Fatalf("sources %+v pasted %v", b.PreviewedSources, iw.Pasted())
	}
}

func TestImportWizard_a_folder_kind_ignores_pasted_text(t *testing.T) {
	iw, _, _, _ := newImportWizard(t)
	iw.SetKind(flowi.ImportBruno)
	iw.SetPaste("anything")

	if iw.HasSource() || iw.Pasted() {
		t.Fatal("Bruno reads a folder")
	}
}

func TestImportWizard_Import_Given_a_request_that_exists_and_keep_mine_Then_the_outcome_says_what_was_kept(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	b.Plan.Files = []flowi.PlannedFile{
		{Rel: "descriptions/ws.asyncapi.yaml", Kind: flowi.PlannedDocument},
		{Rel: "requests/echo.req.yaml", Kind: flowi.PlannedRequest, Name: "Echo", Exists: true},
		{Rel: "requests/receiveTicks.req.yaml", Kind: flowi.PlannedRequest, Name: "Receive ticks"},
	}
	iw.SetFile(file)
	iw.Preview()
	w.Wait()

	iw.Import(vm.ImportSkipExisting, false)
	w.Wait()

	// Before: "Imported 2 files into Shop": the kept document counted, the kept request did not, and nothing said so.
	out := iw.Outcome()
	if out.Headline != "Imported 1 request into Shop" || out.Wrote != 1 {
		t.Fatalf("outcome %+v", out)
	}
	notes := strings.Join(out.Notes, "\n")
	if !strings.Contains(notes, "Kept your request “Echo” (requests/echo.req.yaml)") || len(out.Notes) != 1 {
		t.Fatalf("notes %q", notes)
	}
	// The copy of the document is a detail, not something to look at: a dialog may close over it.
	if len(out.Details) != 1 || !strings.Contains(out.Details[0], "descriptions/ws.asyncapi.yaml") {
		t.Fatalf("details %q", out.Details)
	}
}

func TestImportWizard_Import_Given_everything_exists_and_keep_mine_Then_it_says_nothing_was_imported_and_why(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	b.Plan.Files = []flowi.PlannedFile{
		{Rel: "requests/echo.req.yaml", Kind: flowi.PlannedRequest, Name: "Echo", Exists: true},
		{Rel: "requests/receiveTicks.req.yaml", Kind: flowi.PlannedRequest, Name: "Receive ticks", Exists: true},
	}
	iw.SetFile(file)
	iw.Preview()
	w.Wait()

	iw.Import(vm.ImportSkipExisting, false)
	w.Wait()

	out := iw.Outcome()
	if out.Headline != "Nothing was imported into Shop" || out.Wrote != 0 || !strings.Contains(out.Notes[0], "Overwrite them") || len(out.Notes) != 3 || len(out.Details) != 0 {
		t.Fatalf("outcome %+v", out)
	}
}

func TestImportWizard_after_an_import_the_plan_is_not_shown_again(t *testing.T) {
	iw, _, w, file := newImportWizard(t)
	iw.SetFile(file)
	iw.Preview()
	w.Wait()

	iw.Import(vm.ImportOverwrite, false)
	w.Wait()

	// Before: the whole preview stayed under "Imported …", so a screen opened later showed what was already imported
	// as if it were about to be.
	if len(iw.Rows()) != 0 || iw.Summary() != "" || iw.Done() == "" {
		t.Fatalf("rows %d summary %q done %q", len(iw.Rows()), iw.Summary(), iw.Done())
	}
}

// manualScheduler holds the one callback it was given until fire is called, like a timer nobody waits for.
type manualScheduler struct {
	f       func()
	delay   time.Duration
	started int
}

func (m *manualScheduler) After(d time.Duration, f func()) func() {
	m.started++
	m.f, m.delay = f, d
	return func() { m.f = nil }
}

func (m *manualScheduler) fire() {
	if f := m.f; f != nil {
		m.f = nil
		f()
	}
}

func TestImportWizard_a_source_is_previewed_by_itself_once_the_choices_settle(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	sched := &manualScheduler{}
	iw.SetScheduler(sched)

	iw.SetFile(file)
	iw.SetOverlay("") // a second change before the delay ends: still one preview
	if len(b.PreviewedSources) != 0 {
		t.Fatal("nothing is read before the delay")
	}
	sched.fire()
	w.Wait()

	// Before: a preview came only from a button, and for a pasted address it sat behind "More options".
	if len(b.PreviewedSources) != 1 || !iw.CanImport() || sched.delay != vm.ImportPreviewDelay {
		t.Fatalf("previews %d, can import %v, delay %v", len(b.PreviewedSources), iw.CanImport(), sched.delay)
	}
}

func TestImportWizard_pasting_text_previews_it_by_itself(t *testing.T) {
	iw, b, w, _ := newImportWizard(t)
	sched := &manualScheduler{}
	iw.SetScheduler(sched)

	iw.SetPaste("curl https://example.test")
	sched.fire()
	w.Wait()

	if len(b.PreviewedSources) != 1 || string(b.PreviewedSources[0].Data) != "curl https://example.test" {
		t.Fatalf("sources %+v", b.PreviewedSources)
	}
}

func TestImportWizard_nothing_is_scheduled_without_a_source_or_for_a_half_typed_address(t *testing.T) {
	iw, _, _, _ := newImportWizard(t)
	sched := &manualScheduler{}
	iw.SetScheduler(sched)

	iw.SetKind(flowi.ImportAsyncAPI)
	if sched.started != 0 || !strings.Contains(iw.Waiting(), "Choose a file or paste") {
		t.Fatalf("scheduled %d, waiting %q", sched.started, iw.Waiting())
	}
	iw.SetFile("https://exa")
	if sched.started != 0 || iw.Waiting() != "Waiting for a complete address." {
		t.Fatalf("scheduled %d, waiting %q", sched.started, iw.Waiting())
	}
	iw.SetFile("https://example.com/api.yaml")
	if sched.started != 1 || iw.Waiting() != "" {
		t.Fatalf("scheduled %d, waiting %q", sched.started, iw.Waiting())
	}
}

// Before: each pause in typing a path was previewed, so the half-typed "/tmp/api" showed a "no such file" failure on the
// way to "/tmp/api.yaml".
func TestImportWizard_a_path_with_nothing_at_it_is_waited_for_not_previewed(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	sched := &manualScheduler{}
	iw.SetScheduler(sched)

	iw.SetFile(file[:len(file)-len(".yaml")])
	sched.fire()
	w.Wait()

	if sched.started != 0 || len(b.PreviewedSources) != 0 || iw.Err() != "" || iw.Waiting() != "There is no file at that path yet." {
		t.Fatalf("scheduled %d, previews %d, err %q, waiting %q", sched.started, len(b.PreviewedSources), iw.Err(), iw.Waiting())
	}
	iw.SetFile(file)
	if sched.started != 1 || iw.Waiting() != "" {
		t.Fatalf("scheduled %d, waiting %q", sched.started, iw.Waiting())
	}
	iw.SetKind(flowi.ImportBruno)
	iw.SetFile(filepath.Join(t.TempDir(), "nowhere"))
	if iw.Waiting() != "There is no folder at that path yet." {
		t.Fatalf("waiting %q", iw.Waiting())
	}
}

// Before: it kept the kind of the import before it, or the one a pasted text was recognised as.
func TestImportWizard_StartOver_returns_to_the_first_kind(t *testing.T) {
	iw, _, _, _ := newImportWizard(t)
	iw.SetDetected(flowi.ImportAsyncAPI, "AsyncAPI 3.0.0")

	iw.StartOver()

	if iw.Kind() != flowi.ImportOpenAPI || iw.Detected() != "" {
		t.Fatalf("kind %q detected %q", iw.Kind(), iw.Detected())
	}
}

func TestImportWizard_changing_the_folder_previews_again_and_a_stale_preview_is_dropped(t *testing.T) {
	iw, b, w, file := newImportWizard(t)
	sched := &manualScheduler{}
	iw.SetScheduler(sched)
	iw.SetFile(file)
	sched.fire()
	w.Wait()

	iw.SetTarget("C1", "demo")
	sched.fire()
	w.Wait()

	if len(b.PreviewedSources) != 2 || b.PreviewedSources[1].Into != "demo" || iw.Previewing() || !iw.CanImport() {
		t.Fatalf("sources %+v previewing %v", b.PreviewedSources, iw.Previewing())
	}
}

func TestImportWizard_StartOver_forgets_the_source_and_the_result_but_not_the_place(t *testing.T) {
	iw, _, w, file := newImportWizard(t)
	iw.SetTarget("C1", "demo")
	iw.SetFile(file)
	iw.Preview()
	w.Wait()
	iw.Import(vm.ImportOverwrite, false)
	w.Wait()

	iw.StartOver()

	if iw.File() != "" || iw.Done() != "" || iw.Outcome().Headline != "" || iw.HasSource() || iw.Folder() != "demo" || iw.Collection() != "C1" {
		t.Fatalf("file %q done %q folder %q", iw.File(), iw.Done(), iw.Folder())
	}
}
