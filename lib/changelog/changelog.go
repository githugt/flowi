// SPDX-License-Identifier: Apache-2.0

// Package changelog reads a CHANGELOG.md in the Keep a Changelog layout
// that git-cliff produces from Conventional Commits, so the applications can
// show "what's new" from the same file the release page uses.
package changelog

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const unreleased = "Unreleased"

// Section is a group of entries under a "###" heading ("Features",
// "Bug Fixes"). Title is empty for entries that come before any heading.
type Section struct {
	Title   string
	Entries []string
}

// Release is one "##" block.
type Release struct {
	Version    string // "1.2.0" or "Unreleased"
	Date       time.Time
	Unreleased bool
	Sections   []Section
}

var (
	releaseHeading = regexp.MustCompile(`^##\s+\[?([^\]\s]+)\]?(?:\s+-\s+(\S+))?\s*$`)
	// commitLink is the "([abc1234](https://…))" git-cliff appends to an entry.
	commitLink = regexp.MustCompile(`\s*\(\[[0-9a-f]{4,40}\]\([^)]*\)\)\s*$`)
)

// Parse reads the releases of a changelog, newest first as written.
// Anything before the first release heading is ignored.
func Parse(text string) []Release {
	var releases []Release
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t\r")
		switch {
		case strings.HasPrefix(line, "## "):
			releases = append(releases, newRelease(line))
		case len(releases) == 0:
			// preamble
		case strings.HasPrefix(line, "### "):
			cur := &releases[len(releases)-1]
			cur.Sections = append(cur.Sections, Section{Title: strings.TrimSpace(strings.TrimPrefix(line, "### "))})
		case strings.HasPrefix(line, "- "):
			addEntry(&releases[len(releases)-1], strings.TrimSpace(strings.TrimPrefix(line, "- ")))
		case strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "":
			continueEntry(&releases[len(releases)-1], strings.TrimSpace(line))
		}
	}
	return releases
}

func newRelease(line string) Release {
	m := releaseHeading.FindStringSubmatch(line)
	if m == nil {
		return Release{Version: strings.TrimSpace(strings.TrimPrefix(line, "## "))}
	}
	r := Release{Version: m[1], Unreleased: strings.EqualFold(m[1], unreleased)}
	if r.Unreleased {
		r.Version = unreleased
	}
	if d, err := time.Parse("2006-01-02", m[2]); err == nil {
		r.Date = d
	}
	return r
}

// currentSection returns the release's open section, starting an untitled
// one for entries that precede any heading.
func currentSection(r *Release) *Section {
	if len(r.Sections) == 0 {
		r.Sections = append(r.Sections, Section{})
	}
	return &r.Sections[len(r.Sections)-1]
}

func addEntry(r *Release, entry string) {
	s := currentSection(r)
	s.Entries = append(s.Entries, commitLink.ReplaceAllString(entry, ""))
}

// continueEntry appends an indented line to the previous entry.
func continueEntry(r *Release, line string) {
	s := currentSection(r)
	if len(s.Entries) == 0 {
		return
	}
	last := len(s.Entries) - 1
	s.Entries[last] = commitLink.ReplaceAllString(s.Entries[last]+" "+line, "")
}

// Latest returns the n newest releases. Released versions come first;
// "Unreleased" work is shown only when nothing has been released.
func Latest(rs []Release, n int) []Release {
	var released []Release
	for _, r := range rs {
		if !r.Unreleased {
			released = append(released, r)
		}
	}
	if len(released) == 0 {
		released = rs
	}
	return released[:min(n, len(released))]
}

// Since returns the releases written before version in the file, i.e. the
// newer ones, including unreleased work. ok is false when version is not in
// the changelog.
func Since(rs []Release, version string) ([]Release, bool) {
	for i, r := range rs {
		if r.Version == version {
			return rs[:i], true
		}
	}
	return nil, false
}

func (r Release) heading() string {
	if r.Date.IsZero() {
		return r.Version
	}
	return fmt.Sprintf("%s (%s)", r.Version, r.Date.Format("2006-01-02"))
}

// Markdown renders the release for the GUI.
func (r Release) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n", r.heading())
	for _, s := range r.Sections {
		if s.Title != "" {
			fmt.Fprintf(&b, "\n### %s\n", s.Title)
		}
		b.WriteString("\n")
		for _, e := range s.Entries {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}
	return b.String()
}

// Text renders the release for a terminal: markdown emphasis is removed.
func (r Release) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", r.heading())
	for _, s := range r.Sections {
		if s.Title != "" {
			fmt.Fprintf(&b, "  %s\n", s.Title)
		}
		for _, e := range s.Entries {
			fmt.Fprintf(&b, "    • %s\n", plain(e))
		}
	}
	return b.String()
}

// plain drops the ** emphasis git-cliff puts around scopes.
func plain(s string) string { return strings.ReplaceAll(s, "**", "") }
