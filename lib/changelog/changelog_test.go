// SPDX-License-Identifier: Apache-2.0

package changelog

import (
	"strings"
	"testing"
	"time"
)

func sample() string {
	return `# Changelog

All notable changes to this project are documented here.

## [Unreleased]

### Features

- **serve:** hot reload of schedules

## [1.1.0] - 2026-10-01

### Features

- **replay:** answer in sequence ([a1b2c3d](https://example.com/commit/a1b2c3d))
- **otel:** export traces
  with a second line

### Bug Fixes

- keep large numbers exact

## [1.0.0] - 2026-09-01

### Features

- first release
`
}

func TestParse_should_read_releases_sections_and_entries_newest_first(t *testing.T) {
	rs := Parse(sample())

	if len(rs) != 3 {
		t.Fatalf("%d releases: %+v", len(rs), rs)
	}
	if !rs[0].Unreleased || rs[0].Version != "Unreleased" || len(rs[0].Sections) != 1 {
		t.Fatalf("first %+v", rs[0])
	}
	r := rs[1]
	if r.Version != "1.1.0" || r.Unreleased || !r.Date.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("second %+v", r)
	}
	if len(r.Sections) != 2 || r.Sections[0].Title != "Features" || r.Sections[1].Title != "Bug Fixes" {
		t.Fatalf("sections %+v", r.Sections)
	}
	if got := r.Sections[0].Entries; len(got) != 2 || got[1] != "**otel:** export traces with a second line" {
		t.Fatalf("entries %q", got)
	}
}

func TestParse_should_drop_commit_links_from_entries(t *testing.T) {
	rs := Parse(sample())

	if got := rs[1].Sections[0].Entries[0]; got != "**replay:** answer in sequence" {
		t.Fatalf("entry %q", got)
	}
}

func TestParse_Given_text_without_releases_Then_none(t *testing.T) {
	for _, in := range []string{"", "# Changelog\n\nNothing yet.\n", "no headings at all"} {
		if rs := Parse(in); len(rs) != 0 {
			t.Errorf("Parse(%q) = %+v", in, rs)
		}
	}
}

func TestParse_should_tolerate_odd_headings(t *testing.T) {
	rs := Parse("## 2.0.0\n\n- bare entry before any section\n\n## [1.9.0] - not-a-date\n\n### Fixed\n- x\n")

	if len(rs) != 2 || rs[0].Version != "2.0.0" || rs[1].Version != "1.9.0" || !rs[1].Date.IsZero() {
		t.Fatalf("releases %+v", rs)
	}
	if len(rs[0].Sections) != 1 || rs[0].Sections[0].Title != "" || rs[0].Sections[0].Entries[0] != "bare entry before any section" {
		t.Fatalf("sections %+v", rs[0].Sections)
	}
}

func TestLatest_should_prefer_released_versions(t *testing.T) {
	rs := Parse(sample())

	one := Latest(rs, 1)
	two := Latest(rs, 2)
	onlyUnreleased := Latest(rs[:1], 1)

	if len(one) != 1 || one[0].Version != "1.1.0" || len(two) != 2 || two[1].Version != "1.0.0" {
		t.Fatalf("one %+v two %+v", one, two)
	}
	if len(onlyUnreleased) != 1 || !onlyUnreleased[0].Unreleased {
		t.Fatalf("a changelog with only unreleased work still has something to show: %+v", onlyUnreleased)
	}
}

func TestSince_should_return_releases_newer_than_a_version(t *testing.T) {
	rs := Parse(sample())

	got, ok := Since(rs, "1.0.0")
	_, unknown := Since(rs, "9.9.9")

	if !ok || len(got) != 2 || got[0].Version != "Unreleased" || got[1].Version != "1.1.0" {
		t.Fatalf("got %+v", got)
	}
	if unknown {
		t.Fatal("an unknown version has no 'since'")
	}
}

func TestRelease_Markdown_and_Text_should_render_a_release(t *testing.T) {
	r := Parse(sample())[1]

	md, text := r.Markdown(), r.Text()

	if !strings.HasPrefix(md, "## 1.1.0 (2026-10-01)\n") || !strings.Contains(md, "### Bug Fixes\n\n- keep large numbers exact") {
		t.Fatalf("markdown:\n%s", md)
	}
	if !strings.HasPrefix(text, "1.1.0 (2026-10-01)\n") || !strings.Contains(text, "  Features\n    • replay: answer in sequence\n") {
		t.Fatalf("text:\n%s", text)
	}
}

func FuzzParse(f *testing.F) {
	f.Add(sample())
	f.Add("## [\n### \n- \n")
	f.Fuzz(func(_ *testing.T, s string) {
		for _, r := range Parse(s) {
			_ = r.Markdown()
			_ = r.Text()
		}
	})
}
