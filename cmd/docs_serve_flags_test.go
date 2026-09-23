// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// docsServeFlagRow matches one row of a flag table in docs/deployment.md.
var docsServeFlagRow = regexp.MustCompile("(?m)^\\| `(--[^`]*)` \\|")

// serveFlagSections are the headings in docs/deployment.md whose tables
// together document `tix serve`. The SSH flags sit with the listener they turn
// on rather than in the main table, because that is where an operator reading
// about the listener is; the guard therefore reads both and treats their union
// as the documented set.
var serveFlagSections = []string{"\n## Flags\n", "\n### Both listeners in one process\n"}

// TestDocumentedServeFlagsMatchTheRealOnes ties the flag tables in
// docs/deployment.md to the flags `tix serve` actually registers, in both
// directions. --no-webhook-dispatcher shipped without a row, which nothing
// noticed, and the way it was found was reading `tix serve --help` rather than
// the documentation.
func TestDocumentedServeFlagsMatchTheRealOnes(t *testing.T) {
	path := filepath.Join("..", "docs", "deployment.md")
	body, err := os.ReadFile(path) // #nosec G304 -- a fixed path inside the repository
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	documented := map[string]bool{}
	for _, heading := range serveFlagSections {
		for _, row := range docsServeFlagRow.FindAllStringSubmatch(docSection(t, string(body), heading), -1) {
			for _, name := range splitFlagCell(row[1]) {
				documented[name] = true
			}
		}
	}
	if len(documented) == 0 {
		t.Fatalf("%s has no serve flag rows; the parser or the tables changed shape", path)
	}

	real := map[string]bool{}
	for _, name := range registeredFlag.FindAllStringSubmatch(serveCmd(t).Flags().FlagUsages(), -1) {
		real[name[1]] = true
		if !documented[name[1]] {
			t.Errorf("tix serve registers %s, which is in no flag table in %s", name[1], path)
		}
	}
	for name := range documented {
		if !real[name] {
			t.Errorf("%s documents %s, which tix serve does not register", path, name)
		}
	}
}

// serveCmd returns the serve command off a freshly built root, so the flags
// under test are the ones a person running the binary sees.
func serveCmd(t *testing.T) *cobra.Command {
	t.Helper()
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	for _, sub := range root.Commands() {
		if sub.Name() == "serve" {
			return sub
		}
	}
	t.Fatal("the root command has no serve subcommand")
	return nil
}

// docSection returns the body under one heading alone, so a row from another
// command's table is never read as this one's.
func docSection(t *testing.T, body, heading string) string {
	t.Helper()
	at := strings.Index(body, heading)
	if at < 0 {
		t.Fatalf("docs/deployment.md has no %q heading", strings.TrimSpace(heading))
	}
	rest := body[at+len(heading):]
	if end := regexp.MustCompile(`(?m)^#{2,3} `).FindStringIndex(rest); end != nil {
		return rest[:end[0]]
	}
	return rest
}
