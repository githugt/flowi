// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"strings"
	"testing"
)

// Before: the About screen showed a version and nothing else a bug report needs.
func TestDiagnosticsText_should_name_the_build_system_workspace_and_counts_only(t *testing.T) {
	d := diagnosticsInfo{Version: "v1.2.3", OS: "linux", Arch: "amd64", Go: "go1.27", Workspace: "Shop", Folder: "/home/a/shop",
		Collections: 2, Environments: 3, Selected: "staging"}

	got := diagnosticsText(d)

	for _, want := range []string{"Flowi v1.2.3", "linux/amd64, go1.27", "Shop (/home/a/shop)", "Collections: 2", "Environments: 3, selected: staging"} {
		if !strings.Contains(got, want) {
			t.Errorf("report misses %q:\n%s", want, got)
		}
	}
}

func TestDiagnosticsText_should_say_none_and_unknown_instead_of_leaving_blanks(t *testing.T) {
	got := diagnosticsText(diagnosticsInfo{Version: "dev", OS: "linux", Arch: "arm64", Go: "go1.27", Workspace: "W"})

	if !strings.Contains(got, "selected: none") || !strings.Contains(got, "W (unknown)") {
		t.Fatalf("report:\n%s", got)
	}
}

func TestSystemService_Diagnostics_should_report_the_open_workspace(t *testing.T) {
	app, _, _ := twoCollections(t)

	got := (&SystemService{app: app}).Diagnostics()

	if !strings.HasPrefix(got, "Flowi ") || !strings.Contains(got, "Collections: 2") {
		t.Fatalf("report:\n%s", got)
	}
}
