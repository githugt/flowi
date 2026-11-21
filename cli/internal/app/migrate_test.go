// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrate_Given_current_collections_Then_check_passes(t *testing.T) {
	e := newSendEnv(t)

	code, out, errOut := e.run(context.Background(), "migrate", "--check")

	if code != 0 || !strings.Contains(out, "format 1, current") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestMigrate_Given_a_collection_from_a_newer_format_Then_check_fails_and_names_it(t *testing.T) {
	e := newSendEnv(t)
	manifest := filepath.Join(e.dir, "tools", "flowi.yaml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, manifest, strings.Replace(string(data), "format: 1", "format: 9", 1))

	plain, out, _ := e.run(context.Background(), "migrate")
	code, _, _ := e.run(context.Background(), "migrate", "--check")

	if plain != 0 || code != 1 || !strings.Contains(out, "Tools: format 9, too new") {
		t.Fatalf("exits %d and %d\n%s", plain, code, out)
	}
}
