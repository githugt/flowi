// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
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
	if first != nil || second != nil || r.Edit == nil || r.Edit.URL != "http://two" || r.EditBase != base || strings.Count(string(data), "\nedit:") != 1 {
		t.Fatalf("errs %v %v, edit %+v, file:\n%s", first, second, r.Edit, data)
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

func TestStoreEdit_Given_comments_Then_refused_with_FLW_WRITE_001(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	original, _ := os.ReadFile(r.File)
	_ = os.WriteFile(r.File, append([]byte("# keep me\n"), original...), 0o644)

	err := w.StoreEdit(target, r.Versions[0].Label, editOf("http://x"), SaveOptions{})

	after, _ := os.ReadFile(r.File)
	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeWriteDropsComments}) || !strings.HasPrefix(string(after), "# keep me") {
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
