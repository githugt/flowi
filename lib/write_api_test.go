// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/store"
)

func requestOf(t *testing.T, w *Workspace, target RequestTarget) model.Request {
	t.Helper()
	_, r, err := w.requestFor(target)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSaveRequest_should_write_canonical_and_reload(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	r.Description = "Edited in the GUI."
	r.Versions[0].URL = "{{env.base_url}}/v1/signin"

	err := w.SaveRequest(target, r, SaveOptions{})

	data, _ := os.ReadFile(r.File)
	reloaded := requestOf(t, w, target)
	if err != nil || !strings.Contains(string(data), "/v1/signin") || reloaded.Versions[0].URL != r.Versions[0].URL || reloaded.Description != r.Description {
		t.Fatalf("err %v, file:\n%s", err, data)
	}
}

func TestSaveRequest_Given_comments_Then_FLW_WRITE_001_unless_DropComments(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	original, _ := os.ReadFile(r.File)
	if err := os.WriteFile(r.File, append([]byte("# keep me\n"), original...), 0o644); err != nil {
		t.Fatal(err)
	}

	refused := w.SaveRequest(target, r, SaveOptions{})
	afterRefusal, _ := os.ReadFile(r.File)
	forced := w.SaveRequest(target, r, SaveOptions{DropComments: true})
	afterForce, _ := os.ReadFile(r.File)

	if codes := codesOfErr(refused); !slices.Equal(codes, []diag.Code{diag.CodeWriteDropsComments}) || !strings.HasPrefix(string(afterRefusal), "# keep me") {
		t.Fatalf("refused %v, file %q", refused, afterRefusal[:20])
	}
	if forced != nil || strings.Contains(string(afterForce), "# keep me") {
		t.Fatalf("forced %v, file:\n%s", forced, afterForce)
	}
}

func TestSaveRequest_Given_unknown_request_Then_FLW_REQ_001(t *testing.T) {
	w := demoWorkspace(t)
	target := loginTarget(t, w)
	r := requestOf(t, w, target)
	target.Request = "01JA0000000000000000000099"

	err := w.SaveRequest(target, r, SaveOptions{})

	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeRequestNotFound}) {
		t.Fatalf("got %v", err)
	}
}

func TestSaveEnvironment_should_round_trip_secret_flag_and_never_write_values(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	env := c.Environments[slices.IndexFunc(c.Environments, func(e model.Environment) bool { return e.Name == "local" })]
	env.Vars = append(env.Vars, model.EnvVar{Name: "api_key", Value: "should-not-be-written", Secret: true}, model.EnvVar{Name: "region", Value: "eu"})

	err := w.SaveEnvironment(env, SaveOptions{})

	data, _ := os.ReadFile(env.File)
	got := w.Collections()[0].Environments
	i := slices.IndexFunc(got, func(e model.Environment) bool { return e.Name == "local" })
	if err != nil || strings.Contains(string(data), "should-not-be-written") || !strings.Contains(string(data), "region: eu") {
		t.Fatalf("err %v, file:\n%s", err, data)
	}
	if !slices.ContainsFunc(got[i].Vars, func(v model.EnvVar) bool { return v.Name == "api_key" && v.Secret }) {
		t.Fatalf("reloaded %+v", got[i].Vars)
	}
}

func TestNewRequest_should_create_and_be_findable(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]

	target, err := w.NewRequest(c.Manifest.ID, "users/create")

	found, findErr := w.FindRequest("users/create", "")
	r := requestOf(t, w, found)
	if err != nil || findErr != nil || found != target || len(r.Versions) != 1 || r.Versions[0].Label != "1" || r.Versions[0].Method != "GET" {
		t.Fatalf("err %v %v, request %+v", err, findErr, r)
	}
	if _, statErr := os.Stat(filepath.Join(c.Root, "requests", "users", "create.req.yaml")); statErr != nil {
		t.Fatal(statErr)
	}
}

func TestNewRequest_Given_existing_or_bad_path_Then_FLW_WRITE_002(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	for _, path := range []string{"auth/login", "../escape", "", "a//b", ".hidden"} {
		t.Run(path, func(t *testing.T) {
			_, err := w.NewRequest(c.Manifest.ID, path)

			if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeWriteRequestPath}) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

// Load problems of a file must follow its reloads: before, Validate kept
// the problems found at open time and never saw ones found on reload.
func TestReload_should_replace_the_files_load_problems(t *testing.T) {
	w := demoWorkspace(t)
	r := requestOf(t, w, loginTarget(t, w))
	original, _ := os.ReadFile(r.File)
	if err := os.WriteFile(r.File, []byte("id: [unclosed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hasYAMLError := func() bool {
		return slices.ContainsFunc(w.Validate(), func(d diag.Diagnostic) bool { return d.File == r.File && d.Code == diag.CodeYAMLSyntax })
	}

	change := Change{Root: w.Collections()[0].Root, File: r.File, Op: store.ChangeModified}

	_, breakErr := w.Reload(change)
	afterBreak := hasYAMLError()
	if err := os.WriteFile(r.File, original, 0o644); err != nil {
		t.Fatal(err)
	}
	_, fixErr := w.Reload(change)
	afterFix := hasYAMLError()

	if breakErr != nil || fixErr != nil || !afterBreak || afterFix {
		t.Fatalf("reload %v / %v, error after break %v, after fix %v", breakErr, fixErr, afterBreak, afterFix)
	}
}

func TestNewEnvironment_should_create_an_empty_environment_and_reject_bad_or_taken_names(t *testing.T) {
	w := demoWorkspace(t)

	file, err := w.NewEnvironment("qa-box")
	_, bad := w.NewEnvironment("../x")
	_, dup := w.NewEnvironment("qa-box")

	if err != nil || filepath.Base(file) != "qa-box.yaml" {
		t.Fatalf("file %q err %v", file, err)
	}
	reloaded := w.Collections()[0]
	if !slices.ContainsFunc(reloaded.Environments, func(e model.Environment) bool { return e.File == file && e.Name == "qa-box" && len(e.Vars) == 0 }) {
		t.Fatalf("environments %+v", reloaded.Environments)
	}
	for _, e := range []error{bad, dup} {
		if codes := codesOfErr(e); !slices.Equal(codes, []diag.Code{diag.CodeWriteRequestPath}) {
			t.Fatalf("got %v", e)
		}
	}
}

func TestNewFolder_should_create_a_folder_that_loads_even_when_empty_and_refuse_bad_or_taken_paths(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]

	err := w.NewFolder(c.Manifest.ID, "requests", "billing/invoices")
	again := w.NewFolder(c.Manifest.ID, "requests", "billing/invoices")
	bad := w.NewFolder(c.Manifest.ID, "requests", "../x")
	unknownTree := w.NewFolder(c.Manifest.ID, "secrets", "a")
	flows := w.NewFolder(c.Manifest.ID, "flows", "nightly")

	got := w.Collections()[0].Folders
	has := func(dir string) bool {
		return slices.ContainsFunc(got, func(f model.Folder) bool { return f.Dir == dir })
	}
	if err != nil || flows != nil || !has("requests/billing/invoices") || !has("flows/nightly") {
		t.Fatalf("err %v flows %v folders %+v", err, flows, got)
	}
	for _, e := range []error{again, bad, unknownTree} {
		if !slices.Equal(codesOfErr(e), []diag.Code{diag.CodeWriteRequestPath}) {
			t.Fatalf("got %v", e)
		}
	}
}

func TestMoveFolder_should_move_the_folder_with_its_requests_and_keep_their_ids(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	login := requestIDAt(t, w, "auth/login")

	err := w.MoveFolder(c.Manifest.ID, "requests", "auth", "identity/auth")
	again := w.MoveFolder(c.Manifest.ID, "requests", "auth", "x")
	intoItself := w.MoveFolder(c.Manifest.ID, "requests", "identity", "identity/inner")

	moved, found := w.FindRequest("identity/auth/login", "")
	if err != nil || found != nil || moved.Request != login {
		t.Fatalf("err %v find %v moved %+v want id %s", err, found, moved, login)
	}
	for _, e := range []error{again, intoItself} {
		if !slices.Equal(codesOfErr(e), []diag.Code{diag.CodeWriteRequestPath}) {
			t.Fatalf("got %v", e)
		}
	}
}

func TestDeleteFolder_should_remove_the_folder_and_everything_in_it(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]

	err := w.DeleteFolder(c.Manifest.ID, "requests", "auth")

	_, found := w.FindRequest("auth/login", "")
	bad := w.DeleteFolder(c.Manifest.ID, "requests", "../x")
	if err != nil || found == nil || !slices.Equal(codesOfErr(bad), []diag.Code{diag.CodeWriteRequestPath}) {
		t.Fatalf("err %v find %v bad %v", err, found, bad)
	}
}

func requestIDAt(t *testing.T, w *Workspace, ref string) model.ID {
	t.Helper()
	target, err := w.FindRequest(ref, "")
	if err != nil {
		t.Fatal(err)
	}
	return target.Request
}

func TestSetFolderOrder_should_keep_the_order_in_the_folder_file_and_load_it_back(t *testing.T) {
	w := demoWorkspace(t)
	id := w.Collections()[0].Manifest.ID

	err := w.SetFolderOrder(id, "requests", "auth", []string{"verify", "login"}, SaveOptions{})
	errTop := w.SetFolderOrder(id, "requests", "", []string{"onboarding", "auth"}, SaveOptions{})

	if err != nil || errTop != nil {
		t.Fatalf("errors %v %v", err, errTop)
	}
	var auth, top *model.Folder
	for _, f := range w.Collections()[0].Folders {
		switch f.Dir {
		case "requests/auth":
			auth = &f
		case "requests":
			top = &f
		}
	}
	if auth == nil || len(auth.Order) != 2 || auth.Order[0] != "verify" || top == nil || top.Order[0] != "onboarding" {
		t.Fatalf("folders %+v", w.Collections()[0].Folders)
	}
	text, _ := os.ReadFile(filepath.Join(w.Collections()[0].Root, "requests", "auth", "_folder.yaml"))
	if !strings.Contains(string(text), "- verify") {
		t.Fatalf("the folder file:\n%s", text)
	}
}

func TestSetFolderOrder_Given_a_bad_tree_or_name_Then_refuses_and_writes_nothing(t *testing.T) {
	w := demoWorkspace(t)
	id := w.Collections()[0].Manifest.ID

	errTree := w.SetFolderOrder(id, "environments", "", []string{"a"}, SaveOptions{})
	errName := w.SetFolderOrder(id, "requests", "auth", []string{"../x"}, SaveOptions{})
	errDir := w.SetFolderOrder(id, "requests", "../escape", []string{"a"}, SaveOptions{})

	if errTree == nil || errName == nil || errDir == nil {
		t.Fatalf("errors %v %v %v", errTree, errName, errDir)
	}
}
