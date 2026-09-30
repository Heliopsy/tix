// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// columnBlockLines returns one column's own block, split into lines with its
// styling removed. Reading one column rather than the frame is what keeps an
// assertion about this column from passing on the column beside it.
func columnBlockLines(t *testing.T, m Model, index int) []string {
	t.Helper()
	layout := LayoutFor(m.width, m.height, len(m.columns))
	start, end := VisibleRange(len(m.columns), layout.VisibleColumns, m.sel.Col)
	if index < start || index >= end {
		t.Fatalf("column %d is not on screen; the board shows %d..%d", index, start, end)
	}
	widths := m.columnWidths(layout, start, end)
	layout.ColumnWidth = widths[index-start]
	return strings.Split(stripANSI(m.columnBlock(index, layout, false)), "\n")
}

// TestALongTitleWrapsToAnIndentedContinuation is the defect the card was
// redesigned for: a title that did not fit wrapped to an unindented, re-
// truncated fragment at column zero, spending two lines to show twelve
// characters. The continuation now starts in the same text column as the first
// line, and only the last line shown may carry an ellipsis.
func TestALongTitleWrapsToAnIndentedContinuation(t *testing.T) {
	tests := []struct {
		name  string
		title string
		width int
		want  []string
	}{
		{"a title that fits is one line", "rotate the keys", 20, []string{"rotate the keys"}},
		{
			name:  "a title that does not fit breaks on a word",
			title: "Default-deny network policies in the staging namespace",
			width: 24,
			want:  []string{"Default-deny network", "policies in the staging…"},
		},
		{
			name:  "exactly two lines is not truncated",
			title: "rotate the long lived deploy secrets",
			width: 20,
			want:  []string{"rotate the long", "lived deploy secrets"},
		},
		{
			name:  "a word longer than the line is broken at its edge",
			title: "supercalifragilistic expialidocious",
			width: 10,
			want:  []string{"supercalif", "ragilisti…"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WrapTitle(tc.title, tc.width, CardTitleLines)
			if len(got) != len(tc.want) {
				t.Fatalf("WrapTitle = %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("WrapTitle = %q, want %q", got, tc.want)
				}
			}
			for i, line := range got {
				if n := len([]rune(line)); n > tc.width {
					t.Fatalf("line %d is %d cells wide, want at most %d: %q", i, n, tc.width, line)
				}
				if i < len(got)-1 && strings.HasSuffix(line, "…") {
					t.Fatalf("line %d is not the last and still carries an ellipsis: %q", i, line)
				}
			}
		})
	}
}

// TestACardSpendsNoLineOnNothing is the second half: a card whose title fits on
// one line costs one line plus its meta, where every card used to cost two
// whether or not the second carried anything.
func TestACardSpendsNoLineOnNothing(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	short := core.Task{Ref: "infra-1", Title: "short", Priority: core.PriorityNormal}
	long := core.Task{Ref: "infra-2", Priority: core.PriorityNormal,
		Title: "a title far too long to sit on one line of a narrow column"}
	if got := CardOf(short, 24, now, "").Lines(); got != 2 {
		t.Fatalf("a one line title costs %d lines, want 2", got)
	}
	if got := CardOf(long, 24, now, "").Lines(); got != CardTitleLines+1 {
		t.Fatalf("a wrapped title costs %d lines, want %d", got, CardTitleLines+1)
	}
}

// TestTheCardMetaSeparatesThePriorityFromItsMarkers pins the reported defect:
// a card read "P2*", which is one token nothing in the interface explains.
func TestTheCardMetaSeparatesThePriorityFromItsMarkers(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	soon := now.Add(48 * time.Hour)
	past := now.Add(-48 * time.Hour)
	far := now.Add(90 * 24 * time.Hour)
	tests := []struct {
		name string
		task core.Task
		want string
	}{
		{"no markers", core.Task{Ref: "infra-1", Priority: core.PriorityNormal}, "infra-1 · P3"},
		// A due date draws a marker only while the deadline is pressing. The
		// marker removed before was drawn for any due date at all, which put it
		// on nearly every card in a real backlog and so distinguished nothing.
		{"a deadline this week", core.Task{Ref: "infra-1", Priority: core.PriorityHigh, DueAt: &soon},
			"infra-1 · P2 · due"},
		{"a deadline past", core.Task{Ref: "infra-1", Priority: core.PriorityHigh, DueAt: &past},
			"infra-1 · P2 · due!"},
		{"a deadline next quarter draws nothing", core.Task{Ref: "infra-1", Priority: core.PriorityHigh, DueAt: &far},
			"infra-1 · P2"},
		{"blocked, with a deadline this week", core.Task{Ref: "infra-1", Priority: core.PriorityHigh, Blocked: true, DueAt: &soon},
			"infra-1 · P2 · due · !"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CardMeta(tc.task, now, "")
			if got != tc.want {
				t.Fatalf("CardMeta = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "P"+priorityDigit(tc.task.Priority)+"*") {
				t.Fatalf("the priority is still glued to a marker: %q", got)
			}
		})
	}
}

// TestSelectionSurvivesWithoutColour is the constraint the card design was
// chosen for. A colourless run writes no escape anywhere, so the only thing
// left to say which card is selected is the shape of its bar.
func TestSelectionSurvivesWithoutColour(t *testing.T) {
	m := boardModel(t)
	if m.theme.Color {
		t.Fatal("the board under test is drawing in colour, so this proves nothing")
	}
	frame := m.Frame()
	if strings.Contains(frame, "\x1b") {
		t.Fatalf("a colourless frame emitted an escape sequence:\n%q", frame)
	}
	selected, _ := TaskAt(m.columns, m.sel)
	block := columnBlockLines(t, m, m.sel.Col)
	var bars []string
	for _, l := range block {
		if i := strings.Index(l, CardBarSelected); i >= 0 {
			bars = append(bars, l)
		}
	}
	if len(bars) != CardOf(selected, 20, m.now(), "").Lines() && len(bars) == 0 {
		t.Fatalf("no card in the selected column carries the selected bar:\n%s", strings.Join(block, "\n"))
	}
	if !strings.Contains(strings.Join(bars, "\n"), selected.Ref) {
		t.Fatalf("the bar is not on the selected card %q:\n%s", selected.Ref, strings.Join(block, "\n"))
	}
	// The other half: an unselected card carries the plain bar, and the two
	// glyphs are the same width so nothing shifts between them.
	other := columnBlockLines(t, m, m.sel.Col+1)
	if strings.Contains(strings.Join(other, "\n"), CardBarSelected) {
		t.Fatalf("an unfocused column drew a selected bar:\n%s", strings.Join(other, "\n"))
	}
	if len([]rune(CardBarSelected)) != len([]rune(CardBarPlain)) {
		t.Fatal("the two bars are different widths and would shift the text beside them")
	}
}

// TestAnEmptyColumnIsNarrowerThanABusyOne pins the weighting. An equal split
// gave the column whose whole content is the word "empty" as much room as the
// column holding the work, so the busy ones wrapped every title.
// realBoardModel is a board whose titles are the length real titles are, which
// is what the weighting is for: a board of six character titles demands the
// same of every column and is weighted the same way an equal split would be.
func realBoardModel(t *testing.T) Model {
	t.Helper()
	m := boardModel(t)
	titles := []string{
		"Default-deny network policies in the staging namespace",
		"Run a disaster recovery drill in the secondary region",
		"Bring the platform onto one observability stack",
	}
	tasks := make([]core.Task, 0, len(m.tasks))
	for i, tk := range m.tasks {
		tk.Title = titles[i%len(titles)]
		tasks = append(tasks, tk)
	}
	m, _ = m.reduce(boardMsg{project: m.project, workflow: testWorkflow(), tasks: tasks})
	return m
}

func TestAnEmptyColumnIsNarrowerThanABusyOne(t *testing.T) {
	m := realBoardModel(t)
	layout := LayoutFor(m.width, m.height, len(m.columns))
	widths := m.columnWidths(layout, 0, len(m.columns))
	if len(widths) != len(m.columns) {
		t.Fatalf("got %d widths for %d columns", len(widths), len(m.columns))
	}
	busy, empty := -1, -1
	for i, col := range m.columns {
		if len(col.Tasks) > 0 && busy < 0 {
			busy = i
		}
		if len(col.Tasks) == 0 && empty < 0 {
			empty = i
		}
	}
	if busy < 0 || empty < 0 {
		t.Fatalf("the board under test has no busy and empty pair: %v", widths)
	}
	if widths[empty] >= widths[busy] {
		t.Fatalf("the empty column %q took %d cells and the busy column %q %d",
			m.columns[empty].Key, widths[empty], m.columns[busy].Key, widths[busy])
	}
	if widths[busy] < MinColumnWidth {
		t.Fatalf("a column holding work was given %d cells, below the floor of %d", widths[busy], MinColumnWidth)
	}
	if widths[empty] < MinEmptyColumnWidth {
		t.Fatalf("an empty column was given %d cells, below its floor of %d", widths[empty], MinEmptyColumnWidth)
	}
	total := len(widths) - 1
	for _, w := range widths {
		total += w
	}
	if total != m.width {
		t.Fatalf("the columns take %d cells of a %d cell terminal", total, m.width)
	}
}

// TestAnEmptyColumnIsDrawnShort is the other saving: an empty column used to be
// drawn to the full height of the terminal, so a board with two of them was
// mostly blank frame.
func TestAnEmptyColumnIsDrawnShort(t *testing.T) {
	m := boardModel(t)
	empty, busy := -1, -1
	for i, col := range m.columns {
		if len(col.Tasks) == 0 && empty < 0 {
			empty = i
		}
		if len(col.Tasks) > 0 && busy < 0 {
			busy = i
		}
	}
	if empty < 0 || busy < 0 {
		t.Fatal("the board under test has no busy and empty pair")
	}
	short, tall := len(columnBlockLines(t, m, empty)), len(columnBlockLines(t, m, busy))
	if short >= tall {
		t.Fatalf("the empty column is %d lines tall and the busy one %d", short, tall)
	}
	if body := LayoutFor(m.width, m.height, len(m.columns)).BodyHeight; short >= body {
		t.Fatalf("the empty column still fills the body: %d lines of %d", short, body)
	}
}

// TestEveryMarkerACardDrawsIsInTheLegend closes the gap between the legend and
// the code that draws it.
//
// TestDocsMarkerTableMatchesTheLegend holds docs/tui.md against CardLegend, in
// both directions, and never consults CardFlags. So CardLegend is a claim about
// the code that nothing checked: renaming the dependency marker to "*" left the
// code drawing a marker no legend lists and not drawing one every legend does,
// and the whole suite stayed green. The legend is what `?` shows a reader, so a
// legend that disagrees with the board is an interface lying at the one moment
// somebody asked it for help.
func TestEveryMarkerACardDrawsIsInTheLegend(t *testing.T) {
	src, err := os.ReadFile("view.go")
	if err != nil {
		t.Fatalf("reading view.go: %v", err)
	}
	body := string(src)
	start := strings.Index(body, "func CardFlags(")
	if start < 0 {
		t.Fatal("view.go no longer defines CardFlags")
	}
	end := strings.Index(body[start:], "\n}")
	if end < 0 {
		t.Fatal("CardFlags is never closed")
	}
	drawn := regexp.MustCompile(`b\.WriteString\(([^)]+)\)`).
		FindAllStringSubmatch(body[start:start+end], -1)
	if len(drawn) == 0 {
		t.Fatal("found no markers in CardFlags; this guard is reading the wrong thing")
	}

	legend := make(map[string]bool, len(CardLegend))
	for _, e := range CardLegend {
		m, _, _ := strings.Cut(e, " ")
		legend[m] = true
	}
	for _, m := range drawn {
		lit := strings.TrimSpace(m[1])
		// A constant rather than a literal: resolve the ones this file owns.
		if lit == "DeletedMarker" {
			lit = `"` + DeletedMarker + `"`
		}
		marker, err := strconv.Unquote(lit)
		if err != nil {
			t.Fatalf("CardFlags writes %s, which this guard cannot resolve; teach it or use a literal", lit)
		}
		if !legend[marker] {
			t.Errorf("a card can draw %q and CardLegend does not list it, so `?` will not explain it", marker)
		}
	}
}
