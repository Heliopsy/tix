// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// registeredFlag matches the long name in one line of pflag's usage listing.
var registeredFlag = regexp.MustCompile(`(?m)^\s+(?:-\w, )?(--[a-z0-9-]+)`)

// docsFlagRow matches one row of the global flag table in docs/configuration.md.
var docsFlagRow = regexp.MustCompile("(?m)^\\| `(-[^`]*)` \\|")

// TestDocumentedGlobalFlagsMatchTheRealOnes ties the global flag table in
// docs/configuration.md to the flags the root command actually registers.
// --allow-network-fs shipped without a row, which nothing noticed.
func TestDocumentedGlobalFlagsMatchTheRealOnes(t *testing.T) {
	path := filepath.Join("..", "docs", "configuration.md")
	body, err := os.ReadFile(path) // #nosec G304 -- a fixed path inside the repository
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	documented := map[string]bool{}
	for _, row := range docsFlagRow.FindAllStringSubmatch(globalFlagSection(t, string(body)), -1) {
		for _, name := range splitFlagCell(row[1]) {
			documented[name] = true
		}
	}
	if len(documented) == 0 {
		t.Fatalf("%s has no global flag rows; the parser or the table changed shape", path)
	}

	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	real := map[string]bool{}
	for _, name := range registeredFlag.FindAllStringSubmatch(root.PersistentFlags().FlagUsages(), -1) {
		real[name[1]] = true
		if !documented[name[1]] {
			t.Errorf("flag %s is registered but not in the table in %s", name[1], path)
		}
	}
	for name := range documented {
		if len(name) == 2 && name[0] == '-' {
			continue
		}
		if !real[name] {
			t.Errorf("%s documents %s, which the root command does not register", path, name)
		}
	}
}

// globalFlagSection returns the body of the "Global flags" heading alone, so
// rows from a command's own flag table are not read as global ones.
func globalFlagSection(t *testing.T, body string) string {
	t.Helper()
	const heading = "\n## Global flags\n"
	at := strings.Index(body, heading)
	if at < 0 {
		t.Fatalf("docs/configuration.md has no %q heading", "## Global flags")
	}
	rest := body[at+len(heading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end]
	}
	return rest
}

// splitFlagCell reads the flag names out of a cell like "-o, --output".
func splitFlagCell(cell string) []string {
	var out []string
	for _, part := range regexp.MustCompile(`,\s*`).Split(cell, -1) {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
