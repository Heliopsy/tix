// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// githubAnchor is the anchor GitHub gives a heading: lower case, punctuation
// other than hyphens and underscores dropped, spaces turned into hyphens.
func githubAnchor(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// docAnchors are the anchors of every heading in a docs/ page, numbered for a
// repeated heading the way GitHub numbers them. Lines in fenced code blocks
// are not headings, however they start.
func docAnchors(t *testing.T, page string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", page))
	if err != nil {
		t.Fatalf("reading docs/%s: %v", page, err)
	}
	out, seen, fenced := map[string]bool{}, map[string]int{}, false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		text := strings.TrimLeft(line, "#")
		if fenced || text == line || !strings.HasPrefix(text, " ") || len(line)-len(text) > 6 {
			continue
		}
		a := githubAnchor(strings.TrimSpace(text))
		if n := seen[a]; n > 0 {
			out[a+"-"+strconv.Itoa(n)] = true
		} else {
			out[a] = true
		}
		seen[a]++
	}
	return out
}

func TestGithubAnchorMatchesGitHub(t *testing.T) {
	cases := map[string]string{
		"The task list": "the-task-list",
		"My work, saved filters and recently viewed": "my-work-saved-filters-and-recently-viewed",
		"Negation: a leading `-`":                    "negation-a-leading--",
		"Changing a project's key":                   "changing-a-projects-key",
		"tix claim exec":                             "tix-claim-exec",
	}
	for in, want := range cases {
		if got := githubAnchor(in); got != want {
			t.Errorf("githubAnchor(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestEveryHelpLinkLandsOnAHeading fails on a help link to a page docs/ does
// not hold or to a heading the page does not have, which GitHub would open at
// the top with no word of the screen the reader asked about.
func TestEveryHelpLinkLandsOnAHeading(t *testing.T) {
	anchors := map[string]map[string]bool{}
	for route, d := range helpDocs {
		if !slices.Contains(docPages, d.Page) {
			t.Errorf("%s: help opens %s, which the docs redirect does not serve", route, d.Page)
			continue
		}
		if anchors[d.Page] == nil {
			anchors[d.Page] = docAnchors(t, d.Page)
		}
		if !anchors[d.Page][d.Section] {
			t.Errorf("%s: docs/%s has no heading with the anchor #%s", route, d.Page, d.Section)
		}
		if !docsSection.MatchString(d.Section) {
			t.Errorf("%s: #%s is not an anchor the redirect passes on", route, d.Section)
		}
	}
}

// TestHelpNamesOnlyScreens fails on a help entry for a route that draws no
// screen, which is a route renamed or removed with its entry left behind.
func TestHelpNamesOnlyScreens(t *testing.T) {
	screens := map[string]bool{}
	for _, b := range Bindings() {
		if b.Method == http.MethodGet && b.Template != "" {
			screens[b.Pattern] = true
		}
	}
	for route := range helpDocs {
		if !screens[route] {
			t.Errorf("help maps %s, which is not a screen", route)
		}
	}
}
