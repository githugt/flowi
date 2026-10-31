// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/diag"
)

func TestMoveRequest_should_move_a_request_to_another_collection_keeping_its_id_and_edit(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)
	other, err := w.InitCollection(filepath.Join(t.TempDir(), "other"), "Other", true)
	if err != nil {
		t.Fatal(err)
	}
	_ = w.StoreEdit(target, source.Versions[0].Label, editOf("http://kept-edit"), SaveOptions{})

	moved, err := w.MoveRequest(target, other.ID, "users/signin")

	_, after, findErr := w.requestFor(moved)
	_, _, goneErr := w.requestFor(target)
	_, statErr := os.Stat(source.File)
	if err != nil || findErr != nil || after.ID != source.ID || after.Path != "users/signin" || after.Edit == nil || after.Edit.URL != "http://kept-edit" {
		t.Fatalf("err %v find %v request %+v", err, findErr, after)
	}
	if goneErr == nil || !os.IsNotExist(statErr) {
		t.Fatalf("the source must be gone: lookup %v stat %v", goneErr, statErr)
	}
}

func TestMoveRequest_should_rename_within_a_collection(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)

	moved, err := w.MoveRequest(target, target.Collection, "auth/renamed")

	_, after, _ := w.requestFor(moved)
	if err != nil || after.Path != "auth/renamed" || after.ID != source.ID || filepath.Base(after.File) != "renamed.req.yaml" {
		t.Fatalf("err %v request %+v", err, after)
	}
}

func TestMoveRequest_Given_an_existing_destination_or_bad_path_Then_FLW_WRITE_002_and_nothing_moves(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)
	other := w.Collections()[0]
	verify := ""
	for _, r := range other.Requests {
		if r.ID != source.ID {
			verify = r.Path
			break
		}
	}

	_, exists := w.MoveRequest(target, target.Collection, verify)
	_, bad := w.MoveRequest(target, target.Collection, "../escape")

	_, stillThere, err := w.requestFor(target)
	if !slices.Equal(codesOfErr(exists), []diag.Code{diag.CodeWriteRequestPath}) || !slices.Equal(codesOfErr(bad), []diag.Code{diag.CodeWriteRequestPath}) || err != nil || stillThere.File != source.File {
		t.Fatalf("exists %v bad %v err %v", exists, bad, err)
	}
}

func TestMoveRequest_Given_an_unknown_collection_Then_FLW_WS_005(t *testing.T) {
	w := demoWorkspace(t)

	_, err := w.MoveRequest(loginTarget(t, w), "01JA0000000000000000000099", "x")

	if !slices.Equal(codesOfErr(err), []diag.Code{diag.CodeWSUnknownCollection}) {
		t.Fatalf("err %v", err)
	}
}
