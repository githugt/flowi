// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteData_Given_append_mode_When_written_twice_Then_the_file_grows_and_replace_mode_overwrites(t *testing.T) {
	file := filepath.Join(t.TempDir(), "deep", "dir", "log.txt")

	for _, chunk := range []string{"one\n", "two\n"} {
		if err := writeData(file, []byte(chunk), true); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	if got, _ := os.ReadFile(file); string(got) != "one\ntwo\n" {
		t.Fatalf("after appends: %q", got)
	}

	if err := writeData(file, []byte("fresh"), false); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got, _ := os.ReadFile(file); string(got) != "fresh" {
		t.Fatalf("after replace: %q", got)
	}
}

func TestSecretTracker_Given_derived_values_When_masked_Then_strings_inside_maps_and_lists_are_hidden_but_numbers_are_not(t *testing.T) {
	tr := &secretTracker{}
	tr.derived(map[string]any{"basic": "dXNlcjpwdw==", "list": []any{"tok-123", 8}})

	got := tr.String("auth dXNlcjpwdw== and tok-123 with 8 items")
	if strings.Contains(got, "dXNlcjpwdw==") || strings.Contains(got, "tok-123") {
		t.Fatalf("derived secrets leaked: %q", got)
	}
	if got := tr.String("8"); got != "8" {
		t.Fatalf("a number must not be registered as a secret, got %q", got)
	}
}

// Before: saved data and responses in the data directory were created 0644, readable by other users of the machine.
func TestWriteData_should_keep_files_in_the_data_directory_private(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix file modes")
	}
	dir := t.TempDir()
	replaced := filepath.Join(dir, "replaced.txt")
	appended := filepath.Join(dir, "appended.txt")

	if err := writeData(replaced, []byte("x"), false); err != nil {
		t.Fatal(err)
	}
	if err := writeData(appended, []byte("x"), true); err != nil {
		t.Fatal(err)
	}

	for _, file := range []string{replaced, appended} {
		info, err := os.Stat(file)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("%s: mode %v, err %v", filepath.Base(file), info.Mode().Perm(), err)
		}
	}
}
