// SPDX-License-Identifier: AGPL-3.0-or-later

package preflight_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/internal/preflight"
)

func TestParseOSRelease_should_name_the_family_of_the_system(t *testing.T) {
	tests := []struct {
		name string
		text string
		want preflight.Family
	}{
		{"ubuntu", "NAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\n", preflight.FamilyDebian},
		{"debian", "ID=debian\n", preflight.FamilyDebian},
		{"arch", "ID=arch\n", preflight.FamilyArch},
		{"cachyos is arch by ID_LIKE", "NAME=\"CachyOS Linux\"\nID=cachyos\nID_LIKE=arch\n", preflight.FamilyArch},
		{"fedora", "ID=fedora\n", preflight.FamilyFedora},
		{"rocky is fedora by ID_LIKE", "ID=\"rocky\"\nID_LIKE=\"rhel centos fedora\"\n", preflight.FamilyFedora},
		{"unknown id falls back to the first known ID_LIKE word", "ID=mydistro\nID_LIKE=\"foo ubuntu debian\"\n", preflight.FamilyDebian},
		{"quoted values and comments", "# a comment\nID='arch'\n", preflight.FamilyArch},
		{"nothing known", "ID=nixos\n", preflight.FamilyOther},
		{"empty file", "", preflight.FamilyOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preflight.ParseOSRelease(tt.text)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstallCommand_should_name_the_packages_of_the_family(t *testing.T) {
	tests := []struct {
		family preflight.Family
		want   string
	}{
		{preflight.FamilyArch, "sudo pacman -S gtk4 webkitgtk-6.0 libsoup3 bubblewrap xdg-dbus-proxy"},
		{preflight.FamilyDebian, "sudo apt install libgtk-4-1 libwebkitgtk-6.0-4 libsoup-3.0-0 bubblewrap xdg-dbus-proxy"},
		{preflight.FamilyFedora, "sudo dnf install gtk4 webkitgtk6.0 libsoup3 bubblewrap xdg-dbus-proxy"},
		{preflight.FamilyOther, "WebKitGTK 6.0"},
	}
	for _, tt := range tests {
		t.Run(string(tt.family), func(t *testing.T) {
			got := preflight.InstallCommand(tt.family)

			if !strings.Contains(got, tt.want) {
				t.Fatalf("got %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

// fakeSystem is a machine with the files, programs and os-release it is given.
type fakeSystem struct {
	files     map[string]bool
	programs  map[string]bool
	osRelease string
}

func (f fakeSystem) FileExists(path string) bool { return f.files[path] }
func (f fakeSystem) OnPath(program string) bool  { return f.programs[program] }
func (f fakeSystem) ReadFile(path string) string {
	if path == "/etc/os-release" {
		return f.osRelease
	}
	return ""
}

// Glob matches the patterns against the fake files, as filepath.Match does for a directory.
func (f fakeSystem) Glob(pattern string) []string {
	var dirs []string
	for file := range f.files {
		dir := filepath.Dir(file)
		if ok, _ := filepath.Match(pattern, dir); ok {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func readySystem() fakeSystem {
	return fakeSystem{
		files: map[string]bool{
			"/usr/lib/webkitgtk-6.0/WebKitNetworkProcess": true,
			"/usr/lib/webkitgtk-6.0/WebKitWebProcess":     true,
		},
		programs:  map[string]bool{"bwrap": true, "xdg-dbus-proxy": true},
		osRelease: "ID=arch\n",
	}
}

func TestCheck_Given_everything_installed_Then_nothing_is_missing(t *testing.T) {
	report := preflight.Check(readySystem())

	if !report.OK() {
		t.Fatalf("missing %v", report.Missing)
	}
}

func TestCheck_Given_the_helpers_in_a_multiarch_directory_Then_they_are_found(t *testing.T) {
	sys := readySystem()
	sys.files = map[string]bool{
		"/usr/lib/x86_64-linux-gnu/webkitgtk-6.0/WebKitNetworkProcess": true,
		"/usr/lib/x86_64-linux-gnu/webkitgtk-6.0/WebKitWebProcess":     true,
	}

	report := preflight.Check(sys)

	if !report.OK() {
		t.Fatalf("missing %v", report.Missing)
	}
}

func TestCheck_Given_no_helpers_and_no_sandbox_tools_Then_each_is_reported(t *testing.T) {
	sys := fakeSystem{osRelease: "ID=cachyos\nID_LIKE=arch\n"}

	report := preflight.Check(sys)

	got := strings.Join(report.Missing, "|")
	for _, want := range []string{"WebKitNetworkProcess", "WebKitWebProcess", "bwrap", "xdg-dbus-proxy"} {
		if !strings.Contains(got, want) {
			t.Errorf("%s is not reported: %v", want, report.Missing)
		}
	}
	if report.Family != preflight.FamilyArch {
		t.Errorf("family %q, want arch", report.Family)
	}
}

func TestCheck_Given_only_bubblewrap_missing_Then_only_it_is_reported(t *testing.T) {
	sys := readySystem()
	sys.programs = map[string]bool{"xdg-dbus-proxy": true}

	report := preflight.Check(sys)

	if len(report.Missing) != 1 || report.Missing[0] != "bwrap" {
		t.Fatalf("missing %v", report.Missing)
	}
}

func TestReport_Message_should_say_what_is_missing_and_the_command_for_the_distribution(t *testing.T) {
	report := preflight.Check(fakeSystem{osRelease: "ID=fedora\n"})

	msg := report.Message()

	if !strings.Contains(msg, "bwrap") || !strings.Contains(msg, "sudo dnf install gtk4 webkitgtk6.0") {
		t.Fatalf("message:\n%s", msg)
	}
}
