// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestTheStatisticsBreakdownNamesAndColoursEveryCategory reads the rows the
// statistics screen renders, not the page. Each of the six categories has to
// reach the table under its own word and its own class, because the class is
// how the browser gives a category the colour the terminal gives it, and the
// word is what a reader has when the colour is gone.
func TestTheStatisticsBreakdownNamesAndColoursEveryCategory(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/stats")

	for _, c := range core.StateCategories() {
		row := categoryCell(t, page, c)
		if !strings.Contains(row, categoryWord(c)) {
			t.Errorf("the %q row does not name it %q: %s", c, categoryWord(c), row)
		}
		if !strings.Contains(row, categoryCSSClass(c)) {
			t.Errorf("the %q row carries no category class: %s", c, row)
		}
	}
	// A row that named every category the same word, or gave them one class,
	// would satisfy the loop above while telling a reader nothing.
	words := map[string]bool{}
	for _, c := range core.StateCategories() {
		words[categoryWord(c)] = true
	}
	if len(words) != len(core.StateCategories()) {
		t.Fatalf("the six categories are named by %d distinct words", len(words))
	}
}

// categoryCell returns the one table cell the breakdown renders for a
// category, found by the title attribute that carries the raw value. Reading
// the cell rather than the page is the point: the words "Done" and "Blocked"
// appear all over a statistics screen, so a page-wide search would pass with
// the breakdown missing entirely.
func categoryCell(t *testing.T, page string, c core.StateCategory) string {
	t.Helper()
	marker := `<td title="` + string(c) + `">`
	at := strings.Index(page, marker)
	if at < 0 {
		t.Fatalf("the statistics breakdown has no row for category %q", c)
	}
	end := strings.Index(page[at:], "</td>")
	if end < 0 {
		t.Fatalf("the %q cell never closes", c)
	}
	return page[at : at+end+len("</td>")]
}

// TestNoTwoCategoriesShareAColourInTheBrowser is the browser half of the
// collision guard the terminal carries. Six categories are worth having only
// if a reader can tell them apart, and the pair that mattered is cancelled
// against done: drawn alike, abandoned work reads as finished work.
func TestNoTwoCategoriesShareAColourInTheBrowser(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	sheet := body(t, b.get("/assets/app.css"))

	seen := map[string]core.StateCategory{}
	for _, c := range core.StateCategories() {
		rule := declarations(t, sheet, "."+categoryCSSClass(c)+" {")
		colour := colourDeclaration(t, rule, c)
		if other, clash := seen[colour]; clash {
			t.Errorf("categories %q and %q are both %s", other, c, colour)
		}
		seen[colour] = c
	}
	if len(seen) != len(core.StateCategories()) {
		t.Fatalf("%d categories resolve to %d colours", len(core.StateCategories()), len(seen))
	}
	// The hue "waiting" uses is the one the semantic tokens do not carry, so
	// it is declared as its own token. A token declared nowhere resolves to
	// nothing and the cell renders in the ordinary text colour, which is
	// indistinguishable from a rule that was never written.
	if !strings.Contains(sheet, "--cat-waiting:") {
		t.Error("--cat-waiting is used but never declared, so the waiting row has no colour")
	}
}

// colourDeclaration returns the colour a category rule sets, failing when the
// rule sets none: a rule present but silent about colour would make the
// collision check above pass by comparing two empty strings.
func colourDeclaration(t *testing.T, rule string, c core.StateCategory) string {
	t.Helper()
	at := strings.Index(rule, "color:")
	if at < 0 {
		t.Fatalf("the %q rule sets no colour: %s", c, rule)
	}
	end := strings.Index(rule[at:], ";")
	if end < 0 {
		t.Fatalf("the %q rule's colour declaration never ends: %s", c, rule)
	}
	return strings.TrimSpace(rule[at+len("color:") : at+end])
}

// categoryWord is the word the breakdown labels a category with. It is
// written here rather than read from the package under test, so the labels
// this screen shows are asserted against something independent of the map
// that produces them.
func categoryWord(c core.StateCategory) string {
	switch c {
	case core.CategoryTodo:
		return "To do"
	case core.CategoryInProgress:
		return "In progress"
	case core.CategoryBlocked:
		return "Blocked"
	case core.CategoryWaiting:
		return "Waiting"
	case core.CategoryDone:
		return "Done"
	case core.CategoryCancelled:
		return "Cancelled"
	}
	return ""
}

// categoryCSSClass derives the class a category's cell carries the same way
// the renderer's slug does: the token with everything but its letters and
// digits removed.
func categoryCSSClass(c core.StateCategory) string {
	return "cat-" + strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, string(c))
}
