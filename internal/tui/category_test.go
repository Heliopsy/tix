// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// categoryWorkflow puts one state in each of the six categories, so a board
// drawn from it exercises the whole vocabulary rather than the three states
// the shipped workflow happens to use.
func categoryWorkflow() *core.WorkflowDefinition {
	return &core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "doing", Label: "Doing", Category: core.CategoryInProgress},
			{Key: "stuck", Label: "Blocked", Category: core.CategoryBlocked},
			{Key: "held", Label: "Waiting", Category: core.CategoryWaiting},
			{Key: "done", Label: "Done", Terminal: true, Category: core.CategoryDone},
			{Key: "dropped", Label: "Dropped", Terminal: true, Category: core.CategoryCancelled},
		},
	}
}

// categoryModel is a board drawn in colour, since the styling is what is under
// test and a monochrome model renders every style the same.
func categoryModel(t *testing.T) Model {
	t.Helper()
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Color: boolPtr(true)})
	m.width, m.height = 160, 40
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: categoryWorkflow(),
		tasks: []core.Task{
			task("1", "todo", 1, core.PriorityNormal),
			task("2", "doing", 2, core.PriorityNormal),
			task("3", "stuck", 3, core.PriorityNormal),
			task("4", "held", 4, core.PriorityNormal),
			task("5", "done", 5, core.PriorityNormal),
			task("6", "dropped", 6, core.PriorityNormal),
		},
	})
	return m
}

// TestEveryCategoryIsDrawnInItsOwnColour is the collision guard the widened
// vocabulary needs. Six categories are only worth having if a reader can tell
// them apart, and two categories sharing a hue is the failure this change was
// made to fix: cancelled drawn in the green of done said the abandoned work
// was finished.
//
// The assertion is on the theme rather than on a frame, so it names the pair
// that collided instead of reporting that some board somewhere looks wrong.
func TestEveryCategoryIsDrawnInItsOwnColour(t *testing.T) {
	t.Parallel()
	m := categoryModel(t)
	rendered := map[string][]core.StateCategory{}
	for _, c := range core.StateCategories() {
		if _, ok := CategoryColor(c); !ok {
			t.Fatalf("category %q carries no colour, so the board draws it plainly", c)
		}
		key := m.theme.Category(c).Render("x")
		rendered[key] = append(rendered[key], c)
	}
	for _, sharing := range rendered {
		if len(sharing) > 1 {
			t.Errorf("categories %v are drawn identically", sharing)
		}
	}
	if len(rendered) != len(core.StateCategories()) {
		t.Fatalf("%d categories draw %d distinct styles", len(core.StateCategories()), len(rendered))
	}
	// A category this build does not know still draws plainly, which is the
	// rule that keeps a guessed colour off a user-defined workflow.
	if m.theme.Category("invented").Render("x") != m.theme.Style().Render("x") {
		t.Error("an unknown category was given a colour")
	}
}

// TestCancelledIsNotDrawnLikeDoneOnTheBoard is the substantive pair, asserted
// through the thing that renders it rather than by looking for a colour. The
// column heading and the card bar are both built from Theme.Category, so a
// heading written with a hard-wired escape, or one that read the state key
// instead of its category, fails here.
func TestCancelledIsNotDrawnLikeDoneOnTheBoard(t *testing.T) {
	t.Parallel()
	m := categoryModel(t)
	columns := map[core.StateCategory]Column{}
	for _, col := range m.columns {
		columns[col.Category] = col
	}
	for _, c := range core.StateCategories() {
		if _, ok := columns[c]; !ok {
			t.Fatalf("the board has no column in category %q", c)
		}
	}

	heading := func(c core.StateCategory) string { return m.columnHeading(columns[c], 40) }
	want := func(c core.StateCategory) string {
		return m.quiet(m.theme.Category(c).Bold(m.theme.Color)).
			Render(Truncate(ColumnHeadingText(columns[c]), 40))
	}
	for _, c := range core.StateCategories() {
		if heading(c) != want(c) {
			t.Errorf("the %q heading is not drawn by the theme's category style:\n got %q\nwant %q",
				c, heading(c), want(c))
		}
	}
	if heading(core.CategoryCancelled) == heading(core.CategoryDone) {
		t.Error("the cancelled column is drawn exactly like the done column")
	}
	if heading(core.CategoryBlocked) == heading(core.CategoryTodo) {
		t.Error("the blocked column is drawn exactly like the todo column")
	}

	// The card bar carries the same category, so the two halves of a column
	// cannot disagree about what a state is.
	bar := func(c core.StateCategory) string {
		return strings.Join(m.cardLines(columns[c].Tasks[0], c, 40, false), "\n")
	}
	if !strings.Contains(bar(core.CategoryCancelled),
		m.quiet(m.theme.Category(core.CategoryCancelled)).Render(CardBar(false))) {
		t.Errorf("a cancelled card's bar is not drawn in its category's style:\n%q",
			bar(core.CategoryCancelled))
	}
	if strings.Contains(bar(core.CategoryCancelled),
		m.quiet(m.theme.Category(core.CategoryDone)).Render(CardBar(false))) {
		t.Errorf("a cancelled card's bar is drawn in the done style:\n%q",
			bar(core.CategoryCancelled))
	}
}

// TestEveryCategoryDegradesOnAColourlessTerminal is the other half of "colour
// is never the only cue". The widened vocabulary must not make a board
// unreadable where no escape is written at all: every category renders plainly
// and the column keeps its label, which is the word a reader falls back on.
func TestEveryCategoryDegradesOnAColourlessTerminal(t *testing.T) {
	t.Parallel()
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 160, 40
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: categoryWorkflow(),
		tasks:    []core.Task{task("6", "dropped", 6, core.PriorityNormal)},
	})
	plain := m.theme.Style().Render("x")
	for _, c := range core.StateCategories() {
		if got := m.theme.Category(c).Render("x"); got != plain {
			t.Errorf("category %q writes an attribute on a colourless terminal: %q", c, got)
		}
	}
	for _, col := range m.columns {
		heading := m.columnHeading(col, 40)
		if strings.Contains(heading, "\x1b[") {
			t.Errorf("a colourless column heading carries an escape sequence: %q", heading)
		}
		if !strings.Contains(heading, col.Label) {
			t.Errorf("the %q column loses its label, which is all a colourless terminal has: %q",
				col.Category, heading)
		}
	}
}
