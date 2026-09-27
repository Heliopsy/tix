// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
)

func TestLayoutFor(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		columns       int
		wantMode      LayoutMode
		wantVisible   int
	}{
		{"full board", 120, 40, 4, LayoutBoard, 4},
		{"board scrolls columns", 65, 40, 4, LayoutBoard, 3},
		{"two columns still a board", 44, 24, 4, LayoutBoard, 2},
		{"narrow falls back to one column", 30, 24, 4, LayoutSingleColumn, 1},
		{"too narrow explains itself", 20, 24, 4, LayoutTooSmall, 0},
		{"too short explains itself", 120, 5, 4, LayoutTooSmall, 0},
		{"no columns still lays out", 120, 40, 0, LayoutBoard, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LayoutFor(tc.width, tc.height, tc.columns)
			if got.Mode != tc.wantMode {
				t.Fatalf("mode = %v, want %v", got.Mode, tc.wantMode)
			}
			if got.VisibleColumns != tc.wantVisible {
				t.Fatalf("visible = %d, want %d", got.VisibleColumns, tc.wantVisible)
			}
			if got.Mode != LayoutTooSmall && got.ColumnWidth < 1 {
				t.Fatalf("column width %d is unusable", got.ColumnWidth)
			}
		})
	}
}

func TestLayoutColumnsNeverOverflowTheTerminal(t *testing.T) {
	for width := MinWidth; width <= 200; width++ {
		l := LayoutFor(width, 40, 6)
		if l.Mode == LayoutTooSmall {
			continue
		}
		total := l.ColumnWidth*l.VisibleColumns + (l.VisibleColumns - 1)
		if total > width {
			t.Fatalf("width %d: columns take %d cells", width, total)
		}
	}
}

func TestVisibleRangeKeepsSelectionInside(t *testing.T) {
	tests := []struct {
		name                     string
		total, visible, selected int
		wantStart, wantEnd       int
	}{
		{"everything fits", 3, 5, 2, 0, 3},
		{"first column", 6, 3, 0, 0, 3},
		{"middle column", 6, 3, 3, 2, 5},
		{"last column", 6, 3, 5, 3, 6},
		{"nothing to show", 0, 3, 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := VisibleRange(tc.total, tc.visible, tc.selected)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("VisibleRange = %d,%d want %d,%d", start, end, tc.wantStart, tc.wantEnd)
			}
			if tc.total > 0 && (tc.selected < start || tc.selected >= end) {
				t.Fatalf("selected column %d is outside %d..%d", tc.selected, start, end)
			}
		})
	}
}

func TestScrollOffset(t *testing.T) {
	tests := []struct {
		name                     string
		offset, selected, height int
		want                     int
	}{
		{"already visible", 0, 2, 5, 0},
		{"scrolls down", 0, 7, 5, 3},
		{"scrolls up", 4, 1, 5, 1},
		{"no room", 3, 9, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScrollOffset(tc.offset, tc.selected, tc.height); got != tc.want {
				t.Fatalf("ScrollOffset = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWindowLines(t *testing.T) {
	lines := []string{"a", "b", "c", "d"}
	if got := WindowLines(lines, 1, 2); len(got) != 2 || got[0] != "b" {
		t.Fatalf("WindowLines = %v", got)
	}
	if got := WindowLines(lines, 10, 2); len(got) != 1 || got[0] != "d" {
		t.Fatalf("offset past the end = %v", got)
	}
	if got := WindowLines(nil, 0, 3); got != nil {
		t.Fatalf("empty input = %v", got)
	}
	if got := WindowLines(lines, 0, 0); got != nil {
		t.Fatalf("zero height = %v", got)
	}
}

func TestTruncateKeepsTitlesReadable(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"fits", "rotate keys", 20, "rotate keys"},
		{"cut", "rotate the keys now", 10, "rotate th…"},
		{"single cell", "rotate", 1, "r"},
		{"no room", "rotate", 0, ""},
		{"multibyte", "ротация ключей", 5, "рота…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Truncate(tc.in, tc.width)
			if got != tc.want {
				t.Fatalf("Truncate = %q, want %q", got, tc.want)
			}
			if len([]rune(got)) > tc.width {
				t.Fatalf("Truncate produced %d runes for width %d", len([]rune(got)), tc.width)
			}
		})
	}
}

func TestPadFillsToWidth(t *testing.T) {
	if got := pad("ab", 5); got != "ab   " || len(got) != 5 {
		t.Fatalf("pad = %q", got)
	}
	if got := pad("abcdef", 3); got != "abcdef" {
		t.Fatalf("pad shortened its input: %q", got)
	}
	if strings.TrimSpace(pad("", 4)) != "" {
		t.Fatal("pad of an empty string is not blank")
	}
}

// TestLayoutForAtItsBoundaries pins the exact geometry either side of every
// threshold LayoutFor names, rather than only well inside each band. The
// smallest usable terminal, the width one cell below it, and the width at
// which a second column first fits are all exact-fit cases that a rendering
// assertion passes through without noticing.
func TestLayoutForAtItsBoundaries(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		columns       int
		wantMode      LayoutMode
		wantVisible   int
		wantColumn    int
		wantBody      int
	}{
		{"the smallest usable terminal", MinWidth, MinHeight, 4, LayoutSingleColumn, 1, MinWidth, MinHeight - ChromeHeight},
		{"one cell narrower is too small", MinWidth - 1, MinHeight, 4, LayoutTooSmall, 0, 0, MinHeight - ChromeHeight},
		{"one line shorter is too small", MinWidth, MinHeight - 1, 4, LayoutTooSmall, 0, 0, MinHeight - 1 - ChromeHeight},
		{"the widest single column", 40, 24, 4, LayoutSingleColumn, 1, 40, 19},
		{"one cell wider fits two columns", 41, 24, 4, LayoutBoard, 2, 20, 19},
		{"the widest two column board", 61, 24, 4, LayoutBoard, 2, 30, 19},
		{"one cell wider fits three", 62, 24, 4, LayoutBoard, 3, 20, 19},
		{"a board never shows more columns than exist", 120, 40, 2, LayoutBoard, 2, 59, 35},
		{"as many columns as the board has", 41, 24, 2, LayoutBoard, 2, 20, 19},
		{"no columns still lays one out", 120, 40, 0, LayoutBoard, 1, 120, 35},
		{"chrome leaves one line at its own height", 120, ChromeHeight, 4, LayoutTooSmall, 0, 0, 1},
		{"one line above the chrome leaves one line", 120, ChromeHeight + 1, 4, LayoutTooSmall, 0, 0, 1},
		{"two lines above the chrome leaves two", 120, ChromeHeight + 2, 4, LayoutTooSmall, 0, 0, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LayoutFor(tc.width, tc.height, tc.columns)
			if got.Mode != tc.wantMode {
				t.Errorf("mode = %v, want %v", got.Mode, tc.wantMode)
			}
			if got.VisibleColumns != tc.wantVisible {
				t.Errorf("visible columns = %d, want %d", got.VisibleColumns, tc.wantVisible)
			}
			if got.ColumnWidth != tc.wantColumn {
				t.Errorf("column width = %d, want %d", got.ColumnWidth, tc.wantColumn)
			}
			if got.BodyHeight != tc.wantBody {
				t.Errorf("body height = %d, want %d", got.BodyHeight, tc.wantBody)
			}
			if got.Width != tc.width || got.Height != tc.height {
				t.Errorf("layout reported %dx%d for a %dx%d terminal", got.Width, got.Height, tc.width, tc.height)
			}
		})
	}
}

// TestLayoutInnerWidthAndCardRowsAtTheirFloors asserts the exact room a column
// reports, including the point at which the border and the heading have eaten
// all of it and the floor of one takes over.
func TestLayoutInnerWidthAndCardRowsAtTheirFloors(t *testing.T) {
	tests := []struct {
		name                     string
		columnWidth, body        int
		wantInnerWidth, wantRows int
	}{
		{"a roomy column", 20, 12, 18, 9},
		{"the border takes two cells", 3, 12, 1, 9},
		{"a column narrower than its border floors at one", 2, 12, 1, 9},
		{"a column of no width floors at one", 0, 12, 1, 9},
		{"the last row a heading leaves", 20, 4, 18, 1},
		{"a body with no room floors at one row", 20, 3, 18, 1},
		{"an empty body floors at one row", 20, 0, 18, 1},
		{"one row above the floor", 20, 5, 18, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := Layout{ColumnWidth: tc.columnWidth, BodyHeight: tc.body}
			if got := l.InnerWidth(); got != tc.wantInnerWidth {
				t.Errorf("InnerWidth = %d, want %d", got, tc.wantInnerWidth)
			}
			if got := l.CardRows(); got != tc.wantRows {
				t.Errorf("CardRows = %d, want %d", got, tc.wantRows)
			}
		})
	}
}

// TestVisibleRangeAtItsBoundaries covers the exact-fit window, the window one
// column short of fitting, and both ends of the track.
func TestVisibleRangeAtItsBoundaries(t *testing.T) {
	tests := []struct {
		name                     string
		total, visible, selected int
		wantStart, wantEnd       int
	}{
		{"the window is exactly the board", 4, 4, 3, 0, 4},
		{"one column too many", 5, 4, 4, 1, 5},
		{"one column too many, first selected", 5, 4, 0, 0, 4},
		{"a window of one follows the selection", 6, 1, 4, 4, 5},
		{"the last column pins the window to the end", 6, 3, 5, 3, 6},
		{"the first column pins it to the start", 6, 3, 0, 0, 3},
		{"the window is wider than the board", 3, 9, 1, 0, 3},
		{"a single column board", 1, 3, 0, 0, 1},
		{"no columns", 0, 3, 0, 0, 0},
		{"no room", 6, 0, 3, 0, 0},
		{"negative room", 6, -1, 3, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := VisibleRange(tc.total, tc.visible, tc.selected)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("VisibleRange = %d,%d want %d,%d", start, end, tc.wantStart, tc.wantEnd)
			}
			if tc.total > 0 && tc.visible > 0 {
				if end-start != min(tc.visible, tc.total) {
					t.Fatalf("window %d..%d is %d wide, want %d", start, end, end-start, min(tc.visible, tc.total))
				}
				if tc.selected < start || tc.selected >= end {
					t.Fatalf("selected column %d is outside %d..%d", tc.selected, start, end)
				}
			}
		})
	}
}

// TestWindowLinesAtItsBoundaries asserts the exact slice returned when the
// window ends precisely on the last line, one line past it, and when the
// offset sits on the last line or beyond it.
func TestWindowLinesAtItsBoundaries(t *testing.T) {
	lines := []string{"a", "b", "c", "d"}
	tests := []struct {
		name           string
		in             []string
		offset, height int
		want           []string
	}{
		{"the window is exactly the list", lines, 0, 4, []string{"a", "b", "c", "d"}},
		{"the window ends on the last line", lines, 2, 2, []string{"c", "d"}},
		{"the window runs one line past the end", lines, 2, 3, []string{"c", "d"}},
		{"the offset sits on the last line", lines, 3, 2, []string{"d"}},
		{"the offset sits one line past the end", lines, 4, 2, []string{"d"}},
		{"the offset is far past the end", lines, 40, 2, []string{"d"}},
		{"a negative offset starts at the top", lines, -3, 2, []string{"a", "b"}},
		{"a window of one line", lines, 1, 1, []string{"b"}},
		{"no height", lines, 1, 0, nil},
		{"negative height", lines, 1, -1, nil},
		{"no lines", nil, 0, 3, nil},
		{"a single line list", []string{"a"}, 0, 3, []string{"a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WindowLines(tc.in, tc.offset, tc.height)
			if len(got) != len(tc.want) {
				t.Fatalf("WindowLines = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("WindowLines = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestScrollOffsetAtItsBoundaries covers the row that is the last visible one
// and the row immediately below it, which is where an off-by-one either scrolls
// a line early or leaves the selection one line off the bottom.
func TestScrollOffsetAtItsBoundaries(t *testing.T) {
	tests := []struct {
		name                     string
		offset, selected, height int
		want                     int
	}{
		{"the last visible row does not scroll", 0, 4, 5, 0},
		{"the row below it scrolls by one", 0, 5, 5, 1},
		{"two rows below it scrolls by two", 0, 6, 5, 2},
		{"the first visible row does not scroll", 3, 3, 5, 3},
		{"the row above it scrolls up by one", 3, 2, 5, 2},
		{"a window of one row follows every step", 4, 5, 1, 5},
		{"no room", 3, 9, 0, 0},
		{"negative room", 3, 9, -1, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScrollOffset(tc.offset, tc.selected, tc.height)
			if got != tc.want {
				t.Fatalf("ScrollOffset = %d, want %d", got, tc.want)
			}
			if tc.height > 0 && (tc.selected < got || tc.selected >= got+tc.height) {
				t.Fatalf("row %d is outside the window %d..%d", tc.selected, got, got+tc.height)
			}
		})
	}
}

// TestVisibleRowsGivesUpARowOnlyWhenItMustAtTheBoundary asserts the exact-fit
// case: a list of exactly the body's height keeps every row, and one row more
// gives a row up to the hint that says so.
func TestVisibleRowsGivesUpARowOnlyWhenItMustAtTheBoundary(t *testing.T) {
	tests := []struct {
		name          string
		height, total int
		want          int
	}{
		{"the list is exactly the body", 5, 5, 5},
		{"one row too many gives a row to the hint", 5, 6, 4},
		{"one row short keeps the whole body", 5, 4, 5},
		{"an empty list keeps the whole body", 5, 0, 5},
		{"a body of one row that fits", 1, 1, 1},
		{"a body of one row that does not fit floors at one", 1, 2, 1},
		{"a body of two rows that does not fit", 2, 3, 1},
		{"no body floors at one", 0, 0, 1},
		{"no body with a list floors at one", 0, 4, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := VisibleRows(tc.height, tc.total); got != tc.want {
				t.Fatalf("VisibleRows(%d, %d) = %d, want %d", tc.height, tc.total, got, tc.want)
			}
		})
	}
}

// TestTruncateAtItsExactFit asserts the string of exactly the available width
// is returned whole. One cell narrower is the first that loses a character to
// the ellipsis, and the ellipsis takes a cell of its own.
func TestTruncateAtItsExactFit(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"exactly the width", "rotate", 6, "rotate"},
		{"one cell too narrow", "rotate", 5, "rota…"},
		{"one cell to spare", "rotate", 7, "rotate"},
		{"two cells", "rotate", 2, "r…"},
		{"a single cell has no room for an ellipsis", "rotate", 1, "r"},
		{"a single cell string in a single cell", "r", 1, "r"},
		{"no room", "rotate", 0, ""},
		{"negative room", "rotate", -1, ""},
		{"an empty string", "", 5, ""},
		{"multibyte counts runes, not bytes", "ротация", 7, "ротация"},
		{"multibyte one rune too narrow", "ротация", 6, "ротац…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Truncate(tc.in, tc.width)
			if got != tc.want {
				t.Fatalf("Truncate(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
			if tc.width > 0 && len([]rune(got)) > tc.width {
				t.Fatalf("Truncate produced %d runes for width %d", len([]rune(got)), tc.width)
			}
		})
	}
}

// TestScrollWindowAtItsBoundaries asserts the exact-fit list never scrolls and
// the list one row too long scrolls by exactly one.
func TestScrollWindowAtItsBoundaries(t *testing.T) {
	tests := []struct {
		name                            string
		offset, selected, height, total int
		want                            int
	}{
		{"the list is exactly the window", 0, 4, 5, 5, 0},
		{"an exact fit pulls a stale offset back", 3, 4, 5, 5, 0},
		{"one row too many, last selected", 0, 5, 5, 6, 1},
		{"one row too many, first selected", 1, 0, 5, 6, 0},
		{"the window never runs past the end", 0, 5, 5, 6, 1},
		{"a window of one row", 0, 3, 1, 4, 3},
		{"no rows", 2, 0, 5, 0, 0},
		{"no height", 2, 0, 0, 5, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScrollWindow(tc.offset, tc.selected, tc.height, tc.total)
			if got != tc.want {
				t.Fatalf("ScrollWindow = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestMoreBelowAtTheLastRow asserts a window ending exactly on the last row
// reports nothing below it, which is what keeps a full list from claiming it
// is cut off.
func TestMoreBelowAtTheLastRow(t *testing.T) {
	tests := []struct {
		name                  string
		offset, height, total int
		want                  bool
	}{
		{"the window ends on the last row", 0, 5, 5, false},
		{"one row hidden", 0, 5, 6, true},
		{"a scrolled window ending on the last row", 2, 3, 5, false},
		{"a scrolled window with one row left", 2, 3, 6, true},
		{"the window is longer than the list", 0, 9, 5, false},
		{"no height", 0, 0, 5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MoreBelow(tc.offset, tc.height, tc.total); got != tc.want {
				t.Fatalf("MoreBelow(%d, %d, %d) = %v, want %v", tc.offset, tc.height, tc.total, got, tc.want)
			}
		})
	}
}

// TestScrollHintCountsExactly asserts the numbers in the hint, not merely the
// arrows: a hint that says the wrong count is worse than no hint.
func TestScrollHintCountsExactly(t *testing.T) {
	tests := []struct {
		name                  string
		offset, height, total int
		want                  string
	}{
		{"nothing hidden at an exact fit", 0, 5, 5, ""},
		{"nothing hidden with room to spare", 0, 9, 5, ""},
		{"one row below", 0, 5, 6, "↓ 1 more"},
		{"five rows below", 0, 5, 10, "↓ 5 more"},
		{"one row above", 1, 5, 6, "↑ 1 more"},
		{"both ways", 2, 3, 10, "↑ 2 more   ↓ 5 more"},
		{"one each way", 1, 3, 5, "↑ 1 more   ↓ 1 more"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScrollHint(tc.offset, tc.height, tc.total); got != tc.want {
				t.Fatalf("ScrollHint(%d, %d, %d) = %q, want %q", tc.offset, tc.height, tc.total, got, tc.want)
			}
		})
	}
}
