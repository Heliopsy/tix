// SPDX-License-Identifier: AGPL-3.0-or-later

package capability_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/docsmd"
)

// The page and the section these assertions read. Scoping to the section
// matters: the same digits appear elsewhere on the page as a terminal size and
// as an event bound, and a sweep of the whole file would accept either in
// place of the gap count.
const (
	tuiPage    = "docs/tui.md"
	gapSection = "## What the terminal does not do"
)

// tuiFigures are the counts the section may state in digits, against what the
// registry actually holds. The page hard-coded them beside a test that
// hard-codes the same numbers, which is a second copy of a constant; this
// makes the page derive from the registry instead.
func tuiFigures() map[int]string {
	gaps := 0
	exemptions := 0
	for _, a := range capability.Exemptions() {
		if a.Surface != capability.SurfaceTUI {
			continue
		}
		exemptions++
		if a.Gap {
			gaps++
		}
	}
	return map[int]string{
		gaps:                         "terminal gaps",
		len(capability.Operations()): "operations in the registry",
		exemptions - gaps:            "terminal absences that are not gaps",
	}
}

// gapProse returns the section with its code fences removed. A fence is a
// command to run, not a claim about a count.
func gapProse(t *testing.T) string {
	t.Helper()
	md, err := docsmd.Read(tuiPage)
	if err != nil {
		t.Fatalf("reading the page under test: %v", err)
	}
	section, err := docsmd.Section(md, gapSection)
	if err != nil {
		t.Fatalf("%s: %v", tuiPage, err)
	}
	parts := strings.Split(section, "```")
	var prose []string
	for i, part := range parts {
		if i%2 == 0 {
			prose = append(prose, part)
		}
	}
	return strings.Join(prose, "\n")
}

// figureList renders the derivable counts for a failure message.
func figureList(figures map[int]string) string {
	var out []string
	for n, what := range figures {
		out = append(out, strconv.Itoa(n)+" "+what)
	}
	return strings.Join(out, ", ")
}

var digits = regexp.MustCompile(`\b\d+\b`)

// TestDocsGapFigureMatchesTheRegistry holds every number the section states in
// digits against the registry. The count has been 59, 58, 53, 44 and 40, and
// the page named the old one each time.
func TestDocsGapFigureMatchesTheRegistry(t *testing.T) {
	t.Parallel()
	figures := tuiFigures()
	for _, found := range digits.FindAllString(gapProse(t), -1) {
		n, err := strconv.Atoi(found)
		if err != nil {
			continue
		}
		if _, ok := figures[n]; !ok {
			t.Errorf("%s: %q names %d, which is not a figure the registry holds; it holds %s",
				tuiPage, gapSection, n, figureList(figures))
		}
	}
}

// TestDocsGapSectionStatesTheCounts is the other direction. A section that
// stops naming the figure passes the check above by saying nothing, so the two
// counts the paragraph is about are required to appear.
func TestDocsGapSectionStatesTheCounts(t *testing.T) {
	t.Parallel()
	prose := gapProse(t)
	for n, what := range tuiFigures() {
		if what == "terminal absences that are not gaps" {
			continue
		}
		if !strings.Contains(prose, strconv.Itoa(n)) {
			t.Errorf("%s: %q never states the %s, which is %d; state it or drop this assertion",
				tuiPage, gapSection, what, n)
		}
	}
}

// words spells the counts the section writes out rather than in digits.
var words = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"thirty-two": 32, "thirty-one": 31, "thirty-three": 33,
}

// spelled patterns are the section's written-out counts, each with what it is
// counting. They are checked only where the sentence is still there, so a
// rewrite that drops a breakdown is a prose edit and not a failure.
var spelled = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"the administration share of the gaps", regexp.MustCompile(`(?is)That\s+is\s+([a-z-]+)\s+of\s+the\s+count`)},
	{"the bulk and data movement share of the gaps", regexp.MustCompile(`(?is)That\s+is\s+the\s+other\s+([a-z-]+)`)},
}

// TestDocsGapBreakdownAddsUp checks the two shares against the gap count. The
// shares are a judgement about which gaps belong together, so what is asserted
// is that they still account for every gap and not how they were grouped.
func TestDocsGapBreakdownAddsUp(t *testing.T) {
	t.Parallel()
	prose := gapProse(t)
	gaps := 0
	for _, a := range capability.Gaps() {
		if a.Surface == capability.SurfaceTUI {
			gaps++
		}
	}
	sum, stated := 0, 0
	for _, s := range spelled {
		m := s.pattern.FindStringSubmatch(prose)
		if m == nil {
			continue
		}
		n, ok := words[strings.ToLower(m[1])]
		if !ok {
			t.Errorf("%s: %q is written as %q, which this test cannot read as a number",
				tuiPage, s.name, m[1])
			continue
		}
		sum += n
		stated++
	}
	if stated == 0 {
		return
	}
	if stated != len(spelled) {
		t.Fatalf("%s: %q states one share of the gaps and not the other, so the breakdown accounts for %d of %d",
			tuiPage, gapSection, sum, gaps)
	}
	if sum != gaps {
		t.Errorf("%s: the shares in %q add up to %d but the registry holds %d terminal gaps; "+
			"the breakdown has lost or gained one", tuiPage, gapSection, sum, gaps)
	}
}
