// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"

	"github.com/githugt/flowi/lib/internal/migrate"
)

// Migration says where one mounted collection stands against the file format this release writes.
type Migration struct {
	Collection string
	Root       string
	Format     int      // the format its flowi.yaml declares
	Current    int      // the format this release reads and writes
	MinVersion string   // the release its flowi.yaml says it needs, "" for any
	Steps      []string // what would upgrade it, one line per step, oldest first; empty when it is current or newer
	// State is "current" (nothing to do), "outdated" (older format: Steps or Problem say what is needed), "too new"
	// (written by a newer release than this one) or "needs newer release" (its min_version is later than this release).
	State   string
	Problem string // why an outdated collection cannot be upgraded, when it cannot
}

// Migrations reports, for every mounted collection, whether its files are in the format this release writes. It
// reads what is already loaded and changes nothing.
func (w *Workspace) Migrations() []Migration {
	var out []Migration
	for _, c := range w.Collections() {
		m := Migration{Collection: c.Manifest.Name, Root: c.Root, Format: c.Manifest.Format, Current: migrate.CurrentFormat, MinVersion: c.Manifest.MinVersion, State: "current"}
		switch {
		case migrate.CheckVersion(c.Manifest.MinVersion, w.opts.Version).Code != "":
			m.State = "needs newer release"
		case c.Manifest.Format > migrate.CurrentFormat:
			m.State = "too new"
		case c.Manifest.Format < migrate.CurrentFormat:
			m.State = "outdated"
			steps, err := migrate.Plan(c.Manifest.Format)
			if err != nil {
				m.Problem = err.Error()
			}
			for _, s := range steps {
				m.Steps = append(m.Steps, fmt.Sprintf("format %d to %d: %s", s.From, s.From+1, s.Describe))
			}
		}
		out = append(out, m)
	}
	return out
}
