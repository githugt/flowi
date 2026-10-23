// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscoverPaths_should_prefer_FLOWI_CONFIG_DIR(t *testing.T) {
	dir := t.TempDir()

	p, err := DiscoverPaths([]string{"FLOWI_CONFIG_DIR=" + dir})

	if err != nil {
		t.Fatal(err)
	}
	if p.ConfigDir != dir {
		t.Fatalf("got %q, want %q", p.ConfigDir, dir)
	}
}

func TestDiscoverPaths_should_default_to_user_config_dir(t *testing.T) {
	p, err := DiscoverPaths(nil)

	if err != nil {
		t.Skipf("no user config dir on this machine: %v", err)
	}
	if filepath.Base(p.ConfigDir) != "flowi" {
		t.Fatalf("got %q", p.ConfigDir)
	}
}

func TestEditor(t *testing.T) {
	fallback := "vi"
	if runtime.GOOS == "windows" {
		fallback = "notepad"
	}
	tests := []struct {
		name    string
		environ []string
		want    string
	}{
		{name: "visual_first", environ: []string{"EDITOR=nano", "VISUAL=code -w"}, want: "code -w"},
		{name: "editor_second", environ: []string{"EDITOR=nano"}, want: "nano"},
		{name: "empty_values_ignored", environ: []string{"VISUAL=", "EDITOR="}, want: fallback},
		{name: "fallback", environ: nil, want: fallback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Editor(tt.environ)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
