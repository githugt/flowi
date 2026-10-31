// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func newImportWizard(t *testing.T) (*vm.ImportWizard, *vmtest.Backend, *vm.Worker, string) {
	t.Helper()
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	b.Plan = flowi.ImportPlan{Collection: "C1", Files: []flowi.PlannedFile{
		{Rel: "requests/pets/getPet.req.yaml", Exists: true},
		{Rel: "requests/pets/getPet.schemas/1-200.json"},
		{Rel: "flows/suggested/a-to-b.arazzo.yaml"},
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
	if got := iw.Summary(); got != "1 request, 1 flow, 3 files; 1 already exists" {
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
	if iw.Done() != "Imported 3 files into Shop" || iw.Err() != "" || iw.CanImport() {
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

	if !b.ApplyOpts[0].SkipExisting || iw.Done() != "Imported 2 files into Shop" {
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
