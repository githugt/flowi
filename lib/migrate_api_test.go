// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrations_Given_a_current_collection_Then_it_is_current(t *testing.T) {
	w := demoWorkspace(t)

	got := w.Migrations()

	if len(got) == 0 || got[0].State != "current" || got[0].Format != got[0].Current || len(got[0].Steps) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestMigrations_Given_a_collection_that_needs_a_newer_release_Then_it_says_so(t *testing.T) {
	// Before: nothing told a CI job that the collection needed a newer CLI, until it failed on an unrelated field.
	e := newEnv(t)
	e.opts.Version = "1.0.0"
	w := e.open(t)
	m, err := w.InitCollection(t.TempDir(), "later", true)
	if err != nil {
		t.Fatal(err)
	}
	var root string
	for _, c := range w.Collections() {
		if c.Manifest.ID == m.ID {
			root = c.Root
		}
	}
	file := filepath.Join(root, "flowi.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(data, "min_version: 99.0.0\n"...), 0o644); err != nil {
		t.Fatal(err)
	}

	_, reloadErr := w.Reload(Change{Root: root, Op: ChangeRescan})

	var got Migration
	for _, mg := range w.Migrations() {
		if mg.Root == root {
			got = mg
		}
	}
	if reloadErr != nil || got.State != "needs newer release" || got.MinVersion != "99.0.0" {
		t.Fatalf("err %v, got %+v", reloadErr, got)
	}
}
