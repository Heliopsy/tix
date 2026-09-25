// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// cssComments matches a comment, which may carry braces and would otherwise
// be counted as block structure.
var cssComments = regexp.MustCompile(`(?s)/\*.*?\*/`)

// enclosingAtRule names the at-rule a byte offset sits inside, or "" when the
// offset is at the top level. A guard about which arrangement a rule decorates
// has to read the rule's own context rather than trust the order the sheet
// happens to declare things in.
func enclosingAtRule(sheet string, idx int) string {
	depth, start, prelude := 0, 0, ""
	for i := 0; i < idx && i < len(sheet); i++ {
		switch sheet[i] {
		case '{':
			if depth == 0 {
				prelude = strings.TrimSpace(sheet[start:i])
			}
			depth++
			start = i + 1
		case '}':
			depth--
			if depth <= 0 {
				depth, prelude = 0, ""
			}
			start = i + 1
		case ';':
			start = i + 1
		}
	}
	if depth == 0 || !strings.HasPrefix(prelude, "@") {
		return ""
	}
	return prelude
}

// The rule drawn between the panel's two sections has to follow the
// arrangement they are actually in. It was a left border unset only under a
// width breakpoint, while the track count was the browser's to choose, so a
// section that wrapped onto its own row at any other width kept a left border
// separating nothing.
func TestTheViewPanelSeparatorFollowsItsArrangement(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := cssComments.ReplaceAllString(body(t, f.as("alice").get("/assets/app.css")), "")

	const selector = ".viewgrid section + section"
	var stacked, sideBySide int
	for at := 0; ; {
		i := strings.Index(sheet[at:], selector)
		if i < 0 {
			break
		}
		i += at
		at = i + len(selector)
		block, _, _ := strings.Cut(sheet[at:], "}")
		media := enclosingAtRule(sheet, i)
		switch {
		case media == "":
			stacked++
			if !strings.Contains(block, "border-top: 1px") {
				t.Errorf("stacked, the sections are separated by nothing:\n%s", block)
			}
			if strings.Contains(block, "border-left: 1px") {
				t.Errorf("a left border is drawn where the sections are stacked:\n%s", block)
			}
		case strings.Contains(media, "min-width"):
			sideBySide++
			if !strings.Contains(block, "border-left: 1px") {
				t.Errorf("side by side, the sections are separated by nothing:\n%s", block)
			}
			if !strings.Contains(block, "border-top: 0") {
				t.Errorf("the stacked rule is still drawn side by side:\n%s", block)
			}
		default:
			t.Errorf("%q decorates the panel under %q, which cannot know how many\n"+
				"tracks the grid has; the arrangement has to be the sheet's own choice",
				selector, media)
		}
	}
	if stacked != 1 || sideBySide != 1 {
		t.Fatalf("found %d stacked and %d side-by-side separator rules, want one of each",
			stacked, sideBySide)
	}
}

// The grid picks its own track count at each width rather than letting the
// browser pick one, because the separator above has to know which it is.
func TestTheViewPanelChoosesItsOwnTrackCount(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := cssComments.ReplaceAllString(body(t, f.as("alice").get("/assets/app.css")), "")

	const selector = ".viewgrid {"
	var found int
	for at := 0; ; {
		i := strings.Index(sheet[at:], selector)
		if i < 0 {
			break
		}
		i += at
		at = i + len(selector)
		block, _, _ := strings.Cut(sheet[at:], "}")
		if !strings.Contains(block, "grid-template-columns") {
			continue
		}
		found++
		if strings.Contains(block, "auto-fit") || strings.Contains(block, "auto-fill") {
			t.Errorf("the panel lets the browser choose how many tracks it has, so the\n"+
				"rule between its sections cannot follow the arrangement:\n%s", block)
		}
	}
	if found < 2 {
		t.Fatalf("found %d .viewgrid templates, want one per arrangement", found)
	}
}

// Two submits in one panel both reading "Apply" say nothing about which is
// which. Each names the choice it commits.
func TestEachViewChoiceNamesWhatItCommits(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	panel := between(t, f.as("alice").page("/tasks"), `class="body viewgrid"`, "</details>")

	for _, want := range []string{">Apply columns<", ">Apply projects<"} {
		if !strings.Contains(panel, want) {
			t.Errorf("the view panel has no submit reading %q:\n%s", want, panel)
		}
	}
	if strings.Contains(panel, ">Apply<") {
		t.Errorf("a submit in the view panel still says only \"Apply\":\n%s", panel)
	}
}
