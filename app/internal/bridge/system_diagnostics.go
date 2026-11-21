// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/githugt/flowi/app/internal/buildinfo"
)

// diagnosticsInfo is what a bug report needs to know about this installation. It holds no secret, no environment
// value and no request: names and counts only.
type diagnosticsInfo struct {
	Version      string
	OS, Arch, Go string
	Workspace    string // its name
	Folder       string // where workspace.yaml lives; "" when unknown
	Collections  int
	Environments int
	Selected     string // the chosen environment's name; "" for none
}

// diagnosticsText writes the report as lines of "name: value", for pasting into an issue.
func diagnosticsText(d diagnosticsInfo) string {
	selected := d.Selected
	if selected == "" {
		selected = "none"
	}
	folder := d.Folder
	if folder == "" {
		folder = "unknown"
	}
	lines := []string{
		"Flowi " + d.Version,
		fmt.Sprintf("System: %s/%s, %s", d.OS, d.Arch, d.Go),
		fmt.Sprintf("Workspace: %s (%s)", d.Workspace, folder),
		fmt.Sprintf("Collections: %d", d.Collections),
		fmt.Sprintf("Environments: %d, selected: %s", d.Environments, selected),
	}
	return strings.Join(lines, "\n")
}

// Diagnostics is the text the About screen copies for a bug report: version, system, workspace folder and counts.
// It contains no secret and no values from any environment or request.
//
// Before: the About screen showed a version and nothing else a report needs.
func (s *SystemService) Diagnostics() string {
	var d diagnosticsInfo
	s.app.actor.Do(func() {
		t := s.app.tree.tree()
		d = diagnosticsInfo{Workspace: t.Name, Folder: t.Folder, Environments: len(t.Environments), Selected: t.Selected}
		for _, n := range t.Roots {
			if n.Kind == "collection" {
				d.Collections++
			}
		}
	})
	d.Version, d.OS, d.Arch, d.Go = buildinfo.Version(), runtime.GOOS, runtime.GOARCH, runtime.Version()
	return diagnosticsText(d)
}
