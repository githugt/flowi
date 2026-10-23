// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/settings"
)

func TestUserSettings_should_layer_file_env_and_flags_without_creating_a_workspace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte("logging:\n  level: debug\n  format: json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags := settings.LayerValues{Layer: settings.LayerFlag, Source: "--log-level", Values: map[string]any{"logging.level": "warn"}}

	r, diags, err := UserSettings(settings.Paths{ConfigDir: dir}, []string{"FLOWI_LOGGING_FORMAT=text"}, flags)

	if err != nil || len(diags) != 0 {
		t.Fatalf("err %v diags %v", err, diags)
	}
	level, _ := r.Get("logging.level")
	format, _ := r.Get("logging.format")
	if level.Value != "warn" || format.Value != "text" {
		t.Fatalf("level %+v format %+v", level, format)
	}
	if _, err := os.Stat(filepath.Join(dir, "workspaces")); !os.IsNotExist(err) {
		t.Fatal("UserSettings created a workspace")
	}
}

func TestUserSettings_should_report_invalid_values(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte("logging:\n  level: loud\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, diags, err := UserSettings(settings.Paths{ConfigDir: dir}, nil, settings.LayerValues{})

	if err != nil || len(diags) != 1 || diags[0].Code != diag.CodeSettingInvalid || diags[0].Pos.Line != 2 {
		t.Fatalf("err %v diags %+v", err, diags)
	}
}

func TestUserSettingsFile_should_live_in_config_dir(t *testing.T) {
	got := UserSettingsFile(settings.Paths{ConfigDir: "/cfg"})

	if got != filepath.Join("/cfg", "settings.yaml") {
		t.Fatalf("got %q", got)
	}
}
