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
		{"board scrolls columns", 60, 40, 4, LayoutBoard, 3},
		{"two columns still a board", 40, 24, 4, LayoutBoard, 2},
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
