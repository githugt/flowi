// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/internal/ids"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

func testIDs() *ids.Source {
	return ids.NewSource(func() time.Time { return time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC) }, rand.NewChaCha8([32]byte{7}))
}

func codec() Codec { return yamlcodec.New(settings.Schema()) }

func identity(p string) (string, error) { return p, nil }

func TestLoadOrCreate_should_create_default_workspace_with_valid_id(t *testing.T) {
	p := settings.Paths{ConfigDir: t.TempDir()}

	ws, diags, err := LoadOrCreate(p, DefaultName, codec(), testIDs())

	if err != nil || len(diags) != 0 {
		t.Fatalf("err %v diags %v", err, diags)
	}
	if !ids.Valid(ws.ID) || ws.Name != "Default" {
		t.Fatalf("got %+v", ws)
	}
	if _, err := os.Stat(filepath.Join(Dir(p, DefaultName), "workspace.yaml")); err != nil {
		t.Fatalf("workspace.yaml not written: %v", err)
	}
}

func TestLoadOrCreate_should_load_existing(t *testing.T) {
	p := settings.Paths{ConfigDir: t.TempDir()}
	first, _, _ := LoadOrCreate(p, DefaultName, codec(), testIDs())

	second, diags, err := LoadOrCreate(p, DefaultName, codec(), testIDs())

	if err != nil || len(diags) != 0 || second.ID != first.ID {
		t.Fatalf("err %v diags %v id %q vs %q", err, diags, second.ID, first.ID)
	}
}

func TestSave_should_round_trip_mounts(t *testing.T) {
	p := settings.Paths{ConfigDir: t.TempDir()}
	ws, _, _ := LoadOrCreate(p, DefaultName, codec(), testIDs())
	ws, _ = Mount(ws, "01J9Z8K3M4N5P6Q7R8S9T0VWXY", "~/code/a b", identity)

	err := Save(p, codec(), ws)
	again, _, loadErr := LoadOrCreate(p, DefaultName, codec(), testIDs())

	if err != nil || loadErr != nil {
		t.Fatal(err, loadErr)
	}
	if len(again.Collections) != 1 || again.Collections[0].Path != filepath.FromSlash("~/code/a b") {
		t.Fatalf("got %+v", again.Collections)
	}
}

func TestMount_should_reject_same_id_or_same_resolved_path(t *testing.T) {
	// Paths are written with the platform's separators: Mount cleans them that way.
	at := filepath.FromSlash
	ws := model.Workspace{Collections: []model.Mount{{ID: "A", Path: at("/x/col")}}}
	resolve := func(p string) (string, error) {
		if p == at("/x/link") {
			return at("/x/col"), nil
		}
		return p, nil
	}
	tests := []struct {
		name string
		id   model.ID
		path string
	}{
		{"same_id", "A", at("/elsewhere")},
		{"same_resolved_path", "B", at("/x/link")},
		{"same_path_unclean", "C", at("/x/./col")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Mount(ws, tt.id, tt.path, resolve)

			if !errors.Is(err, ErrAlreadyMounted) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestMount_should_not_mutate_input(t *testing.T) {
	ws := model.Workspace{Collections: make([]model.Mount, 0, 4)}

	got, err := Mount(ws, "A", "/a", identity)

	if err != nil || len(got.Collections) != 1 || len(ws.Collections) != 0 {
		t.Fatalf("err %v got %d input %d", err, len(got.Collections), len(ws.Collections))
	}
	other, _ := Mount(ws, "B", "/b", identity)
	if got.Collections[0].ID != "A" || other.Collections[0].ID != "B" {
		t.Fatal("mounts share a backing array")
	}
}

func TestUnmount_should_error_when_not_mounted(t *testing.T) {
	_, err := Unmount(model.Workspace{}, "A")

	if !errors.Is(err, ErrNotMounted) {
		t.Fatalf("got %v", err)
	}
}

func TestRelocate_should_change_path(t *testing.T) {
	oldPath, newPath := filepath.FromSlash("/old"), filepath.FromSlash("/new")
	ws := model.Workspace{Collections: []model.Mount{{ID: "A", Path: oldPath}}}

	got, err := Relocate(ws, "A", newPath)

	if err != nil || got.Collections[0].Path != newPath || ws.Collections[0].Path != oldPath {
		t.Fatalf("err %v got %+v input %+v", err, got.Collections, ws.Collections)
	}
}

func TestExpandPath(t *testing.T) {
	home := filepath.FromSlash("/home/u")
	tests := []struct{ in, want string }{
		{"~", home},
		{"~/x", filepath.Join(home, "x")},
		{"/abs", "/abs"},
		{"rel", "rel"},
		{"a ~b", "a ~b"},
		{"~other/x", "~other/x"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := ExpandPath(tt.in, home)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckMounts_should_report_missing_and_keep_others(t *testing.T) {
	home := t.TempDir()
	spaced := filepath.Join(home, "my collections", "a")
	if err := os.MkdirAll(spaced, 0o755); err != nil {
		t.Fatal(err)
	}
	ws := model.Workspace{File: "workspace.yaml", Collections: []model.Mount{
		{ID: "A", Path: "~/my collections/a"},
		{ID: "B", Path: spaced},
		{ID: "C", Path: "~/gone"},
	}}

	got := CheckMounts(ws, home, DirExists)

	if len(got) != 1 || got[0].Code != diag.CodeWSMissing || got[0].Args["id"] != "C" || got[0].Path != "$.collections[2].path" {
		t.Fatalf("got %+v", got)
	}
}

func TestDataDir_should_nest_under_workspace(t *testing.T) {
	p := settings.Paths{ConfigDir: "/cfg"}

	got := DataDir(p, "default", "01J")

	if got != filepath.Join("/cfg", "workspaces", "default", "data", "01J") {
		t.Fatalf("got %q", got)
	}
}
