// SPDX-License-Identifier: Apache-2.0

// Package examples_test checks that the bundled example collections are
// valid and canonical, so they can serve as documentation and fixtures.
package examples_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/examples"
	"github.com/githugt/flowi/lib/settings"
	"github.com/githugt/flowi/lib/workspace"
)

func TestExamples_should_load_validate_and_format_without_changes(t *testing.T) {
	for _, name := range []string{"user-creation"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			paths := settings.Paths{ConfigDir: filepath.Join(home, "config")}
			// Environments are global: the example's become the workspace's.
			collection := installExampleEnvironments(t, name, workspace.EnvironmentsDir(paths, workspace.DefaultName))
			w, _, err := flowi.OpenWorkspace(context.Background(), flowi.WorkspaceOptions{Paths: paths, Home: home})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Mount(collection); err != nil {
				t.Fatal(err)
			}

			diags := w.Validate()
			changes, _, err := w.Format("")

			for _, d := range diags {
				msg, _ := diag.English().Message(d)
				t.Errorf("%s:%d:%d %s %s", d.File, d.Pos.Line, d.Pos.Column, d.Code, msg)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range changes {
				t.Errorf("not canonical: %s\n%s", c.File, c.New)
			}
		})
	}
}

func TestUserCreation_should_embed_the_collection_files(t *testing.T) {
	data, err := fs.ReadFile(examples.UserCreation(), "flowi.yaml")

	if err != nil || !strings.Contains(string(data), "01JA0000000000000000000001") {
		t.Fatalf("got %q, %v", data, err)
	}
	if _, err := fs.Stat(examples.UserCreation(), "requests/_folder.yaml"); err != nil {
		t.Fatalf("folder file not embedded: %v", err)
	}
}

// installExampleEnvironments copies the example to a temporary directory,
// moves its environments into envDir and returns the copy.
func installExampleEnvironments(t *testing.T, name, envDir string) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(name)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(envDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dst, "environments"), envDir); err != nil {
		t.Fatal(err)
	}
	return dst
}
