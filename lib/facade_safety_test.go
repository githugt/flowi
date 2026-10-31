// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/settings"
)

func TestFormat_should_refuse_files_whose_content_it_would_lose(t *testing.T) {
	w := newEnv(t).open(t)
	dir := copyCollection(t, "identity")
	mustMount(t, w, dir)
	verify := filepath.Join(dir, "requests", "auth", "verify.req.yaml")
	broken := "name: Verify\nid: 01J9ZC3M4N5P6Q7R8S9T0VWXYZ\nversions:\n  \"1\":\n    url: /verify\n    settings:\n      timout: 10s\n      timeout: 30\n"
	if err := os.WriteFile(verify, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, "flowi.yaml")
	if err := os.WriteFile(manifest, []byte("name: identity\nformat: 1\nid: 01JB12K3M4N5P6Q7R8S9T0VWXY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w = newEnvFrom(t, w).reopen(t)

	changes, blocked, err := w.Format("")

	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || !strings.HasSuffix(changes[0].File, "flowi.yaml") {
		t.Fatalf("changes %+v", changes)
	}
	codes := map[diag.Code]bool{}
	for _, d := range blocked {
		codes[d.Code] = true
		if !strings.HasSuffix(d.File, "verify.req.yaml") {
			t.Errorf("unexpected blocked file %s", d.File)
		}
	}
	if !codes[diag.CodeSettingUnknown] || !codes[diag.CodeSettingInvalid] {
		t.Fatalf("blocked %+v", blocked)
	}
}

func TestSetSetting_should_refuse_to_overwrite_a_broken_settings_file(t *testing.T) {
	e := newEnv(t)
	file := filepath.Join(e.opts.Paths.ConfigDir, "settings.yaml")
	original := "# my settings\nproxy:\n  http: http://p:3128\nlogging: [\n"
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	w, _, err := OpenWorkspace(context.Background(), e.opts)
	if err != nil {
		t.Fatal(err)
	}

	setErr := w.SetSetting(settings.LayerUser, "timeout", "20s")
	resetErr := w.ResetSetting(settings.LayerUser, "timeout")

	var le *diag.ListError
	if !errors.As(setErr, &le) || le.List[0].Code != diag.CodeYAMLSyntax || !errors.As(resetErr, &le) {
		t.Fatalf("set %v reset %v", setErr, resetErr)
	}
	if got, _ := os.ReadFile(file); string(got) != original {
		t.Fatalf("settings file changed:\n%s", got)
	}
}

func TestWorkspaceWrites_should_refuse_a_broken_workspace_file(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	mustMount(t, w, copyCollection(t, "identity"))
	file := w.Model().File
	data, _ := os.ReadFile(file)
	broken := string(data) + "bogus: [\n"
	if err := os.WriteFile(file, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	w, _, err := OpenWorkspace(context.Background(), e.opts)
	if err != nil {
		t.Fatal(err)
	}

	errs := []error{
		func() error { _, err := w.Mount(copyCollection(t, "app")); return err }(),
		func() error { _, err := w.InitCollection(filepath.Join(t.TempDir(), "x"), "x", true); return err }(),
		w.SetSetting(settings.LayerWorkspace, "defaults.environment", "staging"),
	}

	for i, err := range errs {
		var le *diag.ListError
		if !errors.As(err, &le) {
			t.Errorf("write %d: got %v, want a diagnostic error", i, err)
		}
	}
	if got, _ := os.ReadFile(file); string(got) != broken {
		t.Fatalf("workspace file changed:\n%s", got)
	}
}

func TestOpenWorkspace_should_keep_loading_when_one_collection_is_unreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	e := newEnv(t)
	w := e.open(t)
	locked := copyCollection(t, "identity")
	mustMount(t, w, locked)
	mustMount(t, w, copyCollection(t, "app"))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	again, diags, err := OpenWorkspace(context.Background(), e.opts)

	if err != nil {
		t.Fatal(err)
	}
	if len(again.Collections()) != 1 || len(again.Missing()) != 1 {
		t.Fatalf("collections %d missing %d", len(again.Collections()), len(again.Missing()))
	}
	if !hasCode(diags, diag.CodeFileUnreadable) {
		t.Fatalf("got %v", codesOf(diags))
	}
}

func TestReload_should_rescan_the_whole_collection(t *testing.T) {
	w := newEnv(t).open(t)
	dir := copyCollection(t, "identity")
	mustMount(t, w, dir)
	root := w.Collections()[0].Root
	extra := filepath.Join(dir, "requests", "extra.req.yaml")
	if err := os.WriteFile(extra, []byte("id: 01J9ZH3M4N5P6Q7R8S9T0VWXYZ\nname: Extra\nversions:\n  \"1\":\n    url: /x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := w.Reload(Change{Root: root, File: root, Op: ChangeRescan})

	if err != nil || len(w.Collections()[0].Requests) != 2 {
		t.Fatalf("err %v requests %d", err, len(w.Collections()[0].Requests))
	}
}

func hasCode(l diag.List, c diag.Code) bool {
	for _, d := range l {
		if d.Code == c {
			return true
		}
	}
	return false
}

type reopener struct{ opts WorkspaceOptions }

func newEnvFrom(_ *testing.T, w *Workspace) reopener { return reopener{opts: w.opts} }

func (r reopener) reopen(t *testing.T) *Workspace {
	t.Helper()
	w, _, err := OpenWorkspace(context.Background(), r.opts)
	if err != nil {
		t.Fatal(err)
	}
	return w
}
