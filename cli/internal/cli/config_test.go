// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/samber/do/v2"

	"github.com/githugt/flowi/lib/settings"
)

func TestPlainValue(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{10 * time.Minute, "10m"},
		{time.Hour, "1h"},
		{90 * time.Minute, "1h30m"},
		{[]string{"a.pem", "b.pem"}, "a.pem,b.pem"},
		{int64(10), "10"},
		{true, "true"},
		{nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := plainValue(tt.in); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisplayValue_should_quote_empty(t *testing.T) {
	if got := displayValue(""); got != `""` {
		t.Fatalf("got %q", got)
	}
}

func TestLayerFile_Given_a_config_directory_that_does_not_exist_When_the_user_file_is_wanted_Then_it_is_created(t *testing.T) {
	// Before: the file was created without its directory, so `config edit` failed on a fresh machine.
	dir := filepath.Join(t.TempDir(), "not", "yet", "there")
	i := do.New()
	do.ProvideValue(i, Env{})
	do.ProvideValue(i, &GlobalOptions{ConfigDir: dir})

	file, err := layerFile(i, nil, settings.LayerUser)
	if err != nil {
		t.Fatalf("layerFile: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("the settings file was not created: %v", err)
	}
	if filepath.Dir(file) != dir {
		t.Fatalf("file %q is not in %q", file, dir)
	}
}
