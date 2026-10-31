// SPDX-License-Identifier: Apache-2.0

package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
)

func TestLoad_should_load_tree_with_nested_folders(t *testing.T) {
	root := copyTree(t, "basic")

	c, diags, err := newStore().Load(root)

	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics %v", codesOf(diags))
	}
	if c.Manifest.Name != "Basic" || len(c.Environments) != 1 || len(c.Requests) != 2 || len(c.Flows) != 1 {
		t.Fatalf("unexpected collection %+v", c)
	}
	if c.Environments[0].Name != "local" {
		t.Fatalf("environment name %q, want file stem", c.Environments[0].Name)
	}
	if c.Requests[0].Path != "auth/login" || c.Requests[1].Path != "onboarding/form-page-1" || c.Flows[0].Path != "journeys/signup" {
		t.Fatalf("paths %q %q %q", c.Requests[0].Path, c.Requests[1].Path, c.Flows[0].Path)
	}
	if len(c.Folders) != 1 || c.Folders[0].Dir != "requests" || c.Folders[0].Settings["timeout"] != 10*time.Second {
		t.Fatalf("folders %+v", c.Folders)
	}
	if len(c.Blueprints) != 1 || c.Blueprints[0].Name != "Standard onboarding" || c.Blueprints[0].Path != "standard" ||
		len(c.Blueprints[0].Params) != 1 || c.Blueprints[0].Body.Nodes[0].Request.Request.Param != "api" {
		t.Fatalf("blueprints %+v", c.Blueprints)
	}
}

func TestLoad_should_warn_on_unrecognised_yaml_file(t *testing.T) {
	root := copyTree(t, "basic")
	writeFile(t, filepath.Join(root, "requests", "auth", "notes.yaml"), "a: 1\n")

	_, diags, err := newStore().Load(root)

	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].Code != diag.CodeFileUnrecognised || diags[0].Args["name"] != "notes.yaml" {
		t.Fatalf("got %+v", diags)
	}
}

func TestLoad_Given_newer_format_Then_only_FLW_FMT_001(t *testing.T) {
	root := copyTree(t, "basic")
	writeFile(t, filepath.Join(root, "flowi.yaml"), "format: 9\nid: 01J9Z8K3M4N5P6Q7R8S9T0VWXY\nunknown: x\n")
	writeFile(t, filepath.Join(root, "requests", "bad.req.yaml"), "::: not yaml")

	_, diags, err := newStore().Load(root)

	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].Code != diag.CodeFormatTooNew {
		t.Fatalf("got %v", codesOf(diags))
	}
	if diags[0].Pos.Line != 1 {
		t.Fatalf("FLW-FMT-001 not positioned: %+v", diags[0])
	}
}

func TestLoad_Given_directory_without_manifest_Then_ErrNotCollection(t *testing.T) {
	_, _, err := newStore().Load(t.TempDir())

	if !errors.Is(err, ErrNotCollection) {
		t.Fatalf("got %v", err)
	}
}

func TestLoad_should_accept_CRLF_BOM_file_and_Format_should_rewrite_it_LF_without_BOM(t *testing.T) {
	root := copyTree(t, "basic")
	login := filepath.Join(root, "requests", "auth", "login.req.yaml")
	canonical, _ := os.ReadFile(login)
	windows := append([]byte{0xEF, 0xBB, 0xBF}, bytes.ReplaceAll(canonical, []byte("\n"), []byte("\r\n"))...)
	writeFile(t, login, string(windows))
	s := newStore()

	c, diags, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := s.Format(c)
	if err != nil {
		t.Fatal(err)
	}

	if len(diags) != 1 || diags[0].Code != diag.CodeLintInvisible {
		t.Fatalf("got %v", codesOf(diags))
	}
	if len(changes) != 1 || changes[0].File != c.Requests[0].File || !bytes.Equal(changes[0].New, canonical) {
		t.Fatalf("got %d changes: %+v", len(changes), changes)
	}
}

func TestLoad_should_resolve_symlinked_root(t *testing.T) {
	root := copyTree(t, "basic")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks need extra privileges on Windows: %v", err)
		}
		t.Fatal(err)
	}

	c, _, err := newStore().Load(link)

	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(root)
	if c.Root != want {
		t.Fatalf("root %q, want %q", c.Root, want)
	}
}

func TestLocate_should_fill_position_for_path_diagnostic(t *testing.T) {
	root := copyTree(t, "basic")
	s := newStore()
	c, _, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	d := diag.New(diag.CodeRefUnknownRequest, diag.Location{File: c.Flows[0].File, Path: "$.nodes[1].request.id"}, nil)

	got := s.Locate(d)

	if got.Pos.Line != 30 || got.Pos.Column != 17 {
		t.Fatalf("got %+v", got.Pos)
	}
}

func TestReloadFile_should_replace_only_that_entity(t *testing.T) {
	root := copyTree(t, "basic")
	s := newStore()
	c, _, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	login := c.Requests[0].File
	writeFile(t, login, "id: 01J9ZA3M4N5P6Q7R8S9T0VWXYZ\nname: Sign in\nversions:\n  \"1\":\n    method: POST\n    url: /x\n")

	updated, diags, err := s.ReloadFile(c, login)

	if err != nil || len(diags) != 0 {
		t.Fatalf("err %v diags %v", err, codesOf(diags))
	}
	if updated.Requests[0].Name != "Sign in" || updated.Requests[1].Name != "Form page 1" {
		t.Fatalf("got %q, %q", updated.Requests[0].Name, updated.Requests[1].Name)
	}
	if c.Requests[0].Name != "Login" {
		t.Fatal("ReloadFile mutated its input")
	}
}

func TestReloadFile_should_drop_deleted_and_add_new_files(t *testing.T) {
	root := copyTree(t, "basic")
	s := newStore()
	c, _, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(c.Requests[1].File); err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(root, "flows", "other.arazzo.yaml")
	writeFile(t, added, arazzoFlow("id: 01J9ZG3M4N5P6Q7R8S9T0VWXYZ\nname: Other\n"))

	afterRemove, _, err := s.ReloadFile(c, c.Requests[1].File)
	if err != nil {
		t.Fatal(err)
	}
	afterAdd, _, err := s.ReloadFile(afterRemove, added)

	if err != nil {
		t.Fatal(err)
	}
	if len(afterAdd.Requests) != 1 || len(afterAdd.Flows) != 2 {
		t.Fatalf("requests %d flows %d", len(afterAdd.Requests), len(afterAdd.Flows))
	}
}

func TestLoad_should_report_unreadable_entries_and_keep_loading(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks and permission bits differ on Windows")
	}
	root := copyTree(t, "basic")
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, "requests", "auth", "dangling.req.yaml")); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(root, "flows", "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	c, diags, err := newStore().Load(root)

	if err != nil {
		t.Fatal(err)
	}
	if len(c.Requests) != 2 || len(c.Flows) != 1 {
		t.Fatalf("requests %d flows %d", len(c.Requests), len(c.Flows))
	}
	unreadable := 0
	for _, d := range diags {
		if d.Code == diag.CodeFileUnreadable {
			unreadable++
		}
	}
	if os.Geteuid() != 0 && unreadable != 2 {
		t.Fatalf("got %v", codesOf(diags))
	}
}

func TestLoad_should_ignore_editor_lock_and_backup_files(t *testing.T) {
	root := copyTree(t, "basic")
	writeFile(t, filepath.Join(root, "requests", "auth", ".#login.req.yaml"), "lock")
	writeFile(t, filepath.Join(root, "requests", "auth", "login.req.yaml~"), "backup")
	writeFile(t, filepath.Join(root, "flows", "#signup.arazzo.yaml#"), "autosave")

	_, diags, err := newStore().Load(root)

	if err != nil || len(diags) != 0 {
		t.Fatalf("err %v diags %v", err, codesOf(diags))
	}
}

func TestLoad_should_not_create_entities_for_empty_or_broken_files(t *testing.T) {
	root := copyTree(t, "basic")
	writeFile(t, filepath.Join(root, "requests", "empty.req.yaml"), "")
	writeFile(t, filepath.Join(root, "requests", "comment.req.yaml"), "# todo\n")
	writeFile(t, filepath.Join(root, "flows", "broken.arazzo.yaml"), "nodes: [\n")
	writeFile(t, filepath.Join(root, "environments", "list.yaml"), "- a\n- b\n")

	c, diags, err := newStore().Load(root)

	if err != nil {
		t.Fatal(err)
	}
	if len(c.Requests) != 2 || len(c.Flows) != 1 || len(c.Environments) != 1 {
		t.Fatalf("requests %d flows %d envs %d", len(c.Requests), len(c.Flows), len(c.Environments))
	}
	if len(diags) != 4 {
		t.Fatalf("got %v", codesOf(diags))
	}
}

func TestReloadFile_should_drop_entities_under_removed_directory(t *testing.T) {
	root := copyTree(t, "basic")
	s := newStore()
	c, _, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(c.Root, "requests", "auth")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	got, _, err := s.ReloadFile(c, dir)

	if err != nil || len(got.Requests) != 1 || got.Requests[0].Path != "onboarding/form-page-1" {
		t.Fatalf("err %v requests %+v", err, got.Requests)
	}
}

func TestLoad_should_warn_on_a_blueprints_file_without_the_blueprint_suffix(t *testing.T) {
	root := copyTree(t, "basic")
	writeFile(t, filepath.Join(root, "blueprints", "notes.yaml"), "a: 1\n")

	c, diags, err := newStore().Load(root)

	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].Code != diag.CodeFileUnrecognised || len(c.Blueprints) != 1 {
		t.Fatalf("diags %+v blueprints %d", diags, len(c.Blueprints))
	}
}

func TestReloadFile_should_replace_add_and_drop_blueprints(t *testing.T) {
	root := copyTree(t, "basic")
	s := newStore()
	c, _, err := s.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	existing := c.Blueprints[0].File
	writeFile(t, existing, "id: 01J9ZH3M4N5P6Q7R8S9T0VWXYZ\nname: Renamed\n")
	added := filepath.Join(root, "blueprints", "team", "second.blueprint.yaml")
	writeFile(t, added, "id: 01J9ZJ3M4N5P6Q7R8S9T0VWXYZ\nname: Second\n")

	renamed, _, err := s.ReloadFile(c, existing)
	if err != nil {
		t.Fatal(err)
	}
	both, _, err := s.ReloadFile(renamed, added)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(existing); err != nil {
		t.Fatal(err)
	}
	onlySecond, _, err := s.ReloadFile(both, existing)

	if err != nil {
		t.Fatal(err)
	}
	if renamed.Blueprints[0].Name != "Renamed" || c.Blueprints[0].Name != "Standard onboarding" {
		t.Fatalf("renamed %+v original %+v", renamed.Blueprints, c.Blueprints)
	}
	if len(both.Blueprints) != 2 || both.Blueprints[1].Path != "team/second" {
		t.Fatalf("both %+v", both.Blueprints)
	}
	if len(onlySecond.Blueprints) != 1 || onlySecond.Blueprints[0].Name != "Second" {
		t.Fatalf("onlySecond %+v", onlySecond.Blueprints)
	}
}

// Behaviour change: flows are Arazzo documents now. An old *.flow.yaml is not
// read (the owner has no flows to migrate); it is reported as an unrecognised file.
func TestLoad_should_report_an_old_flow_yaml_as_unrecognised_and_not_load_it(t *testing.T) {
	root := copyTree(t, "basic")
	writeFile(t, filepath.Join(root, "flows", "old.flow.yaml"), "id: 01J9ZG3M4N5P6Q7R8S9T0VWXYZ\nname: Old\n")

	c, diags, err := newStore().Load(root)

	if err != nil || len(c.Flows) != 1 || !slices.Contains(codesOf(diags), diag.CodeFileUnrecognised) {
		t.Fatalf("flows %d err %v codes %v", len(c.Flows), err, codesOf(diags))
	}
}
