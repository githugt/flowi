// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/settings"
)

func copyCollection(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", "collections", name)
	dst := filepath.Join(t.TempDir(), name)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

type env struct {
	opts WorkspaceOptions
	home string
}

func newEnv(t *testing.T, environ ...string) env {
	home := t.TempDir()
	return env{home: home, opts: WorkspaceOptions{Paths: settings.Paths{ConfigDir: filepath.Join(home, "config")}, Environ: environ, Home: home}}
}

func (e env) open(t *testing.T) *Workspace {
	t.Helper()
	w, diags, err := OpenWorkspace(context.Background(), e.opts)
	if err != nil {
		t.Fatal(err)
	}
	if diags.HasErrors() {
		t.Fatalf("open diagnostics: %+v", diags)
	}
	return w
}

func mustMount(t *testing.T, w *Workspace, dir string) {
	t.Helper()
	if _, err := w.Mount(dir); err != nil {
		t.Fatal(err)
	}
}

func codesOf(l diag.List) []diag.Code {
	var out []diag.Code
	for _, d := range l {
		out = append(out, d.Code)
	}
	return out
}

func TestOpenWorkspace_should_create_default_and_load_mounted_collections(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	mustMount(t, w, copyCollection(t, "identity"))
	mustMount(t, w, copyCollection(t, "app"))

	again := e.open(t)

	if again.Model().Name != "default-workspace" || len(again.Collections()) != 2 {
		t.Fatalf("got %+v with %d collections", again.Model(), len(again.Collections()))
	}
	if diags := again.Validate(); len(diags) != 0 {
		t.Fatalf("unexpected %v", codesOf(diags))
	}
}

func TestOpenWorkspace_Given_missing_mount_Then_FLW_WS_001_and_other_collections_loaded(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	gone := copyCollection(t, "identity")
	mustMount(t, w, gone)
	mustMount(t, w, copyCollection(t, "app"))
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	again, diags, err := OpenWorkspace(context.Background(), e.opts)

	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].Code != diag.CodeWSMissing || diags[0].Pos.Line == 0 {
		t.Fatalf("got %+v", diags)
	}
	if len(again.Collections()) != 1 || len(again.Missing()) != 1 {
		t.Fatalf("collections %d missing %d", len(again.Collections()), len(again.Missing()))
	}
}

func TestValidate_should_fill_positions(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	app := copyCollection(t, "app")
	mustMount(t, w, app)

	diags := w.Validate()

	if len(diags) != 1 || diags[0].Code != diag.CodeRefNotMounted {
		t.Fatalf("got %v", codesOf(diags))
	}
	if diags[0].Pos.Line != 23 || diags[0].Pos.Column != 25 {
		t.Fatalf("position %+v", diags[0].Pos)
	}
}

func TestValidateDir_should_resolve_dependencies_from_mounted_collections(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	mustMount(t, w, copyCollection(t, "identity"))
	unmounted := copyCollection(t, "app")

	diags, err := w.ValidateDir(unmounted)

	if err != nil || len(diags) != 0 {
		t.Fatalf("err %v diags %v", err, codesOf(diags))
	}
}

func TestValidateDir_Given_non_collection_Then_FLW_WS_003(t *testing.T) {
	w := newEnv(t).open(t)

	_, err := w.ValidateDir(t.TempDir())

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeWSNotCollection {
		t.Fatalf("got %v", err)
	}
}

func TestFormat_should_repair_cross_collection_path_hint(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	identity := copyCollection(t, "identity")
	if err := os.Rename(filepath.Join(identity, "requests", "auth"), filepath.Join(identity, "requests", "id")); err != nil {
		t.Fatal(err)
	}
	mustMount(t, w, identity)
	mustMount(t, w, copyCollection(t, "app"))

	changes, _, err := w.Format("")

	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || !strings.Contains(string(changes[0].New), "path: id/verify") {
		t.Fatalf("got %d changes: %+v", len(changes), changes)
	}
}

func TestInitCollection_should_create_and_mount(t *testing.T) {
	w := newEnv(t).open(t)
	dir := filepath.Join(t.TempDir(), "fresh")

	m, err := w.InitCollection(dir, "fresh", true)

	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "fresh" || len(w.Collections()) != 1 || w.Collections()[0].Manifest.ID != m.ID {
		t.Fatalf("manifest %+v collections %d", m, len(w.Collections()))
	}
}

func TestMount_should_reject_non_collection_dir_with_FLW_WS_003(t *testing.T) {
	w := newEnv(t).open(t)

	_, err := w.Mount(t.TempDir())

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeWSNotCollection {
		t.Fatalf("got %v", err)
	}
}

func TestMount_should_reject_duplicate_with_FLW_WS_002(t *testing.T) {
	w := newEnv(t).open(t)
	dir := copyCollection(t, "identity")
	mustMount(t, w, dir)

	_, err := w.Mount(dir)

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeWSAlreadyMounted {
		t.Fatalf("got %v", err)
	}
}

func TestUnmount_and_Relocate(t *testing.T) {
	w := newEnv(t).open(t)
	dir := copyCollection(t, "identity")
	mustMount(t, w, dir)
	id := w.Collections()[0].Manifest.ID
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}

	relocateErr := w.Relocate(id, moved)
	unmountErr := w.Unmount(id)
	again := w.Unmount(id)

	if relocateErr != nil || unmountErr != nil {
		t.Fatal(relocateErr, unmountErr)
	}
	if again == nil || len(w.Collections()) != 0 {
		t.Fatalf("second unmount %v, collections %d", again, len(w.Collections()))
	}
}

func TestSettings_should_layer_user_workspace_collection_folder_env_flag(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	app := copyCollection(t, "app")
	mustMount(t, w, app)
	appID := w.Collections()[0].Manifest.ID
	if err := w.SetSetting(settings.LayerUser, "timeout", "9s"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetSetting(settings.LayerWorkspace, "timeout", "8s"); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		environ   []string
		flags     settings.LayerValues
		scope     Scope
		want      time.Duration
		wantLayer settings.Layer
	}{
		{name: "workspace_beats_user", scope: Scope{}, want: 8 * time.Second, wantLayer: settings.LayerWorkspace},
		{name: "collection", scope: Scope{Collection: appID}, want: 5 * time.Second, wantLayer: settings.LayerCollection},
		{name: "innermost_folder", scope: Scope{Collection: appID, Folder: "requests/auth"}, want: 3 * time.Second, wantLayer: settings.LayerFolder},
		{name: "env", environ: []string{"FLOWI_TIMEOUT=2s"}, scope: Scope{Collection: appID, Folder: "requests/auth"}, want: 2 * time.Second, wantLayer: settings.LayerEnv},
		{name: "flag", environ: []string{"FLOWI_TIMEOUT=2s"}, flags: settings.LayerValues{Layer: settings.LayerFlag, Source: "--timeout", Values: map[string]any{"timeout": time.Second}}, scope: Scope{Collection: appID}, want: time.Second, wantLayer: settings.LayerFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := e.opts
			opts.Environ, opts.Flags = tt.environ, tt.flags
			ws, _, err := OpenWorkspace(context.Background(), opts)
			if err != nil {
				t.Fatal(err)
			}

			v, ok := ws.Settings(tt.scope).Get("timeout")

			if !ok || v.Value != tt.want || v.Layer != tt.wantLayer {
				t.Fatalf("got %+v", v)
			}
		})
	}
}

func TestSetSetting_should_reject_collection_layer_and_personal_at_workspace(t *testing.T) {
	w := newEnv(t).open(t)
	tests := []struct {
		name  string
		layer settings.Layer
		key   string
		value string
		code  diag.Code
	}{
		{"collection_layer", settings.LayerCollection, "timeout", "1s", diag.CodeSettingWrongLayer},
		{"personal_at_workspace", settings.LayerWorkspace, "appearance.theme", "dark", diag.CodeSettingWrongLayer},
		{"invalid_value", settings.LayerUser, "http.max_redirects", "500", diag.CodeSettingInvalid},
		{"unknown_key", settings.LayerUser, "timout", "1s", diag.CodeSettingUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := w.SetSetting(tt.layer, tt.key, tt.value)

			var le *diag.ListError
			if !errors.As(err, &le) || le.List[0].Code != tt.code {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestResetSetting_should_return_to_default(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	if err := w.SetSetting(settings.LayerUser, "timeout", "9s"); err != nil {
		t.Fatal(err)
	}

	err := w.ResetSetting(settings.LayerUser, "timeout")
	v, _ := e.open(t).Settings(Scope{}).Get("timeout")

	if err != nil || v.Layer != settings.LayerDefault {
		t.Fatalf("err %v value %+v", err, v)
	}
}

func TestReload_should_update_collection_after_file_change(t *testing.T) {
	w := newEnv(t).open(t)
	dir := copyCollection(t, "identity")
	mustMount(t, w, dir)
	file := filepath.Join(dir, "requests", "auth", "verify.req.yaml")
	if err := os.WriteFile(file, []byte("id: 01J9ZC3M4N5P6Q7R8S9T0VWXYZ\nname: Check\nversions:\n  \"1\":\n    url: /v\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diags, err := w.Reload(Change{Root: w.Collections()[0].Root, File: file})

	if err != nil || len(diags) != 0 || w.Collections()[0].Requests[0].Name != "Check" {
		t.Fatalf("err %v diags %v name %q", err, codesOf(diags), w.Collections()[0].Requests[0].Name)
	}
}

// arazzoFlow wraps a flow's own YAML (id, name, nodes, …) in the minimal
// Arazzo document a flow file is: Flowi reads the flow from x-flowi-flow.
func arazzoFlow(body string) string {
	var sb strings.Builder
	sb.WriteString("arazzo: 1.1.0\nworkflows:\n  - workflowId: w\n    x-flowi-flow:\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		sb.WriteString("      " + line + "\n")
	}
	return sb.String()
}
