// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func editOf(url string) model.RequestVersion { return model.RequestVersion{Method: "POST", URL: url} }

func TestStoreEdit_should_keep_exactly_one_slot(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	base := requestOf(t, w, target).Versions[0].Label

	first := w.StoreEdit(target, base, editOf("http://one"), SaveOptions{})
	second := w.StoreEdit(target, base, editOf("http://two"), SaveOptions{})

	r := requestOf(t, w, target)
	data, _ := os.ReadFile(r.File)
	kept, _ := os.ReadFile(w.editFile(target.Collection, target.Request))
	info, _ := os.Stat(w.editFile(target.Collection, target.Request))
	if first != nil || second != nil || r.Edit == nil || r.Edit.URL != "http://two" || r.EditBase != base {
		t.Fatalf("errs %v %v, edit %+v", first, second, r.Edit)
	}
	// Before: the edit was written into the tracked request file, so `git add -A` committed half-typed edits.
	if strings.Contains(string(data), "edit") || strings.Count(string(kept), "\nedit:")+strings.Count(string(kept), "edit:") < 1 || !strings.Contains(string(kept), "http://two") || strings.Contains(string(kept), "http://one") {
		t.Fatalf("request file:\n%s\nkept edit:\n%s", data, kept)
	}
	if info == nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the kept edit must be private to the person: %v", info)
	}
}

func TestStoreEdit_should_not_write_the_request_file_at_all(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	before, _ := os.ReadFile(r.File)

	err := w.StoreEdit(target, r.Versions[0].Label, editOf("http://typing"), SaveOptions{})

	after, _ := os.ReadFile(r.File)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("err %v; the tracked file changed:\n%s", err, after)
	}
}

func TestEdit_Given_an_earlier_release_wrote_it_into_the_request_file_Then_loading_moves_it_out(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	original, _ := os.ReadFile(r.File)
	legacy := append(append([]byte{}, original...), []byte("edit:\n  method: POST\n  url: http://legacy-edit\nedit_base: \""+r.Versions[0].Label+"\"\n")...)
	if err := os.WriteFile(r.File, legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := w.Reload(Change{Root: w.collections[0].Root, File: r.File, Op: ChangeModified}); err != nil {
		t.Fatal(err)
	}
	loaded := requestOf(t, w, target)
	kept, _ := os.ReadFile(w.editFile(target.Collection, target.Request))
	saveErr := w.SaveRequest(target, loaded, SaveOptions{}) // the next write of the file leaves the edit out
	file, _ := os.ReadFile(r.File)

	if loaded.Edit == nil || loaded.Edit.URL != "http://legacy-edit" || !strings.Contains(string(kept), "http://legacy-edit") {
		t.Fatalf("loaded edit %+v, kept:\n%s", loaded.Edit, kept)
	}
	if saveErr != nil || strings.Contains(string(file), "legacy-edit") || requestOf(t, w, target).Edit == nil {
		t.Fatalf("save %v, file:\n%s", saveErr, file)
	}
}

func TestStoreEdit_should_leave_saved_versions_untouched(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	before := requestOf(t, w, target).Versions

	err := w.StoreEdit(target, before[0].Label, editOf("http://x"), SaveOptions{})

	after := requestOf(t, w, target).Versions
	if err != nil || !slices.EqualFunc(before, after, func(a, b model.RequestVersion) bool {
		return a.Label == b.Label && a.URL == b.URL && a.Method == b.Method
	}) {
		t.Fatalf("err %v, before %+v after %+v", err, before, after)
	}
}

func TestPromoteEdit_Given_empty_label_Then_overwrites_the_base_and_clears_the_slot(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	base := requestOf(t, w, target).Versions[0].Label
	_ = w.StoreEdit(target, base, editOf("http://promoted"), SaveOptions{})

	err := w.PromoteEdit(target, "", SaveOptions{})

	r := requestOf(t, w, target)
	if err != nil || r.Edit != nil || r.EditBase != "" || r.Versions[0].URL != "http://promoted" || r.Versions[0].Label != base {
		t.Fatalf("err %v, request %+v", err, r)
	}
}

func TestPromoteEdit_Given_new_label_Then_adds_a_version_and_clears_the_slot(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r0 := requestOf(t, w, target)
	_ = w.StoreEdit(target, r0.Versions[0].Label, editOf("http://new"), SaveOptions{})

	err := w.PromoteEdit(target, "9.9", SaveOptions{})

	r := requestOf(t, w, target)
	i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == "9.9" })
	if err != nil || r.Edit != nil || i < 0 || r.Versions[i].URL != "http://new" || len(r.Versions) != len(r0.Versions)+1 {
		t.Fatalf("err %v, request %+v", err, r)
	}
}

func TestPromoteEdit_Given_existing_label_Then_FLW_WRITE_004_and_slot_kept(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r0 := requestOf(t, w, target)
	_ = w.StoreEdit(target, r0.Versions[0].Label, editOf("http://x"), SaveOptions{})

	err := w.PromoteEdit(target, r0.Versions[0].Label, SaveOptions{})

	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeWriteEditLabel}) || requestOf(t, w, target).Edit == nil {
		t.Fatalf("err %v", err)
	}
}

func TestPromoteEdit_Given_missing_base_and_empty_label_Then_FLW_WRITE_004(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	_ = w.StoreEdit(target, "no-such-version", editOf("http://x"), SaveOptions{})

	err := w.PromoteEdit(target, "", SaveOptions{})

	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeWriteEditLabel}) {
		t.Fatalf("err %v", err)
	}
}

func TestPromoteEdit_Given_no_edit_Then_FLW_WRITE_003(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)

	err := w.PromoteEdit(target, "", SaveOptions{})

	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeWriteEditNone}) {
		t.Fatalf("err %v", err)
	}
}

func TestDiscardEdit_should_clear_the_slot(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	_ = w.StoreEdit(target, requestOf(t, w, target).Versions[0].Label, editOf("http://x"), SaveOptions{})

	err := w.DiscardEdit(target, SaveOptions{})

	r := requestOf(t, w, target)
	data, _ := os.ReadFile(r.File)
	if err != nil || r.Edit != nil || strings.Contains(string(data), "edit") {
		t.Fatalf("err %v, file:\n%s", err, data)
	}
}

func TestStoreEdit_Given_the_file_has_comments_Then_the_edit_is_still_kept_and_the_comments_stay(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	original, _ := os.ReadFile(r.File)
	_ = os.WriteFile(r.File, append([]byte("# keep me\n"), original...), 0o644)

	err := w.StoreEdit(target, r.Versions[0].Label, editOf("http://x"), SaveOptions{})

	after, _ := os.ReadFile(r.File)
	if err != nil || !strings.HasPrefix(string(after), "# keep me") || requestOf(t, w, target).Edit == nil {
		t.Fatalf("err %v", err)
	}
}

func TestSend_should_ignore_the_edit_slot(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	_ = w.StoreEdit(target, r.Versions[0].Label, editOf("http://must-not-be-used.invalid"), SaveOptions{})

	_, saved, err := w.requestFor(target)

	if err != nil || saved.Versions[0].URL == "http://must-not-be-used.invalid" || saved.Default != r.Default {
		t.Fatalf("err %v, versions %+v", err, saved.Versions)
	}
}

// Review finding 11: "1.0" names the same version as "1", and a file with
// both fails to load.
func TestPromoteEdit_Given_a_label_equal_to_an_existing_one_Then_FLW_WRITE_004(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r0 := requestOf(t, w, target)
	_ = w.StoreEdit(target, r0.Versions[0].Label, editOf("http://x"), SaveOptions{})
	same := r0.Versions[0].Label + ".0"

	err := w.PromoteEdit(target, same, SaveOptions{})

	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeWriteEditLabel}) || requestOf(t, w, target).Edit == nil {
		t.Fatalf("err %v", err)
	}
}

// Before: "Version 9.9" was refused as not a number, so the CLI and the GUI could
// disagree about what a person may type; the library now strips the word.
func TestPromoteEdit_Given_a_label_with_a_version_word_Then_it_is_saved_without_it(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r0 := requestOf(t, w, target)
	_ = w.StoreEdit(target, r0.Versions[0].Label, editOf("http://new"), SaveOptions{})

	err := w.PromoteEdit(target, "Version 9.9", SaveOptions{})

	r := requestOf(t, w, target)
	i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == "9.9" })
	if err != nil || i < 0 {
		t.Fatalf("err %v, versions %v", err, r.Versions)
	}
}

func TestEdit_should_survive_a_full_reload_and_go_away_with_a_promotion_or_a_discard(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	base := r.Versions[0].Label
	_ = w.StoreEdit(target, base, editOf("http://typed"), SaveOptions{})
	root := w.collections[0].Root

	_, rescanErr := w.Reload(Change{Root: root, File: root, Op: ChangeRescan})
	afterRescan := requestOf(t, w, target)
	_, statKept := os.Stat(w.editFile(target.Collection, target.Request))
	discardErr := w.DiscardEdit(target, SaveOptions{})
	_, statDiscarded := os.Stat(w.editFile(target.Collection, target.Request))
	_ = w.StoreEdit(target, base, editOf("http://again"), SaveOptions{})
	promoteErr := w.PromoteEdit(target, "", SaveOptions{})
	_, statPromoted := os.Stat(w.editFile(target.Collection, target.Request))
	promoted := requestOf(t, w, target)

	if rescanErr != nil || afterRescan.Edit == nil || afterRescan.Edit.URL != "http://typed" || statKept != nil {
		t.Fatalf("rescan %v edit %+v kept %v", rescanErr, afterRescan.Edit, statKept)
	}
	if discardErr != nil || !os.IsNotExist(statDiscarded) {
		t.Fatalf("discard %v stat %v", discardErr, statDiscarded)
	}
	if promoteErr != nil || !os.IsNotExist(statPromoted) || promoted.Edit != nil || promoted.Versions[0].URL != "http://again" {
		t.Fatalf("promote %v stat %v request %+v", promoteErr, statPromoted, promoted)
	}
}

func TestEdit_Given_the_kept_file_is_unreadable_Then_the_request_loads_without_an_edit(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	_ = w.StoreEdit(target, r.Versions[0].Label, editOf("http://typed"), SaveOptions{})
	if err := os.WriteFile(w.editFile(target.Collection, target.Request), []byte("edit: [broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := w.collections[0].Root

	_, err := w.Reload(Change{Root: root, File: root, Op: ChangeRescan})

	got := requestOf(t, w, target)
	if err != nil || got.Edit != nil || len(got.Versions) == 0 {
		t.Fatalf("err %v request %+v", err, got)
	}
}
