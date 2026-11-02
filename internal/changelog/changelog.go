// SPDX-License-Identifier: AGPL-3.0-or-later

// Package changelog reads the release-please changelog this binary was built
// with.
package changelog

import (
	"regexp"
	"strings"
	"sync"
)

// Source is CHANGELOG.md, assigned by package main from its embedded copy.
// An embed directive here cannot reach the repository root, and package main
// is the only one that can, so it hands the text over the way the linker
// hands over version.Version. Empty in tests and in any binary built without it.
var Source string

// Entry is one release in the changelog.
type Entry struct {
	// Version is the release's major.minor.patch, without a leading v.
	Version string
	// Date is the release date as written, YYYY-MM-DD, or empty.
	Date string
	// Body is the Markdown under the release heading, links untouched.
	Body string
}

// heading matches a release heading as release-please writes it, linked or
// not: "## [0.31.0](compare-url) (2026-11-02)" or "## 0.1.0 (2026-09-23)".
var heading = regexp.MustCompile(`^#{2,3}\s+\[?v?([0-9]+\.[0-9]+\.[0-9]+[0-9A-Za-z.+-]*)\]?(?:\([^)]*\))?(?:\s+\(([0-9]{4}-[0-9]{2}-[0-9]{2})\))?\s*$`)

// Parse splits a changelog into its entries, newest first as written. Text
// before the first release heading is dropped.
func Parse(src string) []Entry {
	var out []Entry
	var body []string
	flush := func() {
		if len(out) > 0 {
			out[len(out)-1].Body = strings.TrimSpace(strings.Join(body, "\n"))
		}
		body = body[:0]
	}
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		if m := heading.FindStringSubmatch(line); m != nil {
			flush()
			out = append(out, Entry{Version: m[1], Date: m[2]})
			continue
		}
		body = append(body, line)
	}
	flush()
	return out
}

var (
	once    sync.Once
	entries []Entry
)

// Entries is Source parsed, once.
func Entries() []Entry {
	once.Do(func() { entries = Parse(Source) })
	return entries
}
