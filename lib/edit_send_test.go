// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"strings"
	"testing"
)

func TestPreviewRequest_should_render_the_given_version_instead_of_the_saved_one(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	saved := requestOf(t, w, target).Versions[0]
	edited := saved
	edited.URL, edited.Method = "http://edited.example/path", "PUT"

	got, err := w.PreviewRequest(target, edited, SendOptions{}, false)
	same, err2 := w.PreviewRequest(target, saved, SendOptions{}, false)

	if err != nil || err2 != nil {
		t.Fatalf("errs %v %v", err, err2)
	}
	if got.Masked.URL != "http://edited.example/path" || got.Masked.Method != "PUT" || strings.Contains(same.Masked.URL, "edited.example") {
		t.Fatalf("edited %+v saved %+v", got.Masked, same.Masked)
	}
}

func TestSendRequest_Given_UseEdit_Then_the_edit_replaces_its_base_version(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	base := requestOf(t, w, target).Versions[0].Label
	_ = w.StoreEdit(target, base, editOf("http://sent-from-edit.example"), SaveOptions{})
	c, r, _ := w.requestFor(target)

	with := sendInput(c, r, SendOptions{UseEdit: true})
	without := sendInput(c, r, SendOptions{})
	other := sendInput(c, r, SendOptions{UseEdit: true, Version: "99"})

	if with.PinnedVersion != base || with.Request.Versions[0].URL != "http://sent-from-edit.example" || with.Request.Versions[0].Label != base {
		t.Fatalf("with edit: %+v", with.Request.Versions)
	}
	if without.Request.Versions[0].URL == "http://sent-from-edit.example" || other.Request.Versions[0].URL == "http://sent-from-edit.example" {
		t.Fatalf("edit leaked: without %q other %q", without.Request.Versions[0].URL, other.Request.Versions[0].URL)
	}
}
