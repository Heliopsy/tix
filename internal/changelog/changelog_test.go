// SPDX-License-Identifier: AGPL-3.0-or-later

package changelog

import (
	"strings"
	"testing"
)

// fixture is the shape release-please writes: a title, linked headings with a
// date, a blank pair under each, and an unlinked first release.
const fixture = `# Changelog

## [0.31.0](https://github.com/Heliopsy/tix/compare/v0.30.0...v0.31.0) (2026-11-02)


### Features

* **web:** redesign the release screen ([#149](https://github.com/Heliopsy/tix/issues/149))

## [0.30.0](https://github.com/Heliopsy/tix/compare/v0.29.1...v0.30.0) (2026-11-01)


### Bug Fixes

* **web:** fold sidebar groups

## 0.1.0 (2026-09-23)


### Features

* bootstrap tix
`

func TestParseSplitsReleasePleaseEntries(t *testing.T) {
	t.Parallel()
	got := Parse(fixture)
	want := []struct{ version, date, first, last string }{
		{"0.31.0", "2026-11-02", "### Features", "issues/149))"},
		{"0.30.0", "2026-11-01", "### Bug Fixes", "fold sidebar groups"},
		{"0.1.0", "2026-09-23", "### Features", "bootstrap tix"},
	}
	if len(got) != len(want) {
		t.Fatalf("Parse found %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		e := got[i]
		if e.Version != w.version || e.Date != w.date {
			t.Errorf("entry %d is %s (%s), want %s (%s)", i, e.Version, e.Date, w.version, w.date)
		}
		if !strings.HasPrefix(e.Body, w.first) || !strings.HasSuffix(e.Body, w.last) {
			t.Errorf("entry %s body runs %q, want it to open %q and close %q", e.Version, e.Body, w.first, w.last)
		}
		if strings.Contains(e.Body, "## [") || strings.Contains(e.Body, "# Changelog") {
			t.Errorf("entry %s carries another heading: %q", e.Version, e.Body)
		}
	}
}

func TestParseIgnoresSectionHeadings(t *testing.T) {
	t.Parallel()
	for _, src := range []string{"", "# Changelog\n\nnothing yet", "### Features\n\n* a"} {
		if got := Parse(src); len(got) != 0 {
			t.Errorf("Parse(%q) = %+v, want no entries", src, got)
		}
	}
}

// Entries parses Source, which package main assigns before anything reads it.
func TestEntriesReadsSource(t *testing.T) {
	Source = fixture
	if got := Entries(); len(got) != 3 || got[0].Version != "0.31.0" {
		t.Errorf("Entries() = %+v, want the fixture's three releases", got)
	}
}
