// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/url"
	"strings"
	"testing"
)

// A project page's header is the name and one muted line: the workflow (a
// link to its editor), the summary and the field definitions. They were
// three lines, the summary said "across 1 project" of a project's own page,
// and an (i) explained the page beside its own name.
func TestTheProjectHeaderIsTheNameAndOneLine(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "header line")

	page := b.page("/projects/infra")
	sub := between(t, page, `<div class="meta projhead-meta pagehead-sub">`, `<div class="pagehead-tools">`)
	for _, want := range []string{`href="/workflows/`, `<div id="tasks-summary">`, `class="lede summary"`,
		"to go", `href="/projects/infra/fields"`} {
		if !strings.Contains(sub, want) {
			t.Errorf("the line under the project's name has no %q:\n%s", want, sub)
		}
	}
	if strings.Contains(sub, "across 1 project") {
		t.Errorf("a project's own page still counts the projects it spans:\n%s", sub)
	}
	if head := between(t, page, `<div class="headtip"><h1>`, "</div>"); strings.Contains(head, "field-info") {
		t.Errorf("the project's name still carries an (i):\n%s", head)
	}
	if strings.Count(page, `id="tasks-summary"`) != 1 {
		t.Error("the summary is drawn more than once, or not at all")
	}
	if tasks := b.page("/tasks"); !strings.Contains(tasks, "across 1 project") {
		t.Error("the task list stopped saying how many projects it spans")
	}
}

// A board of one project draws no workflow line of its own and no (i) on a
// line by itself; a board of several keeps the line, with its (i) on it.
func TestOnlyASharedBoardNamesItsWorkflow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "one board")

	one := b.page("/projects/infra?view=board")
	if !strings.Contains(one, `class="board`) {
		t.Fatal("the project page did not draw a board")
	}
	if strings.Contains(one, "boardmeta") || strings.Contains(one, `aria-label="About Board"`) {
		t.Errorf("a one-project board still repeats its workflow:\n%s", between(t, one, `<div id="tasks-listing">`, `<div class="board`))
	}

	made := b.post("/projects", url.Values{"key": {"second"}, "name": {"Second"}})
	_ = made.Body.Close()
	b.createTask("second", "another board")
	both := b.page("/tasks?view=board")
	meta := between(t, both, `<div class="headtip boardmeta">`, `<div class="board`)
	if !strings.Contains(meta, "sharing it") || !strings.Contains(meta, `aria-label="About Board"`) {
		t.Errorf("a shared board does not name its workflow with its (i) on the same line:\n%s", meta)
	}
}

// Quick add and the filter toolbar are one row's two halves, and what either
// draws under itself comes after that row rather than inside it.
func TestQuickAddAndTheToolbarShareARow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "row")

	for _, path := range []string{"/tasks", "/projects/infra", "/projects/infra?view=lanes&q=tag:x"} {
		page := b.page(path)
		row := between(t, page, `<div class="taskbar"><div class="taskbar-row">`, `<div id="tasks-chips">`)
		quick := strings.Index(row, `class="quickadd"`)
		bar := strings.Index(row, `data-filter-toolbar`)
		if quick < 0 || bar < 0 || quick > bar {
			t.Errorf("%s: quick add and the toolbar are not one row, quick add first", path)
		}
		for _, under := range []string{`id="quickadd-preview"`, `class="fbchips"`} {
			if strings.Contains(row[:strings.LastIndex(row, `</div></div>`)], under) {
				t.Errorf("%s: %s is drawn inside the row", path, under)
			}
		}
	}

	sheet := body(t, b.get("/assets/app.css"))
	if grid := ruleIn(t, sheet, "@container taskbar (min-width: 84rem)", ".taskbar-row"); !strings.Contains(grid, "grid-template-columns") {
		t.Errorf("a wide column does not put quick add and the toolbar side by side: %s", grid)
	}
	if one := ruleIn(t, sheet, "@container taskbar (min-width: 84rem)", ".taskbar-row > .tbar"); !strings.Contains(one, "flex-wrap: nowrap") {
		t.Errorf("the toolbar may wrap under quick add on a wide column: %s", one)
	}
}

// A board card is the star and title on one line and a small line of marks
// led by the reference. Move is a corner button that a mouse sees on hover or
// focus and a touch screen always, and still a plain form inside a fold.
func TestABoardCardIsTwoLinesWithMoveInItsCorner(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "compact card")

	page := b.page("/projects/infra?view=board")
	card := between(t, page, `<article id="t-`+ref+`"`, "</article>")
	top := between(t, card, `<div class="card-top">`, `<div class="card-meta">`)
	if !strings.Contains(top, `class="starform"`) || !strings.Contains(top, `class="card-title"`) {
		t.Errorf("the star and the title are not on one line:\n%s", top)
	}
	if meta := between(t, card, `<div class="card-meta">`, "</div>"); !strings.Contains(meta, `<span class="ref">`) {
		t.Errorf("the reference does not lead the card's small line:\n%s", meta)
	}
	move := between(t, card, `<details class="card-move">`, "</details>")
	for _, want := range []string{`aria-label="Move `, `action="/tasks/` + ref + `/transition"`, `name="route"`} {
		if !strings.Contains(move, want) {
			t.Errorf("the card's Move menu lost %q:\n%s", want, move)
		}
	}

	sheet := body(t, b.get("/assets/app.css"))
	if corner := ruleIn(t, sheet, "", "details.card-move > summary"); !strings.Contains(corner, "position: absolute") {
		t.Errorf("Move is not a button in the card's corner: %s", corner)
	}
	hidden := ruleIn(t, sheet, "@media (hover: hover) and (pointer: fine)",
		".column .card:not(:hover, :focus-within) details.card-move:not([open]) > summary")
	if !strings.Contains(hidden, "opacity: 0") || strings.Contains(hidden, "display: none") || strings.Contains(hidden, "visibility") {
		t.Errorf("Move is hidden from the keyboard, or not hidden at rest with a mouse: %s", hidden)
	}
	if touch := ruleIn(t, sheet, "@media (max-width: 899px), (pointer: coarse)", "details.card-move > summary"); !strings.Contains(touch, "min-height: 44px") {
		t.Errorf("Move is not a 44px target on a touch screen: %s", touch)
	}
}
