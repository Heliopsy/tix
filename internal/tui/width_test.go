// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/heliopsy/tix/internal/core"
)

// A rune is not a cell. Everything in this file exists because the measuring
// helpers counted runes, which is the same number as cells only for the ASCII
// every other test in this package is written in. The inputs here are
// deliberately East Asian, emoji and combining marks, because those are the
// only inputs that can tell a correct measurement from a wrong one.

// TestTruncateCutsToCellsNotRunes is the primitive the board, the project list
// and the statistics view all narrow their text with.
func TestTruncateCutsToCellsNotRunes(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
	}{
		{"east asian", "日本語のタイトル", 6},
		{"emoji", "🤖🤖🤖🤖", 4},
		{"emoji at its exact fit", "🤖🤖", 4},
		{"mixed", "infra-1 データベース移行", 14},
		{"combining marks are free", "ééé", 3},
		{"one cell", "日本語", 1},
		{"two cells cannot hold a wide char and an ellipsis", "日本語", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Truncate(tc.in, tc.width)
			if n := lipgloss.Width(got); n > tc.width {
				t.Fatalf("Truncate(%q, %d) = %q which draws %d cells, %d too many",
					tc.in, tc.width, got, n, n-tc.width)
			}
		})
	}
}

// TestTruncateLeavesAFittingStringWhole is the other half: a string that fits
// in cells must not be cut just because it has more runes than cells.
func TestTruncateLeavesAFittingStringWhole(t *testing.T) {
	const s = "ééé" // six runes, three cells
	if got := Truncate(s, 3); got != s {
		t.Fatalf("Truncate(%q, 3) = %q, want it whole: it draws %d cells",
			s, got, lipgloss.Width(s))
	}
}

// TestWrapBreaksAtCellsNotRunes guards the wrapping helper. A line wider than
// its column is re-wrapped by lipgloss inside the column's border, which
// detaches a card's bar from its own title and makes the column taller than
// the frame budgeted for it.
func TestWrapBreaksAtCellsNotRunes(t *testing.T) {
	for _, width := range []int{3, 6, 10, 12} {
		for _, in := range []string{
			"データベース を 移行する",
			"データベースの移行を完了させる作業日本語",
			"ab日",
		} {
			for _, line := range Wrap(in, width) {
				if n := lipgloss.Width(line); n > width {
					t.Fatalf("Wrap(%q, %d) produced %q at %d cells", in, width, line, n)
				}
			}
		}
	}
}

// TestWrapKeepsACombiningMarkOnItsLetter is the zero-width case: a decomposed
// string that fits must not be split, least of all between a letter and the
// accent that belongs to it, which draws as a floating mark on the next line.
func TestWrapKeepsACombiningMarkOnItsLetter(t *testing.T) {
	const s = "ééé"
	if got := Wrap(s, 3); len(got) != 1 || got[0] != s {
		t.Fatalf("Wrap(%q, 3) = %q, want one line: it draws %d cells",
			s, got, lipgloss.Width(s))
	}
}

// TestWrapTitleFitsItsColumn covers the card title, which is where a wrapped
// over-wide line is drawn beside a bar glyph.
func TestWrapTitleFitsItsColumn(t *testing.T) {
	for _, width := range []int{6, 12, 20} {
		for _, line := range WrapTitle("データベースの移行を完了させる作業", width, CardTitleLines) {
			if n := lipgloss.Width(line); n > width {
				t.Fatalf("WrapTitle(..., %d) produced %q at %d cells", width, line, n)
			}
		}
	}
}

// TestColumnDemandMeasuresCells guards the width a column asks the board for.
// Measured in runes, a column carrying East Asian work asked for half the room
// it needed, so every title in it wrapped and was then cut.
func TestColumnDemandMeasuresCells(t *testing.T) {
	head := "進行中 (3)"
	want := lipgloss.Width(head) + CardBarWidth + BorderWidth + ColumnGutter
	if got := ColumnDemand(head, nil); got != want {
		t.Fatalf("ColumnDemand(%q) = %d, want %d: the heading draws %d cells, not %d runes",
			head, got, want, lipgloss.Width(head), len([]rune(head)))
	}
	tasks := []core.Task{{Title: "データベース移行"}}
	if got := ColumnDemand("x", tasks); got != lipgloss.Width("データベース移行")+CardBarWidth+BorderWidth+ColumnGutter {
		t.Fatalf("ColumnDemand with a wide title = %d", got)
	}
}

// TestColumnFloorMeasuresCells covers the other half of the same split. The
// heading has to draw more than MinEmptyColumnWidth cells for the difference
// to be observable at all: a shorter one is swallowed by that floor, which is
// why the obvious short case is not the one asserted here.
func TestColumnFloorMeasuresCells(t *testing.T) {
	head := "レビュー待ちの作業 (0)" // 22 cells, 13 runes
	if lipgloss.Width(head) <= MinEmptyColumnWidth {
		t.Fatalf("the fixture is too narrow to tell runes from cells: %q", head)
	}
	want := max(MinEmptyColumnWidth, min(MinColumnWidth, lipgloss.Width(head)+BorderWidth+ColumnGutter))
	if got := ColumnFloor(head, nil); got != want {
		t.Fatalf("ColumnFloor(%q) = %d, want %d: the heading draws %d cells, not %d runes",
			head, got, want, lipgloss.Width(head), len([]rune(head)))
	}
}

// wideBoard is a board whose titles are East Asian, at one terminal size.
func wideBoard(t *testing.T, n, w, h int) Model {
	t.Helper()
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"}, Now: func() time.Time {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}})
	m.width, m.height = w, h
	tasks := make([]core.Task, 0, n)
	for i := range n {
		tk := task(string(rune('a'+i)), "todo", int64(i+1), core.PriorityNormal)
		tk.Title = "データベースの移行を完了させる作業日本語"
		tasks = append(tasks, tk)
	}
	m, _ = m.reduce(boardMsg{project: core.Project{ID: "p1", Key: "infra"}, workflow: testWorkflow(), tasks: tasks})
	return m
}

// TestTheBoardNeverGrowsPastItsTerminal is the defect the measuring bug
// actually produced: a card title cut to a rune count wider than the column
// was re-wrapped by lipgloss inside the border, so the column drew more lines
// than the layout budgeted and the frame ran off the bottom of the screen.
func TestTheBoardNeverGrowsPastItsTerminal(t *testing.T) {
	for _, n := range []int{1, 2, 3, 5, 8, 12} {
		for _, h := range []int{10, 12, 14, 16, 20, 24} {
			for _, w := range []int{42, 50, 60, 80, 100, 120} {
				frame := wideBoard(t, n, w, h).Frame()
				if got := len(strings.Split(frame, "\n")); got > h {
					t.Fatalf("%d cards at %dx%d: the frame is %d lines, %d past the screen:\n%s",
						n, w, h, got, got-h, frame)
				}
			}
		}
	}
}

// TestTheProjectListNeverRunsPastItsTerminal covers the one list whose only
// width guard is Truncate: nothing below it applies a cap, so a row measured
// in runes wraps in the terminal and pushes the footer off the screen.
func TestTheProjectListNeverRunsPastItsTerminal(t *testing.T) {
	projects := []core.Project{
		{ID: "p1", Key: "agents", Name: "Agent Platform", Icon: "🤖"},
		{ID: "p2", Key: "データ", Name: "データ基盤の移行プロジェクト", Icon: "📚"},
	}
	for _, w := range []int{24, 30, 40, 60, 80} {
		m := New(Config{Access: fullAccess(), Environ: []string{"NO_COLOR=1"}})
		m.width, m.height = w, 20
		m, _ = m.reduce(projectsMsg{projects: projects})
		for i, line := range strings.Split(m.Frame(), "\n") {
			if n := lineWidth(line); n > w {
				t.Fatalf("project list at width %d: line %d draws %d cells: %q",
					w, i, n, stripANSI(line))
			}
		}
	}
}

// TestTheStatisticsColumnsLineUp guards the two padded columns in the
// statistics view. They were padded with %-24s and %-*s, which count BYTES:
// one non-ASCII handle moved the count column on that row alone.
func TestTheStatisticsColumnsLineUp(t *testing.T) {
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 100, 40
	m.view = viewStats
	m, _ = m.reduce(statsMsg{stats: &core.Stats{
		LeaderboardMeasure: "moves",
		TopActors: []core.StatsActor{
			{ActorID: "a1", Handle: "alice", Moved: 7},
			{ActorID: "a2", Handle: "さくら", Moved: 7},
		},
		Oldest: []core.StatsAgeing{
			{Ref: "infra-1", Title: "migrate", Age: core.Duration(0)},
			{Ref: "infra-2", Title: "データベースの移行", Age: core.Duration(0)},
		},
	}})
	var counts, ages []int
	for _, line := range strings.Split(m.Frame(), "\n") {
		s := stripANSI(line)
		switch {
		case strings.Contains(s, "alice"), strings.Contains(s, "さくら"):
			counts = append(counts, lipgloss.Width(s[:strings.LastIndex(s, "7")]))
		case strings.Contains(s, "infra-"):
			ages = append(ages, lipgloss.Width(s[:strings.LastIndex(s, "0s")]))
		}
	}
	if len(counts) != 2 || counts[0] != counts[1] {
		t.Fatalf("leaderboard counts start at cells %v; the column is padded in bytes", counts)
	}
	if len(ages) != 2 || ages[0] != ages[1] {
		t.Fatalf("ageing ages start at cells %v; the column is padded in bytes", ages)
	}
}

// TestTheActivityDetailIsBudgetedInCells guards the one place that subtracts a
// measured head from the terminal width. The frame is not the assertion: a
// too-long line is clipped afterwards by fit, so the only thing a frame-wide
// check would see is that the cut lost its ellipsis. The line itself has to
// already fit before anything clips it.
func TestTheActivityDetailIsBudgetedInCells(t *testing.T) {
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 60, 20
	m.view = viewActivity
	e := core.Event{
		ID: "e1", Type: "task.updated", OccurredAt: time.Unix(0, 0).UTC(),
		Payload: map[string]any{"ref": "インフラ-1", "title": "データベースの移行を完了させる作業日本語"},
	}
	for _, selected := range []bool{false, true} {
		line := stripANSI(m.eventLine(e, selected))
		if n := lipgloss.Width(line); n > m.width {
			t.Fatalf("eventLine(selected=%v) draws %d cells for a %d cell terminal before anything clips it: %q",
				selected, n, m.width, line)
		}
	}
}
