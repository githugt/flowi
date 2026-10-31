// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/diag"
)

func TestDuplicateRequest_should_copy_the_request_with_a_new_id_and_no_edit(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)
	_ = w.StoreEdit(target, source.Versions[0].Label, editOf("http://unsaved"), SaveOptions{})

	copied, err := w.DuplicateRequest(target, "auth/login-copy")

	after := requestOf(t, w, copied)
	original := requestOf(t, w, target)
	if err != nil || after.ID == source.ID || after.Path != "auth/login-copy" || after.Edit != nil || len(after.Versions) != len(source.Versions) {
		t.Fatalf("err %v copy %+v", err, after)
	}
	if original.Edit == nil || original.Edit.URL != "http://unsaved" {
		t.Fatalf("the original keeps its edit: %+v", original.Edit)
	}
}

func TestDuplicateRequest_Given_an_existing_path_or_bad_path_Then_FLW_WRITE_002(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)

	_, exists := w.DuplicateRequest(target, source.Path)
	_, bad := w.DuplicateRequest(target, "../escape")

	if !slices.Equal(codesOfErr(exists), []diag.Code{diag.CodeWriteRequestPath}) || !slices.Equal(codesOfErr(bad), []diag.Code{diag.CodeWriteRequestPath}) {
		t.Fatalf("exists %v bad %v", exists, bad)
	}
}

func TestDuplicateRequestTo_should_copy_into_another_collection_and_leave_the_original(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)
	other, err := w.InitCollection(filepath.Join(t.TempDir(), "other"), "Other", true)
	if err != nil {
		t.Fatal(err)
	}

	copied, err := w.DuplicateRequestTo(target, other.ID, "users/signin")

	_, after, findErr := w.requestFor(copied)
	_, original, originalErr := w.requestFor(target)
	if err != nil || findErr != nil || copied.Collection != other.ID || after.ID == source.ID || after.Path != "users/signin" {
		t.Fatalf("err %v find %v copy %+v", err, findErr, after)
	}
	if originalErr != nil || original.File != source.File {
		t.Fatalf("the original must stay: %v %+v", originalErr, original)
	}
}

func TestDuplicateRequestTo_Given_an_unknown_collection_Then_FLW_WS_005(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)

	_, err := w.DuplicateRequestTo(target, "NOPE", "x")

	if !slices.Equal(codesOfErr(err), []diag.Code{diag.CodeWSUnknownCollection}) {
		t.Fatalf("err %v", err)
	}
}

func TestDeleteRequest_should_remove_the_file_and_the_request(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	source := requestOf(t, w, target)

	err := w.DeleteRequest(target)

	_, _, findErr := w.requestFor(target)
	_, statErr := os.Stat(source.File)
	if err != nil || findErr == nil || !os.IsNotExist(statErr) {
		t.Fatalf("err %v find %v stat %v", err, findErr, statErr)
	}
}

func TestDeleteRequest_Given_an_unknown_request_Then_FLW_REQ_001(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	target.Request = "nope"

	err := w.DeleteRequest(target)

	if !slices.Equal(codesOfErr(err), []diag.Code{diag.CodeRequestNotFound}) {
		t.Fatalf("err %v", err)
	}
}

func TestDuplicateFlow_should_copy_the_flow_with_a_new_id(t *testing.T) {
	w := demoWorkspace(t)
	target, source := userCreation(t, w)

	copied, err := w.DuplicateFlow(target, "user-creation-copy")

	_, after, findErr := w.flowFor(copied)
	if err != nil || findErr != nil || after.ID == source.ID || after.Path != "user-creation-copy" || len(after.Nodes) != len(source.Nodes) {
		t.Fatalf("err %v find %v flow %+v", err, findErr, after)
	}
}

func TestDeleteFlow_should_remove_the_flow_and_its_layout_file(t *testing.T) {
	w := demoWorkspace(t)
	target, source := userCreation(t, w)
	layout := layoutFile(source.File)
	if err := os.WriteFile(layout, []byte("nodes: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := w.DeleteFlow(target)

	_, _, findErr := w.flowFor(target)
	_, flowStat := os.Stat(source.File)
	_, layoutStat := os.Stat(layout)
	if err != nil || findErr == nil || !os.IsNotExist(flowStat) || !os.IsNotExist(layoutStat) {
		t.Fatalf("err %v find %v flow %v layout %v", err, findErr, flowStat, layoutStat)
	}
}

func TestDeleteEnvironment_should_remove_the_file_and_the_environment(t *testing.T) {
	w := demoWorkspace(t)
	file, err := w.NewEnvironment("scratch-env")
	if err != nil {
		t.Fatal(err)
	}

	err = w.DeleteEnvironment(file)

	_, statErr := os.Stat(file)
	after := w.Collections()[0]
	for _, e := range after.Environments {
		if e.File == file {
			t.Fatalf("environment still loaded: %+v", e)
		}
	}
	if err != nil || !os.IsNotExist(statErr) {
		t.Fatalf("err %v stat %v", err, statErr)
	}
}

func TestDeleteEnvironment_Given_an_unknown_file_Then_FLW_REQ_004(t *testing.T) {
	w := demoWorkspace(t)

	err := w.DeleteEnvironment("/nowhere/x.yaml")

	if !slices.Equal(codesOfErr(err), []diag.Code{diag.CodeRequestEnvUnknown}) {
		t.Fatalf("err %v", err)
	}
}

// Before: a duplicate lost the original's custom name (it became the file name),
// while a duplicated flow kept it.
func TestDuplicateRequest_should_keep_a_custom_name(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	r.Name = "Sign in with a password"
	if err := w.SaveRequest(target, r, SaveOptions{DropComments: true}); err != nil {
		t.Fatal(err)
	}

	copied, err := w.DuplicateRequest(target, "auth/login-copy")

	if after := requestOf(t, w, copied); err != nil || after.Name != "Sign in with a password" {
		t.Fatalf("err %v name %q", err, after.Name)
	}
}
