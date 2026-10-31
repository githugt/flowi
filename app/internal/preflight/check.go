// SPDX-License-Identifier: AGPL-3.0-or-later

package preflight

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// System is what the check asks of the machine; tests give it a fake.
type System interface {
	// FileExists reports whether a file (not necessarily a regular one) exists at the absolute path.
	FileExists(path string) bool
	// Glob lists the paths matching a pattern.
	Glob(pattern string) []string
	// OnPath reports whether a program is found on the PATH.
	OnPath(program string) bool
	// ReadFile returns a file's text, or "" when it cannot be read.
	ReadFile(path string) string
}

// helperPrograms are the programs WebKitGTK starts for the network and the web content. A distribution puts them in
// its own directory; a library built elsewhere looks in the wrong one, which is the usual way this check fails.
func helperPrograms() []string { return []string{"WebKitNetworkProcess", "WebKitWebProcess"} }

// helperDirPatterns are where distributions install those helpers.
func helperDirPatterns() []string {
	return []string{
		"/usr/lib/webkitgtk-6.0",
		"/usr/lib64/webkitgtk-6.0",
		"/usr/libexec/webkitgtk-6.0",
		"/usr/lib/*/webkitgtk-6.0",
	}
}

// sandboxTools are what WebKit runs the web content under.
func sandboxTools() []string { return []string{"bwrap", "xdg-dbus-proxy"} }

// Report says what is missing; it is empty when the system is ready.
type Report struct {
	Missing []string
	Family  Family
}

// OK reports whether nothing is missing.
func (r Report) OK() bool { return len(r.Missing) == 0 }

// Message is what to tell the user: what is missing, and how to install it.
func (r Report) Message() string {
	var b strings.Builder
	b.WriteString("Flowi cannot start its window: this system is missing\n")
	for _, m := range r.Missing {
		fmt.Fprintf(&b, "  - %s\n", m)
	}
	fmt.Fprintf(&b, "Install them with:\n  %s\n", InstallCommand(r.Family))
	return b.String()
}

// Check looks for the helpers and the sandbox tools on the system.
func Check(sys System) Report {
	report := Report{Family: ParseOSRelease(sys.ReadFile("/etc/os-release"))}
	for _, program := range helperPrograms() {
		if !helperFound(sys, program) {
			report.Missing = append(report.Missing, "WebKitGTK's helper "+program)
		}
	}
	for _, tool := range sandboxTools() {
		if !sys.OnPath(tool) {
			report.Missing = append(report.Missing, tool)
		}
	}
	return report
}

// helperFound looks in each helper directory of the system. The paths are Linux paths, always written with "/", so this
// uses package path and not filepath, which would use "\\" on Windows.
func helperFound(sys System, program string) bool {
	for _, pattern := range helperDirPatterns() {
		for _, dir := range sys.Glob(pattern) {
			if sys.FileExists(path.Join(dir, program)) {
				return true
			}
		}
	}
	return false
}

// Run checks the real system and, when something is missing, writes the message to w and returns false. On systems
// other than Linux there is nothing to check.
func Run(w io.Writer) bool {
	if runtime.GOOS != "linux" {
		return true
	}
	report := Check(hostSystem{})
	if report.OK() {
		return true
	}
	_, _ = io.WriteString(w, report.Message())
	return false
}

// hostSystem is the machine the program runs on.
type hostSystem struct{}

func (hostSystem) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (hostSystem) Glob(pattern string) []string {
	matches, _ := filepath.Glob(pattern) // the only error is a malformed pattern, and these are fixed
	return matches
}

func (hostSystem) OnPath(program string) bool {
	_, err := exec.LookPath(program)
	return err == nil
}

func (hostSystem) ReadFile(path string) string {
	data, err := os.ReadFile(path) // a system without os-release is treated as an unknown distribution
	if err != nil {
		return ""
	}
	return string(data)
}
